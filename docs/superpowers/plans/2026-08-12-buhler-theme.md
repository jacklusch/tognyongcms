# Bühler 风格前台主题 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 仅改造 `themes/default/`，参考 buhlergroup.com 实现"经典 Bühler 工业风"前台：深色导航（含语言切换与两级下拉）、全宽 Hero 大图、产品卡片网格、新闻动态、深色单栏页脚，并让列表/详情/分类归档页风格统一。

**架构：** 全部改动在主题目录内。`theme.yaml` 的模板 key / `type_map` 不变，后台渲染流程零改动。模板只消费 `theme.Data` 现有字段；语言切换用 `.Meta.HrefLangs`（seo 已生成各语言 URL，跳过 `x-default`）；首页产品卡片用 `.Items`（article 列表）驱动。样式与交互在 `static/` 下重写，Hero 用 `static/img/hero.svg` 手写深色工业纹理 + CSS 渐变回退。

**技术栈：** Go html/template（模板只读数据）、原生 CSS/JS（无构建步骤）。后台 Go 代码**不改**。

**前置基线：** 规格 `docs/superpowers/specs/2026-08-12-buhler-theme-design.md` 已确认。当前 `themes/default/` 为默认主题（index/list/single + nav/footer partials + main.css + main.js + zh/en locales）。

**环境约束（所有任务遵守）：**
- **不 git 提交**（仓库约定，直接 master 文件级审查）；不运行 `go mod tidy`
- Go 验证：`go test -count=1 ./...`、`go vet ./...`、`gofmt -l .`（后台无改动应保持绿）
- 主题在 `themes/`（非 go:embed，运行期从磁盘读取）——改模板/静态文件后 `server.debug: true` 热重载或重启即可，**无需 go build / 无需构建 admin SPA**
- 冒烟前确认 8080 无残留 dulizhan 进程（否则 500 是端口占用）
- 错误消息/模板注释用中文；CSS/JS 无构建，直接写原生文件

**验证基线（写任何代码前先跑一次，确认环境可用）：**
```
go test -count=1 ./...
go vet ./...
gofmt -l .
```
预期：全绿、无 diff。若环境找不到 go 命令，先解决 PATH 问题（AGENTS.md：Go 1.26 已装）。

---

### 任务 1：Hero 背景静态图

**文件：**
- 创建：`themes/default/static/img/hero.svg`

手写一张深色工业风矢量背景（SVG），尺寸约 1600×900，包含机械感几何图形（圆形/齿盘/矩形构件轮廓、细网格线），深灰底 `#222`、构件描边 `#333`/`#262626`，无文字。后续模板用 `asset "/img/hero.svg"` 引用，CSS 深色渐变作回退。

- [ ] **步骤 1：创建 hero.svg**

用 Write 工具创建 `themes/default/static/img/hero.svg`，内容为无文字纯 SVG（示例结构）：
```svg
<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="900">
  <rect width="1600" height="900" fill="#222222"/>
  <circle cx="200" cy="180" r="120" fill="none" stroke="#333333" stroke-width="40"/>
  <circle cx="520" cy="200" r="160" fill="none" stroke="#333333" stroke-width="50"/>
  <rect x="820" y="80" width="300" height="180" fill="none" stroke="#333333" stroke-width="40"/>
  <circle cx="1400" cy="400" r="220" fill="none" stroke="#262626" stroke-width="70"/>
  <rect x="100" y="560" width="420" height="120" fill="none" stroke="#333333" stroke-width="34"/>
  <circle cx="900" cy="640" r="130" fill="none" stroke="#262626" stroke-width="46"/>
  <rect x="1150" y="600" width="260" height="160" fill="none" stroke="#333333" stroke-width="36"/>
</svg>
```
（图形可自行增减，但保持：无文字、深灰底、描边几何、工业感）

- [ ] **步骤 2：验证文件存在且为合法 XML**

运行：
```powershell
Get-ChildItem themes\default\static\img\hero.svg | Select-Object Name,Length
```
预期：文件存在、长度 > 0。`svgs` 无构建工具，用浏览器打开确认不报 XML 错误（或忽略——模板引用的是 URL 而非内联）。

---

### 任务 2：locales 文案扩展

**文件：**
- 修改：`themes/default/locales/zh.yaml`
- 修改：`themes/default/locales/en.yaml`

