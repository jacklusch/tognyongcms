package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"dulizhan/internal/errs"
	"dulizhan/internal/store"
)

// db 抽象 *sql.DB 与 *sql.Tx 共有的查询方法，使 Store 可复用于事务内（WithTx）。
type db interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Store struct {
	db db
}

func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写者，限制连接数
	db.SetConnMaxLifetime(0)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := migrateCategoriesParent(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := migrateCategoriesNameEn(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	// I3：断言安全，非硬转——在事务 Store 上调用应报错而非 panic。
	db, ok := s.db.(*sql.DB)
	if !ok {
		return fmt.Errorf("Close 需在顶层 Store 上调用")
	}
	return db.Close()
}

// WithTx 在事务中执行 fn，成功提交失败回滚。
func (s *Store) WithTx(ctx context.Context, fn func(tx store.Store) error) error {
	// I3：断言安全，非硬转——在事务 Store 上再开事务应报错而非 panic。
	db, ok := s.db.(*sql.DB)
	if !ok {
		return fmt.Errorf("WithTx 需在顶层 Store 上调用")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// I3：defer 兜底回滚——fn panic 或提交失败时自动回滚；Commit 成功后 defer 调 Rollback 返回 ErrTxDone，忽略无害。
	defer tx.Rollback()
	txStore := &Store{db: tx}
	if err := fn(txStore); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ContentTypeRepo() store.ContentTypeRepo { return &contentTypeRepo{db: s.db} }
func (s *Store) ContentRepo() store.ContentRepo         { return &contentRepo{db: s.db} }
func (s *Store) SettingRepo() store.SettingRepo         { return &settingRepo{db: s.db} }

const tsLayout = time.RFC3339

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

func wrapErr(err error) error {
	if isNoRows(err) {
		return errs.ErrNotFound
	}
	return err
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func wrapUnique(err error, msg string) error {
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: %s", errs.ErrValidation, msg)
	}
	return err
}

// --- content types ---

type contentTypeRepo struct{ db db }

func (r *contentTypeRepo) List(ctx context.Context) ([]store.ContentType, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, label, fields, config FROM content_types ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.ContentType
	for rows.Next() {
		var ct store.ContentType
		if err := rows.Scan(&ct.ID, &ct.Name, &ct.Label, &ct.Fields, &ct.Config); err != nil {
			return nil, err
		}
		out = append(out, ct)
	}
	return out, rows.Err()
}

func (r *contentTypeRepo) GetByName(ctx context.Context, name string) (store.ContentType, error) {
	var ct store.ContentType
	err := r.db.QueryRowContext(ctx,
		"SELECT id, name, label, fields, config FROM content_types WHERE name = ?", name,
	).Scan(&ct.ID, &ct.Name, &ct.Label, &ct.Fields, &ct.Config)
	return ct, wrapErr(err)
}

func (r *contentTypeRepo) Create(ctx context.Context, ct *store.ContentType) error {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO content_types (name, label, fields, config) VALUES (?, ?, ?, ?)",
		ct.Name, ct.Label, ct.Fields, ct.Config)
	if err != nil {
		return wrapUnique(err, "内容类型名已存在")
	}
	ct.ID, err = res.LastInsertId()
	return err
}

func (r *contentTypeRepo) Update(ctx context.Context, ct *store.ContentType) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE content_types SET label = ?, fields = ?, config = ? WHERE id = ?",
		ct.Label, ct.Fields, ct.Config, ct.ID)
	return err
}

func (r *contentTypeRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM content_types WHERE id = ?", id)
	return err
}

// --- content ---

type contentRepo struct{ db db }

const contentCols = "c.id, c.content_type_id, c.content_id, c.lang, c.slug, c.title, c.status, c.created_by, c.created_at, c.updated_at, c.published_at, c.payload"

