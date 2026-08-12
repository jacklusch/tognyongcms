# 演示数据与默认主题展示 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 扩展 seed 生成演示数据（news/about/products 分类 + 双语文章 + 网络封面图下载存媒体库），并增强默认主题展示（首页/详情显示封面图与分类、导航预置分类菜单）。

**架构：** `seed.Run` 签名扩为 `Run(ctx, st, svc, med)`（建分类 + 双语文章 + 封面下载）；新增 `media_download.go`（http.Get→media.Save，失败降级原 URL）；默认主题 index/list/single.html 加缩略图/大图/分类；seed 建 zh/en main 菜单（首页+分类链接）；main.go seed 分支适配。

**技术栈：** Go（gin、net/http、modernc.org/sqlite）、默认主题 html/template。

**前置基线：** 独立分类系统已完成。规格：`docs/superpowers/specs/2026-08-06-demo-data-theme-design.md`。

**环境约束（所有任务遵守）：**
- **不 git 提交**（仓库约定）；**勿运行 `go mod tidy`**
- Go 验证：`go test -count=1 <包> -v`；阶段收尾 `go test -count=1 ./...`
- 主题模板改动在 `themes/`（go:embed）——改后需 `go build` 重新编译，**无需构建 admin SPA**
- 网络图片下载用 picsum.photos；测试用 httptest mock 图片端点（不依赖真实外网）
- 错误消息用中文；后端依赖锁定勿动

**现状关键点：**
- `internal/seed/seed.go`：`Run(ctx, svc)` 只建 article/page + hello-zh/en（无分类无图）；`EnsureAuth(ctx, st, authSvc)`
- `cmd/dulizhan/main.go`：seed 分支（line 44-54）有 authSvc/svc，无 med；正常路径 line 80-89 构造 med
- `store.CategoryRepo`（List/GetByID/GetBySlug/Create/Update/Delete/CountContent）、`MenuRepo.ListByLang` 已存在
- `content.Service`：Create/CreateTranslation/SetStatus/GetPublishedBySlugLang 已存在
- 分类系统 renderSingle 已注入 `Data.EntryCategory`（分类名）

---

### 任务 1：seed 封面图下载 helper

**文件：**
- 创建：`internal/seed/media_download.go`
- 测试：`internal/seed/media_download_test.go`

- [ ] **步骤 1：编写失败测试**（`internal/seed/media_download_test.go`）

```go
package seed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dulizhan/internal/media"
)

// mockMediaStore 最小 MediaStore（记录 Save 的 URL）。
type mockMediaStore struct {
	media.MediaStore
	savedURL string
}

func (m *mockMediaStore) Save(ctx context.Context, key string, r io.Reader, contentType string) (string, error) {
	b, _ := io.ReadAll(r)
	if len(b) == 0 {
		return "", fmt.Errorf("空内容")
	}
	m.savedURL = "/media/" + key
	return m.savedURL, nil
}

func TestDownloadCoverSuccess(t *testing.T) {
	// httptest mock 图片端点（返回图片字节）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer ts.Close()

	mock := &mockMediaStore{}
	url := downloadCover(context.Background(), mock, ts.URL+"/img.jpg")
	if url == "" || url != mock.savedURL {
		t.Errorf("downloadCover 应返回媒体库 URL, got %q, saved=%q", url, mock.savedURL)
	}
}

func TestDownloadCoverFallback(t *testing.T) {
	// 网络失败（不可达）→ 降级返回原 URL
	mock := &mockMediaStore{}
	url := downloadCover(context.Background(), mock, "http://127.0.0.1:1/unreachable.jpg")
	if url != "http://127.0.0.1:1/unreachable.jpg" {
		t.Errorf("失败应降级原 URL, got %q", url)
	}
	// 非 200 → 降级
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	url = downloadCover(context.Background(), mock, ts.URL+"/err.jpg")
	if url != ts.URL+"/err.jpg" {
		t.Errorf("非 200 应降级原 URL, got %q", url)
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/seed/ -run TestDownloadCover -v`
预期：FAIL（`downloadCover` 未定义）

- [ ] **步骤 3：实现 downloadCover**（`internal/seed/media_download.go`）