- [ ] **步骤 1：读现有 locales**

运行：`Get-Content themes\default\locales\zh.yaml; Get-Content themes\default\locales\en.yaml`
确认现有 key（`home`/`latest`/`articles`/`no_content`）。

- [ ] **步骤 2：扩 zh.yaml**

追加 key（保留现有 key 不动）：
```yaml
products: 产品中心
news: 新闻动态
view_more: 查看详情
learn_more: 了解更多
about: 关于我们
contact: 联系我们
```

- [ ] **步骤 3：扩 en.yaml**

追加对应英文（保留现有 key 不动）：
```yaml
products: Products
news: News
view_more: View details
learn_more: Learn more
about: About Us
contact: Contact Us
```

- [ ] **步骤 4：验证 YAML 合法**

运行：
```powershell
go test -count=1 ./themes/... -run TestTheme -v
```
预期：主题加载测试 PASS（主题 Load 会解析 locales，语法错误会 FAIL）。

---

### 任务 3：主样式表（CSS）

**文件：**
- 修改：`themes/default/static/css/main.css`

定义 Bühler 工业风全量样式：CSS 变量配色、排版、导航/下拉、Hero、卡片网格、页脚、分页、响应式（≤900px 汉堡菜单 + 单列网格）。覆盖现有 default 样式（直接用 Write 覆盖，不增删冲突）。

- [ ] **步骤 1：编写完整 main.css**

用 Write 覆盖 `themes/default/static/css/main.css`，需包含以下可复用 class（与后续模板 task 的 class 名严格一致）：

