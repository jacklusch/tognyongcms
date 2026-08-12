# Dulizhan CMS — 阶段 4：可视化内容类型构建器 设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`（权威设计）、`2026-08-06-phase3-admin-spa-design.md`（阶段 3 基线）
- 基线：阶段 3 已验收通过（SPA 管理端、`/admin` go:embed、动态表单引擎 12 控件）

## 1. 目标与范围

构建**可视化内容类型构建器**，让后台定义全新内容类型、前台主题通过 type_map 渲染（设计文档阶段 4 验收点）。含 relation/repeat 字段类型补全 + 权限点细化。

**包含**：
- 内容类型构建器页面（列表 + 字段面板双栏）
- relation/repeat 字段控件 + 后端 Field 扩展（RelationType/SubFields）
- 类型级权限点（`content.read.<type>` 前缀匹配）+ 字段级 UI 限制
- 内容类型级联删除
- 动态表单引擎完整化（14 种字段类型全控件）

**排除**（阶段 5）：用户/角色管理界面、S3 媒体、主题热重载、PG/MySQL。

## 2. 后端 schema 扩展

### 2.1 Field 结构扩展（`internal/schema/schema.go`）

```go
type Field struct {
	Name         string    `json:"name"`
	Label        string    `json:"label"`
	Type         FieldType `json:"type"`
	Required     bool      `json:"required,omitempty"`
	Indexed      bool      `json:"indexed,omitempty"`
	Translatable bool      `json:"translatable,omitempty"`
	Default      any       `json:"default,omitempty"`
	Options      []string  `json:"options,omitempty"`
	MaxLength    int       `json:"max_length,omitempty"`
	Pattern      string    `json:"pattern,omitempty"`
	Min          *float64  `json:"min,omitempty"`
	Max          *float64  `json:"max,omitempty"`
	RelationType string    `json:"relation_type,omitempty"` // relation: 目标内容类型名
	SubFields    []Field   `json:"sub_fields,omitempty"`    // repeat: 子字段组
}
```

### 2.2 校验器扩展（`internal/schema/validate.go`）

- **ValidateContentType**：relation 字段校验 `RelationType` 非空且目标类型存在于 Registry（配置时）；repeat 字段校验 `SubFields` 非空且逐子字段递归校验（字段名唯一、类型合法）。
- **ValidateDocument**：`TypeRelation` 校验值非空字符串（`content_id`）；`TypeRepeat` 校验 `[]any` 且逐行递归校验子字段（行内值为 map）。
- **IndexField**：仅 text/textarea 可作为索引列（relation/repeat 不参与）。
- 新增递归辅助：`validateSubFields(subFields []Field) error`（复用既有字段级规则）。

### 2.3 新增测试

- `TestValidateContentTypeRelationRepeat`：合法/缺 RelationType/缺 SubFields/非法子字段
- `TestValidateDocumentRelationRepeat`：合法 repeat 数组/缺值/非法子字段值/空 relation

## 3. 后端 RBAC 前缀匹配（权限点细化）

### 3.1 HasPerm 前缀匹配（`internal/auth/service.go`）

