# 前台导航菜单接线 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 让管理端配置的导航菜单渲染到前台：theme.Data 加 Menus、后端按语言解析为渲染就绪（语言前缀 URL + 二级 children）、默认主题 nav.html 遍历渲染、管理端菜单编辑器升级为可视化树形。

**架构：** 后端 `handleFrontend` 查菜单（当前语言 + 回退默认语言）→ 解析 Items JSON 为 `theme.MenuItem`（含语言前缀 URL、递归 children）→ 存 `theme.Data.Menus`；模板函数 `menuData(d, name)` 取指定菜单；nav.html 遍历渲染 + CSS 二级下拉；管理端 NavItemEditor 递归组件 + MenuManageView 树形化。

**技术栈：** Go（gin、html/template）、Vue 3 + TS + Element Plus。

**前置基线：** 阶段 5 全部完成。规格：`docs/superpowers/specs/2026-08-06-frontend-nav-menu-design.md`。

**环境约束（所有任务遵守）：**
- **不 git 提交**（仓库约定）；**勿运行 `go mod tidy`**
- Go 验证：`go test -count=1 <包> -v`；阶段收尾 `go test -count=1 ./...`
- 前端验证：`cd admin && npx vue-tsc --noEmit && npx vitest run`；改完 SPA 后 `npx vite build` + 同步 `internal/server/dist/`
- 错误消息用中文；后端依赖锁定勿动

**现状关键点：**
- `internal/theme/theme.go`：`Data{Site, Lang, Langs, Entry, Items, Total, Page, PerPage, TypeName, Meta}`（无 Menus）
- `internal/theme/funcs.go`：`pagerURL` 已是 `func(d *Data, page int)` 模式——`menu` 用同样签名
- `internal/server/frontend.go`：`handleFrontend` 构造 data；`s.i18n`/`s.store` 已持有
- `internal/store/model.go`：`Menu{ID, Name, Lang, Items string}`（Items 为 JSON）
- `MenuRepo.ListByLang(ctx, lang)` 已存在
- 管理端 `MenuManageView.vue`：`formItems` 文本 JSON 编辑；`api/menu.ts` 有 listMenus/createMenu/updateMenu/deleteMenu

---

### 任务 1：主题层数据模型（theme.Data.Menus + store.MenuItem）

**文件：**
- 修改：`internal/theme/theme.go`（MenuItem/Menu 类型 + Data.Menus 字段）
- 修改：`internal/store/model.go`（MenuItem 存储模型）
- 测试：无新增（类型定义，编译验证）

- [ ] **步骤 1：theme 类型**（`internal/theme/theme.go`）

在 `Data` 前追加类型定义，`Data` 加字段：
```go
// MenuItem 渲染就绪的导航项。
type MenuItem struct {
	Label    string     `json:"label"`
	URL      string     `json:"url"`
	Children []MenuItem `json:"children,omitempty"`
}

// Menu 一个具名菜单（如主导航 main / 页脚 footer）。
type Menu struct {
	Name  string     `json:"name"`
	Items []MenuItem `json:"items"`
}

type Data struct {
	Site     SiteInfo
	Lang     string
	Langs    []i18n.Lang
	Entry    *content.Entry
	Items    []content.Entry
	Total    int
	Page     int
	PerPage  int
	TypeName string
	Meta     seo.Meta
	Menus    []Menu `json:"menus"`
}
```

- [ ] **步骤 2：store.MenuItem**（`internal/store/model.go`）

```go
// MenuItem 存储层导航项（管理端编辑器保存的结构）。
type MenuItem struct {
	Label    string     `json:"label"`
	Type     string     `json:"type"`      // "home" | "type:<name>" | "custom"
	URL      string     `json:"url"`       // custom 时填；home/type 时可空
	Children []MenuItem `json:"children,omitempty"`
}
```

- [ ] **步骤 3：验证编译**

