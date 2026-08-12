# 主题开发

主题是前台所有页面的渲染模板（Go `html/template` 语法），由后台"内容/分类/菜单/媒体"等数据驱动。本文档从模板结构、可用标签（函数）、数据字段三方面给出完整参考，每个标签附可复制代码片段，并标注与后台管理菜单的对应关系。

## 目录结构

```
themes/<name>/
├── theme.yaml            # 主题清单
├── templates/            # 模板（index/list/single 等）
│   └── partials/         # 公共片段（nav/footer）
├── static/               # CSS/JS/图片
└── locales/              # zh.yaml / en.yaml 主题文案
```

## 模板渲染约定

- 每个页面模板定义两个 block：`{{define "head"}}`（放在 `<head>` 内）与 `{{define "body"}}`（放在 `<body>` 内）；`base.html` 统一包裹 HTML 骨架（含 SEO meta）。
- 模板根数据即 `theme.Data`（见"数据字段"节），顶层用 `.` 引用，如 `.Site.Name`。
- 在 `range` 循环内访问外层根数据需用 `$`（如 `$.Lang`）或用 `{{$main := ...}}` 绑定变量。
- 修改模板后热重载：config `server.debug: true`（mtime 检测）；否则重启生效。

### theme.yaml 必填项

```yaml
name: default
version: 1.0.0
templates:
  index: templates/index.html      # 首页
  list: templates/list.html        # 列表/分类归档
  single: templates/single.html    # 详情
partials:
  - templates/partials/nav.html    # 公共片段（可在任意模板 {{template "nav" .}} 引用）
  - templates/partials/footer.html
type_map:                          # 内容类型 → 模板 key（后台"内容类型"菜单可新建类型）
  article: single
  page: single
fields:                            # 允许的字段类型清单（后台构建器用）
  - text
  - textarea
  - richtext
  - image
  - slug
  - date
  - datetime
  - boolean
  - number
  - select
  - multiselect
locales:
  zh: locales/zh.yaml
  en: locales/en.yaml
```

---

## 模板标签（函数）速查

| 标签 | 作用 | 后台对应 |
|---|---|---|
| `t` | 主题文案翻译 | 主题 locales/ 文件 |
| `url` | 内容详情 URL（带语言前缀） | 内容管理 |
| `typeURL` | 内容类型列表 URL | 内容类型 |
| `home` | 首页 URL | — |
| `menu` / `firstMenu` | 导航菜单渲染 | 菜单管理 |
| `asset` | 主题静态资源 URL | — |
| `media` | 媒体 URL 解析 | 媒体库 |
| `raw` | 输出原始 HTML（富文本） | 内容正文 |
| `pages` / `pagerURL` | 分页 | 内容列表 |
| `url`（对 SubCategories） | 分类归档 URL | 分类管理 |

---

## 1. 文案翻译 `t` —— 对应主题 locales/

签名：`t "语言" "key"`，返回 `locales/<lang>.yaml` 中 key 的值。用于界面固定文案，如"首页/文章/最新发布"。

**locales/zh.yaml**
```yaml
home: 首页
articles: 文章
latest: 最新发布
no_content: 暂无内容
```

**模板用法**
```html
<h2>{{t $.Lang "latest"}}</h2>
<p>{{t $.Lang "no_content"}}</p>
```
`$.Lang` 是当前访问语言（zh/en），自动命中对应语言文件。若要新增文案，直接往 locales 两个文件加同 key。

---

## 2. 内容链接 `url` / `typeURL` / `home` —— 对应"内容管理""内容类型"

**`url "语言" 条目`** —— 内容详情页 URL。第二参是 `range` 循环里的条目（`content.Entry`），会自动拼 `/{类型}/{slug}` 并带语言前缀。

**首页列表（`index.html`，对应后台"内容管理"发布的内容）**
```html
{{range .Items}}
<li class="entry-item">
  {{if .Fields.cover}}<img src="{{media .Fields.cover}}" class="entry-thumb" alt="">{{end}}
  <a href="{{url $.Lang .}}">{{.Content.Title}}</a>
  {{if .Content.PublishedAt}}<time>{{.Content.PublishedAt}}</time>{{end}}
</li>
{{end}}
```

**`typeURL "语言" "类型名"`** —— 内容类型列表页 URL（如 `/article`），对应后台"内容类型"里定义的任何类型。