func scanContent(row interface{ Scan(...any) error }) (store.Content, error) {
	var c store.Content
	var created, updated string
	var published *string
	err := row.Scan(&c.ID, &c.ContentTypeID, &c.ContentID, &c.Lang, &c.Slug, &c.Title,
		&c.Status, &c.CreatedBy, &created, &updated, &published, &c.Payload)
	if err != nil {
		return c, wrapErr(err)
	}
	if c.CreatedAt, err = time.Parse(tsLayout, created); err != nil {
		return c, fmt.Errorf("解析 created_at: %w", err)
	}
	if c.UpdatedAt, err = time.Parse(tsLayout, updated); err != nil {
		return c, fmt.Errorf("解析 updated_at: %w", err)
	}
	if published != nil {
		t, e := time.Parse(tsLayout, *published)
		if e != nil {
			return c, fmt.Errorf("解析 published_at: %w", e)
		}
		c.PublishedAt = &t
	}
	return c, nil
}

func (r *contentRepo) Create(ctx context.Context, c *store.Content) error {
	now := time.Now().UTC()
	c.CreatedAt, c.UpdatedAt = now, now
	var pub any
	if c.PublishedAt != nil {
		pub = c.PublishedAt.UTC().Format(tsLayout)
	}
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO content (content_type_id, content_id, lang, slug, title, status, created_by, created_at, updated_at, published_at, payload) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
		c.ContentTypeID, c.ContentID, c.Lang, c.Slug, c.Title, c.Status, c.CreatedBy,
		c.CreatedAt.Format(tsLayout), c.UpdatedAt.Format(tsLayout), pub, c.Payload)
	if err != nil {
		return wrapUnique(err, "slug 已存在")
	}
	c.ID, err = res.LastInsertId()
	return err
}

func (r *contentRepo) Update(ctx context.Context, c *store.Content) error {
	c.UpdatedAt = time.Now().UTC()
	var pub any
	if c.PublishedAt != nil {
		pub = c.PublishedAt.UTC().Format(tsLayout)
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE content SET content_type_id=?, content_id=?, lang=?, slug=?, title=?, status=?, created_by=?, updated_at=?, published_at=?, payload=? WHERE id=?",
		c.ContentTypeID, c.ContentID, c.Lang, c.Slug, c.Title, c.Status, c.CreatedBy,
		c.UpdatedAt.Format(tsLayout), pub, c.Payload, c.ID)
	if err != nil {
		return wrapUnique(err, "slug 已存在")
	}
	return err
}

func (r *contentRepo) GetByID(ctx context.Context, id int64) (store.Content, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+contentCols+" FROM content c WHERE c.id = ?", id)
	return scanContent(row)
}

func (r *contentRepo) GetBySlugLangStatus(ctx context.Context, typeName, slug, lang, status string) (store.Content, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+contentCols+" FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.slug=? AND c.lang=? AND c.status=?",
		typeName, slug, lang, status)
	return scanContent(row)
}

