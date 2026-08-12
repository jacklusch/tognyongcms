package adminapi

import (
	"dulizhan/internal/auth"
	"dulizhan/internal/config"
	"dulizhan/internal/content"
	"dulizhan/internal/media"
	"dulizhan/internal/store"
)

type Deps struct {
	Store   store.Store
	Auth    *auth.Service
	Content *content.Service
	Media   media.MediaStore
	Cfg     *config.Config
}