```go
// internal/seed/media_download.go
package seed

import (
	"context"
	"io"
	"net/http"
	"time"

	"dulizhan/internal/media"
)

// downloadCover 从网络下载图片存媒体库；失败降级返回原 URL（前台 media 函数仍可渲染）。
func downloadCover(ctx context.Context, med media.MediaStore, url string) string {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return url
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return url
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
> `media.RandomKey` 返回 `YYYYMM/<hex>.jpg`。mockMediaStore.Save 记录 `/media/<key>`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/seed/ -run TestDownloadCover -v`
预期：PASS

- [ ] **步骤 5：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿（`media.MediaStore` 接口 mock 需实现 Save/Delete/URL——`mockMediaStore` 内嵌接口 + 覆盖 Save）

---

### 任务 2：seed 扩展（分类 + 双语演示文章 + 菜单）

**文件：**
- 修改：`internal/seed/seed.go`（Run 签名 + 分类 + 演示文章 + 菜单）
- 修改：`internal/seed/seed_test.go`（TestSeedDemoData）
- 修改：`cmd/dulizhan/main.go`（seed 分支构造 med + 传参）

- [ ] **步骤 1：编写失败测试**（追加到 `internal/seed/seed_test.go`）

```go
func TestSeedDemoData(t *testing.T) {
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	reg := schema.NewRegistry()
	svc := content.New(st, reg, []string{"zh", "en"})
	mock := &mockMediaStore{} // 任务 1 定义

	if err := Run(ctx, st, svc, mock); err != nil {
		t.Fatal(err)
	}
	// 分类建了 3 个
	cats, err := st.CategoryRepo().List(ctx)
	if err != nil || len(cats) != 3 {
		t.Errorf("分类数 = %d, %v", len(cats), err)
	}
	// 每分类双语文章存在（zh 已发布 + en 同组翻译）
	for _, slug := range []string{"news-1", "about-1", "products-1"} {
		zh, err := svc.GetPublishedBySlugLang(ctx, "article", slug, "zh")
		if err != nil {
			t.Errorf("zh 文章 %s: %v", slug, err)
			continue
		}
		en, err := svc.GetPublishedBySlugLang(ctx, "article", slug+"-en", "en")
		if err != nil {
			t.Errorf("en 翻译 %s: %v", slug, err)
			continue
		}
		if zh.Content.ContentID != en.Content.ContentID {
			t.Errorf("zh/en 应同翻译组: %q vs %q", zh.Content.ContentID, en.Content.ContentID)
		}
		if zh.Fields["category"] == "" {
			t.Errorf("zh %s 缺 category", slug)
		}
	}
	// main 菜单（zh）已建
	menus, err := st.MenuRepo().ListByLang(ctx, "zh")
	if err != nil || len(menus) == 0 {
		t.Errorf("zh main 菜单缺失: %v", err)
	}
	// 幂等：二次 Run 不报错、数量不变
	if err := Run(ctx, st, svc, mock); err != nil {
		t.Errorf("二次 Run 应幂等: %v", err)
	}
	cats2, _ := st.CategoryRepo().List(ctx)
	if len(cats2) != 3 {
		t.Errorf("幂等后分类数 = %d, want 3", len(cats2))
	}
}
```
> 需 import `schema`/`content`（seed_test.go 已有，任务 1 后加）。`mockMediaStore` 在 media_download_test.go 定义——**同包测试文件共享** ✓。en slug 约定 `slug+"-en"`（如 news-1-en）。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/seed/ -run TestSeedDemoData -v`
预期：FAIL（`Run` 参数数量不匹配）

- [ ] **步骤 3：Run 签名扩展 + 分类 + 演示文章 + 菜单**（`internal/seed/seed.go`）

签名与主体：
```go
// Run 创建内置内容类型、分类与演示内容（幂等：已存在则跳过）。
func Run(ctx context.Context, st store.Store, svc *content.Service, med media.MediaStore) error {
	// ...（既有 article/page 类型 + hello-zh/en，保持不变）
	// hello-zh/en 归入 news 分类（分类创建后设置）
	if err := seedDemoData(ctx, st, svc, med); err != nil {
		return err
	}
	return nil
}