```css
:root{
  --red:#E31E24; --ink:#111; --dark:#1a1a1a; --gray:#f5f5f5;
  --muted:#666; --line:#e8e8e8;
}
*{box-sizing:border-box}
body{margin:0;font-family:"Helvetica Neue",Helvetica,Arial,"PingFang SC","Microsoft YaHei",sans-serif;color:var(--ink);line-height:1.7;background:#fff}
a{color:inherit;text-decoration:none}
/* 顶栏 */
.topbar{background:var(--dark);color:#fff;font-size:12px;padding:6px 0}
.topbar-inner{max-width:1200px;margin:0 auto;padding:0 24px;display:flex;justify-content:space-between;align-items:center}
.lang-switch{display:flex;gap:14px}
.lang-switch a{color:#bbb;font-size:12px}
.lang-switch a:hover{color:var(--red)}
/* 导航 */
.site-nav{background:var(--dark);border-top:1px solid #333}
.nav-inner{max-width:1200px;margin:0 auto;padding:0 24px;display:flex;align-items:center;justify-content:space-between;height:64px}
.nav-brand{color:#fff;font-size:22px;font-weight:800;letter-spacing:.5px}
.nav-brand em{color:var(--red);font-style:normal}
.nav-links{display:flex;gap:28px;align-items:center}
.nav-links>a,.nav-item>a{color:#fff;font-size:14px;font-weight:500;text-transform:uppercase;letter-spacing:.5px;padding:6px 0}
.nav-links>a:hover,.nav-item>a:hover{color:var(--red)}
.nav-item{position:relative}
.has-children .dropdown{display:none;position:absolute;top:100%;left:0;background:var(--dark);min-width:180px;border-top:3px solid var(--red);z-index:20;padding:8px 0}
.has-children:hover .dropdown{display:block}
.dropdown a{display:block;color:#ccc;padding:8px 20px;font-size:14px}
.dropdown a:hover{color:var(--red);background:#242424}
.nav-toggle{display:none;background:none;border:none;color:#fff;font-size:24px;cursor:pointer;padding:0}
@media(max-width:900px){
  .nav-toggle{display:block}
  .nav-links{display:none;position:absolute;top:calc(64px + 29px);left:0;right:0;background:var(--dark);flex-direction:column;gap:0;padding:12px 24px}
  .nav-links.open{display:flex}
  .nav-links>a,.nav-item>a{padding:12px 0;border-bottom:1px solid #333;width:100%}
  .has-children .dropdown{position:static;display:none;border-top:none;padding:0 0 0 16px}
  .has-children.open .dropdown{display:block}
}
/* 容器 */
.container{max-width:1200px;margin:0 auto;padding:0 24px}
/* Hero */
.hero{position:relative;min-height:480px;display:flex;align-items:center;justify-content:center;text-align:center;color:#fff;background:linear-gradient(180deg,rgba(0,0,0,.55),rgba(0,0,0,.78)),var(--dark)}
.hero-bg{position:absolute;inset:0;background:url("/themes/default/img/hero.svg") center/cover no-repeat;z-index:0}
.hero>*{position:relative;z-index:1}
.hero h1{font-size:clamp(40px,6vw,64px);font-weight:900;letter-spacing:2px;text-transform:uppercase;max-width:900px;margin:0 auto}
.hero p{margin:16px auto 0;font-size:18px;max-width:640px;color:#e8e8e8}
.hero .cta{margin-top:32px;display:inline-block;background:var(--red);color:#fff;padding:14px 44px;font-size:15px;font-weight:700;letter-spacing:1px;text-transform:uppercase}
.hero .cta:hover{background:#b91a1e}
/* 区块 */
.section{max-width:1200px;margin:0 auto;padding:64px 24px}
.section-title{font-size:32px;font-weight:800;text-transform:uppercase;letter-spacing:1px;position:relative;padding-bottom:14px;margin:0 0 14px}
.section-title::after{content:"";position:absolute;left:0;bottom:0;width:64px;height:4px;background:var(--red)}
.section-sub{color:var(--muted);margin:0 0 32px}
/* 卡片网格 */
.grid{display:grid;grid-template-columns:repeat(3,1fr);gap:28px}
.card{border:1px solid var(--line);background:#fff;transition:box-shadow .2s;display:flex;flex-direction:column}
.card:hover{box-shadow:0 12px 32px rgba(0,0,0,.12)}
.card img{width:100%;height:200px;object-fit:cover;display:block;background:var(--gray)}
.card-img-placeholder{width:100%;height:200px;background:var(--gray)}
.card-body{padding:20px;display:flex;flex-direction:column;flex:1}
.card-body h3{font-size:18px;font-weight:700;margin:0}
.card-body p{font-size:14px;color:var(--muted);margin:8px 0 0}
.card-body .more{margin-top:auto;padding-top:14px;color:var(--red);font-weight:600;font-size:14px}
/* 新闻 */
.feature{background:var(--gray)}
.news-list{display:grid;grid-template-columns:repeat(3,1fr);gap:28px}
.news-item{background:#fff;padding:24px;border-top:4px solid var(--red)}
.news-item time{font-size:12px;color:var(--muted)}
.news-item h4{font-size:16px;margin:8px 0 0;line-height:1.5}
.news-item a:hover h4{color:var(--red)}
/* 页脚 */
.site-footer{background:var(--dark);color:#bbb;margin-top:0}
.footer-inner{max-width:1200px;margin:0 auto;padding:40px 24px;text-align:center}
.footer-inner a{color:#999;margin:0 16px;font-size:14px}
.footer-inner a:hover{color:var(--red)}
.footer-inner .copy{margin-top:20px;font-size:13px;color:#777}
/* 列表页 */
.page-header{max-width:1200px;margin:0 auto;padding:48px 24px 0}
.page-title{font-size:36px;font-weight:800;text-transform:uppercase;position:relative;padding-bottom:14px;margin:0}
.page-title::after{content:"";position:absolute;left:0;bottom:0;width:64px;height:4px;background:var(--red)}
.sub-categories{display:flex;flex-wrap:wrap;gap:10px;margin:24px 0}
.sub-cat{display:inline-block;padding:6px 18px;border:1px solid var(--line);border-radius:20px;color:var(--ink);font-size:14px}
.sub-cat:hover{background:var(--red);border-color:var(--red);color:#fff}
/* 分页 */
.pager{display:flex;gap:8px;justify-content:center;padding:40px 0}
.pager a,.pager span{display:inline-block;min-width:36px;padding:6px 10px;text-align:center;border:1px solid var(--line);color:var(--ink);font-size:14px}
.pager a:hover{border-color:var(--red);color:var(--red)}
.pager span.current{background:var(--red);border-color:var(--red);color:#fff}
/* 详情页 */
.entry-hero{width:100%;max-height:400px;object-fit:cover;display:block}
.entry-category{display:inline-block;background:var(--gray);padding:3px 14px;border-radius:16px;font-size:13px;margin:24px 0 0}
.entry-category a{color:var(--red);font-weight:600}
.entry h1{font-size:36px;font-weight:800;margin:16px 0 8px}
.entry-date{color:var(--muted);font-size:14px}
.entry-content{font-size:16px;line-height:1.9;margin-top:24px}
.entry-content img{max-width:100%}
/* 返回顶部 */
.back-top{display:inline-block;margin-top:8px;color:var(--red)}
@media(max-width:900px){
  .grid,.news-list{grid-template-columns:1fr}
  .hero{min-height:360px}
  .hero h1{font-size:32px}
  .hero p{font-size:16px}
  .section{padding:40px 24px}
}
```

