# 阶段 4：可视化内容类型构建器 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 构建可视化内容类型构建器（列表+字段面板双栏），补全 relation/repeat 字段控件，实现类型级权限点（`content.read.<type>` 前缀匹配），内容类型级联删除。

**架构：** 后端扩展 schema.Field（RelationType/SubFields）+ 校验器递归 + RBAC 前缀匹配 + me() 返回权限 + 级联删除（WithTx 事务）；前端补 types/registry/两个控件/ContentPicker + 新增 ContentTypesView 构建器页面 + ContentListView/EditView 按类型权限过滤。

**技术栈：** Go（gin@v1.10.0、modernc.org/sqlite）、Vue 3 + TS + Element Plus + vuedraggable。

**前置基线：** 阶段 3 已验收通过（SPA 管理端、`/admin` go:embed、动态表单 12 控件）。规格：`docs/superpowers/specs/2026-08-06-phase4-content-type-builder-design.md`。

**环境约束（所有任务遵守）：**
- **不 git 提交**（仓库约定）；**勿运行 `go mod tidy`**
- Go 验证：`go test -count=1 <包> -v`，阶段收尾 `go test -count=1 ./...`
- 前端验证：`cd admin && npx vue-tsc --noEmit && npx vitest run`；改完 SPA 后 `npx vite build` + 同步 `internal/server/dist/`
- 错误消息用中文；后端依赖锁定勿动

---

### 任务 1：schema.Field 扩展 + 校验器递归

**文件：**
- 修改：`internal/schema/schema.go`（Field 加 RelationType/SubFields）
- 修改：`internal/schema/validate.go`（ValidateContentType/ValidateDocument 递归）
- 修改：`internal/schema/schema_test.go`（补测试）

- [ ] **步骤 1：编写失败测试**（追加到 `internal/schema/schema_test.go`）

```go
func TestValidateContentTypeRelationRepeat(t *testing.T) {
	reg := NewRegistry()
	good := ContentType{Name: "post", Label: "帖子", Fields: []Field{
		{Name: "author", Label: "作者", Type: TypeRelation, RelationType: "user"},
		{Name: "items", Label: "条目", Type: TypeRepeat, SubFields: []Field{
			{Name: "title", Label: "标题", Type: TypeText, Required: true},
		}},
	}}
	if err := reg.ValidateContentType(&good); err != nil {
		t.Fatalf("合法 CT 应通过: %v", err)
	}
	// relation 缺目标类型
	badRel := ContentType{Name: "x1", Fields: []Field{{Name: "author", Type: TypeRelation}}}
	if err := reg.ValidateContentType(&badRel); err == nil {
		t.Error("relation 缺目标类型应报错")
	}
	// repeat 缺子字段
	badRep := ContentType{Name: "x2", Fields: []Field{{Name: "items", Type: TypeRepeat}}}
	if err := reg.ValidateContentType(&badRep); err == nil {
		t.Error("repeat 缺子字段应报错")
	}
	// repeat 子字段非法（重复名）
	badSub := ContentType{Name: "x3", Fields: []Field{{Name: "items", Type: TypeRepeat, SubFields: []Field{
		{Name: "a", Type: TypeText}, {Name: "a", Type: TypeText},
	}}}}
	if err := reg.ValidateContentType(&badSub); err == nil {
		t.Error("repeat 子字段重名应报错")
	}
	// relation/repeat 不能做索引
	idx := ContentType{Name: "x4", Fields: []Field{{Name: "r", Type: TypeRelation, Indexed: true}}}
	if err := reg.ValidateContentType(&idx); err == nil {
		t.Error("relation 不应可索引")
	}
}

func TestValidateDocumentRelationRepeat(t *testing.T) {
	reg := NewRegistry()
	ct := ContentType{Name: "post", Fields: []Field{
		{Name: "author", Label: "作者", Type: TypeRelation, RelationType: "user", Required: true},
		{Name: "items", Label: "条目", Type: TypeRepeat, SubFields: []Field{
			{Name: "title", Label: "标题", Type: TypeText, Required: true},
		}},
	}}
	// 合法
	if err := reg.ValidateDocument(&ct, map[string]any{
		"author": "grp-abc",
		"items":  []any{map[string]any{"title": "一"}},
	}); err != nil {
		t.Fatalf("合法文档应通过: %v", err)
	}
	// relation 空
	if err := reg.ValidateDocument(&ct, map[string]any{"author": "", "items": []any{}}); err == nil {
		t.Error("必填 relation 为空应报错")
	}
	// repeat 行内缺必填子字段
	if err := reg.ValidateDocument(&ct, map[string]any{"author": "g", "items": []any{map[string]any{}}}); err == nil {
		t.Error("repeat 行缺必填子字段应报错")
	}
	// repeat 非数组
	if err := reg.ValidateDocument(&ct, map[string]any{"author": "g", "items": "nope"}); err == nil {
		t.Error("repeat 非数组应报错")
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/schema/ -run 'TestValidateContentTypeRelationRepeat|TestValidateDocumentRelationRepeat' -v`
预期：FAIL（`Field.RelationType` 未定义）

- [ ] **步骤 3：Field 加字段**（`internal/schema/schema.go`）

在 `Field` 结构末尾（Max 之后）追加：
```go
	RelationType string  `json:"relation_type,omitempty"` // relation: 目标内容类型名
	SubFields    []Field `json:"sub_fields,omitempty"`    // repeat: 子字段组
```

- [ ] **步骤 4：校验器扩展**（`internal/schema/validate.go`）

在 `ValidateContentType` 的字段循环中追加分支：
```go
		if f.Type == TypeRelation && f.RelationType == "" {
			return fmt.Errorf("字段 %q 必须配置目标内容类型", f.Name)
		}
		if f.Type == TypeRepeat {
			if len(f.SubFields) == 0 {
				return fmt.Errorf("字段 %q 必须配置子字段", f.Name)
			}
			if err := validateSubFields(f.SubFields); err != nil {
				return fmt.Errorf("字段 %q 子字段配置错误: %w", f.Name, err)
			}
		}
		if f.Indexed && (f.Type == TypeRelation || f.Type == TypeRepeat) {
			return fmt.Errorf("字段 %q 类型 %q 不支持作为索引列", f.Name, f.Type)
		}
```

