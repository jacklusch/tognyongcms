# SEO 审计修复（canonical/og/JSON-LD/sitemap/品牌化）设计文档

- 日期：2026-08-20
- 状态：已确认（用户逐节审阅通过）
- 背景：playwright 无头浏览器抓取 38 页得出 SEO 审计报告，本设计修复报告列出的问题。

## 1. 目标

修复 SEO 审计发现的严重/高优先级问题：分类页 canonical 指向 404、全站缺 og:image/twitter:card、description 过短且语言不匹配、首页 title 无关键词、结构化数据不足、sitemap 缺分类页且收录测试内容；顺带处理 favicon、首图 alt、尾斜杠重复、视频嵌入懒加载。

## 2. 范围与决策（头脑风暴确认）

| 决策点 | 结论 |
|---|---|
| 处理范围 | A(修复)+B(标签增强)+C(品牌化)+D(内容治理)+E(性能) 全做 |
| 品牌名 | `金博威机械`（site.name），英文品牌 `Jinbowei Machinery` |
| 站点域名 | 暂无，config `site.url` 保留 localhost 占位，机制按可配置实现 |
| 首页 title/desc | 由开发拟文案（含关键词），见 §4 |
| og 默认图 | 生成 1200×630 PNG 品牌占位图（含「金博威机械」），以后可换 logo |
| 尾斜杠方向 | 统一 301 到**无尾斜杠**版本 |
| 产品判定 | 详情页分类链首段 == `site.home_products_category` 即为产品页 → Product schema |
| Product offers | 无公开价格，省略 offers |
| 测试内容 | 删除测试内容记录（下架+删除），sitemap 自然不再收录 |
| git | 不提交（仓库约定），仅落盘 |

## 3. 配置（`internal/config` + `config.yaml` + `config.example.yaml` + `docs/deployment.md`）

```yaml
site:
  name: "金博威机械"              # 默认品牌名（zh）
  names:                          # 可选：按语言的品牌名（title 后缀 / og:site_name / JSON-LD name）
    zh: "金博威机械"
    en: "Jinbowei Machinery"
  url: "http://localhost:8080"    # 上线后改为正式域名
  description: "金博威机械专注食品加工设备研发制造，主营斩拌机、香肠机、拌馅机等肉类深加工机械，为食品企业提供高效、卫生、稳定的整体解决方案。"
  descriptions:                   # 可选：按语言的 meta description（回退 description）
    zh: "金博威机械专注食品加工设备研发制造，主营斩拌机、香肠机、拌馅机等肉类深加工机械，为食品企业提供高效、卫生、稳定的整体解决方案。"
    en: "Jinbowei Machinery designs and builds food processing equipment — meat choppers, sausage making machines and mixers — for efficient, hygienic and reliable production."
  og_image: "/themes/default/img/og-default.png"   # 相对路径，输出时拼 site.url
  home_products_category: "products"
  home_news_category: "news"
```

- `SiteConfig` 增 `Names map[string]string`、`Descriptions map[string]string`、`OGImage string`（`yaml:"og_image"`）；`Name/Description` 作为默认值兼容。
- 新增方法 `SiteName(lang string) string` / `SiteDescription(lang string) string`：lang 有则取 map，否则回退默认。
- env 覆盖保持现有模式；`config.example.yaml` 补注释说明。
- 首页 title 特殊格式（含关键词），其余页统一「页面标题 - 品牌名」。

## 4. 文案（已确认）

| 项 | zh | en |
|---|---|---|
| 首页 title | `金博威机械 - 斩拌机/香肠机/拌馅机食品机械厂家` | `Jinbowei Machinery - Meat Chopper, Sausage Machine & Mixer Manufacturer` |
| 首页 description | 见 §3 中文 | 见 §3 英文 |
| hero_title (H1) | `食品加工设备专业制造商` | `Innovations for Food Processing`（保持） |
| og:site_name | 金博威机械 | Jinbowei Machinery |

## 5. `internal/seo/seo.go`

- `Builder` 字段改为：`names/descriptions map[string]string`、`ogImage string`（追加）。
- `NewBuilder(opts Options, reg *i18n.Registry) *Builder`（签名变更，同步更新 server.go 与测试）；`Options` 含 `SiteURL/Names/Descriptions/HomeTitles/DefaultLang/OGImage`（产品判定由 server 层按分类链首段判断后传入 `isProduct`）。
- `Meta` 增 `Image string`。
- `localized(key-lang helper)`：`name(lang)`、`description(lang)`、`defaultImage(lang)` → `siteURL + ogImage`。
- `BuildHome(lang)`：title=§4 首页格式、desc/name 按语言、Image=默认图；JSON-LD = WebSite + Organization（name/url/logo）。
- `BuildList(lang, typeName, page)`：title 保持现状 `typeName - 品牌名`，desc/name 按语言，Image=默认图。
- 新增 `BuildCategory(lang string, chain []string, catName string, items []content.Entry) Meta`：
  - canonical = `siteURL + reg.URLPath(lang, "/category/"+strings.Join(chain, "/"))`
  - title = `catName - 品牌名`；desc 按语言；Image=默认图
  - JSON-LD = CollectionPage + `mainEntity` ItemList（items 前 10 条的 name/url）