- [ ] **步骤 2：验证 CSS 语法（无构建）**

CSS 无编译步骤。用浏览器打开任意渲染页确认样式应用（在任务 5-7 完成后统一冒烟；本步仅确认文件已覆盖、长度合理）：
```powershell
Get-ChildItem themes\default\static\css\main.css | Select-Object Name,Length
```
预期：文件存在、长度显著大于原 2.3KB（新增样式）。

---

### 任务 4：导航与页脚 partials

**文件：**
- 修改：`themes/default/templates/partials/nav.html`
- 修改：`themes/default/templates/partials/footer.html`

nav 提供深色顶栏（站点名 + 语言切换）+ 深色导航（logo + firstMenu + 两级下拉 + 汉堡按钮）。语言切换遍历 `.Langs`，优先用 `.Meta.HrefLangs` 找当前页各语言 URL，跳过 `x-default`；找不到回退 `home`。footer 深色单栏（`menu $ "footer"` + 版权）。

- [ ] **步骤 1：编写 nav.html**

用 Write 覆盖 `themes/default/templates/partials/nav.html`。语言切换统一链到对应语言的首页（`home .Code`）；当前页各语言的精准链接已由 `base.html` 通过 `.Meta.HrefLangs` 输出 `<link rel="alternate" hreflang>`（SEO），UI 切换器跳首页是刻意简化（YAGNI）。

```html
{{define "nav"}}
<div class="topbar">
  <div class="topbar-inner">
    <span class="topbar-site">{{.Site.Name}}</span>
    <span class="lang-switch">
      {{$current := .Lang}}
      {{range .Langs}}
        {{if eq .Code $current}}
          <span class="current">{{.Label}}</span>
        {{else}}
          <a href="{{home .Code}}">{{.Label}}</a>
        {{end}}
      {{end}}
    </span>
  </div>
</div>
<nav class="site-nav">
  <div class="nav-inner">
    <a class="nav-brand" href="{{home $.Lang}}">{{.Site.Name}}</a>
    <button class="nav-toggle" type="button" aria-label="菜单">☰</button>
    <div class="nav-links" id="nav-links">
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
  </div>
</nav>
{{end}}
```

- [ ] **步骤 2：编写 footer.html**

用 Write 覆盖 `themes/default/templates/partials/footer.html`：
```html
{{define "footer"}}
<footer class="site-footer">
  <div class="footer-inner">
    <div>
      {{range menu $ "footer"}}<a href="{{.URL}}">{{.Label}}</a>{{end}}
      <a href="{{home $.Lang}}">{{t $.Lang "home"}}</a>
    </div>
    <p class="copy">© {{.Site.Name}} · {{t $.Lang "copyright"}}</p>
    <a class="back-top" href="#top">BACK TO TOP ↑</a>
  </div>
</footer>
{{end}}
```

- [ ] **步骤 3：验证模板可解析**

运行：
```powershell
go test -count=1 ./internal/theme/... -run TestTheme -v
go test -count=1 ./themes/... -v
```
预期：PASS（模板解析失败会 FAIL）。若 `.Langs` 为空或 `.Meta` 为 nil 时模板 panic，检查 `eq .Code $current` 空值语义——`.Langs` 由 server 总是填充（i18n.All()），`.Meta.HrefLangs` 不再被 nav 使用，无 nil 风险。

---

### 任务 5：首页 index.html

**文件：**
- 修改：`themes/default/templates/index.html`

