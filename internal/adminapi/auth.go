package adminapi

import (
	"sort"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (d *Deps) HandleLogin(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	token, err := d.Auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil || token == "" { // final-fix：Authenticate 失败 nil 防护
		unauthorized(c)
		return
	}
	c.SetCookie(auth.SessionCookieName, token, 7*24*3600, "/", "", false, true)
	respondOK(c, gin.H{"token": token})
}

func (d *Deps) HandleLogout(c *gin.Context) {
	token := auth.TokenFromRequest(c)
	if err := d.Auth.Logout(c.Request.Context(), token); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"logged_out": true})
}

func (d *Deps) HandleMe(c *gin.Context) {
	us, ok := auth.UserFromContext(c)
	if !ok || us == nil { // final-fix：nil 防护
		unauthorized(c)
		return
	}
	perms := make([]string, 0, len(us.Perms))
	for p := range us.Perms {
		perms = append(perms, p)
	}
	sort.Strings(perms)
	respondOK(c, gin.H{
		"user":        gin.H{"id": us.User.ID, "username": us.User.Username, "role": us.Role.Name},
		"permissions": perms,
	})
}
