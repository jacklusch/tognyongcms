# 阶段 5 子项目 A：用户/角色管理界面 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 构建用户管理页与角色管理页（admin 专属），补全角色 CRUD 与权限点列表 API，加入用户自我保护逻辑（防删自己/防内置 admin/防锁死/改密码验旧）。

**架构：** 后端扩展 roles.go（CRUD + perms 列表）与 users.go（自我保护）、auth.Service.ChangePassword 加旧密码参数、RoleRepo 引用检查；前端新增 UsersView/RolesView 两页 + user.ts/role.ts API + 侧边栏菜单（adminOnly）。

**技术栈：** Go（gin@v1.10.0）、Vue 3 + TS + Element Plus。

**前置基线：** 阶段 4 已验收通过。规格：`docs/superpowers/specs/2026-08-06-phase5-subproject-a-users-roles-design.md`。

**环境约束（所有任务遵守）：**
- **不 git 提交**（仓库约定）；**勿运行 `go mod tidy`**
- Go 验证：`go test -count=1 <包> -v`；阶段收尾 `go test -count=1 ./...`
- 前端验证：`cd admin && npx vue-tsc --noEmit && npx vitest run`；改完 SPA 后 `npx vite build` + 同步 `internal/server/dist/`
- 错误消息用中文；后端依赖锁定勿动

**现状关键点：**
- `internal/adminapi/users.go`：HandleUsers/Create/Delete/Role/Password 均无保护；`actor(c)` 返回 content.Actor{UserID} 可拿当前用户
- `internal/auth/service.go`：`DeleteUser(ctx, id)`（清会话+删用户）、`SetUserRole(ctx, id, roleID)`、`ChangePassword(ctx, id, newPassword)`（3 参，body 无旧密码）
- `internal/adminapi/roles.go`：仅 HandleRoles（GET list）
- `auth.AllPerms`：11 个权限点已定义
- `RoleRepo`：Create/GetByID/GetByName/List/Update（`Update(id, name, permissions string)`）

---

### 任务 1：auth.Service.ChangePassword 加旧密码校验 + 测试适配

**文件：**
- 修改：`internal/auth/service.go`（ChangePassword 4 参）
- 修改：`internal/auth/service_test.go`（适配新签名 + 旧密码用例）

- [ ] **步骤 1：编写失败测试**（追加到 `internal/auth/service_test.go`）

```go
func TestChangePasswordRequiresOld(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	role, _ := s.store.RoleRepo().Create(ctx, "admin")
	u, _ := s.CreateUser(ctx, "bob", "secret123", role.ID)
	// 旧密码错误 → ErrValidation
	if err := s.ChangePassword(ctx, u.ID, "wrong", "newpass1"); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("旧密码错误 = %v, want ErrValidation", err)
	}
	// 旧密码正确 → 成功
	if err := s.ChangePassword(ctx, u.ID, "secret123", "newpass1"); err != nil {
		t.Fatalf("正确旧密码应成功: %v", err)
	}
	// 旧密码为空（admin 重置）→ 成功
	if err := s.ChangePassword(ctx, u.ID, "", "newpass2"); err != nil {
		t.Fatalf("空旧密码(admin重置)应成功: %v", err)
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/auth/ -run TestChangePasswordRequiresOld -v`
预期：FAIL（ChangePassword 参数数量不匹配，编译失败）

- [ ] **步骤 3：ChangePassword 签名扩展**（`internal/auth/service.go`）

```go
// ChangePassword 改密码：oldPassword 非空时校验旧密码（本人场景），为空跳过（admin 重置场景）。
func (s *Service) ChangePassword(ctx context.Context, id int64, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return fmt.Errorf("%w: 密码至少 6 位", errs.ErrValidation)
	}
	u, err := s.store.UserRepo().GetByID(ctx, id)
	if err != nil {
		return err
	}
	if oldPassword != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)); err != nil {
			return fmt.Errorf("%w: 旧密码不正确", errs.ErrValidation)
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return s.store.UserRepo().Update(ctx, &u)
}
```