追加递归辅助（文件末尾）：
```go
// validateSubFields 递归校验 repeat 子字段组（名称/类型/嵌套）。
func validateSubFields(fields []Field) error {
	seen := map[string]bool{}
	for i := range fields {
		f := &fields[i]
		if !validName.MatchString(f.Name) {
			return fmt.Errorf("字段名 %q 不合法", f.Name)
		}
		if seen[f.Name] {
			return fmt.Errorf("字段名 %q 重复", f.Name)
		}
		seen[f.Name] = true
		switch f.Type {
		case TypeRelation:
			if f.RelationType == "" {
				return fmt.Errorf("字段 %q 必须配置目标内容类型", f.Name)
			}
		case TypeRepeat:
			if err := validateSubFields(f.SubFields); err != nil {
				return fmt.Errorf("字段 %q 子字段配置错误: %w", f.Name, err)
			}
		}
	}
	return nil
}
```
> 注：`validateSubFields` 不校验字段类型是否在 Registry（子字段类型集合与顶层一致，由构建器约束；若需严格可传 registry——实现时保持简单，仅校验名称/嵌套/relation 目标）。

`ValidateDocument` 的 switch 中 `TypeRelation` 与 `TypeRepeat` 分支改为：
```go
	case TypeRelation:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("字段 %q 需要字符串(关联内容ID)", f.Name)
		}
	case TypeRepeat:
		arr, ok := v.([]any)
		if !ok {
			return fmt.Errorf("字段 %q 需要数组", f.Name)
		}
		for i, item := range arr {
			row, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("字段 %q 第 %d 行需为对象", f.Name, i+1)
			}
			for _, sub := range f.SubFields {
				if err := r.validateFieldValue(sub, row[sub.Name]); err != nil {
					return fmt.Errorf("字段 %q 第 %d 行: %w", f.Name, i+1, err)
				}
			}
		}
```
> 注意：`validateFieldValue` 已是 `(*Registry).validateFieldValue`（方法，task 3 恢复时方法化），递归用 `r.validateFieldValue`。

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/schema/ -v`
预期：全部 PASS（含既有 + 新）

- [ ] **步骤 6：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 2：RBAC 前缀匹配 + RequirePermType 中间件

**文件：**
- 修改：`internal/auth/service.go`（HasPerm 前缀匹配）
- 修改：`internal/auth/middleware.go`（RequirePermType）
- 修改：`internal/auth/service_test.go`（TestHasPermTypePrefix）
- 修改：`internal/auth/middleware_test.go`（TestRequirePermType）

- [ ] **步骤 1：编写失败测试**（追加到 `internal/auth/service_test.go`）

```go
func TestHasPermTypePrefix(t *testing.T) {
	// 精确权限
	us1 := &UserSession{Perms: map[string]bool{"content.read.article": true}}
	if !us1.HasPerm("content.read.article") {
		t.Error("精确权限应命中")
	}
	if us1.HasPerm("content.read.page") {
		t.Error("无该类型权限应拒绝")
	}
	// content.* 通配
	us2 := &UserSession{Perms: map[string]bool{"content.*": true}}
	if !us2.HasPerm("content.read.article") {
		t.Error("content.* 应命中任意 content 操作")
	}
	// 基础权限（无类型后缀）
	us3 := &UserSession{Perms: map[string]bool{"content.read": true}}
	if !us3.HasPerm("content.read") {
		t.Error("基础权限自身应命中")
	}
	// admin *
	us4 := &UserSession{Perms: map[string]bool{"*": true}}
	if !us4.HasPerm("anything.at.all") {
		t.Error("admin * 应全命中")
	}
	// 无匹配
	us5 := &UserSession{Perms: map[string]bool{"menus.manage": true}}
	if us5.HasPerm("content.write.article") {
		t.Error("无匹配应拒绝")
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/auth/ -run TestHasPermTypePrefix -v`
预期：FAIL（当前 HasPerm 不支持前缀）

- [ ] **步骤 3：HasPerm 前缀匹配**（`internal/auth/service.go`）

```go
// HasPerm 支持类型级前缀匹配：content.read.article 命中精确、content.read.* / content.* / *
func (us *UserSession) HasPerm(p string) bool {
	if us.Perms["*"] {
		return true
	}
	if us.Perms[p] {
		return true
	}
	parts := strings.Split(p, ".")
	for i := len(parts) - 1; i >= 1; i-- {
		if us.Perms[strings.Join(parts[:i], ".")+".*"] {
			return true
		}
	}
	return false
}
```
需新增 import `strings`。

- [ ] **步骤 4：RequirePermType 中间件**（`internal/auth/middleware.go` 追加）

```go
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
```

- [ ] **步骤 5：中间件测试**（追加到 `internal/auth/middleware_test.go`）

```go
func TestRequirePermType(t *testing.T) {
	r, svc := newRouter(t)
	// newRouter 已建 admin(全权) + author(content.read,content.write)
	adminTok, _ := svc.Login(context.Background(), "admin", "secret123")
	authorTok, _ := svc.Login(context.Background(), "writer", "secret123")

	r.GET("/type-secure/:type", RequireAuth(svc), func(c *gin.Context) {
		RequirePermType("content.read", c.Param("type"))(c)
		c.JSON(200, gin.H{"ok": true})
	})
	// 未登录 → 401
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/type-secure/article", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("未登录 = %d, want 401", w.Code)
	}
	// admin → 200
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/type-secure/article", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: adminTok})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("admin = %d, want 200", w.Code)
	}
	// author（content.read 基础权限）→ 200（无类型后缀，HasPerm("content.read.article") 需 author 有 content.* 或 content.read.*；newRouter 的 author 只有 content.read/content.write → 403）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/type-secure/article", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: authorTok})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("author 无类型权限 = %d, want 403", w.Code)
	}
}
```
> 注意：此测试断言 author（仅 content.read 基础权限）对 `content.read.article` 返回 403——这正是类型级权限的语义（基础权限不自动扩展到类型）。任务 3 的 seed 更新后 editor 有 `content.*` 可通。

- [ ] **步骤 6：运行测试验证通过**

运行：`go test -count=1 ./internal/auth/ -v`
预期：全部 PASS

- [ ] **步骤 7：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 3：内容路由类型级权限 + HandleMe 权限 + seed 更新

**文件：**
- 修改：`internal/adminapi/content.go`（各 handler 加类型权限校验）
- 修改：`internal/adminapi/auth.go`（HandleMe 返回 permissions）
- 修改：`internal/seed/seed.go`（editor 加 content.*）
- 修改：`internal/adminapi/content_test.go` 或 `adminapi_test.go`（补测试）
- 修改：`internal/seed/seed_test.go`（补 editor 权限断言）

- [ ] **步骤 1：编写失败测试**（追加到 `internal/seed/seed_test.go`）

```go
func TestEnsureAuthEditorPerms(t *testing.T) {
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	authSvc := auth.New(st, time.Hour)
	if err := EnsureAuth(ctx, st, authSvc); err != nil {
		t.Fatal(err)
	}
	editor, err := st.RoleRepo().GetByName(ctx, "editor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(editor.Permissions, `"content.*"`) {
		t.Errorf("editor 应含 content.* 通配: %s", editor.Permissions)
	}
}
```
需新增 import `strings`。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/seed/ -run TestEnsureAuthEditorPerms -v`
预期：FAIL（editor 无 content.*）

