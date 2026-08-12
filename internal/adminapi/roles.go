package adminapi

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
	"dulizhan/internal/errs"
)

type roleReq struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

func (d *Deps) HandleRoles(c *gin.Context) {
	roles, err := d.Store.RoleRepo().List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": roles})
}

// HandleRolePerms 返回全部可用权限点（供前端勾选面板）。
func (d *Deps) HandleRolePerms(c *gin.Context) {
	respondOK(c, gin.H{"perms": auth.AllPerms})
}

func (d *Deps) HandleRoleCreate(c *gin.Context) {
	var req roleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	if req.Name == "" {
		badRequest(c, "角色名不能为空")
		return
	}
	permJSON := rolePermsJSON(req.Permissions)
	r, err := d.Store.RoleRepo().Create(c.Request.Context(), req.Name)
	if err != nil {
		fail(c, err)
		return
	}
	if err := d.Store.RoleRepo().Update(c.Request.Context(), r.ID, req.Name, permJSON); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"role": r})
}

func (d *Deps) HandleRoleUpdate(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	var req roleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	if req.Name == "" {
		fail(c, fmt.Errorf("%w: 角色名不能为空", errs.ErrValidation))
		return
	}
	r, err := d.Store.RoleRepo().GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	// 内置角色不可改名（允许改权限点），否则最后 admin 保护可被改名绕过
	if isBuiltinRole(r.Name) && req.Name != r.Name {
		fail(c, fmt.Errorf("%w: 内置角色名称不可修改", errs.ErrForbidden))
		return
	}
	if err := d.Store.RoleRepo().Update(c.Request.Context(), id, req.Name, rolePermsJSON(req.Permissions)); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"id": id})
}

// isBuiltinRole 判断是否为内置三角色。
func isBuiltinRole(name string) bool {
	switch name {
	case "admin", "editor", "author":
		return true
	}
	return false
}

func (d *Deps) HandleRoleDelete(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	r, err := d.Store.RoleRepo().GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	// 内置角色不可删
	if isBuiltinRole(r.Name) {
		fail(c, fmt.Errorf("%w: 内置角色不可删除", errs.ErrForbidden))
		return
	}
	// 有用户引用拒删
	users, err := d.Store.UserRepo().List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	for _, u := range users {
		if u.RoleID == id {
			fail(c, fmt.Errorf("%w: 该角色仍有用户引用，请先转移用户", errs.ErrForbidden))
			return
		}
	}
	if err := d.Store.RoleRepo().Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"deleted": id})
}

// rolePermsJSON 把权限切片序列化为 JSON 字符串存入 roles.permissions。
func rolePermsJSON(perms []string) string {
	b, _ := json.Marshal(perms)
	return string(b)
}
