# Dulizhan CMS — 前台导航菜单接线 设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`（权威设计）、`2026-08-06-phase3-admin-spa-design.md`
- 基线：阶段 5 全部完成；管理端已有菜单管理页（存 menus 表，导航项 `{label,url}` JSON），前台主题 nav.html 硬编码（不消费 menus 表）

## 1. 目标与范围

把管理端配置的导航菜单真正渲染到前台页面：前台按语言加载 menus 表，解析为渲染就绪结构（含语言前缀 URL、二级菜单），主题模板遍历渲染；管理端菜单编辑器升级为可视化树形（支持二级菜单、下拉选目标）。

**包含**：
- 主题层数据模型：`theme.MenuItem`（Label/URL/Children）、`theme.Menu`（Name/Items）、`theme.Data.Menus`
- 后端解析：`handleFrontend` 按语言查菜单 + 回退默认语言，解析 Items JSON 为渲染就绪（语言前缀 URL）
- 模板函数 `menu`：按菜单名取渲染就绪项
- 默认主题 nav.html 遍历渲染 + 二级下拉 + CSS
- 管理端菜单编辑器：可视化树形（label/type/url/children，下拉选目标、添加子项/删除/排序）

**排除**：三级以上菜单（children 只支持一层，YAGNI）；菜单权限控制（前台菜单公开）；主题侧菜单配置模板（YAGNI）。

## 2. 主题层数据模型

### 2.1 `internal/theme/theme.go`

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

// Data 增加 Menus 字段（渲染就绪，含语言前缀 URL）。
type Data struct {
	// ...既有（Site/Lang/Langs/Entry/Items/Total/Page/PerPage/TypeName/Meta）
	Menus []Menu `json:"menus"`
}
```

### 2.2 `internal/store/model.go`（存储层 Items 解析目标）

```go
// MenuItem 存储层导航项（管理端编辑器保存的结构）。
type MenuItem struct {
	Label    string     `json:"label"`
	Type     string     `json:"type"`      // "home" | "type:<name>" | "custom"
	URL      string     `json:"url"`       // custom 时填；home/type 时可空
	Children []MenuItem `json:"children,omitempty"`
}
```

## 3. 后端解析与数据流

### 3.1 `internal/server/frontend.go`

`handleFrontend` 构造 data 时加载菜单：
```go
	data := &theme.Data{
		// ...既有
		Menus: s.loadMenus(c.Request.Context(), lang),
	}
```

加载与解析（按语言查 + 回退默认语言）：
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
> 注：旧格式 `{label,url}`（无 type）按 custom 处理；`i18n.URLPath(lang, path)` 对默认语言不加前缀、其他语言加前缀。`server` 已持有 `s.i18n` 与 `s.store`。

## 4. 模板函数

### 4.1 `internal/theme/funcs.go`

```go
// menuData 按菜单名返回渲染就绪导航项（d 是模板根 Data）。作为 FuncMap 闭包注册。
func menuData(d *Data, name string) []MenuItem {
	for _, m := range d.Menus {
		if m.Name == name {
			return m.Items
		}
	}
	return nil
}
// buildFuncs 注册：
"menu": menuData,
```
> **实现裁定**：`menu` 不访问 Theme 状态（只用 d.Menus），定义为包级函数 `menuData` 并以闭包注册。模板调用 `{{menu $ "main"}}`（`$` 为模板根上下文）。避免 Theme 存 currentData 的并发/重入问题。

## 5. 默认主题导航

### 5.1 `themes/default/templates/partials/nav.html`

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
> 无 `main` 菜单时回退硬编码文章链接（兼容 seed 数据无菜单场景）。

### 5.2 `themes/default/static/css/main.css`（追加）

```css
.nav-item { position: relative; display: inline-block; }
.nav-links .nav-item { margin-left: 1rem; }
.has-children .dropdown { display: none; position: absolute; top: 100%; left: 0; background: #fff; border: 1px solid #eee; box-shadow: 0 2px 8px rgba(0,0,0,.08); min-width: 140px; z-index: 10; }
.has-children:hover .dropdown { display: block; }
.has-children .dropdown a { display: block; padding: .5rem .75rem; color: var(--fg); text-decoration: none; }
.has-children .dropdown a:hover { background: #f5f5f5; }
```

## 6. 管理端菜单编辑器（可视化树形）

### 6.1 `admin/src/views/menu-types.ts`

```ts
export type NavType = 'home' | 'custom' | `type:${string}`

export interface NavItem {
  label: string
  type: NavType
  url: string
  children: NavItem[]
}
```

### 6.2 `admin/src/components/NavItemEditor.vue`（递归组件）

每项渲染：label 输入 + type 下拉（首页/内容类型列表/自定义）+ url 输入（custom 时显示）+ 操作（添加子项/删除/上移/下移）；children 递归自渲染。

```vue
<script setup lang="ts">
import type { NavItem } from '../views/menu-types'
defineProps<{ item: NavItem; types: string[] }>()
defineEmits<{ 'update:item': [NavItem]; remove: []; addChild: [] }>()
</script>
```
- type 下拉选项：`首页(home)`、各内容类型（`type:<name>`）、`自定义(custom)`
- 递归渲染 children（用 `NavItemEditor` 自身）

### 6.3 `admin/src/views/MenuManageView.vue` 改造

- 弹层内导航项编辑从 `formItems` 文本 JSON 改为树形（`NavItemEditor` 列表）
- 保存：`items` 传 `NavItem[]`（`createMenu/updateMenu` body，后端 `menuItems` 已兼容数组）
- 载入编辑：解析 `m.items` JSON 为 `NavItem[]`
- 类型下拉数据源：`listContentTypes()` 返回的类型名

## 7. 测试策略

- **后端 Go**：`resolveMenuItems`/`menuURL`/`loadMenus` 单测（frontend_test 或 server_test）——各 type 的 URL 生成（home/type:/custom/旧格式）、语言前缀、二级 children 递归、无菜单回退、默认语言回退
- **前端**：`NavItemEditor` 递归渲染/增删/排序逻辑、`menu-types` 类型、`MenuManageView` 保存 body 形状（vitest 纯逻辑或组件冒烟）
- **主题渲染**：`theme_test` 验证 `menu` 函数取指定菜单、nav.html 遍历渲染（含二级 dropdown、无菜单回退硬编码）

## 8. 验收

1. `go test -count=1 ./...` 全绿 + build/vet/gofmt 干净
2. `cd admin && npx vue-tsc --noEmit && npx vitest run && npx vite build` 通过
3. 端到端冒烟：管理端建 `main` 菜单（含二级子项、下拉选文章列表/首页/自定义）→ 前台首页 nav 渲染出菜单（含语言前缀 URL、二级下拉）→ 无菜单时回退硬编码文章链接
4. 多语言：zh/en 各建菜单 → 对应语言前台显示对应菜单；缺某语言菜单回退默认语言

## 9. 工作流约定

- 不 git 提交；superpowers SDD 驱动；前端构建同步 `internal/server/dist`