运行：`go build ./...`；`go vet ./...`；`go test -count=1 ./internal/theme/ ./internal/store/sqlite/`
预期：全绿（类型新增不影响既有）

---

### 任务 2：后端菜单加载与解析（frontend.go）

**文件：**
- 修改：`internal/server/frontend.go`（loadMenus/fetchMenus/resolveMenuItems/mapMenuItems/menuURL + handleFrontend 填充）
- 修改：`internal/server/frontend_test.go`（补菜单渲染测试）

- [ ] **步骤 1：编写失败测试**（追加到 `internal/server/frontend_test.go`）

```go
func TestFrontendMenus(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"

	ctx := context.Background()
	// 建 main 菜单（zh）：home + type:article + custom 二级
	items := `[{"label":"首页","type":"home","url":""},{"label":"文章","type":"type:article","url":"","children":[{"label":"关于","type":"custom","url":"/page/about"}]}]`
	if err := srv.store.MenuRepo().Create(ctx, &store.Menu{Name: "main", Lang: "zh", Items: items}); err != nil {
		t.Fatal(err)
	}

	code, body := get(t, srv, "/")
	if code != http.StatusOK {
		t.Fatalf("首页 = %d", code)
	}
	// 渲染出菜单：首页链接、文章列表链接（带语言前缀）、二级下拉
	if !strings.Contains(body, `>首页</a>`) {
		t.Errorf("缺首页导航: %s", body)
	}
	if !strings.Contains(body, `/article`) {
		t.Errorf("缺文章导航: %s", body)
	}
	if !strings.Contains(body, `/page/about`) {
		t.Errorf("缺二级导航: %s", body)
	}
}
```
> 需确认 `buildTestServer` 的 `srv.store` 可访问（Server 结构有 `store store.Store`——**检查**：`srv.store` 是未导出字段，测试同包 server 可访问 ✓）。import `store` 包。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/server/ -run TestFrontendMenus -v`
预期：FAIL（body 无菜单链接——nav.html 还是硬编码）

- [ ] **步骤 3：实现后端解析**（`internal/server/frontend.go`）

`handleFrontend` 构造 data 时填充：
```go
	data := &theme.Data{
		Site: theme.SiteInfo{
			Name: s.cfg.Site.Name, URL: s.cfg.Site.URL, Description: s.cfg.Site.Description,
		},
		Lang:    lang,
		Langs:   s.i18n.All(),
		PerPage: defaultPerPage,
		Menus:   s.loadMenus(c.Request.Context(), lang),
	}
```

追加方法（需 import `encoding/json`、`strings`——检查 frontend.go 现有 import）：
```go
// loadMenus 查当前语言菜单，回退默认语言；解析为渲染就绪。
func (s *Server) loadMenus(ctx context.Context, lang string) []theme.Menu {
	menus := s.fetchMenus(ctx, lang)
	if len(menus) == 0 && lang != s.i18n.Default() {
		menus = s.fetchMenus(ctx, s.i18n.Default())
	}
	if len(menus) == 0 {
		return nil
	}
	out := make([]theme.Menu, 0, len(menus))
	for _, m := range menus {
		out = append(out, theme.Menu{Name: m.Name, Items: s.resolveMenuItems(m.Items, lang)})
	}
	return out
}

func (s *Server) fetchMenus(ctx context.Context, lang string) []store.Menu {
	items, err := s.store.MenuRepo().ListByLang(ctx, lang)
	if err != nil {
		return nil
	}
	return items
}

// resolveMenuItems 把存储 Items JSON 解析为渲染就绪（含语言前缀 URL）。
func (s *Server) resolveMenuItems(raw string, lang string) []theme.MenuItem {
	var items []store.MenuItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return s.mapMenuItems(items, lang)
}

