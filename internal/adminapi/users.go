package adminapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
	"dulizhan/internal/errs"
	"dulizhan/internal/store"
)

type userReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	RoleID   int64  `json:"role_id"`
}

func (d *Deps) HandleUsers(c *gin.Context) {
	users, err := d.Auth.ListUsers(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": users})
}

func (d *Deps) HandleUserCreate(c *gin.Context) {
	var req userReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	u, err := d.Auth.CreateUser(c.Request.Context(), req.Username, req.Password, req.RoleID)
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"user": u})
}

// currentUserID 返回当前登录用户 ID。
func (d *Deps) currentUserID(c *gin.Context) int64 {
	us, ok := auth.UserFromContext(c)
	if !ok || us == nil {
		return 0
	}
	return us.User.ID
}

func (d *Deps) HandleUserDelete(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	me := d.currentUserID(c)
	if id == me {
		fail(c, fmt.Errorf("%w: 不能删除自己", errs.ErrForbidden))
		return
	}
	u, err := d.Store.UserRepo().GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	if u.Username == "admin" {
		fail(c, fmt.Errorf("%w: 不能删除内置管理员", errs.ErrForbidden))
		return
	}
	// 最后一个 admin 保护（fail-closed：无法确认 admin 角色时拒绝删除）
	adminID, err := d.adminRoleID(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "无法确认管理员数量"})
		return
	}
	if isAdminUser(u, adminID) {
		users, err := d.Store.UserRepo().List(c.Request.Context())
		if err != nil {
			fail(c, err)
			return
		}
		adminCount := 0
		for _, x := range users {
			if isAdminUser(x, adminID) {
				adminCount++
			}
		}
		if adminCount <= 1 {
			fail(c, fmt.Errorf("%w: 不能删除最后一个管理员", errs.ErrForbidden))
			return
		}
	}
	if err := d.Auth.DeleteUser(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"deleted": id})
}

func (d *Deps) HandleUserRole(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if id == d.currentUserID(c) {
		fail(c, fmt.Errorf("%w: 不能修改自己的角色", errs.ErrForbidden))
		return
	}
	var req struct {
		RoleID int64 `json:"role_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	if err := d.Auth.SetUserRole(c.Request.Context(), id, req.RoleID); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"id": id})
}

func (d *Deps) HandleUserPassword(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	var req struct {
		Password    string  `json:"password"`
		OldPassword *string `json:"old_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	old := ""
	if id == d.currentUserID(c) {
		// 改自己密码必须提供旧密码，空值不得放行
		if req.OldPassword == nil || *req.OldPassword == "" {
			fail(c, fmt.Errorf("%w: 改自己密码需提供旧密码", errs.ErrValidation))
			return
		}
		old = *req.OldPassword
	}
	if err := d.Auth.ChangePassword(c.Request.Context(), id, old, req.Password); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"id": id})
}

// adminRoleID 返回 admin 角色 ID，查询失败返回错误（fail-closed 由调用方处理）。
func (d *Deps) adminRoleID(ctx context.Context) (int64, error) {
	r, err := d.Store.RoleRepo().GetByName(ctx, "admin")
	if err != nil {
		return 0, err
	}
	return r.ID, nil
}

// isAdminUser 判断用户是否为 admin 角色（adminID 为 0 时视为非 admin，防御查询失败）。
func isAdminUser(u store.User, adminID int64) bool {
	return adminID != 0 && u.RoleID == adminID
}