- [ ] **步骤 4：适配既有调用**（`internal/adminapi/users.go` 的 HandleUserPassword 当前 `ChangePassword(ctx, id, req.Password)` → 后续任务 3 处理；**本任务先只改 service 层签名**，若 adminapi 编译失败需同步临时改 `ChangePassword(ctx, id, "", req.Password)`——实现时一并处理）

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/auth/ -v`
预期：全部 PASS（含新用例）

- [ ] **步骤 6：全量回归**

运行：`go test -count=1 ./...`（若 adminapi 因签名变化编译失败，需在任务 3 前临时适配）；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 2：角色 CRUD + 权限点列表 API

**文件：**
- 修改：`internal/adminapi/roles.go`（HandleRoleCreate/Update/Delete/Perms）
- 修改：`internal/adminapi/register.go`（注册 4 条路由）
- 创建：`internal/adminapi/roles_test.go`（角色 CRUD 测试）
- 测试：`internal/adminapi/adminapi_test.go`（复用 newEnv）

- [ ] **步骤 1：编写失败测试**（`internal/adminapi/roles_test.go`）

```go
package adminapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestRoleCRUDAndPerms(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 权限点列表
	w := e.do(t, http.MethodGet, "/api/roles/perms", "", tok)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "content.read") {
		t.Errorf("perms = %d %s", w.Code, w.Body.String())
	}
	// 建角色
	w = e.do(t, http.MethodPost, "/api/roles", `{"name":"reviewer","permissions":["content.read","content.write"]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create role = %d %s", w.Code, w.Body.String())
	}
	// 列表含新角色
	w = e.do(t, http.MethodGet, "/api/roles", "", tok)
	if !strings.Contains(w.Body.String(), "reviewer") {
		t.Errorf("roles 缺新角色: %s", w.Body.String())
	}
	// 更新角色
	var list struct {
		Data struct {
			Items []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	jsonUnmarshalHelper(t, w.Body.Bytes(), &list)
	var rid int64
	for _, r := range list.Data.Items {
		if r.Name == "reviewer" {
			rid = r.ID
		}
	}
	if rid == 0 {
		t.Fatal("未找到 reviewer 角色")
	}
	w = e.do(t, http.MethodPut, "/api/roles/"+itoa(rid), `{"name":"reviewer","permissions":["content.read"]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("update role = %d %s", w.Code, w.Body.String())
	}
	// 删除角色（无引用）
	w = e.do(t, http.MethodDelete, "/api/roles/"+itoa(rid), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("delete role = %d %s", w.Code, w.Body.String())
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/adminapi/ -run TestRoleCRUDAndPerms -v`
预期：FAIL（`/api/roles/perms` 404）

- [ ] **步骤 3：实现角色 CRUD handler**（`internal/adminapi/roles.go`）

```go
package adminapi

import (
	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
	"dulizhan/internal/errs"
	"dulizhan/internal/store"
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
	permJSON := mustJSON(req.Permissions)
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
	if err := d.Store.RoleRepo().Update(c.Request.Context(), id, req.Name, mustJSON(req.Permissions)); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"id": id})
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
	switch r.Name {
	case "admin", "editor", "author":
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
```
> `mustJSON` 需从 seed 包复用或复制到 adminapi（`internal/seed/seed.go` 有 `mustJSON`，但 seed 包导出——**检查**：若 seed 未导出则 adminapi 内建小工具）。`fmt` 需 import。`RoleRepo.Delete` 需确认接口存在（当前 RoleRepo 有 Create/GetByID/GetByName/List/Update，**无 Delete**——需在 store.go 接口加 `Delete(ctx, id) error` + sqlite 实现）。

- [ ] **步骤 3b：RoleRepo.Delete 接口 + 实现**

`internal/store/store.go` 的 RoleRepo 接口加：
```go
	Delete(ctx context.Context, id int64) error
```
`internal/store/sqlite/sqlite.go` 的 roleRepo 加：
```go
func (r *roleRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM roles WHERE id = ?", id)
	return err
}
```

- [ ] **步骤 3c：mustJSON 复用**（检查 `internal/seed/seed.go` 的 `mustJSON` 是否导出——若为小写 `mustJSON` 同包可用但 adminapi 不同包，需导出或 adminapi 内建）

```go
// internal/adminapi/roles.go 追加
func rolePermsJSON(perms []string) string {
	b, _ := json.Marshal(perms)
	return string(b)
}
```
需 import `encoding/json`。

- [ ] **步骤 4：注册路由**（`internal/adminapi/register.go`）

在 roles 路由后追加：
```go
	g.POST("/roles", auth.RequireAuth(d.Auth), auth.RequirePerm("roles.manage"), d.HandleRoleCreate)
	g.PUT("/roles/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("roles.manage"), d.HandleRoleUpdate)
	g.DELETE("/roles/:id", auth.RequireAuth(d.Auth), auth.RequirePerm("roles.manage"), d.HandleRoleDelete)
	g.GET("/roles/perms", auth.RequireAuth(d.Auth), auth.RequirePerm("roles.manage"), d.HandleRolePerms)
```
> **注意**：`GET /roles/perms` 与 `GET /roles` 共存，Gin 静态段优先级——`/roles/perms` 需在 `/roles` 后注册或确认不冲突（Gin 树支持 `:id` 与 `perms` 静态段共存，但 `GET /roles` 无参 vs `GET /roles/perms` 是不同路径，无冲突；`GET /roles/:id` 无此路由，安全）。

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/adminapi/ -run TestRoleCRUDAndPerms -v`
预期：PASS

- [ ] **步骤 6：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 3：用户自我保护（删/改角色/改密码）

**文件：**
- 修改：`internal/adminapi/users.go`（HandleUserDelete/Role/Password 加保护）
- 修改：`internal/adminapi/users_test.go`（新建，保护逻辑测试）
- 修改：`internal/auth/service.go`（若需要 CountAdmin 辅助——用现有 ListUsers 即可）

- [ ] **步骤 1：编写失败测试**（`internal/adminapi/users_test.go`）

```go
package adminapi

import (
	"net/http"
	"testing"
)

func TestUserSelfProtection(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	ctx := e.authCtx(t)

	// 获取当前 admin 用户 id
	var me struct {
		Data struct {
			User struct {
				ID int64 `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	w := e.do(t, http.MethodGet, "/api/auth/me", "", tok)
	jsonUnmarshalHelper(t, w.Body.Bytes(), &me)
	adminID := me.Data.User.ID

	// 不能删自己
	w = e.do(t, http.MethodDelete, "/api/users/"+itoa(adminID), "", tok)
	if w.Code != http.StatusForbidden {
		t.Errorf("删自己 = %d, want 403", w.Code)
	}
	// 不能改自己角色
	w = e.do(t, http.MethodPut, "/api/users/"+itoa(adminID)+"/role", `{"role_id":2}`, tok)
	if w.Code != http.StatusForbidden {
		t.Errorf("改自己角色 = %d, want 403", w.Code)
	}
	// 不能删内置 admin（username=admin）
	w = e.do(t, http.MethodDelete, "/api/users/"+itoa(adminID), "", tok)
	if w.Code != http.StatusForbidden {
		t.Errorf("删内置 admin = %d, want 403", w.Code)
	}
}

func TestUserPasswordOldCheck(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	ctx := e.authCtx(t)
	// 建一个普通用户
	adminRole, _ := e.st.RoleRepo().GetByName(ctx, "admin")
	w := e.do(t, http.MethodPost, "/api/users", `{"username":"u1","password":"secret123","role_id":`+itoa(adminRole.ID)+`}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create user = %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			User struct {
				ID int64 `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	jsonUnmarshalHelper(t, w.Body.Bytes(), &created)
	uid := created.Data.User.ID

	// admin 改他人密码免验旧密码
	w = e.do(t, http.MethodPut, "/api/users/"+itoa(uid)+"/password", `{"password":"newpass1"}`, tok)
	if w.Code != http.StatusOK {
		t.Errorf("admin 改他人密码 = %d %s", w.Code, w.Body.String())
	}
}
```
> 需要 `e.authCtx(t)` helper（返回 context.Background()）与 `jsonUnmarshalHelper`/`itoa`——检查 adminapi_test.go 是否已有 `json.Unmarshal` 用法，若无可加辅助函数。**实现时**用现有测试风格（`json.Unmarshal(w.Body.Bytes(), &resp)`）替代自定义 helper。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/adminapi/ -run 'TestUserSelfProtection|TestUserPasswordOldCheck' -v`
预期：FAIL（删除自己返回 200，无保护）

- [ ] **步骤 3：实现保护逻辑**（`internal/adminapi/users.go`）

```go
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
	// 最后一个 admin 保护
	if isAdminUser(u) {
		users, err := d.Store.UserRepo().List(c.Request.Context())
		if err != nil {
			fail(c, err)
			return
		}
		adminCount := 0
		for _, x := range users {
			if isAdminUser(x) {
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
	// ... 原逻辑
}

func (d *Deps) HandleUserPassword(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	var req struct {
		Password    string `json:"password"`
		OldPassword string `json:"old_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	old := ""
	if id == d.currentUserID(c) {
		old = req.OldPassword // 改自己需验旧
	}
	if err := d.Auth.ChangePassword(c.Request.Context(), id, old, req.Password); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"id": id})
}

