package content

import (
	"context"
	"encoding/json"
	"fmt"

	"dulizhan/internal/errs"
	"dulizhan/internal/schema"
	"dulizhan/internal/store"
)

type Entry struct {
	Content  store.Content  `json:"content"`
	TypeName string         `json:"type_name"`
	Fields   map[string]any `json:"fields"`
}

type Service struct {
	store         store.Store
	reg           *schema.Registry
	reservedNames map[string]bool
}

func New(st store.Store, reg *schema.Registry, reservedNames []string) *Service {
	rn := make(map[string]bool, len(reservedNames))
	for _, n := range reservedNames {
		rn[n] = true
	}
	return &Service{store: st, reg: reg, reservedNames: rn}
}

// --- 内容类型 ---

func (s *Service) checkReservedName(name string) error {
	if s.reservedNames[name] {
		return fmt.Errorf("%w: 内容类型名 %q 与语言代码冲突", errs.ErrValidation, name)
	}
	return nil
}

func (s *Service) AllTypes(ctx context.Context) ([]schema.ContentType, error) {
	rows, err := s.store.ContentTypeRepo().List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]schema.ContentType, 0, len(rows))
	for _, r := range rows {
		ct, err := s.toSchemaType(r)
		if err != nil {
			return nil, err
		}
		out = append(out, ct)
	}
	return out, nil
}

func (s *Service) GetType(ctx context.Context, name string) (schema.ContentType, error) {
	r, err := s.store.ContentTypeRepo().GetByName(ctx, name)
	if err != nil {
		return schema.ContentType{}, err
	}
	return s.toSchemaType(r)
}

func (s *Service) CreateType(ctx context.Context, ct *schema.ContentType) error {
	if err := s.reg.ValidateContentType(ct); err != nil {
		return fmt.Errorf("%w: %v", errs.ErrValidation, err)
	}
	if err := s.checkReservedName(ct.Name); err != nil {
		return err
	}
	fields, err := json.Marshal(ct.Fields)
	if err != nil {
		return err
	}
	cfg, _ := json.Marshal(ct.Config)
	row := &store.ContentType{Name: ct.Name, Label: ct.Label, Fields: string(fields), Config: string(cfg)}
	if err := s.store.ContentTypeRepo().Create(ctx, row); err != nil {
		return err
	}
	ct.ID = row.ID
	return nil
}

func (s *Service) UpdateType(ctx context.Context, ct *schema.ContentType) error {
	if err := s.reg.ValidateContentType(ct); err != nil {
		return fmt.Errorf("%w: %v", errs.ErrValidation, err)
	}
	if err := s.checkReservedName(ct.Name); err != nil {
		return err
	}
	fields, err := json.Marshal(ct.Fields)
	if err != nil {
		return err
	}
	cfg, _ := json.Marshal(ct.Config)
	return s.store.ContentTypeRepo().Update(ctx, &store.ContentType{
		ID: ct.ID, Name: ct.Name, Label: ct.Label, Fields: string(fields), Config: string(cfg),
	})
}

func (s *Service) toSchemaType(r store.ContentType) (schema.ContentType, error) {
	fields, err := schema.DecodeFields(r.Fields)
	if err != nil {
		return schema.ContentType{}, fmt.Errorf("内容类型 %q 字段定义损坏: %w", r.Name, err)
	}
	cfg := map[string]any{}
	_ = json.Unmarshal([]byte(r.Config), &cfg)
	return schema.ContentType{ID: r.ID, Name: r.Name, Label: r.Label, Fields: fields, Config: cfg}, nil
}

// --- 内容 ---

func (s *Service) Create(ctx context.Context, typeName, lang string, data map[string]any, createdBy int64) (Entry, error) {
	ct, err := s.GetType(ctx, typeName)
	if err != nil {
		return Entry{}, err
	}
	if err := s.reg.ValidateDocument(&ct, data); err != nil {
		return Entry{}, fmt.Errorf("%w: %v", errs.ErrValidation, err)
	}
	slug, _ := data["slug"].(string)
	if slug == "" {
		slug = schema.Slugify(fmt.Sprint(data["title"]))
	}
	title := ""
	if f := schema.IndexField(&ct); f != nil {
		title, _ = data[f.Name].(string)
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return Entry{}, err
	}
	row := &store.Content{
		ContentTypeID: ct.ID,
		ContentID:     newContentID(),
		Lang:          lang,
		Slug:          slug,
		Title:         title,
		Status:        "draft",
		CreatedBy:     createdBy,
		Payload:       string(payload),
	}
	if err := s.store.ContentRepo().Create(ctx, row); err != nil {
		return Entry{}, err
	}
	return s.entryFromStore(ctx, ct, *row)
}