```html
<a href="{{typeURL $.Lang "article"}}">{{t $.Lang "articles"}}</a>
```

**`home "语言"`** —— 首页 URL。

```html
<a class="nav-brand" href="{{home $.Lang}}">{{.Site.Name}}</a>
```

---

## 3. 导航菜单 `menu` / `firstMenu` —— 对应后台"菜单管理"

后台"菜单管理"（`/menus`）创建具名菜单（如 `main`、`footer`），每项类型为 `home` / `custom` / `type:<name>`，可嵌套子项（下拉）。前台用以下两个标签渲染：

- `firstMenu .` —— 取第一个有导航项的菜单（主导航通常用它，不依赖菜单名）
- `menu . "菜单名"` —— 按名字精确取（如页脚 `footer`）

**主导航（带下拉，`nav.html`）**
```html
{{define "nav"}}
<nav class="site-nav">
  <a class="nav-brand" href="{{home $.Lang}}">{{.Site.Name}}</a>
  <div class="nav-links">
    {{$main := (firstMenu $)}}
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

**页脚（按名取，`footer.html`）**
```html
{{define "footer"}}
<footer class="site-footer">
  <p>© {{.Site.Name}} · <a href="{{home $.Lang}}">{{t $.Lang "home"}}</a></p>
  <nav>
    {{range menu $ "footer"}}<a href="{{.URL}}">{{.Label}}</a>{{end}}
  </nav>
</footer>
{{end}}
```

导航项字段：`.Label`（文案）、`.URL`（链接，后台按类型自动算好）、`.Children`（子项数组）。后台菜单管理里配置的任何类型项（首页/自定义链接/文章列表/分类页）都会渲染成对应 URL。

---

## 4. 静态资源 `asset` 与媒体 `media`

**`asset "路径"`** —— 主题 `static/` 目录下资源 URL（自动带主题名前缀）。
```html
{{define "head"}}
<link rel="stylesheet" href="{{asset "/css/main.css"}}">
{{end}}
```

**`media 值`** —— 解析媒体 URL（封面图等）。传字段值；为空或相对路径时返回空串（安全）。
```html
{{if .Fields.cover}}<img src="{{media .Fields.cover}}" alt="">{{end}}
```
对应后台"媒体库"上传的文件（`/media/xxx`）与外部图床 URL。

---

## 5. 富文本 `raw` —— 对应内容正文

**`raw 值`** —— 输出字段原始 HTML（仅用于 `richtext` 字段；其他字段不要用，避免 XSS）。内容编辑器（后台"内容管理"→ 新建/编辑 → 富文本字段）写入的 HTML 由此渲染。

```html
{{range $name, $field := .Entry.Fields}}
  {{if eq $name "content"}}
    <div class="entry-content">{{raw $field}}</div>
  {{end}}
{{end}}
```

---

## 6. 分页 `pages` / `pagerURL`

- `pages 总数 每页数` —— 页码数组（`[1 2 3 ...]`）
- `pagerURL 数据 页码` —— 对应页码的 URL

```html
{{if ne $.TypeName "category"}}   {{/* 分类归档隐藏分页（聚合不分页） */}}
<nav class="pager">
  {{range pages $.Total $.PerPage}}
    {{if eq . $.Page}}<span class="current">{{.}}</span>{{else}}<a href="{{pagerURL $ .}}">{{.}}</a>{{end}}
  {{end}}
</nav>
{{end}}
```

---

## 7. 分类 `SubCategories` / `EntryCategory` / `EntryCategoryURL` —— 对应后台"分类管理"

后台"分类管理"（`/categories`）可建**任意层级**分类树（产品 → 斩拌机/香肠机/拌馅机）。前台分类归档路径 `/category/<slug>` 支持多段（`/category/products/chopper`）。

### 列表/归档页（`list.html`）：子分类导航

`.SubCategories` 是当前分类的**直接子分类**数组，每项 `.Name` / `.URL`。归档页在内容列表上方渲染子分类导航：

```html
{{if .SubCategories}}
<div class="sub-categories">
  {{range .SubCategories}}<a class="sub-cat" href="{{.URL}}">{{.Name}}</a>{{end}}