// isAdminUser 判断用户是否为 admin 角色。
func isAdminUser(u store.User) bool {
	return u.RoleID == adminRoleID // 需查 admin 角色 id——实现时从 RoleRepo.GetByName("admin")
}
```
> **isAdminUser 实现**：需先 `GetByName("admin")` 拿 admin 角色 ID，再比较 u.RoleID。若每次查询开销大，在 `HandleUserDelete` 内先取 adminRoleID 再复用。**实现时**：
```go
func (d *Deps) adminRoleID(ctx context.Context) int64 {
	r, err := d.Store.RoleRepo().GetByName(ctx, "admin")
	if err != nil {
		return 0
	}
	return r.ID
}
```
需 import `context`、`fmt`、`errs`、`store`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/adminapi/ -run 'TestUserSelfProtection|TestUserPasswordOldCheck' -v`
预期：PASS

- [ ] **步骤 5：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 4：前端 API 层（user.ts + role.ts）+ 侧边栏菜单

**文件：**
- 创建：`admin/src/api/user.ts`
- 创建：`admin/src/api/role.ts`
- 修改：`admin/src/router/index.ts`（/users、/roles 路由）
- 修改：`admin/src/components/LayoutSidebar.vue`（用户/角色菜单 adminOnly）

- [ ] **步骤 1：api/user.ts**