// seedDemoData 建分类、双语演示文章与 main 菜单（幂等）。
func seedDemoData(ctx context.Context, st store.Store, svc *content.Service, med media.MediaStore) error {
	demoCategories := []struct{ Name, Slug string }{
		{"新闻", "news"}, {"关于", "about"}, {"产品", "products"},
	}
	catIDs := map[string]int64{}
	for _, c := range demoCategories {
		if existing, err := st.CategoryRepo().GetBySlug(ctx, c.Slug); err == nil {
			catIDs[c.Slug] = existing.ID
			continue
		}
		cat := &store.Category{Name: c.Name, Slug: c.Slug}
		if err := st.CategoryRepo().Create(ctx, cat); err != nil {
			return fmt.Errorf("创建分类 %s: %w", c.Slug, err)
		}
		catIDs[c.Slug] = cat.ID
	}

	// 演示文章（每分类 2-3 篇，zh 先建 + en 同组翻译）
	type demoArticle struct {
		Slug    string
		ZhTitle string
		ZhBody  string
		EnTitle string
		EnBody  string
	}
	demoContent := map[string][]demoArticle{
		"news": {
			{"news-1", "公司发布新一代产品", "<p>我们很高兴宣布新一代产品正式发布，带来更强大的性能与更友好的体验。</p>", "Company Launches Next-Gen Product", "<p>We are excited to announce the launch of our next-generation product with improved performance and UX.</p>"},
			{"news-2", "与行业伙伴达成战略合作", "<p>本次合作将整合双方优势，为用户提供更完整的解决方案。</p>", "Strategic Partnership Announced", "<p>This partnership combines our strengths to deliver a more complete solution.</p>"},
		},
		"about": {
			{"about-1", "关于我们", "<p>我们是一支专注创新的团队，致力于用技术创造价值。</p>", "About Us", "<p>We are an innovative team dedicated to creating value through technology.</p>"},
		},
		"products": {
			{"products-1", "旗舰产品一览", "<p>我们的旗舰产品系列涵盖多种场景，满足不同需求。</p>", "Flagship Products Overview", "<p>Our flagship product line covers multiple scenarios.</p>"},
			{"products-2", "新品评测", "<p>第三方评测对新产品给出了高度评价。</p>", "New Product Review", "<p>Third-party reviewers gave high marks to our new product.</p>"},
		},
	}
	for catSlug, arts := range demoContent {
		for _, a := range arts {
			if _, err := svc.GetPublishedBySlugLang(ctx, "article", a.Slug, "zh"); err == nil {
				continue // 已存在
			}
			cover := downloadCover(ctx, med, "https://picsum.photos/seed/"+a.Slug+"/800/500")
			e, err := svc.Create(ctx, "article", "zh", map[string]any{
				"title": a.ZhTitle, "slug": a.Slug,
				"excerpt": firstSentence(a.ZhBody), "content": a.ZhBody,
				"category": fmt.Sprintf("%d", catIDs[catSlug]),
				"cover":    cover,
			}, 0)
			if err != nil {
				return fmt.Errorf("创建演示文章 %s: %w", a.Slug, err)
			}
			if err := svc.SetStatus(ctx, e.Content.ID, "published"); err != nil {
				return err
			}
			te, err := svc.CreateTranslation(ctx, "article", "en", e.Content.ContentID, map[string]any{
				"title": a.EnTitle, "slug": a.Slug + "-en",
				"excerpt": firstSentence(a.EnBody), "content": a.EnBody,
				"category": fmt.Sprintf("%d", catIDs[catSlug]),
				"cover":    cover,
			}, content.Actor{UserID: 0, IsModerator: true})
			if err != nil {
				return fmt.Errorf("创建演示翻译 %s-en: %w", a.Slug, err)
			}
			if err := svc.SetStatus(ctx, te.Content.ID, "published"); err != nil {
				return err
			}
		}
	}

	// hello-zh/en 归入 news 分类（若未设分类）
	if e, err := svc.GetPublishedBySlugLang(ctx, "article", "hello-zh", "zh"); err == nil {
		if e.Fields["category"] == nil {
			// 更新 payload 加分类——用 svc.Update 重存
			fields := e.Fields
			fields["category"] = fmt.Sprintf("%d", catIDs["news"])
			if _, err := svc.Update(ctx, e.Content.ID, fields); err != nil {
				return err
			}
		}
	}

	// main 菜单（zh/en 幂等）
	if err := seedMainMenu(ctx, st, "zh", catIDs); err != nil {
		return err
	}
	if err := seedMainMenu(ctx, st, "en", catIDs); err != nil {
		return err
	}
	return nil
}