```go
// HasPerm 支持类型级前缀匹配：
// content.read.article 命中精确权限、或 content.read.* / content.* / *
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

### 3.2 RequirePermType 中间件（`internal/auth/middleware.go`）

```go
// RequirePermType 按内容类型校验：perm 形如 "content.read"，实际检查 perm+"."+typeName
func RequirePermType(perm, typeName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		us, ok := UserFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
			return
		}
		if !us.HasPerm(perm + "." + typeName) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权限执行此操作"})
			return
		}
		c.Next()
	}
}
```

### 3.3 内容路由类型级校验（`internal/adminapi/content.go`）

- **列表** `GET /api/content`：从 query `type` 取 typeName，handler 内先 `RequirePermType("content.read", typeName)` 判定（复用 `RequirePerm` 后加类型检查）。
- **详情/更新/删除/发布/撤回/翻译**：路径 id → `GetByID` 得 typeName → 判定 `content.<action>.<typeName>`。
- 具体落点：handler 内封装 helper `d.requireTypePerm(c, action, typeName) bool`（失败写 403 返回 false），在各 handler 开头调用。
- 现有 `RequirePerm("content.read")` 等保持（基础权限门禁），类型级检查追加。
- **HandleMe 扩展**：`GET /api/auth/me` 响应增加 `permissions`（角色权限列表，`Role.Permissions` 解析为 `[]string` 或 `us.Perms` 的键集）——供前端 auth store 存权限集、按类型过滤。响应形状：`{user: {id, username, role}, permissions: ["content.read.article", "content.*", ...]}`。

### 3.4 seed 权限更新（`internal/seed/seed.go`）

- editor：权限改为 `content.*` 通配（操作所有类型）+ 既有（content_types.manage/media/menus/settings 等）。
- author：保持 `content.read` + `content.write` + `media.upload`（基础权限，不限定类型——**作者只能操作自己创建的内容**由 service 层 `canManage` 归属校验保证，不依赖类型级）。
- admin：`*` 不变。

### 3.5 新增测试

- `TestHasPermTypePrefix`：精确/`content.*`/`*`/无匹配
- `TestRequirePermType`：中间件 200/403/401
- adminapi 层：author 对 `content.read.article` 可通过、对不存在类型权限 403（按 seed 新权限）

## 4. 内容类型级联删除

### 4.1 后端（`internal/store` + `internal/content`）

- `ContentRepo` 新增 `DeleteByType(ctx, typeName string) error`（DELETE 该类型全部内容）——sqlite 实现 `DELETE FROM content WHERE content_type_id = (SELECT id FROM content_types WHERE name = ?)`。
- `Service.DeleteType` 改造：先 `DeleteByType` 删内容，再 `ContentTypeRepo().Delete` 删类型。**原子性**：给 sqlite.Store 增加事务封装 `WithTx(ctx, fn func(tx Store) error) error`（`Begin`/`Commit`/`Rollback`），`DeleteType` 在事务内执行两步；`SetMaxOpenConns(1)` 已保证串行，事务提供原子性兜底。`WithTx` 接口暴露到 `store.Store`（实现时以最小面为准：仅 `DeleteType` 需要事务，可给 ContentRepo 加 `DeleteByTypeTx` 或 Store 级 WithTx——**定：Store 级 `WithTx(ctx, fn) error`，`DeleteType` 用它组合两步**）。

### 4.2 新增测试

- `TestDeleteTypeCascades`：建类型→建内容→删类型→内容也被删；删不存在的类型→ErrNotFound。

## 5. 前端 types 与动态表单补全

### 5.1 types.ts 扩展（`admin/src/dynamic-form/types.ts`）

```ts
export type FieldType = ... | 'relation' | 'repeat'

