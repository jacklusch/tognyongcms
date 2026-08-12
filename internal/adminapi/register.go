package adminapi

import (
	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
)

// Register 挂载全部 /api 路由，统一加 RequireAuth + 权限点。
func Register(g *gin.RouterGroup, d Deps) {
	g.POST("/auth/login", d.HandleLogin)
	g.POST("/auth/logout", auth.RequireAuth(d.Auth), d.HandleLogout)
	g.GET("/auth/me", auth.RequireAuth(d.Auth), d.HandleMe)

	g.GET("/meta", auth.RequireAuth(d.Auth), d.HandleMeta)

	g.GET("/content-types", auth.RequireAuth(d.Auth), auth.RequirePerm("content.read"), d.HandleContentTypes)
	g.POST("/content-types", auth.RequireAuth(d.Auth), auth.RequirePerm("content_types.manage"), d.HandleContentTypeCreate)
	g.PUT("/content-types/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("content_types.manage"), d.HandleContentTypeUpdate)
	g.DELETE("/content-types/:name", auth.RequireAuth(d.Auth), auth.RequirePerm("content_types.manage"), d.HandleContentTypeDelete)

	g.GET("/content", auth.RequireAuth(d.Auth), auth.RequirePerm("content.read"), d.HandleContentList)
	g.POST("/content", auth.RequireAuth(d.Auth), auth.RequirePerm("content.write"), d.HandleContentCreate)
	g.GET("/content/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("content.read"), d.HandleContentGet)
	g.PUT("/content/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("content.write"), d.HandleContentUpdate)
	g.DELETE("/content/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("content.delete"), d.HandleContentDelete)
	g.POST("/content/:id/publish", auth.RequireAuth(d.Auth), auth.RequirePerm("content.publish"), d.HandleContentPublish)
	g.POST("/content/:id/unpublish", auth.RequireAuth(d.Auth), auth.RequirePerm("content.publish"), d.HandleContentUnpublish)
	g.GET("/content/:id/translations", auth.RequireAuth(d.Auth), auth.RequirePerm("content.read"), d.HandleTranslations)
	g.POST("/content/:id/translate", auth.RequireAuth(d.Auth), auth.RequirePerm("content.write"), d.HandleCreateTranslation)

	g.GET("/search", auth.RequireAuth(d.Auth), auth.RequirePerm("content.read"), d.HandleSearch)

	g.GET("/stats", auth.RequireAuth(d.Auth), auth.RequirePerm("content.read"), d.HandleStats)

	g.POST("/media/upload", auth.RequireAuth(d.Auth), auth.RequirePerm("media.upload"), d.HandleMediaUpload)
	g.GET("/media", auth.RequireAuth(d.Auth), auth.RequirePerm("media.upload"), d.HandleMediaList)
	g.DELETE("/media/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("media.delete"), d.HandleMediaDelete)

	g.GET("/settings", auth.RequireAuth(d.Auth), auth.RequirePerm("settings.manage"), d.HandleSettings)
	g.PUT("/settings", auth.RequireAuth(d.Auth), auth.RequirePerm("settings.manage"), d.HandleSettingsUpdate)
	g.GET("/menus", auth.RequireAuth(d.Auth), auth.RequirePerm("menus.manage"), d.HandleMenus)
	g.POST("/menus", auth.RequireAuth(d.Auth), auth.RequirePerm("menus.manage"), d.HandleMenuCreate)
	g.PUT("/menus/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("menus.manage"), d.HandleMenuUpdate)
	g.DELETE("/menus/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("menus.manage"), d.HandleMenuDelete)
	g.GET("/users", auth.RequireAuth(d.Auth), auth.RequirePerm("users.manage"), d.HandleUsers)
	g.POST("/users", auth.RequireAuth(d.Auth), auth.RequirePerm("users.manage"), d.HandleUserCreate)
	g.DELETE("/users/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("users.manage"), d.HandleUserDelete)
	g.PUT("/users/:id/role", auth.RequireAuth(d.Auth), auth.RequirePerm("users.manage"), d.HandleUserRole)
	g.PUT("/users/:id/password", auth.RequireAuth(d.Auth), auth.RequirePerm("users.manage"), d.HandleUserPassword)
	g.GET("/roles", auth.RequireAuth(d.Auth), auth.RequirePerm("roles.manage"), d.HandleRoles)
	g.POST("/roles", auth.RequireAuth(d.Auth), auth.RequirePerm("roles.manage"), d.HandleRoleCreate)
	g.PUT("/roles/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("roles.manage"), d.HandleRoleUpdate)
	g.DELETE("/roles/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("roles.manage"), d.HandleRoleDelete)
	g.GET("/roles/perms", auth.RequireAuth(d.Auth), auth.RequirePerm("roles.manage"), d.HandleRolePerms)

	g.GET("/categories", auth.RequireAuth(d.Auth), auth.RequirePerm("content.read"), d.HandleCategories)
	g.POST("/categories", auth.RequireAuth(d.Auth), auth.RequirePerm("content_types.manage"), d.HandleCategoryCreate)
	g.PUT("/categories/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("content_types.manage"), d.HandleCategoryUpdate)
	g.DELETE("/categories/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("content_types.manage"), d.HandleCategoryDelete)
}