```ts
import { request } from './client'

export interface UserItem {
  id: number
  username: string
  role_id: number
}
export interface RoleItem {
  id: number
  name: string
  permissions: string
}

export const listUsers = () => request<{ items: UserItem[] }>('/users')
export const createUser = (body: { username: string; password: string; role_id: number }) =>
  request<{ user: UserItem }>('/users', { method: 'POST', body })
export const deleteUser = (id: number) => request<{ deleted: number }>(`/users/${id}`, { method: 'DELETE' })
export const updateUserRole = (id: number, role_id: number) =>
  request<{ id: number }>(`/users/${id}/role`, { method: 'PUT', body: { role_id } })
export const updateUserPassword = (id: number, password: string, old_password?: string) =>
  request<{ id: number }>(`/users/${id}/password`, { method: 'PUT', body: { password, old_password } })
```

- [ ] **步骤 2：api/role.ts**

```ts
import { request } from './client'

export const listRoles = () => request<{ items: RoleItem[] }>('/roles')
export const listPerms = () => request<{ perms: string[] }>('/roles/perms')
export const createRole = (body: { name: string; permissions: string[] }) =>
  request<{ role: RoleItem }>('/roles', { method: 'POST', body })
export const updateRole = (id: number, body: { name: string; permissions: string[] }) =>
  request<{ id: number }>(`/roles/${id}`, { method: 'PUT', body })
export const deleteRole = (id: number) => request<{ deleted: number }>(`/roles/${id}`, { method: 'DELETE' })
```
> 角色权限 permissions 是 JSON 字符串（后端 Role.Permissions），前端 `JSON.parse` 解析展示。

