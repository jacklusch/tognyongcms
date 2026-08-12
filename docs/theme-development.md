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

## 开发热重载
config `server.debug: true` 时改模板立即生效（mtime 检测）。

## 前台主题切换
改 config `site.theme`（设置页的 `settings.theme` 当前仅存储到 DB，前台渲染仍读 config，改后需重启）。