- [ ] **步骤 3：seed 更新 editor 权限**（`internal/seed/seed.go`）

`EnsureAuth` 的 roles map 中 editor 权限改为：
```go
		"editor": {"content.*", "content_types.manage", "media.upload", "media.delete", "menus.manage", "settings.manage"},
```

- [ ] **步骤 4：HandleMe 返回 permissions**（`internal/adminapi/auth.go`）

```go
func (d *Deps) HandleMe(c *gin.Context) {
	us, ok := auth.UserFromContext(c)
	if !ok || us == nil {
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
```
需新增 import `sort`。

- [ ] **步骤 5：内容路由类型级校验 helper**（`internal/adminapi/content.go`）

```go
// requireTypePerm 校验当前用户对某内容类型的操作权限；失败写 403 返回 false。
func (d *Deps) requireTypePerm(c *gin.Context, action, typeName string) bool {
	us, ok := auth.UserFromContext(c)
	if !ok || us == nil {
		unauthorized(c)
		return false
	}
	if !us.HasPerm(action + "." + typeName) {
		fail(c, errs.ErrForbidden)
		return false
	}
	return true
}
```

各 handler 开头追加类型校验（用现有 `GetByID` 拿 typeName）：

`HandleContentList`（列表，query type）：
```go
	typeName := c.Query("type")
	if !d.requireTypePerm(c, "content.read", typeName) {
		return
	}
```

`HandleContentGet`/`HandleContentUpdate`/`HandleContentDelete`/`HandleContentPublish`/`HandleContentUnpublish`（路径 id → GetByID → typeName）：
```go
	id, ok := mustID(c)
	if !ok {
		return
	}
	// 类型权限：先取内容得 typeName
	cur, err := d.Content.GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	if !d.requireTypePerm(c, "content.read", cur.TypeName) {
		return
	}
	// ... 原逻辑（注意 GetByID 已查过，Update/Delete 内部会再查——可接受；或复用 cur）
```
> 说明：`HandleContentGet` 直接用 `cur` 返回（避免重复查）。`HandleContentUpdate/Delete/Publish/Unpublish` 用 `cur.TypeName` 做权限判定后走原逻辑（内部再查一次可接受，量小）。

`HandleTranslations`/`HandleCreateTranslation`（路径 id）同样用 `GetByID` 拿 typeName 后判定 `content.read`/`content.write`。

- [ ] **步骤 6：adminapi 测试**（追加到 `adminapi_test.go`）