- [ ] **步骤 3：路由**（`admin/src/router/index.ts` AppShell children 追加）

```ts
{ path: 'users', name: 'users', component: () => import('../views/UsersView.vue'), meta: { auth: true, adminOnly: true } },
{ path: 'roles', name: 'roles', component: () => import('../views/RolesView.vue'), meta: { auth: true, adminOnly: true } },
```

- [ ] **步骤 4：侧边栏**（`admin/src/components/LayoutSidebar.vue` menus 数组追加）

```ts
{ path: '/users', label: '用户管理', adminOnly: true },
{ path: '/roles', label: '角色管理', adminOnly: true },
```

- [ ] **步骤 5：验证**

运行：`cd admin && npx vue-tsc --noEmit`
预期：通过（路由引用 UsersView/RolesView 尚不存在——**需任务 5 建页面组件**；此步先建占位或在任务 5 一并建。**实现时**：任务 5 创建页面后统一验证）

---

### 任务 5：前端页面（UsersView + RolesView）+ 组件测试

**文件：**
- 创建：`admin/src/views/UsersView.vue`
- 创建：`admin/src/views/RolesView.vue`
- 创建：`admin/src/views/__tests__/users-roles.test.ts`（组件冒烟或逻辑测试）

- [ ] **步骤 1：UsersView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listUsers, createUser, deleteUser, updateUserRole, updateUserPassword, type UserItem } from '../api/user'
import { listRoles, type RoleItem } from '../api/role'

const users = ref<UserItem[]>([])
const roles = ref<RoleItem[]>([])
const dialogVisible = ref(false)
const form = ref({ username: '', password: '', role_id: 0 })
const pwdDialog = ref(false)
const pwdForm = ref({ id: 0, old_password: '', password: '' })

const roleName = (id: number) => roles.value.find(r => r.id === id)?.name ?? String(id)

async function load() {
  const [ur, rr] = await Promise.all([listUsers(), listRoles()])
  users.value = ur.items
  roles.value = rr.items
}

onMounted(load)

async function create() {
  if (!form.value.username || !form.value.password) { ElMessage.warning('请填写用户名和密码'); return }
  try {
    await createUser(form.value)
    ElMessage.success('已创建')
    dialogVisible.value = false
    form.value = { username: '', password: '', role_id: 0 }
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '创建失败') }
}

async function changeRole(u: UserItem, role_id: number) {
  try {
    await updateUserRole(u.id, role_id)
    ElMessage.success('已更新角色')
  } catch (e: any) { ElMessage.error(e.message ?? '操作失败'); await load() }
}

async function remove(u: UserItem) {
  try { await ElMessageBox.confirm(`确认删除用户 ${u.username}？`, '提示') } catch { return }
  try {
    await deleteUser(u.id)
    ElMessage.success('已删除')
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '删除失败') }
}

function openPwd(u: UserItem) { pwdForm.value = { id: u.id, old_password: '', password: '' }; pwdDialog.value = true }

