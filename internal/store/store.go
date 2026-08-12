package store

import "context"

type Store interface {
	WithTx(ctx context.Context, fn func(tx Store) error) error
	ContentTypeRepo() ContentTypeRepo
	ContentRepo() ContentRepo
	SettingRepo() SettingRepo
	UserRepo() UserRepo
	RoleRepo() RoleRepo
	SessionRepo() SessionRepo
	MenuRepo() MenuRepo
	MediaRepo() MediaRepo
	CategoryRepo() CategoryRepo
}

type ContentTypeRepo interface {
	List(ctx context.Context) ([]ContentType, error)
	GetByName(ctx context.Context, name string) (ContentType, error)
	Create(ctx context.Context, ct *ContentType) error
	Update(ctx context.Context, ct *ContentType) error
	Delete(ctx context.Context, id int64) error
}

type ContentRepo interface {
	Create(ctx context.Context, c *Content) error
	Update(ctx context.Context, c *Content) error
	GetByID(ctx context.Context, id int64) (Content, error)
	GetBySlugLangStatus(ctx context.Context, typeName, slug, lang, status string) (Content, error)
	ListByContentID(ctx context.Context, contentID string) ([]Content, error)
	ListByTypeLangStatus(ctx context.Context, typeName, lang, status string, offset, limit int) ([]Content, error)
	CountByTypeLangStatus(ctx context.Context, typeName, lang, status string) (int, error)
	CountByTypeStatus(ctx context.Context, typeName, status string) (int, error)
	Delete(ctx context.Context, id int64) error
	DeleteByType(ctx context.Context, typeName string) error
	SearchByTypeLang(ctx context.Context, typeName, lang, q string, offset, limit int) ([]Content, error)
	CountSearch(ctx context.Context, typeName, lang, q string) (int, error)
	ListByCategories(ctx context.Context, typeName, lang string, ids []int64, offset, limit int) ([]Content, error)
	CountByCategories(ctx context.Context, typeName, lang string, ids []int64) (int, error)
}

type SettingRepo interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	All(ctx context.Context) (map[string]string, error)
}

type UserRepo interface {
	Create(ctx context.Context, u *User) error
	GetByUsername(ctx context.Context, username string) (User, error)
	GetByID(ctx context.Context, id int64) (User, error)
	List(ctx context.Context) ([]User, error)
	Update(ctx context.Context, u *User) error
	Delete(ctx context.Context, id int64) error
}

type RoleRepo interface {
	Create(ctx context.Context, name string) (Role, error)
	GetByID(ctx context.Context, id int64) (Role, error)
	GetByName(ctx context.Context, name string) (Role, error)
	List(ctx context.Context) ([]Role, error)
	Update(ctx context.Context, id int64, name, permissions string) error
	Delete(ctx context.Context, id int64) error
}

type SessionRepo interface {
	Create(ctx context.Context, s *Session) error
	Get(ctx context.Context, token string) (Session, error)
	Delete(ctx context.Context, token string) error
	DeleteByUser(ctx context.Context, userID int64) error
}

type MenuRepo interface {
	Create(ctx context.Context, m *Menu) error
	ListByLang(ctx context.Context, lang string) ([]Menu, error)
	GetByID(ctx context.Context, id int64) (Menu, error)
	Update(ctx context.Context, m *Menu) error
	Delete(ctx context.Context, id int64) error
}

type MediaRepo interface {
	Create(ctx context.Context, m *Media) error
	GetByID(ctx context.Context, id int64) (Media, error)
	List(ctx context.Context, offset, limit int) ([]Media, error)
	Delete(ctx context.Context, id int64) error
}

type CategoryRepo interface {
	List(ctx context.Context) ([]Category, error)
	GetByID(ctx context.Context, id int64) (Category, error)
	GetBySlug(ctx context.Context, slug string) (Category, error)
	Create(ctx context.Context, c *Category) error
	Update(ctx context.Context, c *Category) error
	Delete(ctx context.Context, id int64) error
	CountContent(ctx context.Context, id int64) (int, error)
	ListChildren(ctx context.Context, parentID int64) ([]Category, error)
	Descendants(ctx context.Context, id int64) ([]Category, error)
}