深色顶栏/导航（复用 nav partial）+ Hero（hero.svg 背景 + locales 标题 + CTA）+ 产品中心（`.Items` 前 6 条卡片）+ 新闻动态（`.Items` 前 3 条）+ 深色页脚。

- [ ] **步骤 1：编写 index.html**

用 Write 覆盖 `themes/default/templates/index.html`：
```html
{{define "head"}}
<link rel="stylesheet" href="{{asset "/css/main.css"}}">
{{end}}
{{define "body"}}
{{template "nav" .}}
<header class="hero">
  <div class="hero-bg"></div>
  <div>
    <h1>{{t $.Lang "hero_title"}}</h1>
    <p>{{.Site.Description}}</p>
    <a class="cta" href="{{typeURL $.Lang "article"}}">{{t $.Lang "learn_more"}}</a>
  </div>
</header>
<main>
  {{if .Items}}
  <section class="section">
    <h2 class="section-title">{{t $.Lang "products"}}</h2>
    <p class="section-sub">{{t $.Lang "products_sub"}}</p>
    <div class="grid">
      {{range slice .Items 0 6}}
      <a class="card" href="{{url $.Lang .}}">
        {{if .Fields.cover}}<img src="{{media .Fields.cover}}" alt="{{.Content.Title}}">{{else}}<div class="card-img-placeholder"></div>{{end}}
        <div class="card-body">
          <h3>{{.Content.Title}}</h3>
          {{if .Fields.excerpt}}<p>{{.Fields.excerpt}}</p>{{end}}
          <span class="more">{{t $.Lang "view_more"}} →</span>
        </div>
      </a>
      {{end}}
    </div>
  </section>
  <section class="section feature">
    <h2 class="section-title">{{t $.Lang "news"}}</h2>
    <p class="section-sub">{{t $.Lang "news_sub"}}</p>
    <div class="news-list">
      {{range slice .Items 0 3}}
      <a class="news-item" href="{{url $.Lang .}}">
        {{if .Content.PublishedAt}}<time>{{.Content.PublishedAt}}</time>{{end}}
        <h4>{{.Content.Title}}</h4>
      </a>
      {{end}}
    </div>
  </section>
  {{else}}
  <section class="section"><p>{{t $.Lang "no_content"}}</p></section>
  {{end}}
</main>
{{template "footer" .}}
{{end}}
```

> 说明：`slice .Items 0 6` 与 `slice .Items 0 3` 用 `text/template` 内置 `slice` 函数（Go ≥1.13 自带，无需注册）。`{{.Fields.excerpt}}` 输出 string；若 excerpt 缺失则不渲染。`raw` 仅用于富文本，此处不用。

- [ ] **步骤 2：新增首页文案 key（zh/en）**

向 `themes/default/locales/zh.yaml` 追加（en.yaml 对应英文）：
```yaml
hero_title: Innovations for Food Processing
products_sub: 面向食品加工行业的核心产品
news_sub: 行业资讯与公司动态
```
en.yaml：
```yaml
hero_title: Innovations for Food Processing
products_sub: Core products for the food processing industry
news_sub: Industry news and company updates
```
（`copyright` 若未在任务 2 添加，此处一并加 zh: `保留所有权利` / en: `All rights reserved`）

- [ ] **步骤 3：验证模板渲染**

运行：
```powershell
go test -count=1 ./internal/theme/... -v
go test -count=1 ./themes/... -v
```
预期：PASS。若 `slice` 报"未定义函数"，说明模板引擎禁用了内置函数——实际不会（`template.New(key).Funcs(t.funcs)` 合并内置函数）。

---

### 任务 6：列表页 list.html

**文件：**
- 修改：`themes/default/templates/list.html`

页头（类型名）+ 子分类 chip（`.SubCategories`）+ 卡片网格（`.Items`）+ 分页（`TypeName != category` 时显示）。

- [ ] **步骤 1：编写 list.html**