func (s *Service) Update(ctx context.Context, id int64, data map[string]any) (Entry, error) {
	row, err := s.store.ContentRepo().GetByID(ctx, id)
	if err != nil {
		return Entry{}, err
	}
	ct, err := s.GetTypeByID(ctx, row.ContentTypeID)
	if err != nil {
		return Entry{}, err
	}
	if err := s.reg.ValidateDocument(&ct, data); err != nil {
		return Entry{}, fmt.Errorf("%w: %v", errs.ErrValidation, err)
	}
	slug, _ := data["slug"].(string)
	if slug == "" {
		slug = schema.Slugify(fmt.Sprint(data["title"]))
	}
	title := ""
	if f := schema.IndexField(&ct); f != nil {
		title, _ = data[f.Name].(string)
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return Entry{}, err
	}
	row.Slug, row.Title, row.Payload = slug, title, string(payload)
	if err := s.store.ContentRepo().Update(ctx, &row); err != nil {
		return Entry{}, err
	}
	return s.entryFromStore(ctx, ct, row)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.store.ContentRepo().Delete(ctx, id)
}

func (s *Service) SetStatus(ctx context.Context, id int64, status string) error {
	if status != "draft" && status != "published" {
		return fmt.Errorf("%w: 状态必须是 draft 或 published", errs.ErrValidation)
	}
	row, err := s.store.ContentRepo().GetByID(ctx, id)
	if err != nil {
		return err
	}
	if status == "published" && row.Status != "published" {
		now := timeNow()
		row.PublishedAt = &now
	}
	row.Status = status
	return s.store.ContentRepo().Update(ctx, &row)
}

func (s *Service) GetPublishedBySlugLang(ctx context.Context, typeName, slug, lang string) (Entry, error) {
	ct, err := s.GetType(ctx, typeName)
	if err != nil {
		return Entry{}, err
	}
	row, err := s.store.ContentRepo().GetBySlugLangStatus(ctx, typeName, slug, lang, "published")
	if err != nil {
		return Entry{}, err
	}
	return s.entryFromStore(ctx, ct, row)
}

