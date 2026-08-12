package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"dulizhan/internal/errs"
	"dulizhan/internal/store"
)

const SessionCookieName = "dlz_session"

// AllPerms 内置全部权限点，admin 角色拥有。
var AllPerms = []string{
	"content.read", "content.write", "content.publish", "content.delete",
	"content_types.manage", "media.upload", "media.delete",
	"menus.manage", "settings.manage", "users.manage", "roles.manage",
}

type UserSession struct {
	User  store.User
	Role  store.Role
	Perms map[string]bool
}

// HasPerm 支持类型级前缀匹配：content.read.article 命中精确、content.read.* / content.* / *
func (us *UserSession) HasPerm(p string) bool {
	if us.Perms["*"] {
		return true
	}
	if us.Perms[p] {
		return true
	}
	parts := strings.Split(p, ".")
	for i := len(parts) - 1; i >= 1; i-- {
		if us.Perms[strings.Join(parts[:i], ".")+".*"] {
			return true
		}
	}
	return false
}

type Service struct {
	store store.Store
	ttl   time.Duration
}

func New(st store.Store, sessionTTL time.Duration) *Service {
	if sessionTTL <= 0 {
		sessionTTL = 7 * 24 * time.Hour
	}
	return &Service{store: st, ttl: sessionTTL}
}

func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	u, err := s.store.UserRepo().GetByUsername(ctx, username)
	if err != nil {
		return "", errs.ErrForbidden
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", errs.ErrForbidden
	}
	token, err := newToken()
	if err != nil {
		return "", err
	}
	sess := &store.Session{Token: token, UserID: u.ID, ExpiresAt: time.Now().Add(s.ttl).UTC()}
	if err := s.store.SessionRepo().Create(ctx, sess); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.SessionRepo().Delete(ctx, token)
}

func (s *Service) Authenticate(ctx context.Context, token string) (*UserSession, error) {
	if token == "" {
		return nil, errs.ErrForbidden
	}
	sess, err := s.store.SessionRepo().Get(ctx, token)
	if err != nil {
		return nil, errs.ErrForbidden
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = s.store.SessionRepo().Delete(ctx, token)
		return nil, errs.ErrForbidden
	}
	u, err := s.store.UserRepo().GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, errs.ErrForbidden
	}
	role, err := s.store.RoleRepo().GetByID(ctx, u.RoleID)
	if err != nil {
		return nil, errs.ErrForbidden
	}
	return &UserSession{User: u, Role: role, Perms: parsePerms(role.Permissions)}, nil
}

func (s *Service) CreateUser(ctx context.Context, username, password string, roleID int64) (store.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return store.User{}, fmt.Errorf("%w: 用户名不能为空", errs.ErrValidation)
	}
	if len(password) < 6 {
		return store.User{}, fmt.Errorf("%w: 密码至少 6 位", errs.ErrValidation)
	}
	if _, err := s.store.RoleRepo().GetByID(ctx, roleID); err != nil {
		return store.User{}, fmt.Errorf("%w: 角色不存在", errs.ErrValidation)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return store.User{}, err
	}
	u := &store.User{Username: username, PasswordHash: string(hash), RoleID: roleID}
	if err := s.store.UserRepo().Create(ctx, u); err != nil {
		return store.User{}, err
	}
	return *u, nil
}

func (s *Service) ListUsers(ctx context.Context) ([]store.User, error) {
	return s.store.UserRepo().List(ctx)
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	if err := s.store.SessionRepo().DeleteByUser(ctx, id); err != nil {
		return err
	}
	return s.store.UserRepo().Delete(ctx, id)
}

func (s *Service) SetUserRole(ctx context.Context, id, roleID int64) error {
	u, err := s.store.UserRepo().GetByID(ctx, id)
	if err != nil {
		return err
	}
	if _, err := s.store.RoleRepo().GetByID(ctx, roleID); err != nil {
		return fmt.Errorf("%w: 角色不存在", errs.ErrValidation)
	}
	u.RoleID = roleID
	return s.store.UserRepo().Update(ctx, &u)
}

// ChangePassword 改密码：oldPassword 非空时校验旧密码（本人场景），为空跳过（admin 重置场景）。
func (s *Service) ChangePassword(ctx context.Context, id int64, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return fmt.Errorf("%w: 密码至少 6 位", errs.ErrValidation)
	}
	u, err := s.store.UserRepo().GetByID(ctx, id)
	if err != nil {
		return err
	}
	if oldPassword != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)); err != nil {
			return fmt.Errorf("%w: 旧密码不正确", errs.ErrValidation)
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return s.store.UserRepo().Update(ctx, &u)
}

func parsePerms(raw string) map[string]bool {
	m := map[string]bool{}
	var list []string
	_ = jsonUnmarshal([]byte(raw), &list)
	for _, p := range list {
		m[p] = true
	}
	return m
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