export interface SchemaField {
  // ...既有
  relation_type?: string
  sub_fields?: SchemaField[]
}
```

### 5.2 registry 补全（`admin/src/dynamic-form/registry.ts`）

```ts
relation: { component: RelationControl, validate: validateRelation },
repeat: { component: RepeatControl, validate: validateRepeat },
```

校验器：
```ts
export function validateRelation(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  return null // 值 = content_id 字符串，非空即可
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

### 5.3 relation/repeat 控件

**RelationControl.vue**：
- props：`field`（含 `relation_type`）、`modelValue`（content_id 字符串，翻译组键）
- 显示当前值：`ContentPicker` 打开时加载目标类型候选（`listContent({type: relation_type, lang})`）；当前值回显——候选项携带 `content_id` 与标题，若 modelValue 匹配某候选则显示其标题，否则显示 content_id 原文 + "选择内容"按钮允许重选。**回显定位**：`listContent` 的 Entry 已含 `content.content_id` 与 `content.title`，前端在候选数组中 `find(item => item.content.content_id === modelValue)` 即可回显标题，无需额外 API。
- 交互："选择内容"按钮 → `ContentPicker` 弹层 → 单选目标内容 → emit content_id

**ContentPicker.vue**（通用弹层，`admin/src/components/ContentPicker.vue`）：
- 入参：`typeName`、当前值；列出该类型已发布内容（标题/slug），标题搜索（`searchContent`）
- 选中 emit `content_id`（翻译组键）

**RepeatControl.vue**：
- props：`field`（含 `sub_fields`）、`modelValue`（对象数组）
- 行列表：每行 `DynamicForm`（递归渲染 sub_fields，v-model 该行对象）
- "添加行"：追加空对象（子字段默认值）；每行"删除"按钮
- 数组变更 emit 完整数组

### 5.4 新增测试

- Vitest：`validateRelation`/`validateRepeat`（含子字段逐行校验）
- 控件冒烟：`ContentPicker`/`RepeatControl` 渲染（可选，若 @vue/test-utils 成本高可只测 registry）

## 6. 内容类型构建器页面（SPA）

### 6.1 路由与入口

- 新增 `/content-types` 路由（adminOnly/`content_types.manage` 可见——侧边栏菜单加"内容类型"项，仅 admin/editor 显示）
- `ContentTypesView.vue`：列表 + 字段面板双栏

### 6.2 页面结构

```
ContentTypesView.vue
├── 左栏（el-card + 列表）
│   ├── 顶部"新建类型"按钮（POST /api/content-types）
│   ├── 类型列表：name/label/字段数；选中高亮
│   └── 每项操作：编辑（选中）、复制（新类型 name-副本）、删除（级联确认）
├── 右栏（字段编辑面板）
│   ├── 类型信息：label 编辑；name 创建后只读
│   ├── 字段列表：名称/标签/类型/必填/可翻译/索引
│   │   ├── 拖拽排序（vuedraggable）+ 上移/下移
│   │   ├── 复制、删除
│   ├── "添加字段" → FieldConfigForm 弹层
│   └── "保存" → PUT /api/content-types/:id
```

### 6.3 FieldConfigForm.vue（字段配置弹层）

按所选字段类型动态显示配置项：
- 通用：name（校验 `^[a-zA-Z][a-zA-Z0-9_]*$`）、label、required、translatable、indexed（仅 text/textarea）
- text/textarea/slug：max_length、pattern
- number：min、max
- select/multiselect：options（动态标签列表，可增删）
- relation：relation_type（目标内容类型下拉，排除自身）
- repeat：sub_fields（内嵌字段列表，递归用简化版 FieldConfigForm）
- default

**校验**：前端 `validateContentType`（name 格式、字段名唯一、无重复、relation/repeat 配置完整）+ 后端权威（`ValidateContentType` + `checkReservedName`）。

### 6.4 API 适配（`admin/src/api/content-types.ts`）

现有仅 `listContentTypes`。阶段 4 补齐：
- `createContentType({name, label, fields})` → POST /api/content-types（body `{name,label,fields}`，字段数组）
- `updateContentType(id, {label, fields})` → PUT /api/content-types/:id（后端 `HandleContentTypeUpdate` 读 `req.Fields`/`req.Label`，尊重 :id）
- `deleteContentType(name)` → DELETE /api/content-types/:name（后端级联删内容+类型）
- 复制：`createContentType({name: <原name>-副本, label: <原label> 副本, fields: 原 fields})`
- 确认 ContentTypeItem 补 `config` 字段（后端 ContentType 含 config，虽然构建器暂不编辑 config）——以能对齐后端响应为准

### 6.5 新增测试

- Vitest：`validateContentType` 前端校验逻辑（name 格式/字段名唯一/relation 目标存在/repeat 子字段完整）
- 组件冒烟（可选）：ContentTypesView 渲染

## 7. 内容管理页适配类型级权限

- `ContentListView.vue`：类型下拉仅列出当前用户有 `content.read.<type>` 权限的类型（用 me() 返回的权限集过滤；admin 全列）
- `ContentEditView.vue`：发布/撤回按钮条件已有 admin/editor；editor 对无 `content.publish.<type>` 的类型隐藏发布按钮（实现时若权限集含类型级则判断）
- me() 响应扩展：返回 `permissions`（角色权限列表），前端 auth store 存权限集

## 8. 验收标准

1. `go test -count=1 ./...` 全绿 + build/vet/gofmt 干净
2. `cd admin && npx vue-tsc --noEmit` + `npx vitest run` + `npx vite build` 通过
3. 端到端冒烟：登录 → 构建器新建类型（含 relation/repeat 字段）→ 建内容（relation 选目标、repeat 填行）→ 发布 → 前台 type_map 渲染新类型
4. 权限：author 建内容（基础权限）可用、editor 可发布、非 admin 无构建器入口；`content.read.<type>` 前缀匹配生效

## 9. 工作流约定

- 按仓库约定：**不 git 提交**，直接 master 开发，文件级审查
- superpowers SDD（subagent-driven-development）驱动：先 writing-plans 产出实现计划，任务走 TDD
- 前端构建：`cd admin && npm run build` → 同步到 `internal/server/dist/` 后 `go build`