func (r *contentRepo) ListByContentID(ctx context.Context, contentID string) ([]store.Content, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+contentCols+" FROM content c WHERE c.content_id = ? ORDER BY c.id", contentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *contentRepo) ListByTypeLangStatus(ctx context.Context, typeName, lang, status string, offset, limit int) ([]store.Content, error) {
	query := "SELECT " + contentCols + " FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=?"
	args := []any{typeName}
	if lang != "" {
		query += " AND c.lang=?"
		args = append(args, lang)
	}
	if status != "" {
		query += " AND c.status=?"
		args = append(args, status)
	}
	query += " ORDER BY c.published_at DESC, c.id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *contentRepo) ListByTypeLangStatusCategory(ctx context.Context, typeName, lang, status string, categoryID int64, offset, limit int) ([]store.Content, error) {
	query := "SELECT " + contentCols + " FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=?"
	args := []any{typeName}
	if lang != "" {
		query += " AND c.lang=?"
		args = append(args, lang)
	}
	if status != "" {
		query += " AND c.status=?"
		args = append(args, status)
	}
	query += " AND c.payload LIKE ?"
	args = append(args, "%\"category\":\""+itoa64(categoryID)+"\"%")
	query += " ORDER BY c.published_at DESC, c.id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *contentRepo) CountByTypeLangStatusCategory(ctx context.Context, typeName, lang, status string, categoryID int64) (int, error) {
	query := "SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=?"
	args := []any{typeName}
	if lang != "" {
		query += " AND c.lang=?"
		args = append(args, lang)
	}
	if status != "" {
		query += " AND c.status=?"
		args = append(args, status)
	}
	query += " AND c.payload LIKE ?"
	args = append(args, "%\"category\":\""+itoa64(categoryID)+"\"%")
	var n int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

func (r *contentRepo) CountByTypeLangStatus(ctx context.Context, typeName, lang, status string) (int, error) {
	query := "SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=?"
	args := []any{typeName}
	if lang != "" {
		query += " AND c.lang=?"
		args = append(args, lang)
	}
	if status != "" {
		query += " AND c.status=?"
		args = append(args, status)
	}
	var n int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

func (r *contentRepo) CountByTypeStatus(ctx context.Context, typeName, status string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.status=?",
		typeName, status).Scan(&n)
	return n, err
}

func (r *contentRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM content WHERE id = ?", id)
	return err
}

func (r *contentRepo) DeleteByType(ctx context.Context, typeName string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM content WHERE content_type_id = (SELECT id FROM content_types WHERE name = ?)", typeName)
	return err
}

func (r *contentRepo) SearchByTypeLang(ctx context.Context, typeName, lang, q string, offset, limit int) ([]store.Content, error) {
	query := "SELECT " + contentCols + " FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=? AND (c.title LIKE ? OR c.slug LIKE ?) ORDER BY c.id DESC LIMIT ? OFFSET ?"
	like := "%" + q + "%"
	rows, err := r.db.QueryContext(ctx, query, typeName, lang, like, like, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *contentRepo) CountSearch(ctx context.Context, typeName, lang, q string) (int, error) {
	var n int
	like := "%" + q + "%"
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=? AND (c.title LIKE ? OR c.slug LIKE ?)",
		typeName, lang, like, like).Scan(&n)
	return n, err
}

func (r *contentRepo) SearchByTypeLangCategory(ctx context.Context, typeName, lang, q string, categoryID int64, offset, limit int) ([]store.Content, error) {
	query := "SELECT " + contentCols + " FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=? AND (c.title LIKE ? OR c.slug LIKE ?) AND c.payload LIKE ? ORDER BY c.id DESC LIMIT ? OFFSET ?"
	like := "%" + q + "%"
	rows, err := r.db.QueryContext(ctx, query, typeName, lang, like, like, "%\"category\":\""+itoa64(categoryID)+"\"%", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *contentRepo) CountSearchCategory(ctx context.Context, typeName, lang, q string, categoryID int64) (int, error) {
	var n int
	like := "%" + q + "%"
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=? AND (c.title LIKE ? OR c.slug LIKE ?) AND c.payload LIKE ?",
		typeName, lang, like, like, "%\"category\":\""+itoa64(categoryID)+"\"%").Scan(&n)
	return n, err
}

// categoryLikes 为每个分类 id 生成 payload LIKE 匹配模式（"%\"category\":\"<id>\"%"）。
func categoryLikes(ids []int64) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, "%\"category\":\""+itoa64(id)+"\"%")
	}
	return out
}

func (r *contentRepo) ListByCategories(ctx context.Context, typeName, lang string, ids []int64, offset, limit int) ([]store.Content, error) {
	if len(ids) == 0 {
		return []store.Content{}, nil
	}
	likes := categoryLikes(ids)
	cond := strings.TrimSuffix(strings.Repeat("c.payload LIKE ? OR ", len(likes)), " OR ")
	args := make([]any, 0, len(likes)+3)
	args = append(args, typeName, lang)
	for _, l := range likes {
		args = append(args, l)
	}
	args = append(args, limit, offset)
	query := "SELECT " + contentCols + " FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=? AND c.status='published' AND (" + cond + ") ORDER BY c.published_at DESC, c.id DESC LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.Content, 0)
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *contentRepo) CountByCategories(ctx context.Context, typeName, lang string, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	likes := categoryLikes(ids)
	cond := strings.TrimSuffix(strings.Repeat("c.payload LIKE ? OR ", len(likes)), " OR ")
	args := make([]any, 0, len(likes)+2)
	args = append(args, typeName, lang)
	for _, l := range likes {
		args = append(args, l)
	}
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=? AND c.status='published' AND ("+cond+")",
		args...).Scan(&n)
	return n, err
}