func (s *Server) mapMenuItems(items []store.MenuItem, lang string) []theme.MenuItem {
	out := make([]theme.MenuItem, 0, len(items))
	for _, it := range items {
		mi := theme.MenuItem{Label: it.Label, URL: s.menuURL(it, lang)}
		if len(it.Children) > 0 {
			mi.Children = s.mapMenuItems(it.Children, lang)
		}
		out = append(out, mi)
	}
	return out
}

// menuURL 根据 type 生成带语言前缀的 URL。
func (s *Server) menuURL(it store.MenuItem, lang string) string {
	switch {
	case it.Type == "home":
		return s.i18n.URLPath(lang, "/")
	case strings.HasPrefix(it.Type, "type:"):
		typeName := strings.TrimPrefix(it.Type, "type:")
		return s.i18n.URLPath(lang, "/"+typeName)
	default: // custom 或旧格式 {label,url}
		if it.URL == "" || it.URL == "/" {
			return s.i18n.URLPath(lang, "/")
		}
		return s.i18n.URLPath(lang, it.URL)
	}
}
```
> `i18n.URLPath(lang, path)`：默认语言不加前缀、其他语言加前缀。检查 frontend.go 现有 import（`encoding/json`/`strings` 可能需新增）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/server/ -run TestFrontendMenus -v`
预期：PASS（若 nav.html 还是硬编码会失败——**本任务与任务 3 联动**：实现后端解析后，nav.html 仍需遍历渲染。**实现时**：本任务先完成解析 + 测试断言 body 含菜单链接——若 nav.html 未改则 FAIL。**调整**：任务 3 改 nav.html 后本测试转绿。实现顺序：任务 2 写测试（FAIL）→ 任务 3 改模板（GREEN）。或任务 2 步骤 3 同时把 nav.html 改了。**裁定**：任务 2 只做后端解析 + 测试，任务 3 改模板后测试转绿——两任务联动，任务 2 步骤 4 的"验证通过"以任务 3 完成后为准。）

- [ ] **步骤 5：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿（TestFrontendMenus 可能在 nav.html 未改前失败——见步骤 4 说明）

---

### 任务 3：模板函数 menu + 默认主题 nav.html + CSS

**文件：**
- 修改：`internal/theme/funcs.go`（menuData + 注册）
- 修改：`internal/theme/theme_test.go`（menu 函数测试）
- 修改：`themes/default/templates/partials/nav.html`（遍历渲染）
- 修改：`themes/default/static/css/main.css`（二级下拉样式）

- [ ] **步骤 1：menuData 函数**（`internal/theme/funcs.go`）

```go
// menuData 按菜单名返回渲染就绪导航项（d 是模板根 Data）。
func menuData(d *Data, name string) []MenuItem {
	for _, m := range d.Menus {
		if m.Name == name {
			return m.Items
		}
	}
	return nil
}
```
`buildFuncs` 注册：
```go
		"menu": menuData,
```

- [ ] **步骤 2：theme 测试**（追加到 `internal/theme/theme_test.go`）

```go
func TestMenuData(t *testing.T) {
	d := &Data{Menus: []Menu{
		{Name: "main", Items: []MenuItem{{Label: "首页", URL: "/"}}},
	}}
	if got := menuData(d, "main"); len(got) != 1 || got[0].Label != "首页" {
		t.Errorf("menu main = %+v", got)
	}
	if got := menuData(d, "nope"); got != nil {
		t.Errorf("缺名菜单应为 nil, got %+v", got)
	}
}
```

- [ ] **步骤 3：nav.html 遍历渲染**（`themes/default/templates/partials/nav.html`）