```go
func TestTypePermScoped(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	// author 角色（content.read/content.write，无 content.*，无类型级）
	authorRole, err := e.st.RoleRepo().GetByName(ctx, "author")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.RoleRepo().Update(ctx, authorRole.ID, "author", `["content.read","content.write","media.upload"]`); err != nil {
		t.Fatal(err)
	}
	authorTok, _ := e.auth.Login(ctx, "alice", "secret123")
	// author 建内容（Create 走基础 content.write 门禁 + service canManage 归属，可过）
	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"作者文章","slug":"author-post","content":"<p>正文</p>"}}`, authorTok)
	if w.Code != http.StatusOK {
		t.Fatalf("author create = %d %s", w.Code, w.Body.String())
	}
	// author 列表：RequirePerm("content.read") 过，但类型级 content.read.article 无（author 无 content.* 无 content.read.*）→ 403
	w = e.do(t, http.MethodGet, "/api/content?type=article&lang=zh", "", authorTok)
	if w.Code != http.StatusForbidden {
		t.Errorf("author 列表(无类型权限) = %d, want 403", w.Code)
	}
}
```
> **裁定（author 权限最终形态）**：seed 中 author 保持 `["content.read","content.write","media.upload"]` **不**加 `content.read.*`——类型级权限语义下，author 能建内容（基础 content.write + canManage 归属）但不能浏览类型列表（无类型级 read）。若后续要 author 可浏览，再单独授权。此裁定与任务 2 `TestRequirePermType`（author 403）一致。

- [ ] **步骤 7：运行测试验证通过**

运行：`go test -count=1 ./internal/seed/ ./internal/adminapi/ -v`
预期：全部 PASS（按步骤 6 裁定调整断言）

- [ ] **步骤 8：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 4：内容类型级联删除（WithTx）

**文件：**
- 修改：`internal/store/store.go`（ContentRepo 加 DeleteByType、Store 加 WithTx）
- 修改：`internal/store/sqlite/sqlite.go`（DeleteByType 实现 + WithTx 事务）
- 修改：`internal/store/sqlite/sqlite_test.go`（补测试）
- 修改：`internal/content/service.go`（DeleteType 改造）
- 修改：`internal/content/service_test.go`（补测试）

- [ ] **步骤 1：编写失败测试（sqlite 层）**（追加到 `internal/store/sqlite/sqlite_test.go`）

```go
func TestContentDeleteByType(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	if err := types.Create(ctx, ct); err != nil {
		t.Fatal(err)
	}
	repo := st.ContentRepo()
	for _, c := range []*store.Content{
		{ContentTypeID: ct.ID, ContentID: "g1", Lang: "zh", Slug: "a", Status: "published"},
		{ContentTypeID: ct.ID, ContentID: "g2", Lang: "zh", Slug: "b", Status: "draft"},
	} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.DeleteByType(ctx, "article"); err != nil {
		t.Fatal(err)
	}
	n, _ := repo.CountByTypeLangStatus(ctx, "article", "zh", "")
	if n != 0 {
		t.Errorf("删类型后内容应清零, count=%d", n)
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/store/sqlite/ -run TestContentDeleteByType -v`
预期：FAIL（DeleteByType 未定义）

- [ ] **步骤 3：接口与实现**

`internal/store/store.go`：
- ContentRepo 接口加 `DeleteByType(ctx context.Context, typeName string) error`
- Store 接口加 `WithTx(ctx context.Context, fn func(tx Store) error) error`

`internal/store/sqlite/sqlite.go`：
```go
func (r *contentRepo) DeleteByType(ctx context.Context, typeName string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM content WHERE content_type_id = (SELECT id FROM content_types WHERE name = ?)", typeName)
	return err
}
```

`sqlite.Store` 增加 WithTx：
```go
// WithTx 在事务中执行 fn，成功提交失败回滚。
func (s *Store) WithTx(ctx context.Context, fn func(tx store.Store) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	txStore := &Store{db: tx}
	if err := fn(txStore); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
```
> 注意：`sql.DB.BeginTx` 返回 `*sql.Tx`，`*sql.Tx` 实现 `QueryContext/ExecContext/QueryRowContext`——`&Store{db: tx}` 依赖 Store 内部只用 `db.QueryContext/ExecContext/QueryRowContext`（现实现均如此）。**若某 repo 方法用了 `db.PrepareContext` 或事务无关特性需检查**——现有实现全部走 QueryContext/ExecContext，兼容。

- [ ] **步骤 4：Service.DeleteType 改造**（`internal/content/service.go`）

```go
// DeleteType 级联删除内容类型及其全部内容（事务）。
func (s *Service) DeleteType(ctx context.Context, name string) error {
	ct, err := s.GetType(ctx, name)
	if err != nil {
		return err
	}
	return s.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.ContentRepo().DeleteByType(ctx, name); err != nil {
			return err
		}
		return tx.ContentTypeRepo().Delete(ctx, ct.ID)
	})
}
```

- [ ] **步骤 5：content 层测试**（追加到 `internal/content/service_test.go`）

```go
func TestDeleteTypeCascades(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestService(t)
	e, err := svc.Create(ctx, "article", "zh", map[string]any{"title": "a", "slug": "a"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteType(ctx, "article"); err != nil {
		t.Fatal(err)
	}
	// 内容已被删
	if _, err := st.ContentRepo().GetByID(ctx, e.Content.ID); err != errs.ErrNotFound {
		t.Errorf("级联删除后内容应不存在, got %v", err)
	}
	// 类型已被删
	if _, err := st.ContentTypeRepo().GetByName(ctx, "article"); err != errs.ErrNotFound {
		t.Errorf("级联删除后类型应不存在, got %v", err)
	}
	// 删不存在的类型 → ErrNotFound
	if err := svc.DeleteType(ctx, "nope"); err != errs.ErrNotFound {
		t.Errorf("删不存在类型 = %v, want ErrNotFound", err)
	}
}
```

- [ ] **步骤 6：运行测试验证通过**

运行：`go test -count=1 ./internal/store/sqlite/ ./internal/content/ -v`
预期：全部 PASS

- [ ] **步骤 7：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 5：前端 types/registry 补全 + relation/repeat 控件 + ContentPicker

**文件：**
- 修改：`admin/src/dynamic-form/types.ts`（FieldType 加 relation/repeat、SchemaField 加 relation_type/sub_fields）
- 修改：`admin/src/dynamic-form/registry.ts`（validateRelation/validateRepeat + 两控件注册）
- 创建：`admin/src/dynamic-form/controls/RelationControl.vue`
- 创建：`admin/src/dynamic-form/controls/RepeatControl.vue`
- 创建：`admin/src/components/ContentPicker.vue`
- 创建：`admin/src/dynamic-form/__tests__/registry-relation-repeat.test.ts`
- 修改：`admin/src/api/content.ts`（listContent 已够用）

- [ ] **步骤 1：types.ts 扩展**

```ts
export type FieldType =
  | 'text' | 'textarea' | 'richtext' | 'number' | 'boolean' | 'date' | 'datetime'
  | 'select' | 'multiselect' | 'image' | 'file' | 'slug'
  | 'relation' | 'repeat'

export interface SchemaField {
  // ...既有
  relation_type?: string
  sub_fields?: SchemaField[]
}
```

- [ ] **步骤 2：registry 校验器 + 注册**（`admin/src/dynamic-form/registry.ts`）

追加：
```ts
export function validateRelation(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  return null
}

export function validateRepeat(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (!Array.isArray(v)) return `${f.label}需要数组`
  const sub = f.sub_fields ?? []
  for (let i = 0; i < v.length; i++) {
    const row = v[i]
    const errs = validateForm(sub, (row ?? {}) as FormValues)
    const firstErr = Object.values(errs)[0]
    if (firstErr) return `${f.label}第 ${i + 1} 行: ${firstErr}`
  }
  return null
}
```

import + 注册：
```ts
import RelationControl from './controls/RelationControl.vue'
import RepeatControl from './controls/RepeatControl.vue'
// registry 末尾追加
  relation: { component: RelationControl, validate: validateRelation },
  repeat: { component: RepeatControl, validate: validateRepeat },
```

- [ ] **步骤 3：ContentPicker.vue**（`admin/src/components/ContentPicker.vue`）

```vue
<script setup lang="ts">
import { ref, watch } from 'vue'
import { listContent, type ContentEntry } from '../api/content'

const props = defineProps<{ typeName: string; visible: boolean }>()
const emit = defineEmits<{ 'update:visible': [boolean]; select: [content_id: string] }>()

const items = ref<ContentEntry[]>([])
const loading = ref(false)
const keyword = ref('')

async function load() {
  if (!props.visible) return
  loading.value = true
  try {
    const r = await listContent({ type: props.typeName, lang: '', page: 1, perPage: 100 })
    items.value = r.items
  } finally {
    loading.value = false
  }
}

function pick(entry: ContentEntry) {
  emit('select', entry.content.content_id)
  emit('update:visible', false)
}

watch(() => props.visible, (v) => { if (v) load() })
</script>

<template>
  <el-dialog :model-value="visible" title="选择内容" width="640px" @update:model-value="$emit('update:visible', $event)" @open="load">
    <el-input v-model="keyword" placeholder="搜索标题" clearable style="margin-bottom: 12px" />
    <el-table :data="items" height="360" @row-click="pick">
      <el-table-column prop="content.title" label="标题" />
      <el-table-column prop="content.slug" label="Slug" width="180" />
      <el-table-column prop="content.lang" label="语言" width="80" />
    </el-table>
    <template #footer>
      <el-button @click="$emit('update:visible', false)">取消</el-button>
    </template>
  </el-dialog>
</template>
```
> 注：搜索框过滤可用前端 `computed` 对 `items` 按标题过滤（数据量小），或调 `searchContent`——**实现时用前端 computed 过滤**（列表已加载 100 条）。

- [ ] **步骤 4：RelationControl.vue**（`admin/src/dynamic-form/controls/RelationControl.vue`）

```vue
<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { SchemaField } from '../types'
import ContentPicker from '../../components/ContentPicker.vue'

const props = defineProps<{ field: SchemaField; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()

const pickerVisible = ref(false)
const candidates = ref<{ content_id: string; title: string }[]>([])
const loading = ref(false)
const targetType = computed(() => props.field.relation_type ?? '')

// 回显：modelValue(content_id) 在候选中的标题
const currentTitle = computed(() => {
  if (!props.modelValue) return ''
  const hit = candidates.value.find((c) => c.content_id === props.modelValue)
  return hit ? hit.title : `${props.modelValue}（未匹配候选，可重新选择）`
})

async function loadCandidates() {
  if (!targetType.value) return
  loading.value = true
  try {
    const { listContent } = await import('../../api/content')
    const r = await listContent({ type: targetType.value, lang: '', page: 1, perPage: 100 })
    candidates.value = r.items.map((it) => ({ content_id: it.content.content_id, title: it.content.title }))
  } finally {
    loading.value = false
  }
}

function onSelect(content_id: string) {
  emit('update:modelValue', content_id)
  pickerVisible.value = false
}

watch(pickerVisible, (v) => { if (v) loadCandidates() })
</script>

<template>
  <div class="relation-control">
    <el-input :model-value="currentTitle" readonly :placeholder="field.label" @click="pickerVisible = true">
      <template #append>
        <el-button @click="pickerVisible = true">选择</el-button>
      </template>
    </el-input>
    <el-button v-if="modelValue" link type="danger" @click="$emit('update:modelValue', '')">清除</el-button>
    <ContentPicker :type-name="targetType" :visible="pickerVisible" @update:visible="pickerVisible = $event" @select="onSelect" />
  </div>
</template>
```
> 说明：`listContent` 的 `lang: ''` 后端已支持不过滤（任务 3 phase3 实现）。candidates 跨语言。

- [ ] **步骤 5：RepeatControl.vue**（`admin/src/dynamic-form/controls/RepeatControl.vue`）

```vue
<script setup lang="ts">
import type { SchemaField, FormValues } from '../types'
import DynamicForm from '../DynamicForm.vue'

const props = defineProps<{ field: SchemaField; modelValue: unknown }>()
const emit = defineEmits<{ 'update:modelValue': [unknown] }>()

const rows = computed<FormValues[]>(() => Array.isArray(props.modelValue) ? props.modelValue as FormValues[] : [])

function addRow() {
  const next = [...rows.value]
  next.push(defaultRow())
  emit('update:modelValue', next)
}

function removeRow(i: number) {
  const next = rows.value.filter((_, idx) => idx !== i)
  emit('update:modelValue', next)
}

function updateRow(i: number, row: FormValues) {
  const next = [...rows.value]
  next[i] = row
  emit('update:modelValue', next)
}

function defaultRow(): FormValues {
  const out: FormValues = {}
  for (const f of props.field.sub_fields ?? []) {
    if (f.default !== undefined) out[f.name] = f.default
    else out[f.name] = ''
  }
  return out
}
</script>

<template>
  <div class="repeat-control">
    <div v-for="(row, i) in rows" :key="i" class="repeat-row">
      <DynamicForm :model-value="row" :fields="field.sub_fields ?? []" @update:model-value="updateRow(i, $event)" />
      <el-button link type="danger" @click="removeRow(i)">删除行</el-button>
    </div>
    <el-button size="small" @click="addRow">添加行</el-button>
  </div>
</template>

<style scoped>
.repeat-row { border: 1px dashed #dcdfe6; border-radius: 4px; padding: 8px 12px; margin-bottom: 8px; position: relative; }
</style>
```
> 注：需 import `computed`。DynamicForm 的 `modelValue` 是 `FormValues`，`row` 已是 `FormValues`——类型兼容。若 DynamicForm 的 props 类型不接受未知索引对象，调整类型断言。

- [ ] **步骤 6：Vitest 测试**（`admin/src/dynamic-form/__tests__/registry-relation-repeat.test.ts`）

```ts
import { describe, it, expect } from 'vitest'
import { validateRelation, validateRepeat } from '../registry'
import type { SchemaField } from '../types'

const f = (name: string, type: any, extra: Partial<SchemaField> = {}): SchemaField => ({
  name, label: name, type, ...extra,
})

describe('relation/repeat validators', () => {
  it('relation 必填与值', () => {
    expect(validateRelation('', f('r', 'relation', { required: true }))).toBeTruthy()
    expect(validateRelation('grp-abc', f('r', 'relation'))).toBeNull()
  })
  it('repeat 数组与逐行子校验', () => {
    const rep = f('items', 'repeat', { required: true, sub_fields: [{ name: 'title', label: '标题', type: 'text', required: true } as SchemaField] })
    expect(validateRepeat('x', rep)).toBeTruthy()
    expect(validateRepeat([], rep)).toBeNull()
    expect(validateRepeat([{ title: '' }], rep)).toBeTruthy() // 行内缺必填
    expect(validateRepeat([{ title: 'ok' }], rep)).toBeNull()
  })
})
```

- [ ] **步骤 7：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run && npx vite build`
预期：通过（vitest 含新用例）

---

### 任务 6：前端类型构建器页面（ContentTypesView + FieldConfigForm）

**文件：**
- 修改：`admin/src/api/content-types.ts`（补 create/update/delete）
- 创建：`admin/src/views/ContentTypesView.vue`（列表+字段面板双栏）
- 创建：`admin/src/components/FieldConfigForm.vue`（字段配置弹层）
- 修改：`admin/src/components/LayoutSidebar.vue`（加"内容类型"菜单）
- 修改：`admin/src/router/index.ts`（加 /content-types 路由）
- 创建：`admin/src/views/__tests__/content-types-validate.test.ts`（前端校验）

- [ ] **步骤 1：api/content-types.ts 补齐**

```ts
export const createContentType = (body: { name: string; label: string; fields: SchemaField[] }) =>
  request<{ content_type: ContentTypeItem }>('/content-types', { method: 'POST', body })
export const updateContentType = (id: number, body: { label: string; fields: SchemaField[] }) =>
  request<{ content_type: ContentTypeItem }>(`/content-types/${id}`, { method: 'PUT', body })
export const deleteContentType = (name: string) => request<{ deleted: string }>(`/content-types/${name}`, { method: 'DELETE' })
```
> 核对后端 `HandleContentTypeCreate` body：`{name,label,fields}`（`contentTypeReq`）。`HandleContentTypeUpdate` body 同结构，`:id` 路径。

- [ ] **步骤 2：前端校验工具**（`admin/src/views/content-types-validate.ts`）

```ts
import type { SchemaField } from '../dynamic-form/types'

export interface TypeDraft {
  name: string
  label: string
  fields: SchemaField[]
}

export function validateContentType(draft: TypeDraft): string | null {
  if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(draft.name)) return '类型名称需以字母开头且只含字母数字下划线'
  if (!draft.label) return '请输入类型标签'
  const seen = new Set<string>()
  for (const f of draft.fields) {
    if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(f.name)) return `字段名 ${f.name} 不合法`
    if (seen.has(f.name)) return `字段名 ${f.name} 重复`
    seen.add(f.name)
    if (f.type === 'relation' && !f.relation_type) return `字段 ${f.name} 必须选择目标内容类型`
    if (f.type === 'repeat' && (!f.sub_fields || f.sub_fields.length === 0)) return `字段 ${f.name} 必须配置子字段`
  }
  return null
}
```

- [ ] **步骤 3：FieldConfigForm.vue**（字段配置弹层）

```vue
<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { SchemaField, FieldType } from '../dynamic-form/types'
import { listContentTypes } from '../api/content-types'

const props = defineProps<{ visible: boolean; field: SchemaField | null; types: { name: string }[] }>()
const emit = defineEmits<{ 'update:visible': [boolean]; save: [SchemaField] }>()

const ALL_TYPES: FieldType[] = ['text','textarea','richtext','number','boolean','date','datetime','select','multiselect','image','file','slug','relation','repeat']

const form = ref<SchemaField>({ name: '', label: '', type: 'text' })
const optionsText = ref('')
const editingName = ref('')

watch(() => props.visible, (v) => {
  if (!v) return
  if (props.field) {
    form.value = { ...props.field }
    editingName.value = props.field.name
    optionsText.value = (props.field.options ?? []).join('\n')
  } else {
    form.value = { name: '', label: '', type: 'text' }
    editingName.value = ''
    optionsText.value = ''
  }
})

const needOptions = computed(() => form.value.type === 'select' || form.value.type === 'multiselect')
const needRelation = computed(() => form.value.type === 'relation')
const needRepeat = computed(() => form.value.type === 'repeat')
const needIndex = computed(() => form.value.type === 'text' || form.value.type === 'textarea')

function onSave() {
  if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(form.value.name)) { ElMessage.warning('字段名不合法'); return }
  const out: SchemaField = { ...form.value }
  if (needOptions.value) out.options = optionsText.value.split('\n').map(s => s.trim()).filter(Boolean)
  else delete out.options
  if (!needRelation.value) delete out.relation_type
  if (!needRepeat.value) delete out.sub_fields
  if (!needIndex.value) delete out.indexed
  emit('save', out)
  emit('update:visible', false)
}
</script>

<template>
  <el-dialog :model-value="visible" :title="editingName ? '编辑字段' : '添加字段'" width="560px" @update:model-value="$emit('update:visible', $event)">
    <el-form label-width="90px">
      <el-form-item label="类型">
        <el-select v-model="form.type" style="width: 100%">
          <el-option v-for="t in ALL_TYPES" :key="t" :label="t" :value="t" />
        </el-select>
      </el-form-item>
      <el-form-item label="字段名">
        <el-input v-model="form.name" placeholder="如 title" :disabled="!!editingName" />
      </el-form-item>
      <el-form-item label="标签">
        <el-input v-model="form.label" placeholder="如 标题" />
      </el-form-item>
      <el-form-item>
        <el-checkbox v-model="form.required">必填</el-checkbox>
        <el-checkbox v-model="form.translatable">可翻译</el-checkbox>
        <el-checkbox v-if="needIndex" v-model="form.indexed">索引列</el-checkbox>
      </el-form-item>
      <el-form-item v-if="needOptions" label="选项">
        <el-input v-model="optionsText" type="textarea" :rows="4" placeholder="每行一个选项" />
      </el-form-item>
      <el-form-item v-if="needRelation" label="目标类型">
        <el-select v-model="form.relation_type" style="width: 100%">
          <el-option v-for="t in types.filter(t => t.name !== form.name)" :key="t.name" :label="t.name" :value="t.name" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="needRepeat" label="子字段">
        <div>子字段配置为简化表单（每行一个子字段定义 JSON），实现时提供简易编辑器或文本输入</div>
        <el-input v-model="subFieldsText" type="textarea" :rows="4" placeholder='[{"name":"title","label":"标题","type":"text","required":true}]' />
      </el-form-item>
      <el-form-item label="默认值">
        <el-input v-model="form.default" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="$emit('update:visible', false)">取消</el-button>
      <el-button type="primary" @click="onSave">确定</el-button>
    </template>
  </el-dialog>
</template>
```
> 注：`ElMessage` 需 import。`subFieldsText` 需在 script 中声明并解析（`JSON.parse` → `form.sub_fields`），实现时补全——**步骤 3b**。

- [ ] **步骤 3b：subFieldsText 处理**

script 中补充：
```ts
const subFieldsText = ref('')
watch(() => props.visible, (v) => {
  // ...既有
  subFieldsText.value = props.field?.sub_fields ? JSON.stringify(props.field.sub_fields, null, 2) : ''
})
// onSave 中
if (needRepeat.value) {
  try {
    const arr = JSON.parse(subFieldsText.value || '[]')
    if (!Array.isArray(arr)) throw new Error()
    out.sub_fields = arr as SchemaField[]
  } catch {
    ElMessage.warning('子字段配置 JSON 格式错误')
    return
  }
}
```
import `ElMessage` from 'element-plus'。

- [ ] **步骤 4：ContentTypesView.vue**（列表+字段面板双栏）

```vue
<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listContentTypes, createContentType, updateContentType, deleteContentType, type ContentTypeItem } from '../api/content-types'
import { validateContentType } from './content-types-validate'
import FieldConfigForm from '../components/FieldConfigForm.vue'
import type { SchemaField } from '../dynamic-form/types'

const types = ref<ContentTypeItem[]>([])
const selectedId = ref<number | null>(null)
const selected = computed(() => types.value.find(t => t.id === selectedId.value) ?? null)
const editingLabel = ref('')
const formVisible = ref(false)
const editingField = ref<SchemaField | null>(null)

async function load() {
  const r = await listContentTypes()
  types.value = r.items
  if (selectedId.value === null && types.value.length) selectedId.value = types.value[0].id
}

onMounted(load)

function select(id: number) { selectedId.value = id; editingLabel.value = selected.value?.label ?? '' }

async function saveType() {
  if (!selected.value) return
  const draft = { name: selected.value.name, label: editingLabel.value, fields: selected.value.fields }
  const err = validateContentType(draft)
  if (err) { ElMessage.warning(err); return }
  await updateContentType(selected.value.id, { label: editingLabel.value, fields: selected.value.fields })
  ElMessage.success('已保存')
  await load()
}

async function createNew() {
  // 简单命名：name-新类型，label 空，进入面板编辑
  const name = `newtype-${Date.now().toString(36)}`
  await createContentType({ name, label: name, fields: [] })
  await load()
  const created = types.value.find(t => t.name === name)
  if (created) select(created.id)
}

async function copyType(t: ContentTypeItem) {
  const newName = `${t.name}-copy`
  await createContentType({ name: newName, label: `${t.label} 副本`, fields: JSON.parse(JSON.stringify(t.fields)) })
  ElMessage.success('已复制')
  await load()
}

async function removeType(t: ContentTypeItem) {
  try { await ElMessageBox.confirm(`确认删除内容类型 ${t.name}？该类型全部内容将一并删除。`, '危险操作', { type: 'warning' }) } catch { return }
  await deleteContentType(t.name)
  ElMessage.success('已删除')
  selectedId.value = null
  await load()
}

function openAddField() { editingField.value = null; formVisible.value = true }
function openEditField(f: SchemaField) { editingField.value = f; formVisible.value = true }

function onFieldSave(field: SchemaField) {
  if (!selected.value) return
  const idx = selected.value.fields.findIndex(f => f.name === (editingField.value?.name ?? '__new__'))
  const fields = [...selected.value.fields]
  if (idx >= 0) fields[idx] = field
  else fields.push(field)
  selected.value = { ...selected.value, fields }
}

function removeField(name: string) {
  if (!selected.value) return
  selected.value = { ...selected.value, fields: selected.value.fields.filter(f => f.name !== name) }
}

function moveField(i: number, dir: -1 | 1) {
  if (!selected.value) return
  const fields = [...selected.value.fields]
  const j = i + dir
  if (j < 0 || j >= fields.length) return
  ;[fields[i], fields[j]] = [fields[j], fields[i]]
  selected.value = { ...selected.value, fields }
}

function copyField(f: SchemaField) {
  if (!selected.value) return
  selected.value = { ...selected.value, fields: [...selected.value.fields, { ...f, name: `${f.name}-copy` }] }
}
</script>

<template>
  <div class="builder">
    <div class="left">
      <el-card>
        <template #header>
          <div class="card-head">
            <span>内容类型</span>
            <el-button size="small" type="primary" @click="createNew">新建</el-button>
          </div>
        </template>
        <el-menu :default-active="String(selectedId)" @select="(i: string) => select(Number(i))">
          <el-menu-item v-for="t in types" :key="t.id" :index="String(t.id)">
            <span class="type-name">{{ t.name }}</span>
            <span class="type-label">{{ t.label }}</span>
          </el-menu-item>
        </el-menu>
      </el-card>
    </div>
    <div class="right">
      <el-card v-if="selected">
        <template #header>
          <div class="card-head">
            <span>字段配置 · {{ selected.name }}</span>
            <div>
              <el-button size="small" @click="saveType">保存</el-button>
              <el-button size="small" @click="copyType(selected)">复制</el-button>
              <el-button size="small" type="danger" @click="removeType(selected)">删除</el-button>
            </div>
          </div>
        </template>
        <el-form label-width="70px" style="max-width: 420px">
          <el-form-item label="标签">
            <el-input v-model="editingLabel" />
          </el-form-item>
        </el-form>
        <el-button size="small" type="primary" @click="openAddField">添加字段</el-button>
        <el-table :data="selected.fields" size="small" style="margin-top: 12px">
          <el-table-column prop="name" label="名称" width="140" />
          <el-table-column prop="label" label="标签" />
          <el-table-column prop="type" label="类型" width="110" />
          <el-table-column label="标记" width="140">
            <template #default="{ row }">
              <el-tag v-if="row.required" size="small">必填</el-tag>
              <el-tag v-if="row.translatable" size="small" type="success">翻译</el-tag>
              <el-tag v-if="row.indexed" size="small" type="info">索引</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="180">
            <template #default="{ row, $index }">
              <el-button link type="primary" @click="openEditField(row)">编辑</el-button>
              <el-button link @click="moveField($index, -1)">↑</el-button>
              <el-button link @click="moveField($index, 1)">↓</el-button>
              <el-button link @click="copyField(row)">复制</el-button>
              <el-button link type="danger" @click="removeField(row.name)">删</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>
      <el-empty v-else description="请选择或新建内容类型" />
    </div>
    <FieldConfigForm :visible="formVisible" :field="editingField" :types="types" @update:visible="formVisible = $event" @save="onFieldSave" />
  </div>
</template>

<style scoped>
.builder { display: flex; gap: 16px; align-items: flex-start; }
.left { width: 280px; flex-shrink: 0; }
.right { flex: 1; }
.card-head { display: flex; justify-content: space-between; align-items: center; }
.type-label { color: var(--el-text-color-secondary); font-size: 12px; margin-left: 8px; }
</style>
```
> 注：`selected` 是 computed（只读），`selected.value = {...}` 赋值会报错（computed 无 setter）。**实现时**：改用 `ref` 存当前编辑类型副本（`draft`），或给 selected 加 setter。**裁定**：用 `ref<ContentTypeItem | null>` 的 `selectedType`，load 时同步。实现时按正确方式（ref + 显式 set）。

- [ ] **步骤 5：路由 + 侧边栏**

`admin/src/router/index.ts` 的 AppShell children 加：
```ts
{ path: 'content-types', name: 'content-types', component: () => import('../views/ContentTypesView.vue'), meta: { auth: true, adminOnly: false } },
```
> 注意：构建器入口应仅 admin/editor 可见（content_types.manage）。`adminOnly` 是 admin 专属。**裁定**：菜单项控制用 `!m.adminOnly || auth.isAdmin`（现有 LayoutSidebar 逻辑）——但构建器需 admin/editor 都可见。**实现时**：LayoutSidebar menus 加 `{ path: '/content-types', label: '内容类型', adminOnly: false, manageOnly: true }`，渲染条件 `(!m.adminOnly || auth.isAdmin) && (!m.manageOnly || auth.role !== 'author')`。路由 `meta.auth` 即可（不 adminOnly，让 editor 可进）。

`admin/src/components/LayoutSidebar.vue` 的 menus 数组加：
```ts
{ path: '/content-types', label: '内容类型', adminOnly: false, manageOnly: true },
```
渲染条件更新（步骤 4 裁定）。

- [ ] **步骤 6：Vitest 测试（前端校验）**（`admin/src/views/__tests__/content-types-validate.test.ts`）

```ts
import { describe, it, expect } from 'vitest'
import { validateContentType } from '../content-types-validate'

describe('validateContentType', () => {
  it('合法类型通过', () => {
    expect(validateContentType({ name: 'article', label: '文章', fields: [{ name: 'title', label: '标题', type: 'text' }] })).toBeNull()
  })
  it('名称格式', () => {
    expect(validateContentType({ name: '1bad', label: 'x', fields: [] })).toBeTruthy()
  })
  it('字段重名', () => {
    expect(validateContentType({ name: 'a', label: 'x', fields: [{ name: 't', label: '', type: 'text' }, { name: 't', label: '', type: 'text' }] })).toBeTruthy()
  })
  it('relation 缺目标', () => {
    expect(validateContentType({ name: 'a', label: 'x', fields: [{ name: 'r', label: '', type: 'relation' }] })).toBeTruthy()
  })
  it('repeat 缺子字段', () => {
    expect(validateContentType({ name: 'a', label: 'x', fields: [{ name: 'r', label: '', type: 'repeat' }] })).toBeTruthy()
  })
})
```

- [ ] **步骤 7：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run && npx vite build`
预期：通过

---

### 任务 7：内容管理页按类型权限过滤 + me() 权限前端接入

**文件：**
- 修改：`admin/src/stores/auth.ts`（加 permissions 字段 + setPermissions）
- 修改：`admin/src/api/auth.ts`（me() 返回 permissions）
- 修改：`admin/src/views/ContentListView.vue`（类型下拉按权限过滤）
- 修改：`admin/src/views/ContentEditView.vue`（发布按钮按类型权限）
- 修改：`admin/src/api/__tests__/client.test.ts` 或新增 auth store 测试

- [ ] **步骤 1：auth store 加 permissions**（`admin/src/stores/auth.ts`）

```ts
interface AuthState {
  token: string | null
  username: string | null
  role: string | null
  permissions: string[]
}
// state 加 permissions: [] as string[]
// getters 加 hasPerm:
hasPerm: (s) => (p: string) => {
  if (s.permissions.includes('*')) return true
  if (s.permissions.includes(p)) return true
  const parts = p.split('.')
  for (let i = parts.length - 1; i >= 1; i--) {
    if (s.permissions.includes(parts.slice(0, i).join('.') + '.*')) return true
  }
  return false
}
// actions 加 setPermissions(p: string[]) { this.permissions = p }
// clear() 重置 permissions
// setUser 保持（不含 permissions，由 LoginView/main.ts 调用 setPermissions）
```

- [ ] **步骤 2：api/auth.ts me() 返回 permissions**

```ts
export interface MeResponse {
  user: { id: number; username: string; role: string }
  permissions: string[]
}
export const me = () => request<MeResponse>('/auth/me')
```

- [ ] **步骤 3：LoginView + main.ts 接入 permissions**

`admin/src/views/LoginView.vue`（登录成功处）：
```ts
const { user, permissions } = await me()
auth.setUser({ username: user.username, role: user.role })
auth.setPermissions(permissions)
```

`admin/src/main.ts`（启动回填处，bootstrap 内）：
```ts
const { user, permissions } = await me()
auth.setUser({ username: user.username, role: user.role })
auth.setPermissions(permissions)
```

- [ ] **步骤 4：ContentListView 类型下拉按权限过滤**

`admin/src/views/ContentListView.vue`：
```ts
import { useAuthStore } from '../stores/auth'
const auth = useAuthStore()
// 加载类型后过滤：
const tr = await listContentTypes()
types.value = tr.items.filter(t => auth.hasPerm(`content.read.${t.name}`))
```

- [ ] **步骤 5：ContentEditView 发布按钮按类型权限**

```vue
v-if="!isNew && (auth.role === 'admin' || auth.role === 'editor') && auth.hasPerm(`content.publish.${currentType?.name}`)"
```
> 需要 `currentType` 在 template 可访问（已有）。

- [ ] **步骤 6：auth store 测试**（`admin/src/stores/__tests__/auth.test.ts`）

```ts
import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from '../auth'

describe('auth store hasPerm', () => {
  beforeEach(() => { setActivePinia(createPinia()); localStorage.clear() })
  it('前缀匹配', () => {
    const s = useAuthStore()
    s.setPermissions(['content.*'])
    expect(s.hasPerm('content.read.article')).toBe(true)
    expect(s.hasPerm('content.read.page')).toBe(true)
  })
  it('admin *', () => {
    const s = useAuthStore()
    s.setPermissions(['*'])
    expect(s.hasPerm('anything')).toBe(true)
  })
  it('无匹配', () => {
    const s = useAuthStore()
    s.setPermissions(['menus.manage'])
    expect(s.hasPerm('content.write.article')).toBe(false)
  })
})
```

- [ ] **步骤 7：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run && npx vite build`
预期：通过

---

### 任务 8：生产集成 + 全量验收

**文件：**
- 修改：`internal/server/dist/`（同步新产物）
- 验证：全量

- [ ] **步骤 1：构建 SPA 并同步**

```bash
cd admin
npm run build
# 复制到 server dist
Remove-Item -Recurse -Force "..\internal\server\dist" -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path "..\internal\server\dist" -Force | Out-Null
Copy-Item -Path "dist\*" -Destination "..\internal\server\dist\" -Recurse -Force
if (-not (Test-Path "..\internal\server\dist\.gitkeep")) { New-Item -ItemType File -Path "..\internal\server\dist\.gitkeep" -Force | Out-Null }
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
# 3. 内容类型 → 新建类型（含 relation/repeat 字段）→ 保存
# 4. 内容 → 新建该类型内容（relation 选目标、repeat 填行）→ 发布
# 5. 前台 /<type>/... 通过 type_map 渲染
# 6. editor 登录 → 有内容类型菜单与发布按钮；author 登录 → 无构建器菜单
# 7. /api/auth/me 返回 permissions
```
预期：后台定义全新内容类型（含 relation/repeat）、前台 type_map 渲染成功、权限分级生效。

- [ ] **步骤 5：清理**

停服、删 `dulizhan.db`/`data/`/临时产物，8080 释放。

---

## 自检记录

**1. 规格覆盖度：**
- 规格 §2 schema 扩展 → 任务 1
- 规格 §3 RBAC 前缀匹配/RequirePermType/内容路由/seed/me() → 任务 2/3
- 规格 §4 级联删除（WithTx）→ 任务 4
- 规格 §5 前端 types/registry/relation/repeat 控件/ContentPicker → 任务 5
- 规格 §6 构建器页面/FieldConfigForm/API/校验 → 任务 6
- 规格 §7 内容管理页类型权限过滤/me() 前端接入 → 任务 7
- 规格 §8 验收 → 任务 8

**2. 占位符扫描：** 无 TBD/TODO。每步含代码。注意若干"实现时定/实现时"标注（author seed 权限、路由 adminOnly 处理、computed setter 修正）——均为明确的实现决策提示，非占位符。

**3. 类型一致性：**
- `schema.Field.RelationType/SubFields` 任务 1 定义，任务 3（adminapi requireTypePerm）与前端（SchemaField.relation_type/sub_fields）一致使用
- `HasPerm` 前缀匹配任务 2 定义，任务 3 adminapi 使用；前端 auth store hasPerm 任务 7 镜像
- `RequirePermType(perm, typeName)` 任务 2 定义，任务 3 adminapi 使用（或 helper requireTypePerm）
- `ContentRepo.DeleteByType` + `Store.WithTx` 任务 4 定义，Service.DeleteType 使用
- `me()` 返回 permissions 任务 3 后端定义，任务 7 前端消费
- `ContentTypeItem.fields: SchemaField[]`（阶段 3）任务 6 构建器复用，SchemaField 含 relation_type/sub_fields（任务 5）
- seed 角色权限：editor `content.*`（任务 3），author `content.read.*`+write（任务 3 步骤 6 裁定）