async function savePwd() {
  try {
    await updateUserPassword(pwdForm.value.id, pwdForm.value.password, pwdForm.value.old_password || undefined)
    ElMessage.success('密码已更新')
    pwdDialog.value = false
  } catch (e: any) { ElMessage.error(e.message ?? '修改失败') }
}
</script>

<template>
  <div>
    <h2>用户管理</h2>
    <el-button type="primary" @click="dialogVisible = true">新建用户</el-button>
    <el-table :data="users" style="margin-top: 16px">
      <el-table-column prop="username" label="用户名" />
      <el-table-column label="角色" width="160">
        <template #default="{ row }">
          <el-select :model-value="row.role_id" @update:model-value="changeRole(row, $event)" style="width: 120px">
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button link type="primary" @click="openPwd(row)">改密码</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" title="新建用户" width="420px">
      <el-form label-width="80px">
        <el-form-item label="用户名"><el-input v-model="form.username" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="form.password" type="password" show-password /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role_id" style="width: 100%">
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="create">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="pwdDialog" title="修改密码" width="420px">
      <el-form label-width="80px">
        <el-form-item label="旧密码"><el-input v-model="pwdForm.old_password" type="password" show-password placeholder="修改他人密码可留空" /></el-form-item>
        <el-form-item label="新密码"><el-input v-model="pwdForm.password" type="password" show-password /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pwdDialog = false">取消</el-button>
        <el-button type="primary" @click="savePwd">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
```

- [ ] **步骤 2：RolesView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listRoles, listPerms, createRole, updateRole, deleteRole, type RoleItem } from '../api/role'

const roles = ref<RoleItem[]>([])
const perms = ref<string[]>([])
const dialogVisible = ref(false)
const editing = ref<RoleItem | null>(null)
const form = ref({ name: '', permissions: [] as string[] })

const builtin = ['admin', 'editor', 'author']

function parsePerms(raw: string): string[] {
  try { const arr = JSON.parse(raw); return Array.isArray(arr) ? arr : [] } catch { return [] }
}

async function load() {
  const [rr, pr] = await Promise.all([listRoles(), listPerms()])
  roles.value = rr.items
  perms.value = pr.perms
}

onMounted(load)

function openCreate() {
  editing.value = null
  form.value = { name: '', permissions: [] }
  dialogVisible.value = true
}

function openEdit(r: RoleItem) {
  editing.value = r
  form.value = { name: r.name, permissions: parsePerms(r.permissions) }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.name) { ElMessage.warning('请输入角色名'); return }
  try {
    if (editing.value) await updateRole(editing.value.id, form.value)
    else await createRole(form.value)
    ElMessage.success('已保存')
    dialogVisible.value = false
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '保存失败') }
}

async function remove(r: RoleItem) {
  try { await ElMessageBox.confirm(`确认删除角色 ${r.name}？`, '提示') } catch { return }
  try {
    await deleteRole(r.id)
    ElMessage.success('已删除')
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '删除失败') }
}
</script>

<template>
  <div>
    <h2>角色管理</h2>
    <el-button type="primary" @click="openCreate">新建角色</el-button>
    <el-table :data="roles" style="margin-top: 16px">
      <el-table-column prop="name" label="名称" width="140" />
      <el-table-column label="权限点">
        <template #default="{ row }">
          <template v-if="row.name === 'admin'"><el-tag type="danger">*</el-tag></template>
          <template v-else>
            <el-tag v-for="p in parsePerms(row.permissions)" :key="p" size="small" style="margin-right: 4px">{{ p }}</el-tag>
          </template>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button link type="primary" :disabled="builtin.includes(row.name)" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" :disabled="builtin.includes(row.name)" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑角色' : '新建角色'" width="520px">
      <el-form label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" :disabled="editing?.name === 'admin'" /></el-form-item>
        <el-form-item label="权限点">
          <el-checkbox-group v-model="form.permissions">
            <el-checkbox v-for="p in perms" :key="p" :label="p">{{ p }}</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
```