用 Write 覆盖 `themes/default/templates/list.html`：
```html
{{define "head"}}
<link rel="stylesheet" href="{{asset "/css/main.css"}}">
{{end}}
{{define "body"}}
{{template "nav" .}}
<header class="page-header">
  <h1 class="page-title">{{if eq .TypeName "article"}}{{t $.Lang "products"}}{{else}}{{.TypeName}}{{end}}</h1>
</header>
<main class="container">
  {{if .SubCategories}}
  <div class="sub-categories">
    {{range .SubCategories}}<a class="sub-cat" href="{{.URL}}">{{.Name}}</a>{{end}}
  </div>
  {{end}}
  {{if .Items}}
  <div class="grid" style="padding-top:32px">
    {{range .Items}}
    <a class="card" href="{{url $.Lang .}}">
      {{if .Fields.cover}}<img src="{{media .Fields.cover}}" alt="{{.Content.Title}}">{{else}}<div class="card-img-placeholder"></div>{{end}}
      <div class="card-body">
        <h3>{{.Content.Title}}</h3>
        {{if .Fields.excerpt}}<p>{{.Fields.excerpt}}</p>{{end}}
        <span class="more">{{t $.Lang "view_more"}} →</span>
      </div>
    </a>
    {{end}}
  </div>
  {{else}}
  <p style="padding:32px 0">{{t $.Lang "no_content"}}</p>
  {{end}}
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

> 说明：`{{.TypeName}}` 在列表页输出原始类型名（`article`/`page` 等）。为避免英文类型名裸显示，页头标题使用：`{{if eq .TypeName "article"}}{{t $.Lang "products"}}{{else}}{{.TypeName}}{{end}}`（代码块已采用）。

- [ ] **步骤 2：验证**

运行：
```powershell
go test -count=1 ./internal/theme/... -v
go test -count=1 ./themes/... -v
```
预期：PASS。

---

### 任务 7：详情页 single.html

**文件：**
- 修改：`themes/default/templates/single.html`

封面图（`.Entry.Fields.cover`）+ 分类 chip（`.EntryCategory`/`.EntryCategoryURL`）+ 标题 + 时间 + 正文（`raw` 富文本）。

- [ ] **步骤 1：编写 single.html**

用 Write 覆盖 `themes/default/templates/single.html`：
```html
{{define "head"}}
<link rel="stylesheet" href="{{asset "/css/main.css"}}">
{{end}}
{{define "body"}}
{{template "nav" .}}
<main class="container" style="padding-top:40px;padding-bottom:64px">
  <article class="entry">
    {{if .Entry.Fields.cover}}<img src="{{media .Entry.Fields.cover}}" class="entry-hero" alt="">{{end}}
    {{if .EntryCategory}}
    <div class="entry-category">
      {{if .EntryCategoryURL}}<a href="{{.EntryCategoryURL}}">{{.EntryCategory}}</a>{{else}}{{.EntryCategory}}{{end}}
    </div>
    {{end}}
    <h1>{{.Entry.Content.Title}}</h1>
    {{if .Entry.Content.PublishedAt}}<time class="entry-date">{{.Entry.Content.PublishedAt}}</time>{{end}}
    {{range $name, $field := .Entry.Fields}}
      {{if eq $name "content"}}
      <div class="entry-content">{{raw $field}}</div>
      {{end}}
    {{end}}
  </article>
