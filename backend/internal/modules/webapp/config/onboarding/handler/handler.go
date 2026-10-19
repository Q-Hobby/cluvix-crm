package handler

import (
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/storage"

	"gorm.io/gorm"
)

// Deps + Handler — module-local (khuôn `config/user_profile/handler`). S3/Cache dựng sẵn cho
// endpoint `bootstrap` (upload logo + InvalidateUserScope).
type Deps struct {
	DB    *gorm.DB
	Store storage.Store
	Cache *cache.RedisCache
}

type Handler struct {
	Deps
}

func New(d Deps) *Handler { return &Handler{Deps: d} }