func (s *Service) ListPublished(ctx context.Context, typeName, lang string, page, perPage int) ([]Entry, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	ct, err := s.GetType(ctx, typeName)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.store.ContentRepo().ListByTypeLangStatus(ctx, typeName, lang, "published", (page-1)*perPage, perPage)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.ContentRepo().CountByTypeLangStatus(ctx, typeName, lang, "published")
	if err != nil {
		return nil, 0, err
	}
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		e, err := s.entryFromStore(ctx, ct, row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, nil
}

// ListPublishedByCategories 列出指定分类集合下已发布内容（分页，多分类 IN 聚合）。
func (s *Service) ListPublishedByCategories(ctx context.Context, typeName, lang string, ids []int64, page, perPage int) ([]Entry, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	ct, err := s.GetType(ctx, typeName)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.store.ContentRepo().ListByCategories(ctx, typeName, lang, ids, (page-1)*perPage, perPage)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.ContentRepo().CountByCategories(ctx, typeName, lang, ids)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		e, err := s.entryFromStore(ctx, ct, row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, nil
}

func (s *Service) GetTypeByID(ctx context.Context, id int64) (schema.ContentType, error) {
	rows, err := s.store.ContentTypeRepo().List(ctx)
	if err != nil {
		return schema.ContentType{}, err
	}
	for _, r := range rows {
		if r.ID == id {
			return s.toSchemaType(r)
		}
	}
	return schema.ContentType{}, errs.ErrNotFound
}

func (s *Service) entryFromStore(ctx context.Context, ct schema.ContentType, row store.Content) (Entry, error) {
	fields := map[string]any{}
	if err := json.Unmarshal([]byte(row.Payload), &fields); err != nil {
		return Entry{}, fmt.Errorf("内容 %d 载荷损坏: %w", row.ID, err)
	}
	return Entry{Content: row, TypeName: ct.Name, Fields: fields}, nil
}

// --- 阶段 2 扩展 ---

type Actor struct {
	UserID      int64
	IsModerator bool
}

// GetByID 返回单条内容（含类型名与字段）。
func (s *Service) GetByID(ctx context.Context, id int64) (Entry, error) {
	row, err := s.store.ContentRepo().GetByID(ctx, id)
	if err != nil {
		return Entry{}, err
	}
	ct, err := s.GetTypeByID(ctx, row.ContentTypeID)
	if err != nil {
		return Entry{}, err
	}
	return s.entryFromStore(ctx, ct, row)
}

// ListByContentID 返回某翻译组全部语言变体。
func (s *Service) ListByContentID(ctx context.Context, contentID string) ([]Entry, error) {
	rows, err := s.store.ContentRepo().ListByContentID(ctx, contentID)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		ct, err := s.GetTypeByID(ctx, row.ContentTypeID)
		if err != nil {
			return nil, err
		}
		e, err := s.entryFromStore(ctx, ct, row)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// ListAdmin 后台内容列表：status 为空表示不过滤。
func (s *Service) ListAdmin(ctx context.Context, typeName, lang, status string, page, perPage int) ([]Entry, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if _, err := s.GetType(ctx, typeName); err != nil {
		return nil, 0, err
	}
	rows, err := s.store.ContentRepo().ListByTypeLangStatus(ctx, typeName, lang, status, (page-1)*perPage, perPage)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.ContentRepo().CountByTypeLangStatus(ctx, typeName, lang, status)
	if err != nil {
		return nil, 0, err
	}
	ct, _ := s.GetType(ctx, typeName)
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		e, err := s.entryFromStore(ctx, ct, row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, nil
}

// CreateTranslation 给 contentID 翻译组新增一种语言变体。
// 作者只能给自己创建的内容加翻译，moderator 可翻译他人内容。
func (s *Service) CreateTranslation(ctx context.Context, typeName, lang, contentID string, data map[string]any, actor Actor) (Entry, error) {
	if _, err := s.GetType(ctx, typeName); err != nil {
		return Entry{}, err
	}
	parents, err := s.store.ContentRepo().ListByContentID(ctx, contentID)
	if err != nil {
		return Entry{}, err
	}
	if len(parents) == 0 {
		return Entry{}, errs.ErrNotFound
	}
	parent := parents[0]
	if !actor.IsModerator && parent.CreatedBy != actor.UserID {
		return Entry{}, errs.ErrForbidden
	}
	// 用父内容所在类型校验文档（保证必填/格式规则一致）
	ct, err := s.GetTypeByID(ctx, parent.ContentTypeID)
	if err != nil {
		return Entry{}, err
	}
	if err := s.reg.ValidateDocument(&ct, data); err != nil {
		return Entry{}, fmt.Errorf("%w: %v", errs.ErrValidation, err)
	}
	// 语言重复检查
	for _, p := range parents {
		if p.Lang == lang {
			return Entry{}, fmt.Errorf("%w: 该语言翻译已存在", errs.ErrValidation)
		}
	}
	slug, _ := data["slug"].(string)
	if slug == "" {
		slug = schema.Slugify(fmt.Sprint(data["title"]))
	}
	title := ""
	if f := schema.IndexField(&ct); f != nil {
		title, _ = data[f.Name].(string)
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return Entry{}, err
	}
	row := &store.Content{
		ContentTypeID: parent.ContentTypeID,
		ContentID:     contentID,
		Lang:          lang,
		Slug:          slug,
		Title:         title,
		Status:        "draft",
		CreatedBy:     actor.UserID,
		Payload:       string(payload),
	}
	if err := s.store.ContentRepo().Create(ctx, row); err != nil {
		return Entry{}, err
	}
	return s.entryFromStore(ctx, ct, *row)
}

// Search 按关键词搜索某类型某语言的内容（title/slug LIKE 匹配）。
func (s *Service) Search(ctx context.Context, typeName, lang, q string, page, perPage int) ([]Entry, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if _, err := s.GetType(ctx, typeName); err != nil {
		return nil, 0, err
	}
	rows, err := s.store.ContentRepo().SearchByTypeLang(ctx, typeName, lang, q, (page-1)*perPage, perPage)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.ContentRepo().CountSearch(ctx, typeName, lang, q)
	if err != nil {
		return nil, 0, err
	}
	ct, _ := s.GetType(ctx, typeName)
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		e, err := s.entryFromStore(ctx, ct, row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, nil
}

type TypeStat struct {
	TypeName  string `json:"type_name"`
	Published int    `json:"published"`
	Draft     int    `json:"draft"`
}

type Stats struct {
	ByType []TypeStat `json:"by_type"`
	Recent []Entry    `json:"recent"`
}

// Stats 返回仪表盘数据：各内容类型 published/draft 计数 + 最近内容。
func (s *Service) Stats(ctx context.Context) (Stats, error) {
	types, err := s.AllTypes(ctx)
	if err != nil {
		return Stats{}, err
	}
	st := Stats{ByType: make([]TypeStat, 0, len(types))}
	for _, ct := range types {
		pub, err := s.store.ContentRepo().CountByTypeStatus(ctx, ct.Name, "published")
		if err != nil {
			return Stats{}, err
		}
		draft, err := s.store.ContentRepo().CountByTypeStatus(ctx, ct.Name, "draft")
		if err != nil {
			return Stats{}, err
		}
		st.ByType = append(st.ByType, TypeStat{TypeName: ct.Name, Published: pub, Draft: draft})
		// 最近发布：该类型最近 5 条（跨语言，取最新）
		rows, err := s.store.ContentRepo().ListByTypeLangStatus(ctx, ct.Name, "", "published", 0, 5)
		if err != nil {
			return Stats{}, err
		}
		for _, row := range rows {
			e, err := s.entryFromStore(ctx, ct, row)
			if err != nil {
				return Stats{}, err
			}
			st.Recent = append(st.Recent, e)
		}
	}
	return st, nil
}

// DeleteType 级联删除内容类型及其全部内容（事务）。
func (s *Service) DeleteType(ctx context.Context, name string) error {
	ct, err := s.GetType(ctx, name)
	if err != nil {
		return err
	}
	return s.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.ContentRepo().DeleteByType(ctx, name); err != nil {
			return err
		}
		return tx.ContentTypeRepo().Delete(ctx, ct.ID)
	})
}