// seedMainMenu 建 zh/en 的 main 菜单（幂等）。
func seedMainMenu(ctx context.Context, st store.Store, lang string, catIDs map[string]int64) error {
	if menus, err := st.MenuRepo().ListByLang(ctx, lang); err == nil {
		for _, m := range menus {
			if m.Name == "main" {
				return nil // 已有
			}
		}
	}
	labels := map[string]struct{ Zh, En string }{
		"news":     {"新闻", "News"},
		"about":    {"关于", "About"},
		"products": {"产品", "Products"},
	}
	home := "首页"
	if lang == "en" {
		home = "Home"
	}
	items := []store.MenuItem{{Label: home, Type: "home", URL: ""}}
	for _, slug := range []string{"news", "about", "products"} {
		items = append(items, store.MenuItem{Label: labels[slug].Zh, Type: "custom", URL: "/category/" + slug})
	}
	if lang == "en" {
		items = []store.MenuItem{{Label: home, Type: "home", URL: ""},
			{Label: "News", Type: "custom", URL: "/category/news"},
			{Label: "About", Type: "custom", URL: "/category/about"},
			{Label: "Products", Type: "custom", URL: "/category/products"}}
	}
	itemsJSON, _ := json.Marshal(items)
	return st.MenuRepo().Create(ctx, &store.Menu{Name: "main", Lang: lang, Items: string(itemsJSON)})
}
```
`firstSentence` helper（`internal/seed/seed.go`）：
```go
func firstSentence(s string) string {
	// 取正文第一句作为摘要（剥 HTML 标签的简化版）
	if i := strings.Index(s, "。"); i > 0 {
		s = s[:i+1]
	}
	s = strings.TrimPrefix(s, "<p>")
	s = strings.TrimSuffix(s, "</p>")
	return s
}
```
> 需 import `strings`、`dulizhan/internal/media`。`store.MenuItem` 已存在（分类系统任务定义）。

- [ ] **步骤 4：main.go seed 分支适配**（`cmd/dulizhan/main.go`）

seed 分支（line 44-54）在 EnsureAuth 前构造 med 并传参：
```go
	authSvc := auth.New(st, 7*24*time.Hour)
	svc := content.New(st, schema.NewRegistry(), cfg.Site.Languages)
	// seed 分支：构造本地媒体（演示图片下载用），driver 无关
	seedMed := media.NewLocalStore(filepath.Join(cfg.Server.DataDir, "media"), "/media")
	ctx := context.Background()
	if err := seed.EnsureAuth(ctx, st, authSvc); err != nil {
		log.Fatalf("初始化角色失败: %v", err)
	}
	if err := seed.Run(ctx, st, svc, seedMed); err != nil {
		log.Fatalf("初始化数据失败: %v", err)
	}
```
> 正常路径的 `med` 构造不变（line 80-89），但 `seed.Run` 不在那里调用——**注意**：`Run` 只在 seed 子命令调用，正常启动不调用。所以只需改 seed 分支。

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/seed/ -run 'TestSeedDemoData|TestDownloadCover' -v`
预期：PASS

- [ ] **步骤 6：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿（其他包不调 seed.Run）

---

### 任务 3：默认主题展示（缩略图/大图/分类）

**文件：**
- 修改：`themes/default/templates/index.html`（缩略图）
- 修改：`themes/default/templates/list.html`（缩略图）
- 修改：`themes/default/templates/single.html`（大图 + 分类）
- 修改：`themes/default/static/css/main.css`（样式）

- [ ] **步骤 1：index.html 缩略图**

```html
      {{range .Items}}
      <li class="entry-item">
        {{if .Fields.cover}}<img src="{{media .Fields.cover}}" class="entry-thumb" alt="">{{end}}
        <a href="{{url $.Lang .}}">{{.Content.Title}}</a>
        {{if .Content.PublishedAt}}<time>{{.Content.PublishedAt}}</time>{{end}}
      </li>
      {{end}}
```

- [ ] **步骤 2：list.html 缩略图**

```html
    {{range .Items}}
    <li class="entry-item">
      {{if .Fields.cover}}<img src="{{media .Fields.cover}}" class="entry-thumb" alt="">{{end}}
      <a href="{{url $.Lang .}}">{{.Content.Title}}</a>
      {{if .Content.PublishedAt}}<time>{{.Content.PublishedAt}}</time>{{end}}
    </li>
    {{end}}
```