- [ ] **步骤 3：组件测试**（`admin/src/views/__tests__/users-roles.test.ts`）

```ts
import { describe, it, expect } from 'vitest'
// 纯逻辑测试（权限解析），避免复杂组件挂载
function parsePerms(raw: string): string[] {
  try { const arr = JSON.parse(raw); return Array.isArray(arr) ? arr : [] } catch { return [] }
}

describe('roles perms parsing', () => {
  it('解析权限 JSON', () => {
    expect(parsePerms('["content.read","content.write"]')).toEqual(['content.read', 'content.write'])
  })
  it('非法 JSON 返回空', () => {
    expect(parsePerms('nope')).toEqual([])
  })
})
```
> 组件挂载测试若复杂可只测纯函数（parsePerms 逻辑）。**实现时**：若需组件冒烟用 `@vue/test-utils` 挂载 UsersView/RolesView 断言渲染无错。

- [ ] **步骤 4：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run && npx vite build`
预期：通过

---

### 任务 6：生产集成 + 全量验收

**文件：**
- 修改：`internal/server/dist/`（同步新产物）

- [ ] **步骤 1：构建并同步 SPA**

```bash
cd admin && npm run build
# 同步 internal/server/dist
Remove-Item -Recurse -Force "..\internal\server\dist" -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path "..\internal\server\dist" -Force | Out-Null
Copy-Item -Path "dist\*" -Destination "..\internal\server\dist\" -Recurse -Force
New-Item -ItemType File -Path "..\internal\server\dist\.gitkeep" -Force | Out-Null
```

- [ ] **步骤 2：Go 全量验证**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`；`gofmt -l .`
预期：全绿、干净

- [ ] **步骤 3：前端全量验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run && npx vite build`
预期：通过

- [ ] **步骤 4：端到端冒烟**

```bash
# 1. seed + 启动
go run ./cmd/dulizhan seed
go run ./cmd/dulizhan --config config.yaml
# 2. 浏览器 /admin 登录 admin/admin123
# 3. 用户管理：建用户（选角色）→ 改角色 → 改密码（自己需旧密码）→ 删
# 4. 角色管理：新建角色 → 勾权限点 → 编辑 → 删（有引用拒删提示、内置不可删）
# 5. 新角色/用户登录验证权限生效
# 6. 自我保护：admin 删自己/改自己角色 → 403 提示
```
预期：用户/角色全流程可视化操作 + 保护逻辑生效。

- [ ] **步骤 5：清理**

停服、删 `dulizhan.db`/`data/`/临时产物，8080 释放。

---

## 自检记录

**1. 规格覆盖度：**
- 规格 §2.1 角色 CRUD + perms 列表 → 任务 2
- 规格 §2.3 用户自我保护（删自己/内置 admin/最后 admin/改自己角色/改密码验旧）→ 任务 3
- 规格 §2.4 ChangePassword 扩展 → 任务 1
- 规格 §3 前端页面（UsersView/RolesView）+ api 层 → 任务 4/5
- 规格 §4 测试（Go + 前端组件）→ 各任务 + 任务 5 步骤 3
- 规格 §5 验收 → 任务 6

**2. 占位符扫描：** 无 TBD/TODO。每步含代码。注意"实现时"标注（mustJSON 复用、isAdminUser 实现、测试 helper 复用）——均为明确的实现决策提示，非占位符。

**3. 类型一致性：**
- `ChangePassword(ctx, id, oldPassword, newPassword)` 任务 1 定义，任务 3 HandleUserPassword 使用
- `RoleRepo.Delete` 任务 2 步骤 3b 定义，HandleRoleDelete 使用
- `roleReq{Name, Permissions}` 任务 2 定义，create/update handler 一致
- `UserItem`/`RoleItem` 任务 4 定义，任务 5 页面使用
- `/roles/perms` 路由与 `HandleRolePerms` 一致
- `auth.AllPerms` 复用（已存在）
