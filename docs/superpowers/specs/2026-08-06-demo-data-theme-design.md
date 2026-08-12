# Dulizhan CMS — 演示数据与默认主题展示 设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`、`2026-08-06-category-system-design.md`
- 基线：独立分类系统已完成（categories 表 + 管理 API + 前台 /category/<slug> 归档）；seed 建 article/page 类型 + hello-zh/en 双语文章（无分类无图）

## 1. 目标与范围

扩展 seed 生成演示数据（分类 + 双语文章 + 网络下载封面图存媒体库），并增强默认主题展示（首页/详情显示封面图与分类、导航预置分类菜单）。

**包含**：
- seed 扩展：建 news/about/products 分类；每分类 2-3 篇双语文章（zh+en 翻译组）；封面图下载到媒体库
- 默认主题：index.html/list.html 显示缩略图；single.html 显示大图 + 分类；nav 预置分类菜单
- 网络图片下载：picsum.photos 稳定 URL → 下载存媒体库；失败降级（存 picsum 原 URL）
- 幂等：二次 seed 不重复

**排除**：卡片式布局重构；更多分类/文章；视频演示数据（YAGNI）。

## 2. seed 扩展

### 2.1 签名扩展（`internal/seed/seed.go`）

```go
// Run 创建内置内容类型、分类与演示内容（幂等：已存在则跳过）。
func Run(ctx context.Context, svc *content.Service, med media.MediaStore) error
```
main.go 调用处同步传 `med`（已构造）。

### 2.2 分类（幂等）

```go
demoCategories := []struct{ Name, Slug string }{
	{"新闻", "news"}, {"关于", "about"}, {"产品", "products"},
}
for _, c := range demoCategories {
	if _, err := st.CategoryRepo().GetBySlug(ctx, c.Slug); err != nil {
		st.CategoryRepo().Create(ctx, &store.Category{Name: c.Name, Slug: c.Slug})
	}
}
```
> 需访问 `store.Store`——`Run` 签名是否需要 st？当前只有 svc（svc 内部持有 store 但未暴露 CategoryRepo）。**裁定**：`Run` 加 `st store.Store` 参数，或加 `svc` 方法。**实现时**：`Run(ctx, st store.Store, svc *content.Service, med media.MediaStore)`——main 调用处同步（`seed.Run(ctx, st, svc, med)`）。

### 2.3 双语演示文章（幂等）

每分类 2-3 篇，zh 先建 + en 同组翻译（复用 hello-zh/en 的 CreateTranslation 模式）。**demoContent 具体内容见实现计划**（每篇含 ZhTitle/ZhBody/EnTitle/EnBody 完整文本，如新闻分类："公司发布新一代产品" / "Company Launches Next-Gen Product" 等）。现有 hello-zh/en 文章**归入 news 分类**（`category` 字段设为 news 的 id），使首页/归档完整。

每篇 zh 创建：
```go
e, err := svc.Create(ctx, "article", "zh", map[string]any{
	"title": a.ZhTitle, "slug": a.Slug,
	"excerpt": "摘要", "content": a.ZhBody,
	"category": fmt.Sprintf("%d", catID),  // 分类 id 字符串
	"cover":    downloadCover(ctx, med, "https://picsum.photos/seed/"+a.Slug+"/800/500"),
}, 0)
svc.SetStatus(ctx, e.Content.ID, "published")
```
en 翻译（同组）：`svc.CreateTranslation(ctx, "article", "en", e.Content.ContentID, {...英文...}, Actor{IsModerator:true})` + SetStatus。

幂等守卫：`svc.GetPublishedBySlugLang(ctx, "article", a.Slug, "zh")` 已存在则跳过。

### 2.4 封面图下载（`internal/seed/media_download.go`）

```go
// downloadCover 从网络下载图片存媒体库，失败降级为原 URL（前台仍可显示）。
func downloadCover(ctx context.Context, med media.MediaStore, url string) string {
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		return url // 降级：保留原 URL
	}
	defer resp.Body.Close()
	key, err := media.RandomKey(".jpg")
	if err != nil {
		return url
	}
	u, err := med.Save(ctx, key, resp.Body, "image/jpeg")
	if err != nil {
		return url
	}
	return u
}
```
- 图片来源：`https://picsum.photos/seed/<slug>/800/500`（稳定可复现）
- 成功 → 媒体库 URL（`/media/xxx.jpg`）；失败 → picsum 原 URL（主题 `media` 函数都能渲染）