// --- settings ---

type settingRepo struct{ db db }

func (r *settingRepo) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := r.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	return v, wrapErr(err)
}

func (r *settingRepo) Set(ctx context.Context, key, value string) error {
	// 跨库兼容（SQLite/PG/MySQL）：存在则 UPDATE，否则 INSERT。
	// 注：SQLite 单连接单写者下无并发覆盖；PG/MySQL 未来可升级按驱动分支的 upsert 语法。
	var exists int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings WHERE key = ?", key).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		_, err := r.db.ExecContext(ctx, "UPDATE settings SET value = ? WHERE key = ?", value, key)
		return err
	}
	_, err := r.db.ExecContext(ctx, "INSERT INTO settings (key, value) VALUES (?, ?)", key, value)
	return err
}

func (r *settingRepo) All(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (s *Store) UserRepo() store.UserRepo       { return &userRepo{db: s.db} }
func (s *Store) RoleRepo() store.RoleRepo       { return &roleRepo{db: s.db} }
func (s *Store) SessionRepo() store.SessionRepo { return &sessionRepo{db: s.db} }
func (s *Store) MenuRepo() store.MenuRepo       { return &menuRepo{db: s.db} }
func (s *Store) MediaRepo() store.MediaRepo     { return &mediaRepo{db: s.db} }

// --- users ---

type userRepo struct{ db db }

func (r *userRepo) Create(ctx context.Context, u *store.User) error {
	res, err := r.db.ExecContext(ctx, "INSERT INTO users (username, password_hash, role_id) VALUES (?,?,?)", u.Username, u.PasswordHash, u.RoleID)
	if err != nil {
		return wrapUnique(err, "用户名已存在")
	}
	u.ID, err = res.LastInsertId()
	return err
}
func (r *userRepo) GetByUsername(ctx context.Context, username string) (store.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, "SELECT id, username, password_hash, role_id FROM users WHERE username = ?", username))
}
func (r *userRepo) GetByID(ctx context.Context, id int64) (store.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, "SELECT id, username, password_hash, role_id FROM users WHERE id = ?", id))
}
func (r *userRepo) List(ctx context.Context) ([]store.User, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, username, password_hash, role_id FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.User
	for rows.Next() {
		var u store.User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.RoleID); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (r *userRepo) Update(ctx context.Context, u *store.User) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET username=?, password_hash=?, role_id=? WHERE id=?", u.Username, u.PasswordHash, u.RoleID, u.ID)
	return err
}
func (r *userRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	return err
}
func scanUser(row interface{ Scan(...any) error }) (store.User, error) {
	var u store.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.RoleID)
	return u, wrapErr(err)
}

// --- roles ---

type roleRepo struct{ db db }

func (r *roleRepo) Create(ctx context.Context, name string) (store.Role, error) {
	res, err := r.db.ExecContext(ctx, "INSERT INTO roles (name, permissions) VALUES (?, '[]')", name)
	if err != nil {
		return store.Role{}, wrapUnique(err, "角色名已存在")
	}
	id, _ := res.LastInsertId()
	return store.Role{ID: id, Name: name, Permissions: "[]"}, nil
}
func (r *roleRepo) GetByID(ctx context.Context, id int64) (store.Role, error) {
	return scanRole(r.db.QueryRowContext(ctx, "SELECT id, name, permissions FROM roles WHERE id = ?", id))
}
func (r *roleRepo) GetByName(ctx context.Context, name string) (store.Role, error) {
	return scanRole(r.db.QueryRowContext(ctx, "SELECT id, name, permissions FROM roles WHERE name = ?", name))
}
func (r *roleRepo) List(ctx context.Context) ([]store.Role, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, permissions FROM roles ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Role
	for rows.Next() {
		var rl store.Role
		if err := rows.Scan(&rl.ID, &rl.Name, &rl.Permissions); err != nil {
			return nil, err
		}
		out = append(out, rl)
	}
	return out, rows.Err()
}
func (r *roleRepo) Update(ctx context.Context, id int64, name, permissions string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE roles SET name=?, permissions=? WHERE id=?", name, permissions, id)
	return wrapUnique(err, "角色名已存在")
}
func (r *roleRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM roles WHERE id = ?", id)
	return err
}
func scanRole(row interface{ Scan(...any) error }) (store.Role, error) {
	var rl store.Role
	err := row.Scan(&rl.ID, &rl.Name, &rl.Permissions)
	return rl, wrapErr(err)
}