```html
{{define "nav"}}
<nav class="site-nav">
  <a class="nav-brand" href="{{home $.Lang}}">{{.Site.Name}}</a>
  <div class="nav-links">
    {{$main := (menu $ "main")}}
    {{if $main}}
      {{range $main}}
        {{if .Children}}
          <div class="nav-item has-children">
            <a href="{{.URL}}">{{.Label}}</a>
            <div class="dropdown">
              {{range .Children}}<a href="{{.URL}}">{{.Label}}</a>{{end}}
            </div>
          </div>
        {{else}}
          <a href="{{.URL}}">{{.Label}}</a>
        {{end}}
      {{end}}
    {{else}}
      <a href="{{typeURL $.Lang "article"}}">{{t $.Lang "articles"}}</a>
    {{end}}
  </div>
</nav>
{{end}}
```

- [ ] **步骤 4：CSS 二级下拉**（`themes/default/static/css/main.css` 追加）

```css
.nav-item { position: relative; display: inline-block; }
.nav-links .nav-item { margin-left: 1rem; }
.has-children .dropdown { display: none; position: absolute; top: 100%; left: 0; background: #fff; border: 1px solid #eee; box-shadow: 0 2px 8px rgba(0,0,0,.08); min-width: 140px; z-index: 10; }
.has-children:hover .dropdown { display: block; }
.has-children .dropdown a { display: block; padding: .5rem .75rem; color: var(--fg); text-decoration: none; }
.has-children .dropdown a:hover { background: #f5f5f5; }
```

- [ ] **步骤 5：验证**

运行：`go test -count=1 ./internal/theme/ ./internal/server/ -run 'TestMenuData|TestFrontendMenus' -v`
预期：全部 PASS（TestFrontendMenus 现在转绿）

- [ ] **步骤 6：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 4：前端菜单类型 + NavItemEditor 递归组件

**文件：**
- 创建：`admin/src/views/menu-types.ts`
- 创建：`admin/src/components/NavItemEditor.vue`
- 创建：`admin/src/views/__tests__/menu-nav.test.ts`

- [ ] **步骤 1：menu-types.ts**（`admin/src/views/menu-types.ts`）

```ts
export type NavType = 'home' | 'custom' | `type:${string}`

export interface NavItem {
  label: string
  type: NavType
  url: string
  children: NavItem[]
}

export function emptyNavItem(): NavItem {
  return { label: '', type: 'home', url: '', children: [] }
}
```

- [ ] **步骤 2：NavItemEditor.vue**（`admin/src/components/NavItemEditor.vue`）

```vue
<script setup lang="ts">
import { ref } from 'vue'
import type { NavItem } from '../views/menu-types'

const props = defineProps<{ item: NavItem; types: string[] }>()
const emit = defineEmits<{
  'update:item': [NavItem]
  remove: []
  addChild: []
}>()

function updateItem(patch: Partial<NavItem>) {
  emit('update:item', { ...props.item, ...patch })
}

function updateChild(i: number, child: NavItem) {
  const children = [...props.item.children]
  children[i] = child
  emit('update:item', { ...props.item, children })
}

function removeChild(i: number) {
  const children = props.item.children.filter((_, idx) => idx !== i)
  emit('update:item', { ...props.item, children })
}

function addChild() {
  emit('update:item', { ...props.item, children: [...props.item.children, emptyNavItem()] })
}
</script>

<template>
  <div class="nav-editor">
    <div class="nav-row">
      <el-input v-model="item.label" placeholder="显示文字" class="w-120" @update:model-value="updateItem({ label: $event })" />
      <el-select :model-value="item.type" class="w-140" @update:model-value="updateItem({ type: $event as NavItem['type'] })">
        <el-option label="首页" value="home" />
        <el-option v-for="t in types" :key="t" :label="t" :value="`type:${t}`" />
        <el-option label="自定义" value="custom" />
      </el-select>
      <el-input v-if="item.type === 'custom'" v-model="item.url" placeholder="路径如 /page/about" class="w-180" @update:model-value="updateItem({ url: $event })" />
      <el-button link type="primary" @click="addChild">+子项</el-button>
      <el-button link type="danger" @click="$emit('remove')">删</el-button>
    </div>
    <div v-if="item.children.length" class="nav-children">
      <NavItemEditor
        v-for="(child, i) in item.children"
        :key="i"
        :item="child"
        :types="types"
        @update:item="updateChild(i, $event)"
        @remove="removeChild(i)"
      />
    </div>
  </div>
</template>

<style scoped>
.nav-row { display: flex; gap: 8px; align-items: center; padding: 4px 0; }
.nav-children { margin-left: 24px; border-left: 1px dashed #dcdfe6; padding-left: 8px; }
.w-120 { width: 120px; }
.w-140 { width: 140px; }
.w-180 { width: 180px; }
</style>
```
> 注意：递归组件 `NavItemEditor` 自己引用自己（Vue SFC 支持自递归）。**`addChild` 在组件内部直接调用**（不 emit，见 `addChild()` 是 script 内函数，模板 `+子项` 按钮直接 `@click="addChild"`）——每层自己管理自己的 children，无需冒泡到顶层。顶层 MenuManageView 只需 `@addTopItem`（加顶层项，任务 5）。

