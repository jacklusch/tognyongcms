package store

import "time"

type ContentType struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Label  string `json:"label"`
	Fields string `json:"fields"`
	Config string `json:"config"`
}

type Content struct {
	ID            int64      `json:"id"`
	ContentTypeID int64      `json:"content_type_id"`
	ContentID     string     `json:"content_id"`
	Lang          string     `json:"lang"`
	Slug          string     `json:"slug"`
	Title         string     `json:"title"`
	Status        string     `json:"status"`
	CreatedBy     int64      `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	Payload       string     `json:"payload"`
}

type Setting struct {
	Key   string
	Value string
}

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	RoleID       int64  `json:"role_id"`
}

type Role struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
}

type Session struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Menu struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Lang  string `json:"lang"`
	Items string `json:"items"`
}

// MenuItem 存储层导航项（管理端编辑器保存的结构）。
type MenuItem struct {
	Label    string     `json:"label"`
	Type     string     `json:"type"` // "home" | "type:<name>" | "custom"
	URL      string     `json:"url"`  // custom 时填；home/type 时可空
	Children []MenuItem `json:"children,omitempty"`
}

type Media struct {
	ID        int64     `json:"id"`
	Filename  string    `json:"filename"`
	URL       string    `json:"url"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

type Category struct {
	ID          int64     `json:"id"`
	ParentID    int64     `json:"parent_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
