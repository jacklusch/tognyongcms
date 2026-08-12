package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const ctxKeyUser = "dlz.auth.user"

// TokenFromRequest 从 Cookie 或 Authorization: Bearer 头取 token。
func TokenFromRequest(c *gin.Context) string {
	if v, err := c.Cookie(SessionCookieName); err == nil && v != "" {
		return v
	}
	h := c.GetHeader("Authorization")
	if len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	return ""
}

func RequireAuth(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := TokenFromRequest(c)
		us, err := svc.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录或会话已过期"})
			return
		}
		c.Set(ctxKeyUser, us)
		c.Next()
	}
}

func RequirePerm(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		us, ok := UserFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
			return
		}
		if !us.HasPerm(perm) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权限执行此操作"})
			return
		}
		c.Next()
	}
}

// RequirePermType 按内容类型校验：perm 形如 "content.read"，实际检查 perm+"."+typeName。
func RequirePermType(perm, typeName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		us, ok := UserFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "未登录"})
			return
		}
		if !us.HasPerm(perm + "." + typeName) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "message": "无权限执行此操作"})
			return
		}
		c.Next()
	}
}

func ClaimUser(c *gin.Context, us *UserSession) { c.Set(ctxKeyUser, us) }

func UserFromContext(c *gin.Context) (*UserSession, bool) {
	v, ok := c.Get(ctxKeyUser)
	if !ok {
		return nil, false
	}
	us, ok := v.(*UserSession)
	return us, ok
}
