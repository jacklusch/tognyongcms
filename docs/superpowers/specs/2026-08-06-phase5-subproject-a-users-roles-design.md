# Dulizhan CMS — 阶段 5 子项目 A：用户/角色管理界面 设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`、`2026-08-06-phase4-content-type-builder-design.md`
- 基线：阶段 4 已验收通过（构建器、类型级权限、relation/repeat 控件）

## 1. 目标与范围

构建用户管理页 + 角色管理页（admin 专属，`users.manage`/`roles.manage`），补全角色编辑后端 API 与用户自我保护逻辑。

**包含**：
- 用户管理页：列表/新建/改角色/改密码/删除
- 角色管理页：列表/新建/编辑权限点勾选/删除
- 后端：角色 CRUD + 权限点列表 API；用户自我保护（防删自己/防删内置 admin/防锁死/改自己密码验旧）

**排除**：多租户、用户自注册、邀请制、密码找回（YAGNI）。

## 2. 后端新增/扩展 API

### 2.1 角色 CRUD（`internal/adminapi/roles.go` 扩展 + register.go）

```
POST   /roles            roles.manage   建角色 {name, permissions}
PUT    /roles/:id        roles.manage   改角色 {name, permissions}
DELETE /roles/:id        roles.manage   删角色（引用保护 + 内置不可删）
GET    /roles/perms      roles.manage   返回 auth.AllPerms 权限点列表
```

- `RoleRepo` 现有：Create/GetByID/GetByName/List/Update（Update 签名 `Update(id, name, permissions string)`）。角色 CRUD handler 用现有 repo，无需新 repo 方法（除引用检查）。
- 引用检查：`UserRepo.List` 过滤 `RoleID == 目标`，存在用户则拒删（403/422 中文提示）。
- 内置保护：name 为 `admin`/`editor`/`author` 时拒绝删除（403「内置角色不可删除」）。

### 2.2 权限点列表（`internal/adminapi/roles.go`）

`GET /roles/perms` 返回 `auth.AllPerms`（11 个权限点：content.read/write/publish/delete、content_types.manage、media.upload/delete、menus.manage、settings.manage、users.manage、roles.manage）。前端勾选面板用。

### 2.3 用户自我保护（`internal/adminapi/users.go`）

- `DELETE /users/:id`：
  - `id == 当前用户ID` → 403「不能删除自己」
  - 目标 username == "admin" → 403「不能删除内置管理员」
  - 删除前检查最后一个 admin：`UserRepo.List` 中 `RoleID == admin角色ID` 的用户计数 ≤1 且目标是该角色用户 → 403「不能删除最后一个管理员」
- `PUT /users/:id/role`：`id == 当前用户ID` → 403「不能修改自己的角色」
- `PUT /users/:id/password`：body 加 `old_password`。改自己（`id == 当前用户`）需验旧密码；admin 改他人（id != 自己）免验（管理员重置）。`auth.Service.ChangePassword` 签名扩展。

### 2.4 auth.Service.ChangePassword 扩展

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
调用方 `HandleUserPassword`：`oldPassword` 从 body 取，本人场景传它，他人场景传空串。

## 3. 前端页面

### 3.1 用户管理页 `UsersView.vue`（`/users`，adminOnly 菜单）

- 列表：username、角色名（role_id→名称，前端从 `GET /roles` 建映射）、创建时间；操作：改角色（下拉）、改密码（弹层：旧密码+新密码+确认）、删除
- 新建用户：username + password + role_id（角色下拉）
- 后端保护 403 时前端 ElMessage 提示（不能删自己/内置 admin/最后一个 admin）

### 3.2 角色管理页 `RolesView.vue`（`/roles`，adminOnly 菜单）

- 列表：name、权限点 tag（permissions JSON 解析）；admin 角色显示 `*` 特殊标记
- 新建/编辑：name + 权限点勾选面板（`GET /roles/perms` 拿 AllPerms，checkbox；勾选组合权限点）
- 删除：有引用拒删提示、内置角色删除按钮禁用

### 3.3 api 层

- `admin/src/api/user.ts`：listUsers/createUser/deleteUser/updateUserRole/updateUserPassword
- `admin/src/api/role.ts`：listRoles/createRole/updateRole/deleteRole/listPerms

## 4. 测试策略

- **后端 Go**：角色 CRUD（建/改/删/引用拒删/内置拒删/perms 列表）、用户保护（删自己 403/删内置 403/最后一个 admin 403/改自己角色 403/改自己密码验旧/改他人免验）单测（adminapi 层 + auth service 层）
- **前端**：UsersView/RolesView 组件 vitest 冒烟（渲染/列表展示），权限勾选面板逻辑单测
- **验收冒烟**：admin 登录 → 用户页建用户/改角色/改密码/删 → 角色页建角色/勾权限/删 → 新角色登录验证权限生效；保护场景（删自己等）返回 403 提示

## 5. 工作流约定

- 不 git 提交；superpowers SDD 驱动；前端构建同步 `internal/server/dist`