</div>
{{end}}
```

**聚合规则**：访问父分类页（`/category/products`）会递归聚合所有子孙分类的内容（产品 + 斩拌机 + 香肠机 的文章都列出）；子分类页只聚其自身与更下级的。

### 详情页（`single.html`）：分类 chip

`.EntryCategory` 是当前内容所属分类名；`.EntryCategoryURL` 是该分类归档页 URL。两者搭配渲染可点击的分类标签：

```html
{{if .EntryCategory}}
<div class="entry-category">分类：
  {{if .EntryCategoryURL}}<a href="{{.EntryCategoryURL}}">{{.EntryCategory}}</a>{{else}}{{.EntryCategory}}{{end}}
</div>
{{end}}
```

---

## 数据（Data 结构）完整参考

模板根 `.` 是 `theme.Data`，字段如下：

### 站点（对应后台"设置"）
| 字段 | 说明 |
|---|---|
| `.Site.Name` | 站点名（config `site.name`） |
| `.Site.URL` | 站点 URL（config `site.url`） |
| `.Site.Description` | 站点描述 |

### 语言
| 字段 | 说明 |
|---|---|
| `.Lang` | 当前访问语言代码（zh/en） |
| `.Langs` | 全部可用语言数组（`{Code, Name}`），用于语言切换器 |

### 内容条目（对应后台"内容管理"）
| 字段 | 说明 |
|---|---|
| `.Entry` | 详情页当前内容（`single.html`） |
| `.Entry.Content.ID` | 行 id |
| `.Entry.Content.ContentID` | 翻译组 id（多语言同组共享） |
| `.Entry.Content.Slug` | 别名 |
| `.Entry.Content.Title` | 标题 |
| `.Entry.Content.Lang` | 语言 |
| `.Entry.Content.Status` | 状态（draft/published） |
| `.Entry.Content.PublishedAt` | 发布时间（可空） |
| `.Entry.Content.CreatedAt` / `.UpdatedAt` | 创建/更新时间 |
| `.Entry.TypeName` | 内容类型名 |
| `.Entry.Fields.<name>` | 自定义字段（后台内容类型构建器定义的字段名） |
| `.Items` | 列表页条目数组（首页最新、列表页、分类归档） |
| `.Total` / `.Page` / `.PerPage` | 总数 / 当前页 / 每页数 |
| `.TypeName` | 当前页面类型（`article`/`page`/`category`），可用于条件渲染 |

### 分类（对应后台"分类管理"）
| 字段 | 说明 |
|---|---|
| `.EntryCategory` | 详情内容所属分类名 |
| `.EntryCategoryURL` | 该分类归档 URL |
| `.SubCategories` | 当前分类的直接子分类（`[]CategoryInfo{ID, Name, Slug, URL}`） |

### 菜单（对应后台"菜单管理"）
| 字段 | 说明 |
|---|---|
| `.Menus` | 全部菜单（一般用 `menu`/`firstMenu` 标签取，不必直接访问） |

### SEO meta（`base.html` 自动注入，一般无需手写）
| 字段 | 说明 |
|---|---|
| `.Meta.Title` | `<title>` |
| `.Meta.Description` | meta description |
| `.Meta.Canonical` | canonical 链接 |
| `.Meta.OGTags` | Open Graph 属性 map |
| `.Meta.HrefLangs` | hreflang 数组 |
| `.Meta.JSONLDScript` | JSON-LD 完整 `<script>` 片段（base.html 直接输出） |

---

## 完整示例：默认主题三个模板

### index.html（首页，对应后台"内容管理"最近内容 + "菜单管理"导航）
```html
{{define "head"}}
<link rel="stylesheet" href="{{asset "/css/main.css"}}">
{{end}}
{{define "body"}}
{{template "nav" .}}
<main class="container">
  <h1 class="site-title">{{.Site.Name}}</h1>
  <p class="site-desc">{{.Site.Description}}</p>
  <section class="latest">
    <h2>{{t $.Lang "latest"}}</h2>
    {{if .Items}}
    <ul class="entry-list">
      {{range .Items}}
      <li class="entry-item">
        {{if .Fields.cover}}<img src="{{media .Fields.cover}}" class="entry-thumb" alt="">{{end}}
        <a href="{{url $.Lang .}}">{{.Content.Title}}</a>
        {{if .Content.PublishedAt}}<time>{{.Content.PublishedAt}}</time>{{end}}
      </li>
      {{end}}
    </ul>
    {{else}}
    <p>{{t $.Lang "no_content"}}</p>
    {{end}}
  </section>
