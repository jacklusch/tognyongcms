// internal/server/server.go
package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/adminapi"
	"dulizhan/internal/auth"
	"dulizhan/internal/config"
	"dulizhan/internal/content"
	"dulizhan/internal/i18n"
	"dulizhan/internal/media"
	"dulizhan/internal/seo"
	"dulizhan/internal/store"
	"dulizhan/internal/theme"
)

type Server struct {
	engine  *gin.Engine
	cfg     *config.Config
	store   store.Store
	content *content.Service
	i18n    *i18n.Registry
	seo     *seo.Builder
	themes  *theme.Loader
	auth    *auth.Service
	media   media.MediaStore
	log     *slog.Logger
}

func New(cfg *config.Config, st store.Store, svc *content.Service, reg *i18n.Registry, loader *theme.Loader, authSvc *auth.Service, medStore media.MediaStore) (*Server, error) {
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	gin.SetMode(gin.ReleaseMode)
	level := slog.LevelInfo
	if cfg.Server.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	s := &Server{
		engine: nil, cfg: cfg, store: st, content: svc, i18n: reg,
		seo: seo.NewBuilder(seo.Options{
			SiteURL:      cfg.Site.URL,
			Name:         cfg.Site.Name,
			Description:  cfg.Site.Description,
			Names:        cfg.Site.Names,
			Descriptions: cfg.Site.Descriptions,
			HomeTitles:   cfg.Site.HomeTitles,
			DefaultLang:  cfg.Site.DefaultLang,
			OGImage:      cfg.Site.OGImage,
		}, reg),
		themes: loader,
		auth:   authSvc,
		media:  medStore,
		log:    logger,
	}
	eng := gin.New()
	eng.Use(s.accessLog(), s.recovery())
	s.engine = eng
	s.registerRoutes()
	return s, nil
}

// accessLog 访问日志中间件。
func (s *Server) accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		s.log.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
			"ip", c.ClientIP(),
		)
	}
}

// recovery 恢复中间件（记录 panic 到 slog）。
func (s *Server) recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("panic", "err", r, "path", c.Request.URL.Path)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func (s *Server) Run() error {
	return s.engine.Run(s.cfg.Server.Addr)
}

func (s *Server) Engine() http.Handler { return s.engine }

func (s *Server) registerRoutes() {
	eng := s.engine

	eng.GET("/robots.txt", s.handleRobots)
	eng.GET("/sitemap.xml", s.handleSitemap)
	eng.GET("/themes/:name/*path", s.handleThemeStatic)

	// 媒体静态服务
	eng.GET("/media/*key", func(c *gin.Context) {
		key := c.Param("key")
		if strings.Contains(key, "..") {
			c.Status(http.StatusNotFound)
			return
		}
		full := filepath.Join(s.cfg.Server.DataDir, "media", filepath.FromSlash(strings.TrimPrefix(key, "/")))
		if fi, err := os.Stat(full); err != nil || fi.IsDir() {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", "public, max-age=3600")
		c.File(full)
	})

	// 管理 API
	api := eng.Group("/api")
	adminapi.Register(api, adminapi.Deps{
		Store: s.store, Auth: s.auth, Content: s.content, Media: s.media, Cfg: s.cfg,
	})

	// 管理端 SPA（/admin 与 /admin/*，须在 NoRoute 之前注册）
	eng.GET("/admin", s.handleAdmin)
	eng.GET("/admin/*path", s.handleAdmin)

	// 前台渲染走 catch-all
	eng.NoRoute(s.handleFrontend)
}

// adminIndex 从嵌入 FS 读 index.html。
func adminIndex() ([]byte, error) {
	b, err := adminFS.ReadFile("dist/index.html")
	if err != nil {
		return nil, err
	}
	return b, nil
}

// handleAdmin 服务 SPA：/admin 与 /admin/* 返回 index.html（前端路由 fallback），/admin/assets/* 服务静态。
func (s *Server) handleAdmin(c *gin.Context) {
	sub, err := fs.Sub(adminFS, "dist")
	if err != nil {
		c.String(http.StatusInternalServerError, "admin 资源不可用")
		return
	}
	if c.Request.URL.Path == "/admin" || c.Request.URL.Path == "/admin/" {
		c.Status(http.StatusOK)
		if b, e := adminIndex(); e == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", b)
			return
		}
	}
	// /admin/assets/* 或前端路由
	rest := ""
	if len(c.Request.URL.Path) > len("/admin/") {
		rest = c.Request.URL.Path[len("/admin/"):]
	}
	if rest != "" && rest != "index.html" {
		if b, err := fs.ReadFile(sub, rest); err == nil {
			c.Data(http.StatusOK, mimeTypeByExt(rest), b)
			return
		}
	}
	// 前端路由 fallback → index.html
	if b, e := adminIndex(); e == nil {
		c.Data(http.StatusOK, "text/html; charset=utf-8", b)
		return
	}
	c.Status(http.StatusNotFound)
}

func mimeTypeByExt(path string) string {
	switch {
	case strings.HasSuffix(path, ".js"):
		return "application/javascript"
	case strings.HasSuffix(path, ".css"):
		return "text/css"
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	default:
		return "text/plain"
	}
}

func (s *Server) handleThemeStatic(c *gin.Context) {
	name := c.Param("name")
	p := c.Param("path")
	root := filepath.Join(s.cfg.Site.ThemesDir, name, "static")
	if strings.HasSuffix(p, "/") || strings.Contains(p, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	full := filepath.Join(root, filepath.FromSlash(p))
	if fi, err := os.Stat(full); err != nil || fi.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(full)
}