</main>
{{template "footer" .}}
{{end}}
```

- [ ] **步骤 2：验证**

运行：
```powershell
go test -count=1 ./internal/theme/... -v
go test -count=1 ./themes/... -v
```
预期：PASS。

---

### 任务 8：前端交互脚本

**文件：**
- 修改：`themes/default/static/js/main.js`

移动端汉堡菜单切换、触屏下拉点击切换、返回顶部。渐进增强（无 JS 时 CSS hover 菜单仍可用）。

- [ ] **步骤 1：编写 main.js**

用 Write 覆盖 `themes/default/static/js/main.js`：
```js
(function () {
  var toggle = document.querySelector('.nav-toggle');
  var links = document.querySelector('.nav-links');
  if (toggle && links) {
    toggle.addEventListener('click', function () {
      links.classList.toggle('open');
    });
  }
  // 触屏：点击有下拉的父项切换 open（桌面 hover 由 CSS 处理）
  var parents = document.querySelectorAll('.has-children');
  Array.prototype.forEach.call(parents, function (el) {
    var a = el.querySelector(':scope > a');
    if (a) {
      a.addEventListener('click', function (e) {
        if (window.matchMedia && window.matchMedia('(max-width: 900px)').matches) {
          e.preventDefault();
          el.classList.toggle('open');
        }
      });
    }
  });
})();
```

- [ ] **步骤 2：在 base 或各模板引入 JS**

主题引擎只渲染 `{{define "head"}}` 与 `{{define "body"}}`，无统一 footer block。在**每个模板的 `{{define "body"}}` 末尾**（`{{template "footer" .}}` 之后）加：
```html
<script src="{{asset "/js/main.js"}}"></script>
```
需修改的文件：`templates/index.html`、`templates/list.html`、`templates/single.html`。

> 说明：三个模板 body 均以 `{{template "footer" .}}` 结尾，脚本放其后、`{{end}}` 之前。

- [ ] **步骤 3：验证**

运行：
```powershell
go test -count=1 ./... -v
```
预期：全绿。JS 语法无构建检查，冒烟时浏览器 console 无报错即可。

---

### 任务 9：全量验证与冒烟

**文件：**（无代码改动）

- [ ] **步骤 1：Go 全量验证**

运行：
```powershell
go test -count=1 ./...
go vet ./...
gofmt -l .
```
预期：全绿、vet 干净、gofmt 无输出。

- [ ] **步骤 2：配置并启动冒烟**

确认 `config.yaml` 存在（或复制 `config.example.yaml` 生成），设置 `server.debug: true`。确认 8080 无残留进程：
```powershell
Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
```
无输出则端口空闲。启动：
```powershell
go run ./cmd/dulizhan
```
（若 `go` 不在 PATH，先定位 go 可执行文件或临时加入 PATH）

- [ ] **步骤 3：四类页面冒烟**

用浏览器（或 PowerShell `Invoke-WebRequest`）检查：
- `/` 首页：深色导航、Hero（hero.svg 背景 + 大字标题 + CTA）、产品中心卡片网格（≤6 卡片）、新闻动态（≤3 条）、深色页脚、语言切换（zh/en）
- `/article` 列表：页头标题、卡片网格、分页
- `/article/{slug}` 详情：封面、分类 chip、标题、正文（富文本）
- `/category/{slug}` 分类归档：子分类 chip、卡片、无分页
- 切到 `/en`（或站点配置的英文前缀）验证语言切换与英文文案
- 缩到 ≤900px：导航折叠成汉堡、点开菜单、下拉点击切换

预期：全部正常渲染，无 500/模板报错。若 500，查日志区分"端口占用"vs"模板/数据错误"。

- [ ] **步骤 4：收尾**

清理冒烟进程（结束 `go run` 子进程），确认无残留 8080 进程。汇总改动文件清单供审查。

---

## 自检记录

**规格覆盖度：**
- §4 文件清单 → 任务 1（hero.svg）、2（locales）、3（main.css）、4（nav/footer）、5-7（三模板）、8（main.js）✔
- §5 模板与数据映射 → 任务 5/6/7 逐项实现；语言切换用 `.Langs` + `home`（简化版）✔
- §6 视觉规范 → 任务 3 CSS 全覆盖（配色/排版/响应式/下拉/卡片/页脚/分页）✔
- §7 locales key → 任务 2、5 步骤 2 ✔
- §8 JS 行为 → 任务 8 ✔
- §10 验证 → 任务 9 ✔

**占位符扫描：** 任务 4 初稿曾尝试用 `.Meta.HrefLangs` 精确匹配当前页各语言 URL，因模板过于复杂已改为**语言切换统一链 `home .Code`**（hreflang 仍由 base.html 输出），文件中无占位符。任务 5/6 模板与 CSS 的 class 名一致（含 `card-img-placeholder`）。

**类型一致性：**
- `slice`（text/template 内置，Go≥1.13）与 `pages`/`pagerURL`/`firstMenu`/`menu`/`url`/`typeURL`/`home`/`asset`/`media`/`raw`/`t` 均为现有 FuncMap 或内置函数，签名一致。
- `.Langs[].Code/.Label`、`.Meta.HrefLangs[].Lang/.URL`、`.Items`（`[]content.Entry`）、`.Fields.cover/.excerpt` 与 `internal/theme`、`internal/seo`、`internal/content` 定义一致。
- 新增 locales key（`hero_title`/`products_sub`/`news_sub`/`copyright`/`products`/`news`/`view_more`/`learn_more`/`about`/`contact`）在任务 2/5 中定义，模板引用的 key 均在其中。