</main>
{{template "footer" .}}
{{end}}
```

### list.html（列表/分类归档，对应后台"分类管理"子分类 + "内容管理"）
```html
{{define "head"}}
<link rel="stylesheet" href="{{asset "/css/main.css"}}">
{{end}}
{{define "body"}}
{{template "nav" .}}
<main class="container">
  <h1>{{t $.Lang "articles"}}</h1>
  {{if .SubCategories}}
  <div class="sub-categories">
    {{range .SubCategories}}<a class="sub-cat" href="{{.URL}}">{{.Name}}</a>{{end}}
  </div>
  {{end}}
  <ul class="entry-list">
    {{range .Items}}
    <li class="entry-item">
      {{if .Fields.cover}}<img src="{{media .Fields.cover}}" class="entry-thumb" alt="">{{end}}
      <a href="{{url $.Lang .}}">{{.Content.Title}}</a>
      {{if .Content.PublishedAt}}<time>{{.Content.PublishedAt}}</time>{{end}}
    </li>
    {{end}}
  </ul>
  {{if ne $.TypeName "category"}}
  <nav class="pager">
    {{range pages $.Total $.PerPage}}
      {{if eq . $.Page}}<span class="current">{{.}}</span>{{else}}<a href="{{pagerURL $ .}}">{{.}}</a>{{end}}
    {{end}}
  </nav>
  {{end}}
</main>
{{template "footer" .}}
{{end}}
```

### single.html（详情，对应后台"内容管理"编辑内容 + "分类管理"分类）
```html
{{define "head"}}
<link rel="stylesheet" href="{{asset "/css/main.css"}}">
{{end}}
{{define "body"}}
{{template "nav" .}}
<main class="container">
  <article class="entry">
    {{if .Entry.Fields.cover}}<img src="{{media .Entry.Fields.cover}}" class="entry-hero" alt="">{{end}}
    {{if .EntryCategory}}<div class="entry-category">分类：{{if .EntryCategoryURL}}<a href="{{.EntryCategoryURL}}">{{.EntryCategory}}</a>{{else}}{{.EntryCategory}}{{end}}</div>{{end}}
    <h1>{{.Entry.Content.Title}}</h1>
    {{if .Entry.Content.PublishedAt}}<time class="entry-date">{{.Entry.Content.PublishedAt}}</time>{{end}}
    {{range $name, $field := .Entry.Fields}}
      {{if and (ne $name "title") (ne $name "slug") (ne $name "excerpt")}}
        {{if or (eq $name "content")}}
        <div class="entry-content">{{raw $field}}</div>
        {{end}}
      {{end}}
    {{end}}
  </article>
</main>
{{template "footer" .}}
{{end}}
```

---

## 后台菜单 → 前台渲染对照表

| 后台菜单 | 前台对应模板/标签 | 数据来源 |
|---|---|---|
| 内容管理（列表/编辑/发布） | `index.html` / `list.html` / `single.html`：`{{range .Items}}`、`{{url}}`、`{{raw}}` | `.Items`、`.Entry` |
| 分类管理（建分类/子分类） | `list.html` 的 `.SubCategories` 子分类导航；`single.html` 的 `.EntryCategory`/`.EntryCategoryURL`；`/category/<slug>[/<子slug>]` 归档 | `.SubCategories`、`.EntryCategory` |
| 菜单管理（建导航/下拉） | `nav.html` / `footer.html`：`{{firstMenu $}}`、`{{menu $ "名字"}}` | `.Menus` |
| 媒体库（上传图片等） | `{{media .Fields.cover}}` 渲染图片 | `.Fields.<字段名>` |
| 内容类型（定义字段） | `type_map` 决定类型渲染模板；字段经 `.Entry.Fields.<name>` 访问 | `.Entry.Fields` |
| 设置（站点名/URL/描述） | `{{.Site.Name}}` / `{{.Site.Description}}` / `{{.Site.URL}}` | `.Site` |
| 仪表盘 | 无前台对应（后台统计） | — |

---

## 开发热重载
config `server.debug: true` 时改模板立即生效（mtime 检测）。

## 前台主题切换
改 config `site.theme`（设置页的 `settings.theme` 当前仅存储到 DB，前台渲染仍读 config，改后需重启）。