- [ ] **步骤 3：Vitest**（`admin/src/views/__tests__/menu-nav.test.ts`）

```ts
import { describe, it, expect } from 'vitest'
import { emptyNavItem } from '../menu-types'

describe('menu types', () => {
  it('emptyNavItem 结构', () => {
    const n = emptyNavItem()
    expect(n).toEqual({ label: '', type: 'home', url: '', children: [] })
  })
})
```

- [ ] **步骤 4：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run`
预期：通过

---

### 任务 5：MenuManageView 树形化 + 集成

**文件：**
- 修改：`admin/src/views/MenuManageView.vue`（弹层导航项树形编辑）
- 修改：`admin/src/api/menu.ts`（若需 items 类型）
- 修改：`admin/src/views/__tests__/users-roles.test.ts` 或新增 `menu-view.test.ts`

- [ ] **步骤 1：MenuManageView 改造**

弹层内导航项编辑从 `formItems` 文本改为 `NavItemEditor` 列表：

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listMenus, createMenu, updateMenu, deleteMenu, type MenuItem } from '../api/menu'
import { fetchMeta, type MetaData } from '../api/meta'
import { listContentTypes } from '../api/content-types'
import NavItemEditor from '../components/NavItemEditor.vue'
import { emptyNavItem, type NavItem } from './menu-types'

const meta = ref<MetaData | null>(null)
const lang = ref('zh')
const items = ref<MenuItem[]>([])
const editing = ref<MenuItem | null>(null)
const dialogVisible = ref(false)
const formName = ref('')
const navItems = ref<NavItem[]>([])
const types = ref<string[]>([])

function parseItems(raw: string): NavItem[] {
  try { const arr = JSON.parse(raw || '[]'); return Array.isArray(arr) ? arr as NavItem[] : [] } catch { return [] }
}

async function load() {
  const r = await listMenus(lang.value)
  items.value = r.items
}

onMounted(async () => {
  meta.value = await fetchMeta()
  lang.value = meta.value.default_lang
  const tr = await listContentTypes()
  types.value = tr.items.map(t => t.name)
  await load()
})

function openCreate() {
  editing.value = null
  formName.value = ''
  navItems.value = [emptyNavItem()]
  dialogVisible.value = true
}

function openEdit(m: MenuItem) {
  editing.value = m
  formName.value = m.name
  navItems.value = parseItems(m.items)
  dialogVisible.value = true
}

function updateNavItem(i: number, item: NavItem) {
  const next = [...navItems.value]
  next[i] = item
  navItems.value = next
}

function removeNavItem(i: number) {
  navItems.value = navItems.value.filter((_, idx) => idx !== i)
}

function addTopItem() {
  navItems.value = [...navItems.value, emptyNavItem()]
}

async function save() {
  if (!formName.value) { ElMessage.warning('请输入菜单名称'); return }
  const payload = { name: formName.value, lang: lang.value, items: navItems.value }
  if (editing.value) await updateMenu(editing.value.id, payload)
  else await createMenu(payload)
  ElMessage.success('已保存')
  dialogVisible.value = false
  await load()
}
</script>
<!-- 模板：弹层内用 NavItemEditor 列表替换 formItems 文本域 -->
```