// --- sessions ---

type sessionRepo struct{ db db }

func (r *sessionRepo) Create(ctx context.Context, s *store.Session) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO sessions (token, user_id, expires_at) VALUES (?,?,?)",
		s.Token, s.UserID, s.ExpiresAt.UTC().Format(tsLayout))
	return err
}
func (r *sessionRepo) Get(ctx context.Context, token string) (store.Session, error) {
	var s store.Session
	var exp string
	err := r.db.QueryRowContext(ctx, "SELECT token, user_id, expires_at FROM sessions WHERE token = ?", token).Scan(&s.Token, &s.UserID, &exp)
	if err != nil {
		return s, wrapErr(err)
	}
	if s.ExpiresAt, err = time.Parse(tsLayout, exp); err != nil {
		return s, fmt.Errorf("解析 expires_at: %w", err)
	}
	return s, nil
}
func (r *sessionRepo) Delete(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
	return err
}
func (r *sessionRepo) DeleteByUser(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// --- menus ---

type menuRepo struct{ db db }

func (r *menuRepo) Create(ctx context.Context, m *store.Menu) error {
	res, err := r.db.ExecContext(ctx, "INSERT INTO menus (name, lang, items) VALUES (?,?,?)", m.Name, m.Lang, m.Items)
	if err != nil {
		return err
	}
	m.ID, err = res.LastInsertId()
	return err
}
func (r *menuRepo) ListByLang(ctx context.Context, lang string) ([]store.Menu, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, lang, items FROM menus WHERE lang = ? ORDER BY id", lang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Menu
	for rows.Next() {
		var m store.Menu
		if err := rows.Scan(&m.ID, &m.Name, &m.Lang, &m.Items); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (r *menuRepo) GetByID(ctx context.Context, id int64) (store.Menu, error) {
	var m store.Menu
	err := r.db.QueryRowContext(ctx, "SELECT id, name, lang, items FROM menus WHERE id = ?", id).Scan(&m.ID, &m.Name, &m.Lang, &m.Items)
	return m, wrapErr(err)
}
func (r *menuRepo) Update(ctx context.Context, m *store.Menu) error {
	_, err := r.db.ExecContext(ctx, "UPDATE menus SET name=?, lang=?, items=? WHERE id=?", m.Name, m.Lang, m.Items, m.ID)
	return err
}
func (r *menuRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM menus WHERE id = ?", id)
	return err
}

// --- media ---

type mediaRepo struct{ db db }

func (r *mediaRepo) Create(ctx context.Context, m *store.Media) error {
	now := time.Now().UTC()
	m.CreatedAt = now
	res, err := r.db.ExecContext(ctx, "INSERT INTO media (filename, url, mime, size, created_at) VALUES (?,?,?,?,?)",
		m.Filename, m.URL, m.Mime, m.Size, now.Format(tsLayout))
	if err != nil {
		return err
	}
	m.ID, err = res.LastInsertId()
	return err
}
func (r *mediaRepo) GetByID(ctx context.Context, id int64) (store.Media, error) {
	var m store.Media
	var created string
	err := r.db.QueryRowContext(ctx, "SELECT id, filename, url, mime, size, created_at FROM media WHERE id = ?", id).
		Scan(&m.ID, &m.Filename, &m.URL, &m.Mime, &m.Size, &created)
	if err != nil {
		return m, wrapErr(err)
	}
	if m.CreatedAt, err = time.Parse(tsLayout, created); err != nil {
		return m, fmt.Errorf("解析 created_at: %w", err)
	}
	return m, nil
}
func (r *mediaRepo) List(ctx context.Context, offset, limit int) ([]store.Media, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, filename, url, mime, size, created_at FROM media ORDER BY id DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.Media, 0)
	for rows.Next() {
		var m store.Media
		var created string
		if err := rows.Scan(&m.ID, &m.Filename, &m.URL, &m.Mime, &m.Size, &created); err != nil {
			return nil, err
		}
		if m.CreatedAt, err = time.Parse(tsLayout, created); err != nil {
			return nil, fmt.Errorf("解析 created_at: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (r *mediaRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM media WHERE id = ?", id)
	return err
}

// --- categories ---

type categoryRepo struct{ db db }

func (s *Store) CategoryRepo() store.CategoryRepo { return &categoryRepo{db: s.db} }

func itoa64(id int64) string { return strconv.FormatInt(id, 10) }

func (r *categoryRepo) List(ctx context.Context) ([]store.Category, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, parent_id, name, name_en, slug, description, created_at FROM categories ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *categoryRepo) GetByID(ctx context.Context, id int64) (store.Category, error) {
	return scanCategory(r.db.QueryRowContext(ctx, "SELECT id, parent_id, name, name_en, slug, description, created_at FROM categories WHERE id = ?", id))
}

func (r *categoryRepo) GetBySlug(ctx context.Context, slug string) (store.Category, error) {
	return scanCategory(r.db.QueryRowContext(ctx, "SELECT id, parent_id, name, name_en, slug, description, created_at FROM categories WHERE slug = ?", slug))
}

func (r *categoryRepo) Create(ctx context.Context, c *store.Category) error {
	c.CreatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO categories (name, name_en, slug, description, created_at, parent_id) VALUES (?,?,?,?,?,?)",
		c.Name, c.NameEn, c.Slug, c.Description, c.CreatedAt.Format(tsLayout), c.ParentID)
	if err != nil {
		return wrapUnique(err, "分类 slug 已存在")
	}
	c.ID, err = res.LastInsertId()
	return err
}

func (r *categoryRepo) Update(ctx context.Context, c *store.Category) error {
	_, err := r.db.ExecContext(ctx, "UPDATE categories SET name=?, name_en=?, slug=?, description=?, parent_id=? WHERE id=?",
		c.Name, c.NameEn, c.Slug, c.Description, c.ParentID, c.ID)
	return wrapUnique(err, "分类 slug 已存在")
}

func (r *categoryRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM categories WHERE id = ?", id)
	return err
}

func (r *categoryRepo) CountContent(ctx context.Context, id int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM content WHERE payload LIKE ?", "%\"category\":\""+itoa64(id)+"\"%").Scan(&n)
	return n, err
}

func (r *categoryRepo) CountContentByLang(ctx context.Context, id int64, lang string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content WHERE payload LIKE ? AND lang=?`,
		"%\"category\":\""+itoa64(id)+"\"%", lang).Scan(&n)
	return n, err
}

func (r *categoryRepo) ListChildren(ctx context.Context, parentID int64) ([]store.Category, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, parent_id, name, name_en, slug, description, created_at FROM categories WHERE parent_id = ? ORDER BY id", parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Descendants 返回指定分类的全部子孙分类（任意层级，递归 CTE），按 id 排序。
func (r *categoryRepo) Descendants(ctx context.Context, id int64) ([]store.Category, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE descs(id) AS (
			SELECT id FROM categories WHERE parent_id = ?
			UNION ALL
			SELECT c.id FROM categories c JOIN descs d ON c.parent_id = d.id
		)
		SELECT c.id, c.parent_id, c.name, c.name_en, c.slug, c.description, c.created_at
		FROM categories c JOIN descs d ON c.id = d.id ORDER BY c.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanCategory(row interface{ Scan(...any) error }) (store.Category, error) {
	var c store.Category
	var created string
	err := row.Scan(&c.ID, &c.ParentID, &c.Name, &c.NameEn, &c.Slug, &c.Description, &created)
	if err != nil {
		return c, wrapErr(err)
	}
	c.CreatedAt, _ = time.Parse(tsLayout, created)
	return c, nil
}
