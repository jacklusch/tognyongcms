# 主题开发

## 目录结构
```
themes/<name>/
├── theme.yaml            # 主题清单
├── templates/            # 模板（index/list/single/page 等）
│   └── partials/         # 公共片段（nav/footer）
├── static/               # CSS/JS/图片
└── locales/              # zh.yaml / en.yaml 主题文案
```

## theme.yaml
```yaml
name: default
version: 1.0.0
templates:
  index: templates/index.html
  single: templates/single.html
  list: templates/list.html
partials:
  - templates/partials/nav.html
type_map:                 # 内容类型 → 模板 key
  article: single
  page: single
locales:
  zh: locales/zh.yaml
  en: locales/en.yaml
```

## 模板函数
- `t "lang" "key"` —— 界面文案翻译
- `url "lang" entry` —— 内容 URL（带语言前缀）
- `typeURL "lang" "typeName"` —— 类型列表 URL
- `home "lang"` —— 首页 URL
- `asset "/css/x.css"` —— 主题静态资源 URL
- `media value` —— 媒体 URL 解析
- `raw value` —— 原始 HTML（仅 richtext 字段）
- `pages total perPage` —— 分页页码数组
- `pagerURL data page` —— 分页 URL

## 数据（Data 结构）
`.Site.Name/.URL/.Description`、`.Lang`、`.Langs`、`.Entry.Content`、`.Entry.Fields.<name>`、`.Items`、`.Total/.Page/.PerPage`、`.Meta.*`（SEO）。

分类相关字段：
- `.EntryCategory` —— 详情页当前内容所属分类名（`single.html` 用）
- `.EntryCategoryURL` —— 该分类的完整归档 URL（如 `/category/products/chopper`），与 `.EntryCategory` 搭配渲染分类链接
- `.SubCategories` —— 分类归档页的直接子分类列表（`[]CategoryInfo{ID, Name, Slug, URL}`），用于渲染子分类导航（`list.html` 用）

## 分类归档与子分类
前台分类归档路径 `/category/<slug>` 支持任意层级（`/category/products/chopper`）。页面行为：
- 目标分类 `SubCategories` 非空时，`list.html` 顶部渲染子分类导航（默认主题用 `.sub-categories` / `.sub-cat` 样式）
- 归档内容**递归聚合**该分类全部子孙分类的内容（访问 `/category/products` 会同时列出 斩拌机/香肠机 等子分类的文章）
- 详情页 `.EntryCategory` 显示分类名，配合 `.EntryCategoryURL` 链接到对应归档页

## 开发热重载
config `server.debug: true` 时改模板立即生效（mtime 检测）。

## 前台主题切换
改 config `site.theme`（设置页的 `settings.theme` 当前仅存储到 DB，前台渲染仍读 config，改后需重启）。