模板弹层部分：
```html
<el-dialog v-model="dialogVisible" :title="editing ? '编辑菜单' : '新建菜单'" width="640px">
  <el-form label-width="80px">
    <el-form-item label="名称"><el-input v-model="formName" /></el-form-item>
    <el-form-item label="导航项">
      <div class="nav-list">
        <NavItemEditor
          v-for="(it, i) in navItems"
          :key="i"
          :item="it"
          :types="types"
          @update:item="updateNavItem(i, $event)"
          @remove="removeNavItem(i)"
        />
        <el-button size="small" @click="addTopItem">添加导航项</el-button>
      </div>
    </el-form-item>
  </el-form>
  <template #footer>
    <el-button @click="dialogVisible = false">取消</el-button>
    <el-button type="primary" @click="save">保存</el-button>
  </template>
</el-dialog>
```
> 移除 `formItems` ref 与 `parseItems` 旧用法；`menuReq.Items` 后端 `menuItems` 已接受数组（JSON 序列化）。

- [ ] **步骤 2：api/menu.ts**（如需确认 items 类型）

`MenuItem.items` 为 `string`（后端返回）。`createMenu/updateMenu` body 的 items 传 `NavItem[]`——`menuItems(v any)` 后端已 `json.Marshal` 数组，兼容。无需改 api/menu.ts 类型（items 仍 string）。

- [ ] **步骤 3：验证**

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
# 3. 菜单管理：新建 main 菜单（zh），导航项用树形编辑器：首页 + 文章(type:article) + 自定义/关于，给"文章"加二级子项
# 4. 前台 / 首页：nav 渲染出菜单（含语言前缀 URL、二级下拉）
# 5. en 语言建 main 菜单 → /en/ 显示英文菜单；无 en 菜单时回退 zh
# 6. 删除 main 菜单 → 前台回退硬编码文章链接
```
预期：管理端配置的菜单在前台渲染，含二级下拉、语言前缀、回退逻辑。

- [ ] **步骤 5：清理**

停服、删 `dulizhan.db`/`data/`/临时产物，8080 释放。

---

## 自检记录

**1. 规格覆盖度：**
- 规格 §2 theme 数据模型 → 任务 1
- 规格 §3 后端解析 → 任务 2
- 规格 §4 模板函数 menu → 任务 3
- 规格 §5 默认主题 nav.html + CSS → 任务 3
- 规格 §6 管理端编辑器 → 任务 4/5
- 规格 §7 测试 → 各任务
- 规格 §8 验收 → 任务 6

**2. 占位符扫描：** 无 TBD/TODO。每步含代码。任务 2 步骤 4 的"联动说明"（TestFrontendMenus 需任务 3 改模板后转绿）是明确的顺序提示，非占位符。

**3. 类型一致性：**
- `theme.MenuItem{Label, URL, Children}` / `theme.Menu{Name, Items}` / `theme.Data.Menus` 任务 1 定义，任务 2/3 使用
- `store.MenuItem{Label, Type, URL, Children}` 任务 1 定义，任务 2 resolveMenuItems/menuURL 使用
- `menuData(d *Data, name string)` 任务 3 定义，nav.html `{{menu $ "main"}}` 一致
- `NavItem{label, type, url, children}` 任务 4 定义，任务 5 MenuManageView 使用
- `menuURL` 的 `type:` 前缀与前端 `NavType` 的 `type:${string}` 一致
- NavItemEditor 递归组件内部 `addChild()` 直接调用（不 emit）——任务 4 步骤 2 裁定，任务 5 不监听 add-child