- `BuildEntry(lang string, e content.Entry, catChain []string, isProduct bool) Meta`：
  - canonical = `/type/slug`；desc = `Fields.excerpt` 否则按语言默认；Image = `Fields.cover`（mediaFunc 规则）否则默认图
  - JSON-LD：
    - isProduct → Product{name, image, description, brand:{name}}
    - 否则 → Article（现状字段）
    - catChain 非空 → 追加 BreadcrumbList（首页 > 分类链 > 当前）
  - 多 schema 用 JSON 数组包在单个 `<script type="application/ld+json">` 内。

## 6. `internal/server/frontend.go`

- `server.New`：`seo.NewBuilder(Options{SiteURL, Names, Descriptions, HomeTitles, DefaultLang, OGImage}, reg)`，即传 `cfg.Site.URL`、`cfg.Site.Names`、`cfg.Site.Descriptions`、`cfg.Site.HomeTitles`、`cfg.Site.DefaultLang`、`cfg.Site.OGImage`。
- `handleFrontend` 开头：`path != "/"` 且以 `/` 结尾 → `301` 到去尾斜杠（保留 query）。
- `data.Site.Name/Description` 按 `lang` 取本地化值。
- `renderCategory`：改调 `s.seo.BuildCategory(lang, pathSegs, cat.Name, all)`。
- `renderSingle`：解析 catChain（已有逻辑），`isProduct = len(chain)>0 && chain[0] == s.cfg.Site.HomeProductsCategory`，调 `BuildEntry(lang, e, chain, isProduct)`。
- `handleSitemap`：
  - 增首页：`siteURL + reg.URLPath(l, "/")`（各语言）。
  - 增分类页：遍历 `CategoryRepo().All()`（或 ListAll 树），各语言 `categoryURL(l, chain)`。
  - 保持内容页；lastmod 逻辑不变。
- 新增 helper：按分类 id 取 slug 链（复用 `categoryChain`）。

## 7. 模板（`internal/theme/base.html` + 主题）

- `base.html`：
  - `<link rel="icon" type="image/png" href="{{asset "/img/favicon.png"}}">`
  - `{{if .Meta.Image}}<meta property="og:image" content="{{.Meta.Image}}">{{end}}`
  - `<meta name="twitter:card" content="summary_large_image">`
  - `{{if .Meta.Image}}<meta name="twitter:image" content="{{.Meta.Image}}">{{end}}`
  - `og:title/description/url` 已有；补齐 `twitter:title/description`。
- `themes/default/templates/single.html`：cover 图补 `alt="{{.Entry.Content.Title}}"`。
- `themes/default/locales/zh.yaml`：`hero_title` 改中文。
- 新资源：`themes/default/static/img/og-default.png`（1200×630 PNG，金博威机械文字）、`themes/default/static/img/favicon.png`（32×32）。
- 主题 `01` 是旧副本，默认主题为 `default`，仅改 `default`；同步 `01` 仅当用户要求。

## 8. 视频嵌入懒加载

- `admin/src/dynamic-form/controls/RichTextControl.vue`：插入的 iframe HTML 加 `loading="lazy"`。
- `themes/default/static/js/main.js`：`document.querySelectorAll('.entry-content iframe')` 若无 `loading` 属性则补 `loading="lazy"`（覆盖已存内容）。
- 构建：改 SPA 后跑 `npx vue-tsc --noEmit` + `vitest run`，再 `.\build.ps1`。

## 9. 内容治理（D）

- 删除测试内容：后台/脚本删除以下 slug 的已发布记录（以 sitemap 实际输出为准核对）：
  `tiktok`、`youtube`、`news003`、`news`、`news-004/005/006/007/008/009/010`、`run-test-1`、`post-21`、`chopper-mixer-test-data` 等。
- 删除后 sitemap 不再收录；页面 URL 变 404（预期）。
- 若内容组有 en 对应（同一 content_id），一并删除。

## 10. 测试

- `internal/seo/seo_test.go`：
  - `BuildCategory` canonical 正确（`/category/products/chopper`），不出现分类名。
  - `BuildEntry` isProduct → Product schema；非产品 → Article；带 catChain → BreadcrumbList。
  - og:image：entry 有 cover → cover；无 cover → 默认图。
  - description/name 按语言取本地化值。
- `internal/server/frontend_test.go`：
  - 分类页 canonical 含 `/category/<slug>` 链且非 404。
  - 尾斜杠 GET `/chopper/` → 301 到 `/chopper`；根 `/` 不动；`/en/chopper/` → `/en/chopper`。
  - sitemap 含首页、分类页、内容页。
- 主题/静态：og-default.png、favicon.png 存在且可访问（`/themes/default/img/...`）。

## 11. 验证

- `go test -count=1 ./...`、`go build ./...`、`go vet ./...`、`gofmt -l .`
- 前端 `npx vue-tsc --noEmit && npx vitest run`
- `.\build.ps1` 重建；重启服务；浏览器抽查首页/分类页/详情页的 canonical、og:image、JSON-LD、favicon；`curl /sitemap.xml` 确认无测试页。