- [ ] **步骤 3：single.html 大图 + 分类**

在 `<article>` 内、`<h1>` 前加：
```html
    {{if .Entry.Fields.cover}}<img src="{{media .Entry.Fields.cover}}" class="entry-hero" alt="">{{end}}
    {{if .EntryCategory}}<div class="entry-category">分类：{{.EntryCategory}}</div>{{end}}
```

- [ ] **步骤 4：main.css 样式追加**

```css
.entry-item { display: flex; align-items: center; gap: 12px; }
.entry-thumb { width: 120px; height: 80px; object-fit: cover; border-radius: 4px; }
.entry-hero { width: 100%; max-height: 400px; object-fit: cover; border-radius: 8px; margin-bottom: 16px; }
.entry-category { display: inline-block; background: #f0f0f0; padding: 2px 10px; border-radius: 12px; font-size: 13px; color: var(--fg); margin-bottom: 8px; }
```

- [ ] **步骤 5：验证**

运行：`go test -count=1 ./internal/theme/ ./internal/server/ -v`
预期：全部 PASS（渲染不破；TestRenderIndex/TestFrontendMenus 等既有测试）

> **注意**：`media` 模板函数在 `Entry.Fields.cover` 为 URL 时——`mediaFunc` 对 http/绝对路径/`/media/` 透传，空串返回空。缩略图 `{{if .Fields.cover}}` 守卫空值。

- [ ] **步骤 6：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿（主题模板改动重编译）

---

### 任务 4：端到端验收

**文件：**
- 修改：`themes/default/templates/`（若冒烟发现调整）

- [ ] **步骤 1：seed 冒烟**

```bash
go run ./cmd/dulizhan seed
# 输出"种子数据初始化完成"（含分类/演示文章/菜单）
```

- [ ] **步骤 2：启动 + 前台验证**

```bash
go run ./cmd/dulizhan --config config.yaml
# 首页 / 含演示文章（缩略图）
curl http://localhost:8080/ | grep -o 'entry-thumb' | head -1
# 详情页含大图 + 分类
curl http://localhost:8080/article/news-1 | grep -o 'entry-hero'
curl http://localhost:8080/article/news-1 | grep -o 'entry-category'
# 分类归档
curl http://localhost:8080/category/news | grep -o 'news-1'
# 导航菜单（首页/新闻/关于/产品）
curl http://localhost:8080/ | grep -o '/category/news'
# 英文
curl http://localhost:8080/en/ | grep -o '/category/news'  # en 前缀
```
预期：首页缩略图、详情大图+分类、归档、导航分类链接、英文菜单全部渲染。

- [ ] **步骤 3：清理**

停服、删 `dulizhan.db`/`data/`/临时产物，8080 释放。

---

## 自检记录

**1. 规格覆盖度：**
- 规格 §2.1 签名 → 任务 2
- 规格 §2.2 分类 → 任务 2
- 规格 §2.3 双语文章 + hello-zh 归 news → 任务 2
- 规格 §2.4 封面下载 → 任务 1
- 规格 §3 主题展示（index/list/single）→ 任务 3
- 规格 §3.4 导航菜单 → 任务 2（seedMainMenu）
- 规格 §4 CSS → 任务 3
- 规格 §5 测试 → 任务 1/2/3
- 规格 §6 验收 → 任务 4

**2. 占位符扫描：** 无 TBD/TODO。demoContent 文章内容已写全（每篇 ZhTitle/ZhBody/EnTitle/EnBody 完整文本）。每步含代码。

**3. 类型一致性：**
- `Run(ctx, st, svc, med)` 任务 2 定义，main.go 调用一致
- `downloadCover(ctx, med, url)` 任务 1 定义，任务 2 使用
- `mockMediaStore` 任务 1 测试定义，任务 2 测试复用（同包）
- `store.MenuItem{Label,Type,URL,Children}` 分类系统已定义，seedMainMenu 使用
- `store.Category{Name,Slug}` 分类系统已定义，seedDemoData 使用
- 主题模板 `{{media .Fields.cover}}`/`{{.EntryCategory}}` 与 mediaFunc/Data.EntryCategory 一致
- hello-zh 归 news 用 `svc.Update`（ContentID 不变）——**注意**：Update 后 ContentID 保留（store Update 更新 content_id 字段，传回原值），翻译组不破