## 3. 默认主题展示

### 3.1 `index.html` 首页列表（缩略图）

```html
{{range .Items}}
<li class="entry-item">
  {{if .Fields.cover}}<img src="{{media .Fields.cover}}" class="entry-thumb" alt="">{{end}}
  <a href="{{url $.Lang .}}">{{.Content.Title}}</a>
  {{if .Content.PublishedAt}}<time>{{.Content.PublishedAt}}</time>{{end}}
</li>
{{end}}
```

### 3.2 `list.html` 归档列表（缩略图，同 index）

### 3.3 `single.html` 详情（大图 + 分类）

```html
{{if .Entry.Fields.cover}}<img src="{{media .Entry.Fields.cover}}" class="entry-hero" alt="">{{end}}
{{if .EntryCategory}}<div class="entry-category">分类：{{.EntryCategory}}</div>{{end}}
<h1>{{.Entry.Content.Title}}</h1>
```
（`EntryCategory` 已由 renderSingle 注入——分类系统任务实现。）

### 3.4 导航预置分类菜单（seed 建 main 菜单）

seed 建 `main` 菜单（zh/en）：
```go
// zh: 首页(home) + 新闻/关于/产品(custom → /category/<slug>)
// en: Home(home) + News/About/Products(custom → /category/<slug>)
menuItems := []store.MenuItem{
	{Label: "首页", Type: "home", URL: ""},
	{Label: "新闻", Type: "custom", URL: "/category/news"},
	{Label: "关于", Type: "custom", URL: "/category/about"},
	{Label: "产品", Type: "custom", URL: "/category/products"},
}
```
- `MenuRepo.ListByLang("zh")` 已有 main 菜单则跳过（幂等）
- 菜单项 custom URL 由后端 `menuURL` 按 lang 加前缀（`/en/category/news` 自动）

## 4. CSS

`main.css` 追加：
```css
.entry-item { display: flex; align-items: center; gap: 12px; }
.entry-thumb { width: 120px; height: 80px; object-fit: cover; border-radius: 4px; }
.entry-hero { width: 100%; max-height: 400px; object-fit: cover; border-radius: 8px; margin-bottom: 16px; }
.entry-category { display: inline-block; background: #f0f0f0; padding: 2px 10px; border-radius: 12px; font-size: 13px; color: var(--fg); margin-bottom: 8px; }
```

## 5. 测试策略

- **seed 扩展测试**（`internal/seed/seed_test.go`）：
  - 建分类（news/about/products）、每分类双语文章（zh+en 同组）、cover 降级逻辑
  - 用 httptest server 提供图片端点（不依赖真实外网）或 mock media——**实现时**用 httptest mock 图片端点
  - 幂等（二次 Run 不重复建）
- **主题渲染**：`go test ./internal/theme/ ./internal/server/` 冒烟（index/single/list 渲染含图不破）
- **端到端验收**：
  1. `go run ./cmd/dulizhan seed` → 建分类 + 双语文章 + cover 图
  2. 首页 `/`：文章列表含缩略图
  3. `/article/<slug>`：大图 + 分类名
  4. `/category/news`：新闻分类文章
  5. 导航 main 菜单（首页/新闻/关于/产品）可见，点分类进归档
  6. `/en/`：英文菜单 + 英文文章

## 6. 验收

1. `go test -count=1 ./...` 全绿 + build/vet/gofmt 干净
2. `cd admin && npx vue-tsc --noEmit && npx vitest run`（若前端模板相关无改动则仅后端）
3. 端到端：seed 后前台完整演示（图/分类/导航/双语）

## 7. 工作流约定

- 不 git 提交；superpowers SDD 驱动；前端模板改动后构建同步 `internal/server/dist`（本设计模板改动在 Go embed 的 themes，非 admin SPA——**注意**：themes 是 go:embed 的 `themes/`，前端 SPA 在 `admin/`；主题模板改动无需构建 SPA，但需 `go build` 重新编译嵌入）
