# 阶段 3：Vue 管理端 SPA 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 构建 Vue 3 管理端 SPA（登录/仪表盘/内容管理+搜索/媒体库/菜单/设置），让浏览器全程可视化操作内容，构建产物 `go:embed` 进二进制由 `/admin` 托管。

**架构：** `admin/` 内独立 Vite 工程（api 封装/stores/views/dynamic-form/components 分层），开发时 vite 代理 `/api`、`/media`、`/themes` 到后端 8080；生产 `vite build` 产物复制到 `internal/server/dist/`，Go 侧 `//go:embed dist/*` 注册 `/admin` 路由（含 SPA fallback）。后端新增 `/api/search`、`/api/stats`、`/api/meta` 三个端点。

**技术栈：** Vue 3 + TypeScript + Vite、Element Plus、Vue Router、Pinia、wangEditor 5（`@wangeditor/editor-for-vue`）、Vitest（前端单测）；Go 侧沿用 gin@v1.10.0。

**前置基线：** 阶段 1/2 已恢复并验收通过（`go test -count=1 ./...` 全绿）。规格：`docs/superpowers/specs/2026-08-06-phase3-admin-spa-design.md`。

---

### 任务 1：后端 `/api/meta` 端点（站点信息 + 语言 + 主题）

**文件：**
- 修改：`internal/adminapi/deps.go`（Deps 增加 Cfg 字段）
- 修改：`internal/adminapi/register.go`（注册 `/api/meta`）
- 创建：`internal/adminapi/meta.go`
- 创建：`internal/adminapi/meta_test.go`
- 修改：`internal/server/server.go`（装配时传入 cfg）
- 测试：`internal/adminapi/adminapi_test.go`（适配 newEnv 构造）

**说明：** SPA 的多语言 Tab、筛选下拉、设置页需要 `languages`/`default_lang`/`theme`/站点信息，但现有 API 不暴露（这些来自 config.yaml）。这是规格 §6"最小修正（仅当 SPA 受阻）"。

- [ ] **步骤 1：编写失败测试**（`internal/adminapi/meta_test.go`）

```go
package adminapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestMetaEndpoint(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	w := e.do(t, http.MethodGet, "/api/meta", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("meta = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			SiteName    string   `json:"site_name"`
			Languages   []string `json:"languages"`
			DefaultLang string   `json:"default_lang"`
			Theme       string   `json:"theme"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.DefaultLang != "zh" {
		t.Errorf("default_lang = %q, want zh", resp.Data.DefaultLang)
	}
	if len(resp.Data.Languages) != 2 {
		t.Errorf("languages = %v", resp.Data.Languages)
	}
	if resp.Data.Theme == "" {
		t.Error("theme 为空")
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/adminapi/ -run TestMetaEndpoint -v`
预期：FAIL（`/api/meta` 未定义，404）

- [ ] **步骤 3：Deps 增加 Cfg 字段**

修改 `internal/adminapi/deps.go`：

```go
type Deps struct {
	Store   store.Store
	Auth    *auth.Service
	Content *content.Service
	Media   media.MediaStore
	Cfg     *config.Config
}
```
新增 import `dulizhan/internal/config`。

- [ ] **步骤 4：实现 meta handler**（`internal/adminapi/meta.go`）

```go
package adminapi

import "github.com/gin-gonic/gin"

// HandleMeta 返回 SPA 需要的站点配置信息（语言、默认语言、主题、站点名等）。
func (d *Deps) HandleMeta(c *gin.Context) {
	respondOK(c, gin.H{
		"site_name":    d.Cfg.Site.Name,
		"site_url":     d.Cfg.Site.URL,
		"description":  d.Cfg.Site.Description,
		"languages":    d.Cfg.Site.Languages,
		"default_lang": d.Cfg.Site.DefaultLang,
		"theme":        d.Cfg.Site.Theme,
	})
}
```

- [ ] **步骤 5：注册路由**

`internal/adminapi/register.go` 在 auth 路由后追加：

```go
	g.GET("/meta", auth.RequireAuth(d.Auth), d.HandleMeta)
```

- [ ] **步骤 6：装配适配**

- `internal/server/server.go` `registerRoutes` 中 `adminapi.Register(api, adminapi.Deps{Store: s.store, Auth: s.auth, Content: s.content, Media: s.media, Cfg: s.cfg})`
- `internal/adminapi/adminapi_test.go` `newEnv` 的 `Deps` 构造补 `Cfg: &config.Config{}` 并设 `Cfg.Site.Name="测试站点"`、`Cfg.Site.URL="https://example.com"`、`Cfg.Site.Description="描述"`、`Cfg.Site.DefaultLang="zh"`、`Cfg.Site.Languages=[]string{"zh","en"}`、`Cfg.Site.Theme="default"`（需新增 import `dulizhan/internal/config`）

- [ ] **步骤 7：运行测试验证通过**

运行：`go test -count=1 ./internal/adminapi/ -v`
预期：全部 PASS（含新 TestMetaEndpoint）

- [ ] **步骤 8：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 2：后端 `/api/search` 端点

**文件：**
- 修改：`internal/store/store.go`（ContentRepo 增加 SearchByTypeLang/CountSearch）
- 修改：`internal/store/sqlite/sqlite.go`（实现两个方法）
- 修改：`internal/store/sqlite/sqlite_test.go`（补测试）
- 修改：`internal/content/service.go`（Service.Search）
- 修改：`internal/content/service_test.go`（补测试）
- 创建：`internal/adminapi/search.go`（HandleSearch）
- 修改：`internal/adminapi/register.go`（注册路由）
- 创建：`internal/adminapi/search_test.go`
- 测试：三层的对应测试

- [ ] **步骤 1：编写失败测试（sqlite 层）**（追加到 `internal/store/sqlite/sqlite_test.go`）

```go
func TestContentSearch(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	if err := types.Create(ctx, ct); err != nil {
		t.Fatal(err)
	}
	repo := st.ContentRepo()
	for _, c := range []*store.Content{
		{ContentTypeID: ct.ID, ContentID: "g1", Lang: "zh", Slug: "hello-world", Title: "你好世界", Status: "published"},
		{ContentTypeID: ct.ID, ContentID: "g2", Lang: "zh", Slug: "other", Title: "另一篇", Status: "draft"},
		{ContentTypeID: ct.ID, ContentID: "g3", Lang: "en", Slug: "en-hello", Title: "Hello", Status: "published"},
	} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	// 按 title LIKE 命中
	list, err := repo.SearchByTypeLang(ctx, "article", "zh", "世界", 0, 10)
	if err != nil || len(list) != 1 || list[0].Slug != "hello-world" {
		t.Errorf("Search title = %d items, %v", len(list), err)
	}
	n, err := repo.CountSearch(ctx, "article", "zh", "世界")
	if err != nil || n != 1 {
		t.Errorf("CountSearch = %d, %v", n, err)
	}
	// 按 slug LIKE 命中
	list, _ = repo.SearchByTypeLang(ctx, "article", "zh", "hello", 0, 10)
	if len(list) != 1 || list[0].Slug != "hello-world" {
		t.Errorf("Search slug = %d items", len(list))
	}
	// 无结果
	list, _ = repo.SearchByTypeLang(ctx, "article", "zh", "zzz", 0, 10)
	if len(list) != 0 {
		t.Errorf("Search 无结果应为空, got %d", len(list))
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/store/sqlite/ -run TestContentSearch -v`
预期：FAIL（接口方法未定义）

- [ ] **步骤 3：接口扩展**

`internal/store/store.go` 的 ContentRepo 接口追加：

```go
	SearchByTypeLang(ctx context.Context, typeName, lang, q string, offset, limit int) ([]Content, error)
	CountSearch(ctx context.Context, typeName, lang, q string) (int, error)
```

- [ ] **步骤 4：实现 SQLite 方法**（`internal/store/sqlite/sqlite.go` 追加）

```go
func (r *contentRepo) SearchByTypeLang(ctx context.Context, typeName, lang, q string, offset, limit int) ([]store.Content, error) {
	query := "SELECT " + contentCols + " FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=? AND (c.title LIKE ? OR c.slug LIKE ?) ORDER BY c.id DESC LIMIT ? OFFSET ?"
	like := "%" + q + "%"
	rows, err := r.db.QueryContext(ctx, query, typeName, lang, like, like, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *contentRepo) CountSearch(ctx context.Context, typeName, lang, q string) (int, error) {
	var n int
	like := "%" + q + "%"
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=? AND (c.title LIKE ? OR c.slug LIKE ?)",
		typeName, lang, like, like).Scan(&n)
	return n, err
}
```

- [ ] **步骤 5：content 服务 Search 方法**（`internal/content/service.go` 追加）

```go
// Search 按关键词搜索某类型某语言的内容（title/slug LIKE 匹配）。
func (s *Service) Search(ctx context.Context, typeName, lang, q string, page, perPage int) ([]Entry, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if _, err := s.GetType(ctx, typeName); err != nil {
		return nil, 0, err
	}
	rows, err := s.store.ContentRepo().SearchByTypeLang(ctx, typeName, lang, q, (page-1)*perPage, perPage)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.ContentRepo().CountSearch(ctx, typeName, lang, q)
	if err != nil {
		return nil, 0, err
	}
	ct, _ := s.GetType(ctx, typeName)
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		e, err := s.entryFromStore(ctx, ct, row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, nil
}
```

- [ ] **步骤 6：content 层测试**（追加到 `internal/content/service_test.go`）

```go
func TestSearch(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	for _, c := range []map[string]any{
		{"title": "你好世界", "slug": "hello"},
		{"title": "另一篇", "slug": "other"},
	} {
		if _, err := svc.Create(ctx, "article", "zh", c, 1); err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := svc.Search(ctx, "article", "zh", "世界", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].Content.Slug != "hello" {
		t.Errorf("Search = %d items, total=%d, %+v", len(items), total, items)
	}
	// 未知类型 → ErrNotFound
	if _, _, err := svc.Search(ctx, "nope", "zh", "x", 1, 10); err != errs.ErrNotFound {
		t.Errorf("未知类型 = %v, want ErrNotFound", err)
	}
}
```

- [ ] **步骤 7：实现 adminapi handler**（`internal/adminapi/search.go`）

```go
package adminapi

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// HandleSearch 内容搜索：GET /api/search?type=&lang=&q=&page=&per_page=
func (d *Deps) HandleSearch(c *gin.Context) {
	typeName := c.Query("type")
	if typeName == "" {
		badRequest(c, "缺少 type 参数")
		return
	}
	lang := c.Query("lang")
	if lang == "" {
		badRequest(c, "缺少 lang 参数")
		return
	}
	q := c.Query("q")
	if q == "" {
		badRequest(c, "缺少 q 参数")
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.Query("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	items, total, err := d.Content.Search(c.Request.Context(), typeName, lang, q, page, perPage)
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": items, "total": total})
}
```

- [ ] **步骤 8：注册路由 + adminapi 测试**

`internal/adminapi/register.go` 在 content 路由后追加：

```go
	g.GET("/search", auth.RequireAuth(d.Auth), auth.RequirePerm("content.read"), d.HandleSearch)
```

`internal/adminapi/search_test.go`：

```go
package adminapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestSearchEndpoint(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// seed 已有 article/hello-zh（标题 "你好，世界"）
	w := e.do(t, http.MethodGet, "/api/search?type=article&lang=zh&q=%E4%B8%96%E7%95%8C", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("search = %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello-zh") {
		t.Errorf("搜索应命中 seed 文章: %s", w.Body.String())
	}
	// 缺 q → 400
	w = e.do(t, http.MethodGet, "/api/search?type=article&lang=zh", "", tok)
	if w.Code != http.StatusBadRequest {
		t.Errorf("缺 q = %d, want 400", w.Code)
	}
	// 未登录 → 401
	w = e.do(t, http.MethodGet, "/api/search?type=article&lang=zh&q=x", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("未登录 = %d, want 401", w.Code)
	}
}
```

- [ ] **步骤 9：运行测试验证通过**

运行：`go test -count=1 ./internal/store/sqlite/ ./internal/content/ ./internal/adminapi/ -v`
预期：全部 PASS

- [ ] **步骤 10：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 3：后端 `/api/stats` 端点（仪表盘）

**文件：**
- 修改：`internal/content/service.go`（Service.Stats）
- 修改：`internal/content/service_test.go`（补测试）
- 创建：`internal/adminapi/stats.go`（HandleStats）
- 修改：`internal/adminapi/register.go`（注册路由）
- 创建：`internal/adminapi/stats_test.go`

- [ ] **步骤 1：编写失败测试（content 层）**（追加到 `internal/content/service_test.go`）

```go
type typeStat struct {
	TypeName  string `json:"type_name"`
	Published int    `json:"published"`
	Draft     int    `json:"draft"`
}

type statsResult struct {
	ByType []typeStat  `json:"by_type"`
	Recent []Entry     `json:"recent"`
}

func TestStats(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	e1, err := svc.Create(ctx, "article", "zh", map[string]any{"title": "a", "slug": "a"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	svc.Create(ctx, "article", "zh", map[string]any{"title": "b", "slug": "b"}, 1)
	if err := svc.SetStatus(ctx, e1.Content.ID, "published"); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.ByType) != 1 || st.ByType[0].TypeName != "article" {
		t.Fatalf("ByType = %+v", st.ByType)
	}
	if st.ByType[0].Published != 1 || st.ByType[0].Draft != 1 {
		t.Errorf("stats = %+v", st.ByType[0])
	}
	if len(st.Recent) == 0 {
		t.Error("recent 为空")
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/content/ -run TestStats -v`
预期：FAIL（`svc.Stats` 未定义）

- [ ] **步骤 3：实现 Service.Stats**（`internal/content/service.go` 追加）

```go
type TypeStat struct {
	TypeName  string `json:"type_name"`
	Published int    `json:"published"`
	Draft     int    `json:"draft"`
}

type Stats struct {
	ByType []TypeStat `json:"by_type"`
	Recent []Entry    `json:"recent"`
}

// Stats 返回仪表盘数据：各内容类型 published/draft 计数 + 最近内容。
func (s *Service) Stats(ctx context.Context) (Stats, error) {
	types, err := s.AllTypes(ctx)
	if err != nil {
		return Stats{}, err
	}
	st := Stats{ByType: make([]TypeStat, 0, len(types))}
	for _, ct := range types {
		pub, err := s.store.ContentRepo().CountByTypeLangStatus(ctx, ct.Name, "", "published")
		if err != nil {
			return Stats{}, err
		}
		draft, err := s.store.ContentRepo().CountByTypeLangStatus(ctx, ct.Name, "", "draft")
		if err != nil {
			return Stats{}, err
		}
		st.ByType = append(st.ByType, TypeStat{TypeName: ct.Name, Published: pub, Draft: draft})
		// 最近发布：默认语言取每类型前 5
		items, _, err := s.ListAdmin(ctx, ct.Name, "", "published", 1, 5)
		if err != nil {
			return Stats{}, err
		}
		st.Recent = append(st.Recent, items...)
	}
	return st, nil
}
```

> 注意：`CountByTypeLangStatus` 与 `ListAdmin` 的 lang 传 `""` 时，sqlite 层 `status` 空不过滤（task 2 恢复实现），但 **lang 空会匹配不到**（`AND c.lang=?` 空串）。因此需调整：统计用默认语言（"zh"）。**实现时**：Stats 中 lang 固定传 `cfg 默认语言`，但 service 无 cfg——**改为遍历所有 i18n 语言做合计**。更简洁方案：sqlite 层新增 `CountByType(ctx, typeName, status)`（不按 lang），service 遍历 languages 合计。见步骤 3b。

- [ ] **步骤 3b：修正 Stats 实现（按语言遍历合计，不依赖默认语言）**

由于 service 无 cfg/languages，改为**遍历 all langs 由 store 层按 lang 聚合**。给 ContentRepo 增加：

`internal/store/store.go` ContentRepo 接口追加：
```go
	CountByTypeStatus(ctx context.Context, typeName, status string) (int, error)
```

`internal/store/sqlite/sqlite.go` 实现：
```go
func (r *contentRepo) CountByTypeStatus(ctx context.Context, typeName, status string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.status=?",
		typeName, status).Scan(&n)
	return n, err
}
```

修正后的 `Service.Stats`：
```go
func (s *Service) Stats(ctx context.Context) (Stats, error) {
	types, err := s.AllTypes(ctx)
	if err != nil {
		return Stats{}, err
	}
	st := Stats{ByType: make([]TypeStat, 0, len(types))}
	for _, ct := range types {
		pub, err := s.store.ContentRepo().CountByTypeStatus(ctx, ct.Name, "published")
		if err != nil {
			return Stats{}, err
		}
		draft, err := s.store.ContentRepo().CountByTypeStatus(ctx, ct.Name, "draft")
		if err != nil {
			return Stats{}, err
		}
		st.ByType = append(st.ByType, TypeStat{TypeName: ct.Name, Published: pub, Draft: draft})
		// 最近发布：该类型最近 5 条（跨语言，取最新）
		rows, err := s.store.ContentRepo().ListByTypeLangStatus(ctx, ct.Name, "", "published", 0, 5)
		if err != nil {
			return Stats{}, err
		}
		for _, row := range rows {
			e, err := s.entryFromStore(ctx, ct, row)
			if err != nil {
				return Stats{}, err
			}
			st.Recent = append(st.Recent, e)
		}
	}
	return st, nil
}
```
> `ListByTypeLangStatus` 的 lang 传 `""`：sqlite 实现 `WHERE t.name=? AND c.lang=?`（空串无匹配）。**需改为 lang 空不过滤**：修改 `ListByTypeLangStatus`/`CountByTypeLangStatus` 动态拼 WHERE 时 `lang != ""` 才加条件（与 task 2 恢复的 status 处理一致）。在 sqlite_test.go 补空 lang 测试。

- [ ] **步骤 3c：sqlite 空 lang 不过滤 + 测试**

修改 `internal/store/sqlite/sqlite.go` 的 `ListByTypeLangStatus`/`CountByTypeLangStatus`：
```go
func (r *contentRepo) ListByTypeLangStatus(ctx context.Context, typeName, lang, status string, offset, limit int) ([]store.Content, error) {
	query := "SELECT " + contentCols + " FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=?"
	args := []any{typeName}
	if lang != "" {
		query += " AND c.lang=?"
		args = append(args, lang)
	}
	if status != "" {
		query += " AND c.status=?"
		args = append(args, status)
	}
	query += " ORDER BY c.published_at DESC, c.id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	// ... 其余同现有
}
// CountByTypeLangStatus 同理：lang/status 空则不过滤
```
补测试 `TestListByTypeLangStatusEmptyFilter`：不传 lang/status 返回全部。

- [ ] **步骤 4：adminapi handler**（`internal/adminapi/stats.go`）

```go
package adminapi

import "github.com/gin-gonic/gin"

// HandleStats 仪表盘统计：GET /api/stats
func (d *Deps) HandleStats(c *gin.Context) {
	st, err := d.Content.Stats(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, st)
}
```

- [ ] **步骤 5：注册路由 + 测试**

`internal/adminapi/register.go` 追加：
```go
	g.GET("/stats", auth.RequireAuth(d.Auth), auth.RequirePerm("content.read"), d.HandleStats)
```

`internal/adminapi/stats_test.go`：
```go
package adminapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestStatsEndpoint(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	w := e.do(t, http.MethodGet, "/api/stats", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("stats = %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "article") {
		t.Errorf("stats 缺类型: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "recent") {
		t.Errorf("stats 缺 recent: %s", w.Body.String())
	}
}
```

- [ ] **步骤 6：运行测试验证通过**

运行：`go test -count=1 ./internal/store/sqlite/ ./internal/content/ ./internal/adminapi/ -v`
预期：全部 PASS

- [ ] **步骤 7：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 4：`/admin` go:embed 集成（Go 侧）

**文件：**
- 创建：`internal/server/embed.go`
- 创建：`internal/server/dist/.gitkeep`（占位，保证目录存在）
- 修改：`internal/server/server.go`（注册 /admin 路由）
- 创建：`internal/server/admin_test.go`

- [ ] **步骤 1：创建 embed.go**

```go
package server

import "embed"

//go:embed dist/*
var adminFS embed.FS
```

> 若 `internal/server/dist/` 为空目录，`go build` 会因 `dist/*` 无匹配失败。故先创建 `internal/server/dist/.gitkeep`（空文件使 embed 模式 `dist/*` 匹配到它）。实现后再由构建脚本复制真实产物。

- [ ] **步骤 2：创建 dist/.gitkeep**

```powershell
New-Item -ItemType File -Path "internal\server\dist\.gitkeep" -Force
```

- [ ] **步骤 3：编写失败测试**（`internal/server/admin_test.go`）

```go
package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminServed(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	// /admin 返回 index.html 或 200（SPA fallback）
	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	srv.engine.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("/admin = %d", w.Code)
	}
}
```

- [ ] **步骤 4：运行确认失败**

运行：`go test -count=1 ./internal/server/ -run TestAdminServed -v`
预期：FAIL（/admin 落到 NoRoute 前台渲染，返回非预期）

- [ ] **步骤 5：实现 admin 路由**

`internal/server/server.go` 新增方法并在 `registerRoutes` 中注册（**在 NoRoute 之前**）：

```go
import "io/fs"

// adminIndex 从嵌入 FS 读 index.html。
func adminIndex() ([]byte, error) {
	b, err := adminFS.ReadFile("dist/index.html")
	if err != nil {
		return nil, err
	}
	return b, nil
}

// handleAdmin 服务 SPA：/admin 与 /admin/* 返回 index.html（前端路由 fallback），/admin/assets/* 服务静态。
func (s *Server) handleAdmin(c *gin.Context) {
	sub, err := fs.Sub(adminFS, "dist")
	if err != nil {
		c.String(http.StatusInternalServerError, "admin 资源不可用")
		return
	}
	if c.Request.URL.Path == "/admin" || c.Request.URL.Path == "/admin/" {
		c.Status(http.StatusOK)
		if b, e := adminIndex(); e == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", b)
			return
		}
	}
	// /admin/assets/* 或前端路由
	rest := ""
	if len(c.Request.URL.Path) > len("/admin/") {
		rest = c.Request.URL.Path[len("/admin/"):]
	}
	if rest != "" && rest != "index.html" {
		if b, err := fs.ReadFile(sub, rest); err == nil {
			c.Data(http.StatusOK, mimeTypeByExt(rest), b)
			return
		}
	}
	// 前端路由 fallback → index.html
	if b, e := adminIndex(); e == nil {
		c.Data(http.StatusOK, "text/html; charset=utf-8", b)
		return
	}
	c.Status(http.StatusNotFound)
}

func mimeTypeByExt(path string) string {
	switch {
	case strings.HasSuffix(path, ".js"):
		return "application/javascript"
	case strings.HasSuffix(path, ".css"):
		return "text/css"
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	default:
		return "text/plain"
	}
}
```

`registerRoutes` 中在 `eng.NoRoute(s.handleFrontend)` **之前**加：
```go
	eng.GET("/admin", s.handleAdmin)
	eng.GET("/admin/*path", s.handleAdmin)
```
需新增 import `io/fs`、`strings`。

- [ ] **步骤 6：编写 index.html 测试夹具**

`internal/server/admin_test.go` 追加（验证 fallback 与静态服务逻辑）——由于 embed 的是 dist 空目录（仅 .gitkeep），无法直接测真实 index.html。**改用单元测试直接测 handleAdmin 对不存在的 rest 仍返回 404**，并在构建脚本产出真实 dist 后由冒烟验证。**调整步骤 3 测试为**：测试 `adminIndex` 在 dist 无 index.html 时返回错误（当前预期），确保不会 panic；真实 /admin 渲染由任务 14 冒烟覆盖。

修改 `TestAdminServed`：
```go
func TestAdminIndexMissing(t *testing.T) {
	// dist 尚无 index.html（占位阶段），adminIndex 应返回错误而非 panic
	if _, err := adminIndex(); err == nil {
		t.Log("dist/index.html 已存在（构建产物就绪）")
	} else {
		t.Log("dist/index.html 未构建（占位阶段），返回错误属预期")
	}
}
```
并保留 `TestAdminServed` 但允许两种结果（有产物 200 / 无产物 404），不强制：
```go
func TestAdminRouteRegistered(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	srv.engine.ServeHTTP(w, r)
	// 产物存在→200；未构建→404。两者都不该 500/panic。
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Fatalf("/admin = %d", w.Code)
	}
}
```

- [ ] **步骤 7：运行测试验证通过**

运行：`go test -count=1 ./internal/server/ -v`
预期：全部 PASS（含新测试；无产物阶段允许 404）

- [ ] **步骤 8：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 5：前端脚手架（admin/ 工程初始化）

**文件：**
- 创建：`admin/package.json`
- 创建：`admin/tsconfig.json`、`admin/tsconfig.node.json`
- 创建：`admin/vite.config.ts`
- 创建：`admin/index.html`
- 创建：`admin/src/main.ts`、`admin/src/App.vue`
- 创建：`admin/src/vite-env.d.ts`
- 创建：`admin/.gitignore`

- [ ] **步骤 1：初始化 package.json**

```json
{
  "name": "dulizhan-admin",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview",
    "test": "vitest run"
  },
  "dependencies": {
    "@wangeditor/editor": "^5.1.23",
    "@wangeditor/editor-for-vue": "^5.1.12",
    "element-plus": "^2.7.0",
    "pinia": "^2.1.7",
    "vue": "^3.4.0",
    "vue-router": "^4.3.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.0.0",
    "typescript": "^5.4.0",
    "vite": "^5.2.0",
    "vitest": "^1.6.0",
    "@vue/test-utils": "^2.4.0",
    "jsdom": "^24.0.0",
    "vue-tsc": "^2.0.0"
  }
}
```

- [ ] **步骤 2：安装依赖**

```bash
cd admin
npm install
```

- [ ] **步骤 3：vite.config.ts**

```ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/media': 'http://localhost:8080',
      '/themes': 'http://localhost:8080',
    },
  },
})
```

- [ ] **步骤 4：tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "esModuleInterop": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "types": ["vite/client"],
    "noEmit": true
  },
  "include": ["src/**/*.ts", "src/**/*.d.ts", "src/**/*.vue"]
}
```

- [ ] **步骤 5：index.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Dulizhan CMS 管理端</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **步骤 6：main.ts + App.vue + vite-env.d.ts + .gitignore**

`src/vite-env.d.ts`：
```ts
/// <reference types="vite/client" />
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
```

`src/main.ts`：
```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import App from './App.vue'
import router from './router'

createApp(App)
  .use(createPinia())
  .use(router)
  .use(ElementPlus)
  .mount('#app')
```

`src/App.vue`（占位，后续替换为布局壳）：
```vue
<template>
  <router-view />
</template>
```

`src/router/index.ts`（占位，任务 6 完善）：
```ts
import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory('/admin/'),
  routes: [],
})

export default router
```

`admin/.gitignore`：
```
node_modules/
dist/
```

- [ ] **步骤 7：验证可构建**

运行：`cd admin && npx vue-tsc --noEmit && npx vite build`
预期：构建成功（空的 router 表）

> 说明：`router` 的 `createWebHistory('/admin/')` 基路径需与生产 `/admin` 前缀一致；dev 模式 vite 代理下浏览器访问 `http://localhost:5173` 时基路径 `/admin/` 会造成路由怪癖——**dev 下改为 `createWebHistory()`** 由任务 6 处理（用 `import.meta.env` 区分）。实现时确认。

---

### 任务 6：前端 API 客户端与认证（client.ts + auth store + 登录页）

**文件：**
- 创建：`admin/src/api/client.ts`
- 创建：`admin/src/api/auth.ts`
- 创建：`admin/src/api/meta.ts`
- 创建：`admin/src/stores/auth.ts`
- 创建：`admin/src/router/index.ts`（完善路由 + 守卫）
- 创建：`admin/src/views/LoginView.vue`
- 修改：`admin/src/App.vue`（布局壳 + 侧边栏）
- 创建：`admin/src/components/LayoutSidebar.vue`
- 创建：`admin/src/api/__tests__/client.test.ts`（Vitest）

- [ ] **步骤 1：client.ts**（fetch 封装）

```ts
import { useAuthStore } from '../stores/auth'

const BASE = '/api'

export interface ApiError {
  code: number
  message: string
}

export async function request<T>(path: string, opts: { method?: string; body?: unknown } = {}): Promise<T> {
  const auth = useAuthStore()
  const headers: Record<string, string> = {}
  if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  const token = auth.token
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  const res = await fetch(`${BASE}${path}`, {
    method: opts.method ?? 'GET',
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })
  const data = await res.json().catch(() => ({ code: res.status, message: '响应解析失败' }))
  if (!res.ok) {
    if (res.status === 401) {
      auth.clear()
      // 跳登录
      if (window.location.pathname !== '/admin/login') {
        window.location.href = '/admin/login'
      }
    }
    const err: ApiError = { code: data.code ?? res.status, message: data.message ?? '请求失败' }
    throw err
  }
  return data.data as T
}
```

> 注意：`useAuthStore()` 在模块顶层调用需在 pinia 激活后。fetch 拦截 401 依赖 `window`（浏览器环境）；Vitest 中用 jsdom。`/admin` 基路径：dev 下 base 为空路径，生产为 `/admin`——`request` 只拼 `/api`，登录跳转用相对判断，dev/prod 由 router 基路径差异处理。实现时统一。

- [ ] **步骤 2：auth store**（`src/stores/auth.ts`）

```ts
import { defineStore } from 'pinia'

interface AuthState {
  token: string | null
  username: string | null
  role: string | null
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    token: localStorage.getItem('dlz_token'),
    username: null,
    role: null,
  }),
  getters: {
    isLoggedIn: (s) => !!s.token,
    isAdmin: (s) => s.role === 'admin',
  },
  actions: {
    setToken(t: string) {
      this.token = t
      localStorage.setItem('dlz_token', t)
    },
    setUser(u: { username: string; role: string }) {
      this.username = u.username
      this.role = u.role
    },
    clear() {
      this.token = null
      this.username = null
      this.role = null
      localStorage.removeItem('dlz_token')
    },
  },
})
```

- [ ] **步骤 3：api/auth.ts + api/meta.ts**

```ts
// src/api/auth.ts
import { request } from './client'

export interface MeResponse {
  user: { id: number; username: string; role: string }
}

export const login = (username: string, password: string) =>
  request<{ token: string }>('/auth/login', { method: 'POST', body: { username, password } })
export const logout = () => request<{ logged_out: boolean }>('/auth/logout', { method: 'POST' })
export const me = () => request<MeResponse>('/auth/me')
```

```ts
// src/api/meta.ts
import { request } from './client'

export interface MetaData {
  site_name: string
  site_url: string
  description: string
  languages: string[]
  default_lang: string
  theme: string
}

export const fetchMeta = () => request<MetaData>('/meta')
```

- [ ] **步骤 4：router + 守卫**（`src/router/index.ts`）

```ts
import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const base = import.meta.env.DEV ? '/' : '/admin/'

const router = createRouter({
  history: createWebHistory(base),
  routes: [
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue') },
    {
      path: '/',
      component: () => import('../AppShell.vue'),
      children: [
        { path: '', name: 'dashboard', component: () => import('../views/DashboardView.vue'), meta: { auth: true } },
        { path: 'content', name: 'content-list', component: () => import('../views/ContentListView.vue'), meta: { auth: true } },
        { path: 'content/new', name: 'content-new', component: () => import('../views/ContentEditView.vue'), meta: { auth: true } },
        { path: 'content/:id', name: 'content-edit', component: () => import('../views/ContentEditView.vue'), meta: { auth: true } },
        { path: 'media', name: 'media', component: () => import('../views/MediaLibraryView.vue'), meta: { auth: true } },
        { path: 'menus', name: 'menus', component: () => import('../views/MenuManageView.vue'), meta: { auth: true } },
        { path: 'settings', name: 'settings', component: () => import('../views/SettingsView.vue'), meta: { auth: true, adminOnly: true } },
      ],
    },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.auth && !auth.isLoggedIn) {
    return { name: 'login' }
  }
  if (to.meta.adminOnly && !auth.isAdmin) {
    return { name: 'dashboard' }
  }
  if (to.name === 'login' && auth.isLoggedIn) {
    return { name: 'dashboard' }
  }
  return true
})

export default router
```

> 注意：页面组件（DashboardView 等）在后续任务创建，此步路由先写全，但组件未创建前 build 会失败。**实现时**：先创建占位页面组件（空模板），后续任务逐个填充。`AppShell.vue` 是带侧边栏的布局（第 6 步）。

- [ ] **步骤 5：LoginView.vue**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { login, me } from '../api/auth'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const username = ref('')
const password = ref('')
const loading = ref(false)

async function onSubmit() {
  if (!username.value || !password.value) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    const { token } = await login(username.value, password.value)
    auth.setToken(token)
    const { user } = await me()
    auth.setUser({ username: user.username, role: user.role })
    router.push('/')
  } catch (e: any) {
    ElMessage.error(e.message ?? '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <h2>Dulizhan CMS</h2>
      <el-form @submit.prevent="onSubmit">
        <el-form-item>
          <el-input v-model="username" placeholder="用户名" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="password" type="password" placeholder="密码" show-password @keyup.enter="onSubmit" />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" style="width: 100%">登录</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.login-wrap { display: flex; align-items: center; justify-content: center; min-height: 100vh; }
.login-card { width: 360px; }
</style>
```

- [ ] **步骤 6：AppShell.vue + LayoutSidebar.vue + 更新 App.vue**

`src/App.vue`：
```vue
<template>
  <router-view />
</template>
```

`src/AppShell.vue`（带侧边栏布局，登录守卫后容器）：
```vue
<script setup lang="ts">
import { useAuthStore } from '../stores/auth'
import { useRouter } from 'vue-router'
import LayoutSidebar from './components/LayoutSidebar.vue'

const auth = useAuthStore()
const router = useRouter()

function onLogout() {
  auth.clear()
  router.push('/login')
}
</script>

<template>
  <el-container class="shell">
    <LayoutSidebar />
    <el-container>
      <el-header class="shell-header">
        <div class="user-info">
          <span>{{ auth.username }}</span>
          <el-button link type="primary" @click="onLogout">退出</el-button>
        </div>
      </el-header>
      <el-main><router-view /></el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.shell { min-height: 100vh; }
.shell-header { display: flex; justify-content: flex-end; align-items: center; border-bottom: 1px solid #eee; }
.user-info { display: flex; gap: 12px; align-items: center; }
</style>
```

`src/components/LayoutSidebar.vue`：
```vue
<script setup lang="ts">
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const menus = [
  { path: '/', label: '仪表盘', adminOnly: false },
  { path: '/content', label: '内容管理', adminOnly: false },
  { path: '/media', label: '媒体库', adminOnly: false },
  { path: '/menus', label: '菜单管理', adminOnly: false },
  { path: '/settings', label: '设置', adminOnly: true },
]
</script>

<template>
  <el-aside width="200px" class="sidebar">
    <el-menu :default-active="$route.path" router>
      <el-menu-item v-for="m in menus" :key="m.path" :index="m.path" v-if="!m.adminOnly || auth.isAdmin">
        {{ m.label }}
      </el-menu-item>
    </el-menu>
  </el-aside>
</template>

<style scoped>
.sidebar { border-right: 1px solid #eee; }
</style>
```

- [ ] **步骤 7：Vitest 测试（client 401 拦截）**（`src/api/__tests__/client.test.ts`）

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { request } from '../client'
import { useAuthStore } from '../../stores/auth'

describe('client request', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  it('带 token 并解析 data', async () => {
    const auth = useAuthStore()
    auth.setToken('tok-1')
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ code: 200, data: { ok: 1 } }),
    }) as any
    const data = await request<{ ok: number }>('/x')
    expect(data.ok).toBe(1)
    const [, init] = (global.fetch as any).mock.calls[0]
    expect(init.headers.Authorization).toBe('Bearer tok-1')
  })

  it('401 清除 token', async () => {
    const auth = useAuthStore()
    auth.setToken('tok-1')
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ code: 401, message: '未登录' }),
    }) as any
    await expect(request('/x')).rejects.toMatchObject({ code: 401 })
    expect(auth.token).toBeNull()
  })
})
```

- [ ] **步骤 8：创建占位页面组件**（后续任务填充）

创建 `src/views/DashboardView.vue`、`ContentListView.vue`、`ContentEditView.vue`、`MediaLibraryView.vue`、`MenuManageView.vue`、`SettingsView.vue` 的空模板（`<template><div>占位</div></template>`），保证 build 通过。

- [ ] **步骤 9：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vite build && npx vitest run`
预期：build 成功、测试通过

---

### 任务 7：动态表单引擎（types + registry + 基础控件）

**文件：**
- 创建：`admin/src/dynamic-form/types.ts`
- 创建：`admin/src/dynamic-form/registry.ts`
- 创建：`admin/src/dynamic-form/DynamicForm.vue`
- 创建：`admin/src/dynamic-form/controls/TextControl.vue`
- 创建：`admin/src/dynamic-form/controls/TextareaControl.vue`
- 创建：`admin/src/dynamic-form/controls/NumberControl.vue`
- 创建：`admin/src/dynamic-form/controls/BooleanControl.vue`
- 创建：`admin/src/dynamic-form/controls/DateControl.vue`
- 创建：`admin/src/dynamic-form/controls/DateTimeControl.vue`
- 创建：`admin/src/dynamic-form/controls/SelectControl.vue`
- 创建：`admin/src/dynamic-form/controls/MultiSelectControl.vue`
- 创建：`admin/src/dynamic-form/controls/SlugControl.vue`
- 创建：`admin/src/dynamic-form/__tests__/registry.test.ts`（Vitest）

**字段类型与 schema 契约**（对齐后端 `schema.Field`）：
```ts
// types.ts
export type FieldType =
  | 'text' | 'textarea' | 'richtext' | 'number' | 'boolean' | 'date' | 'datetime'
  | 'select' | 'multiselect' | 'image' | 'file' | 'slug'

export interface SchemaField {
  name: string
  label: string
  type: FieldType
  required?: boolean
  indexed?: boolean
  translatable?: boolean
  default?: unknown
  options?: string[]
  max_length?: number
  pattern?: string
  min?: number | null
  max?: number | null
}

export type FormValues = Record<string, unknown>
```

- [ ] **步骤 1：registry.ts（注册表 + 校验器）**

```ts
import type { Component } from 'vue'
import type { SchemaField, FormValues, FieldType } from './types'

export interface FieldControl {
  component: Component
  validate: (v: unknown, f: SchemaField) => string | null
}

export const registry: Record<FieldType, FieldControl> = {
  text: { component: TextControl, validate: validateText },
  textarea: { component: TextareaControl, validate: validateText },
  richtext: { component: RichTextControl, validate: validateText },
  number: { component: NumberControl, validate: validateNumber },
  boolean: { component: BooleanControl, validate: () => null },
  date: { component: DateControl, validate: validateDate },
  datetime: { component: DateTimeControl, validate: validateDateTime },
  select: { component: SelectControl, validate: validateSelect },
  multiselect: { component: MultiSelectControl, validate: validateMultiSelect },
  image: { component: ImageControl, validate: validateText },
  file: { component: FileControl, validate: validateText },
  slug: { component: SlugControl, validate: validateSlug },
}

function isEmpty(v: unknown): boolean {
  return v === undefined || v === null || v === ''
}

export function validateRequired(v: unknown, f: SchemaField): string | null {
  if (f.required && isEmpty(v)) return `请填写${f.label}`
  return null
}

export function validateText(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  const s = String(v)
  if (f.max_length && [...s].length > f.max_length) return `${f.label}超长，上限 ${f.max_length}`
  if (f.pattern) {
    const re = new RegExp(f.pattern)
    if (!re.test(s)) return `${f.label}格式不合法`
  }
  return null
}

export function validateSlug(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (!/^[a-z0-9-]+$/.test(String(v))) return `${f.label}必须是合法别名`
  return null
}

export function validateNumber(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  const n = Number(v)
  if (Number.isNaN(n)) return `${f.label}需要数字`
  if (f.min !== null && f.min !== undefined && n < f.min) return `${f.label}不能小于 ${f.min}`
  if (f.max !== null && f.max !== undefined && n > f.max) return `${f.label}不能大于 ${f.max}`
  return null
}

export function validateDate(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (!/^\d{4}-\d{2}-\d{2}$/.test(String(v))) return `${f.label}日期格式应为 YYYY-MM-DD`
  return null
}

export function validateDateTime(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (Number.isNaN(Date.parse(String(v)))) return `${f.label}时间格式不合法`
  return null
}

export function validateSelect(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (f.options && !f.options.includes(String(v))) return `${f.label}值不在选项内`
  return null
}

export function validateMultiSelect(v: unknown, f: SchemaField): string | null {
  const req = validateRequired(v, f)
  if (req) return req
  if (isEmpty(v)) return null
  if (!Array.isArray(v)) return `${f.label}需要字符串数组`
  return null
}

// 表单级校验：返回 {fieldName: error}
export function validateForm(fields: SchemaField[], values: FormValues): Record<string, string> {
  const errors: Record<string, string> = {}
  for (const f of fields) {
    const ctl = registry[f.type]
    if (!ctl) continue
    const err = ctl.validate(values[f.name], f)
    if (err) errors[f.name] = err
  }
  return errors
}
```

> 注意：控件组件 import 会在步骤 2-4 中创建。registry.ts 的 import 顺序：先建控件再建 registry，或先用 `defineAsyncComponent`。**实现时**：控件用直接 import，先创建控件文件。RichTextControl/ImageControl/FileControl 在任务 8 创建，registry 中对应行暂以 `TextControl` 占位并在任务 8 替换。

- [ ] **步骤 2：基础控件**（TextControl/TextareaControl/NumberControl/BooleanControl）

`TextControl.vue`：
```vue
<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: string }>()
defineEmits<{ 'update:modelValue': [string] }>()
</script>
<template>
  <el-input :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" :placeholder="field.label" />
</template>
```

`TextareaControl.vue`：
```vue
<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: string }>()
defineEmits<{ 'update:modelValue': [string] }>()
</script>
<template>
  <el-input :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" type="textarea" :rows="4" />
</template>
```

`NumberControl.vue`：
```vue
<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: number | null }>()
defineEmits<{ 'update:modelValue': [number | null] }>()
</script>
<template>
  <el-input-number :model-value="modelValue ?? undefined" @update:model-value="$emit('update:modelValue', $event)" />
</template>
```

`BooleanControl.vue`：
```vue
<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: boolean }>()
defineEmits<{ 'update:modelValue': [boolean] }>()
</script>
<template>
  <el-switch :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" />
</template>
```

- [ ] **步骤 3：选择/日期控件**（DateControl/DateTimeControl/SelectControl/MultiSelectControl/SlugControl）

`DateControl.vue`：
```vue
<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: string }>()
defineEmits<{ 'update:modelValue': [string] }>()
</script>
<template>
  <el-date-picker :model-value="modelValue || null" @update:model-value="$emit('update:modelValue', $event ?? '')" type="date" value-format="YYYY-MM-DD" />
</template>
```

`DateTimeControl.vue`：同 DateControl，`type="datetime"`、`value-format="YYYY-MM-DDTHH:mm:ssZ"`。

`SelectControl.vue`：
```vue
<script setup lang="ts">
import type { SchemaField } from '../types'
defineProps<{ field: SchemaField; modelValue: string }>()
defineEmits<{ 'update:modelValue': [string] }>()
</script>
<template>
  <el-select :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" style="width: 100%">
    <el-option v-for="o in field.options" :key="o" :label="o" :value="o" />
  </el-select>
</template>
```

`MultiSelectControl.vue`：同 SelectControl，`multiple`、modelValue 为 `string[]`。

`SlugControl.vue`：同 TextControl，placeholder 提示"小写字母数字横线"。

- [ ] **步骤 4：DynamicForm.vue**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { registry, validateForm } from './registry'
import type { SchemaField, FormValues } from './types'

const props = defineProps<{ fields: SchemaField[]; modelValue: FormValues }>()
const emit = defineEmits<{ 'update:modelValue': [FormValues] }>()

const errors = computed(() => validateForm(props.fields, props.modelValue))

function updateField(name: string, val: unknown) {
  emit('update:modelValue', { ...props.modelValue, [name]: val })
}
</script>

<template>
  <el-form :model="modelValue" label-position="top">
    <el-form-item v-for="f in fields" :key="f.name" :label="f.label" :error="errors[f.name]">
      <component
        :is="registry[f.type]?.component"
        :field="f"
        :model-value="modelValue[f.name]"
        @update:model-value="updateField(f.name, $event)"
      />
      <div v-if="f.required" class="req-hint">*</div>
    </el-form-item>
  </el-form>
</template>

<style scoped>
.req-hint { color: var(--el-color-danger); }
</style>
```

- [ ] **步骤 5：Vitest 测试（registry 校验逻辑）**（`src/dynamic-form/__tests__/registry.test.ts`）

```ts
import { describe, it, expect } from 'vitest'
import { validateText, validateNumber, validateSlug, validateForm } from '../registry'
import type { SchemaField } from '../types'

const f = (name: string, type: any, extra: Partial<SchemaField> = {}): SchemaField => ({
  name, label: name, type, ...extra,
})

describe('registry validators', () => {
  it('必填', () => {
    expect(validateText('', f('t', 'text', { required: true }))).toBeTruthy()
    expect(validateText('x', f('t', 'text', { required: true }))).toBeNull()
  })
  it('max_length', () => {
    expect(validateText('abc', f('t', 'text', { max_length: 2 }))).toBeTruthy()
  })
  it('number 范围', () => {
    expect(validateNumber(5, f('n', 'number', { min: 0, max: 10 }))).toBeNull()
    expect(validateNumber(-1, f('n', 'number', { min: 0 }))).toBeTruthy()
    expect(validateNumber('abc', f('n', 'number'))).toBeTruthy()
  })
  it('slug 格式', () => {
    expect(validateSlug('hello-world', f('s', 'slug'))).toBeNull()
    expect(validateSlug('Hello World', f('s', 'slug'))).toBeTruthy()
  })
  it('validateForm 汇总', () => {
    const fields = [f('a', 'text', { required: true }), f('b', 'number')]
    const errors = validateForm(fields, { a: '', b: 1 })
    expect(errors.a).toBeTruthy()
    expect(errors.b).toBeUndefined()
  })
})
```

- [ ] **步骤 6：验证**

运行：`cd admin && npx vitest run src/dynamic-form/__tests__/registry.test.ts`
预期：通过

---

### 任务 8：富文本/图片/文件控件 + MediaPicker

**文件：**
- 创建：`admin/src/dynamic-form/controls/RichTextControl.vue`
- 创建：`admin/src/dynamic-form/controls/ImageControl.vue`
- 创建：`admin/src/dynamic-form/controls/FileControl.vue`
- 创建：`admin/src/components/MediaPicker.vue`
- 修改：`admin/src/dynamic-form/registry.ts`（richtext/image/file 换真实控件）

- [ ] **步骤 1：MediaPicker.vue**（弹层选媒体）

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listMedia } from '../api/media'
import type { MediaItem } from '../api/media'

const visible = defineModel<boolean>('visible', { default: false })
const selected = defineModel<string>('selected', { default: '' })
const items = ref<MediaItem[]>([])
const loading = ref(false)

async function load() {
  if (!visible.value) return
  loading.value = true
  try {
    const { items: list } = await listMedia()
    items.value = list
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <el-dialog v-model="visible" title="选择媒体" width="720px" @open="load">
    <div class="grid">
      <div
        v-for="m in items"
        :key="m.id"
        class="item"
        :class="{ active: selected === m.url }"
        @click="selected = m.url"
      >
        <img v-if="m.mime.startsWith('image/')" :src="m.url" :alt="m.filename" />
        <div v-else class="file-icon">{{ m.filename }}</div>
        <div class="name">{{ m.filename }}</div>
      </div>
      <el-empty v-if="!loading && items.length === 0" description="暂无媒体" />
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="visible = false">确定</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.item { border: 1px solid #eee; padding: 8px; cursor: pointer; border-radius: 4px; }
.item.active { border-color: var(--el-color-primary); }
.item img { width: 100%; height: 80px; object-fit: cover; }
.name { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
```

> 依赖 `src/api/media.ts`（任务 9 创建 listMedia）。**实现时**：本任务先建 api/media.ts 的 `listMedia`，任务 9 补全上传/删除。

- [ ] **步骤 2：RichTextControl.vue**（wangEditor 5）

```vue
<script setup lang="ts">
import { onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { Editor, Toolbar } from '@wangeditor/editor-for-vue'
import type { IDomEditor } from '@wangeditor/editor'
import '@wangeditor/editor/dist/css/style.css'
import type { SchemaField } from '../types'

const props = defineProps<{ field: SchemaField; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()

const editorRef = shallowRef<IDomEditor>()
const toolbarConfig = {}
const editorConfig = {
  placeholder: '输入内容…',
  onChange: (editor: IDomEditor) => emit('update:modelValue', editor.getHtml()),
}

function handleCreated(editor: IDomEditor) {
  editorRef.value = editor
  if (props.modelValue) {
    editor.setHtml(props.modelValue)
  }
}

watch(() => props.modelValue, (v) => {
  const ed = editorRef.value
  if (ed && v !== ed.getHtml()) {
    ed.setHtml(v ?? '')
  }
})

onBeforeUnmount(() => {
  editorRef.value?.destroy()
})
</script>

<template>
  <div class="richtext">
    <Toolbar :editor="editorRef" :default-config="toolbarConfig" mode="default" />
    <Editor :default-config="editorConfig" mode="default" @on-created="handleCreated" style="height: 300px; overflow-y: hidden;" />
  </div>
</template>
```

> wangEditor 是默认导出 CSS；`Editor/Toolbar` 从 `@wangeditor/editor-for-vue` 导入。若 SSR 无关（纯浏览器）无碍。

- [ ] **步骤 3：ImageControl.vue / FileControl.vue**

`ImageControl.vue`：
```vue
<script setup lang="ts">
import { ref } from 'vue'
import MediaPicker from '../../components/MediaPicker.vue'
import type { SchemaField } from '../types'

defineProps<{ field: SchemaField; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()
const pickerVisible = ref(false)
</script>

<template>
  <div class="img-control">
    <img v-if="modelValue" :src="modelValue" class="preview" />
    <el-button @click="pickerVisible = true">选择图片</el-button>
    <MediaPicker v-model:visible="pickerVisible" v-model:selected="modelValue" @update:selected="emit('update:modelValue', $event)" />
  </div>
</template>

<style scoped>
.preview { max-width: 200px; max-height: 120px; display: block; margin-bottom: 8px; }
</style>
```

> 注意：`v-model:selected="modelValue"` 直接写 props 是反模式。**改为**：用局部 `selectedUrl` ref，确认时 emit。实现时用正确两方绑定（本地 state + emit）。
`FileControl.vue`：同 ImageControl，无 preview 图，显示文件名 + "选择文件"按钮。

- [ ] **步骤 4：registry 换真实控件**

`registry.ts` 更新：
```ts
import RichTextControl from './controls/RichTextControl.vue'
import ImageControl from './controls/ImageControl.vue'
import FileControl from './controls/FileControl.vue'
```
三处注册行替换为真实组件。

- [ ] **步骤 5：创建 api/media.ts（listMedia 部分）**

```ts
// src/api/media.ts
import { request } from './client'

export interface MediaItem {
  id: number
  filename: string
  url: string
  mime: string
  size: number
  created_at: string
}

export const listMedia = (page = 1, perPage = 50) =>
  request<{ items: MediaItem[] }>(`/media?page=${page}&per_page=${perPage}`)
export const uploadMedia = (file: File) => {
  const fd = new FormData()
  fd.append('file', file)
  // 需带 token——用 fetch 手动，或 client 支持 FormData
  return request<{ key: string; url: string }>('/media/upload', { body: fd })
}
```

> 注意：`uploadMedia` 的 FormData 不能设 `Content-Type: application/json`。**实现时**：`client.ts` 的 request 判断 `body instanceof FormData` 时不设 JSON header。此步先实现 listMedia，upload 在任务 9 完善。

- [ ] **步骤 6：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vitest run`
预期：通过（registry 测试 + client 测试）

---

### 任务 9：内容列表 + 搜索 + 仪表盘页面

**文件：**
- 创建：`admin/src/api/content.ts`
- 创建：`admin/src/api/search.ts`
- 创建：`admin/src/api/stats.ts`
- 创建：`admin/src/api/media.ts`（补全上传/删除）
- 创建：`admin/src/api/menu.ts`、`admin/src/api/settings.ts`（供后续任务）
- 创建：`admin/src/views/ContentListView.vue`（填充）
- 创建：`admin/src/views/DashboardView.vue`（填充）

- [ ] **步骤 1：api/content.ts**

```ts
import { request } from './client'

export interface ContentEntry {
  content: {
    id: number
    content_type_id: number
    content_id: string
    lang: string
    slug: string
    title: string
    status: string
    created_by: number
    created_at: string
    updated_at: string
    published_at: string | null
    payload: string
  }
  type_name: string
  fields: Record<string, unknown>
}

export const listContent = (params: { type: string; lang: string; status?: string; page?: number; perPage?: number }) =>
  request<{ items: ContentEntry[]; total: number }>(`/content?type=${params.type}&lang=${params.lang}${params.status ? `&status=${params.status}` : ''}&page=${params.page ?? 1}&per_page=${params.perPage ?? 20}`)

export const getContent = (id: number) => request<{ content: ContentEntry }>(`/content/${id}`)
export const createContent = (type: string, lang: string, data: Record<string, unknown>) =>
  request<{ content: ContentEntry }>('/content', { method: 'POST', body: { type, lang, data } })
export const updateContent = (id: number, data: Record<string, unknown>) =>
  request<{ content: ContentEntry }>(`/content/${id}`, { method: 'PUT', body: { type: '', lang: '', data } })
export const deleteContent = (id: number) => request<{ deleted: number }>(`/content/${id}`, { method: 'DELETE' })
export const publishContent = (id: number) => request<{ id: number; status: string }>(`/content/${id}/publish`, { method: 'POST' })
export const unpublishContent = (id: number) => request<{ id: number; status: string }>(`/content/${id}/unpublish`, { method: 'POST' })
export const listTranslations = (id: number) => request<{ items: ContentEntry[] }>(`/content/${id}/translations`)
export const createTranslation = (id: number, lang: string, data: Record<string, unknown>) =>
  request<{ content: ContentEntry }>(`/content/${id}/translate`, { method: 'POST', body: { type: '', lang, data } })
```

> 注意：`updateContent`/`createTranslation` 的 body 里 `type` 是必需的（`contentReq.Type`），但现有 `Service.Update`/`CreateTranslation` 只在 `Create` 用 type。**实现时核对**：`HandleContentUpdate` 的 `contentReq` 需要 type 吗？——看现有 handler，`HandleContentUpdate` 只读 `req.Data`，type 忽略。`createTranslation` 需要 type（service 校验父类型）。SPA 传入当前类型名即可。

- [ ] **步骤 2：api/search.ts + api/stats.ts**

```ts
// src/api/search.ts
import { request } from './client'
import type { ContentEntry } from './content'

export const searchContent = (params: { type: string; lang: string; q: string; page?: number }) =>
  request<{ items: ContentEntry[]; total: number }>(`/search?type=${params.type}&lang=${params.lang}&q=${encodeURIComponent(params.q)}&page=${params.page ?? 1}`)
```

```ts
// src/api/stats.ts
import { request } from './client'
import type { ContentEntry } from './content'

export interface TypeStat {
  type_name: string
  published: number
  draft: number
}
export interface StatsData {
  by_type: TypeStat[]
  recent: ContentEntry[]
}
export const fetchStats = () => request<StatsData>('/stats')
```

- [ ] **步骤 3：ContentListView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listContent, deleteContent, type ContentEntry } from '../api/content'
import { listContentTypes } from '../api/content-types'
import { searchContent } from '../api/search'
import { fetchMeta, type MetaData } from '../api/meta'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const meta = ref<MetaData | null>(null)
const types = ref<{ name: string; label: string }[]>([])
const typeFilter = ref('')
const langFilter = ref('')
const statusFilter = ref('')
const keyword = ref('')
const items = ref<ContentEntry[]>([])
const total = ref(0)
const page = ref(1)
const perPage = 20
const loading = ref(false)
const searchTimer = ref<number | null>(null)

async function load() {
  loading.value = true
  try {
    if (keyword.value) {
      const r = await searchContent({ type: typeFilter.value, lang: langFilter.value, q: keyword.value, page: page.value })
      items.value = r.items
      total.value = r.total
    } else {
      const r = await listContent({ type: typeFilter.value, lang: langFilter.value, status: statusFilter.value || undefined, page: page.value, perPage })
      items.value = r.items
      total.value = r.total
    }
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  meta.value = await fetchMeta()
  langFilter.value = meta.value.default_lang
  const tr = await listContentTypes()
  types.value = tr.items
  typeFilter.value = types.value[0]?.name ?? ''
  await load()
})

watch([typeFilter, langFilter, statusFilter, page], () => load())

function onSearchInput() {
  if (searchTimer.value) window.clearTimeout(searchTimer.value)
  searchTimer.value = window.setTimeout(() => { page.value = 1; load() }, 400)
}

function edit(id?: number) {
  router.push(id ? `/content/${id}` : '/content/new')
}

async function remove(id: number) {
  try {
    await ElMessageBox.confirm('确认删除该内容？', '提示')
  } catch { return }
  await deleteContent(id)
  ElMessage.success('已删除')
  load()
}

function statusTag(s: string) {
  return s === 'published' ? 'success' : 'info'
}
</script>

<template>
  <div>
    <h2>内容管理</h2>
    <el-form inline>
      <el-form-item label="类型">
        <el-select v-model="typeFilter" style="width: 160px">
          <el-option v-for="t in types" :key="t.name" :label="t.label" :value="t.name" />
        </el-select>
      </el-form-item>
      <el-form-item label="语言">
        <el-select v-model="langFilter" style="width: 120px">
          <el-option v-for="l in meta?.languages ?? []" :key="l" :label="l" :value="l" />
        </el-select>
      </el-form-item>
      <el-form-item label="状态">
        <el-select v-model="statusFilter" style="width: 120px" clearable>
          <el-option label="草稿" value="draft" />
          <el-option label="已发布" value="published" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-input v-model="keyword" placeholder="搜索标题/Slug" clearable @input="onSearchInput" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="router.push('/content/new')">新建</el-button>
      </el-form-item>
    </el-form>

    <el-table :data="items" v-loading="loading">
      <el-table-column prop="content.title" label="标题" />
      <el-table-column prop="content.slug" label="Slug" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }"><el-tag :type="statusTag(row.content.status)">{{ row.content.status }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="content.lang" label="语言" width="80" />
      <el-table-column prop="content.updated_at" label="更新时间" width="180" />
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button link type="primary" @click="edit(row.content.id)">编辑</el-button>
          <el-button link type="danger" @click="remove(row.content.id)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="page"
      :total="total"
      :page-size="perPage"
      layout="prev, pager, next"
      class="pager"
    />
  </div>
</template>

<style scoped>
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
```

> 依赖 `listContentTypes`（`src/api/content-types.ts`，返回 `{items: ContentType[]}`，`ContentType.name/label`）。此步创建该文件。

- [ ] **步骤 4：DashboardView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { fetchStats, type StatsData } from '../api/stats'

const router = useRouter()
const stats = ref<StatsData | null>(null)

onMounted(async () => {
  stats.value = await fetchStats()
})
</script>

<template>
  <div>
    <h2>仪表盘</h2>
    <el-row :gutter="16" v-if="stats">
      <el-col v-for="t in stats.by_type" :key="t.type_name" :span="6">
        <el-card>
          <div class="stat-label">{{ t.type_name }}</div>
          <div class="stat-nums">
            <span>已发布 {{ t.published }}</span>
            <span>草稿 {{ t.draft }}</span>
          </div>
        </el-card>
      </el-col>
    </el-row>
    <el-card v-if="stats && stats.recent.length" class="recent">
      <template #header>最近内容</template>
      <el-table :data="stats.recent" @row-click="(r: any) => router.push(`/content/${r.content.id}`)">
        <el-table-column prop="content.title" label="标题" />
        <el-table-column prop="type_name" label="类型" width="120" />
        <el-table-column prop="content.status" label="状态" width="100" />
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.stat-label { font-weight: 600; margin-bottom: 8px; }
.stat-nums { display: flex; gap: 16px; color: var(--el-text-color-secondary); }
.recent { margin-top: 16px; }
</style>
```

- [ ] **步骤 5：api/content-types.ts + api/media.ts（补全）**

```ts
// src/api/content-types.ts
import { request } from './client'

export interface ContentTypeItem {
  id: number
  name: string
  label: string
  fields: Array<{ name: string; label: string; type: string; required?: boolean; indexed?: boolean; translatable?: boolean; options?: string[]; max_length?: number; pattern?: string; min?: number | null; max?: number | null }>
}
export const listContentTypes = () => request<{ items: ContentTypeItem[] }>('/content-types')
```

```ts
// src/api/media.ts 补全
export const uploadMedia = async (file: File) => {
  const fd = new FormData()
  fd.append('file', file)
  const auth = useAuthStore()
  const res = await fetch('/api/media/upload', {
    method: 'POST',
    headers: { Authorization: `Bearer ${auth.token ?? ''}` },
    body: fd,
  })
  const data = await res.json()
  if (!res.ok) throw { code: data.code ?? res.status, message: data.message ?? '上传失败' }
  return data.data
}
export const deleteMedia = (id: number) => request<{ deleted: number }>(`/media/${id}`, { method: 'DELETE' })
```

- [ ] **步骤 6：api/menu.ts + api/settings.ts（供任务 10/11）**

```ts
// src/api/menu.ts
import { request } from './client'

export interface MenuItem {
  id: number
  name: string
  lang: string
  items: string
}
export const listMenus = (lang: string) => request<{ items: MenuItem[] }>(`/menus?lang=${lang}`)
export const createMenu = (body: { name: string; lang: string; items: unknown }) =>
  request<{ menu: MenuItem }>('/menus', { method: 'POST', body })
export const updateMenu = (id: number, body: { name?: string; lang?: string; items?: unknown }) =>
  request<{ menu: MenuItem }>(`/menus/${id}`, { method: 'PUT', body })
export const deleteMenu = (id: number) => request<{ deleted: number }>(`/menus/${id}`, { method: 'DELETE' })
```

```ts
// src/api/settings.ts
import { request } from './client'

export const fetchSettings = () => request<{ settings: Record<string, string> }>('/settings')
export const updateSettings = (data: Record<string, string>) =>
  request<{ settings: Record<string, string> }>('/settings', { method: 'PUT', body: data })
```

- [ ] **步骤 7：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vite build`
预期：构建成功（页面组件已填充）

---

### 任务 10：内容编辑页（多语言 Tab + 动态表单 + 发布）

**文件：**
- 创建：`admin/src/views/ContentEditView.vue`（填充）
- 创建：`admin/src/api/content-types.ts`（补 getContentType）

- [ ] **步骤 1：ContentEditView.vue**

```vue
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getContent, createContent, updateContent, publishContent, unpublishContent, listTranslations, createTranslation, type ContentEntry } from '../api/content'
import { listContentTypes, type ContentTypeItem } from '../api/content-types'
import { fetchMeta, type MetaData } from '../api/meta'
import DynamicForm from '../dynamic-form/DynamicForm.vue'
import { validateForm } from '../dynamic-form/registry'
import type { FormValues } from '../dynamic-form/types'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const meta = ref<MetaData | null>(null)
const types = ref<ContentTypeItem[]>([])
const id = computed(() => route.params.id ? Number(route.params.id) : null)
const isNew = computed(() => !id.value)

const currentType = ref<ContentTypeItem | null>(null)
const currentLang = ref('')
const activeTab = ref('')
const translations = ref<ContentEntry[]>([])
const form = ref<FormValues>({})
const loading = ref(false)
const saving = ref(false)

const langTabs = computed(() => meta.value?.languages ?? [])
const fields = computed(() => currentType.value?.fields ?? [])

// 非 translatable 字段跨语言共享：从首个已有翻译取共享值
const sharedValues = computed(() => {
  const first = translations.value[0]
  if (!first) return {}
  const out: FormValues = {}
  for (const f of fields.value) {
    if (!f.translatable) out[f.name] = first.fields[f.name]
  }
  return out
})

async function loadType() {
  const tr = await listContentTypes()
  types.value = tr.items
}

async function loadTranslations(contentId: number) {
  const tr = await listTranslations(contentId)
  translations.value = tr.items
  const byLang: Record<string, ContentEntry> = {}
  for (const t of tr.items) byLang[t.content.lang] = t
  return byLang
}

async function loadEdit(contentId: number) {
  const { content } = await getContent(contentId)
  const byLang = await loadTranslations(contentId)
  currentType.value = types.value.find((t) => t.name === content.type_name) ?? null
  currentLang.value = content.content.lang
  activeTab.value = content.content.lang
  const langEntry = byLang[activeTab.value]
  form.value = { ...sharedValues.value, ...(langEntry?.fields ?? {}) }
}

async function initNew() {
  // 路由 query 带 type
  const typeName = String(route.query.type ?? types.value[0]?.name ?? '')
  currentType.value = types.value.find((t) => t.name === typeName) ?? null
  currentLang.value = meta.value?.default_lang ?? ''
  activeTab.value = currentLang.value
  form.value = {}
}

async function switchTab(lang: string) {
  if (id.value) {
    const byLang = await loadTranslations(id.value)
    const entry = byLang[lang]
    if (entry) {
      form.value = { ...sharedValues.value, ...entry.fields }
    } else {
      form.value = { ...sharedValues.value, ...form.value } // 保留当前语言可翻译字段，创建该语言草稿
      activeTab.value = lang
      // 保存时 createTranslation
      needCreate.value = true
      return
    }
  }
  activeTab.value = lang
}

const needCreate = ref(false)

async function save() {
  if (!currentType.value) return
  const errs = validateForm(fields.value, form.value)
  if (Object.keys(errs).length) {
    ElMessage.warning(Object.values(errs)[0])
    return
  }
  saving.value = true
  try {
    const payload: FormValues = { ...sharedValues.value, ...form.value }
    if (isNew.value) {
      const { content } = await createContent(currentType.value.name, activeTab.value, payload)
      router.replace(`/content/${content.content.id}`)
      ElMessage.success('已创建')
      // 刷新 id 后走编辑态
      await loadAfterCreate(content.content.id)
    } else if (id.value) {
      if (needCreate.value) {
        await createTranslation(id.value, activeTab.value, payload)
        needCreate.value = false
      } else {
        await updateContent(id.value, payload)
      }
      ElMessage.success('已保存')
      await loadEdit(id.value)
    }
  } catch (e: any) {
    ElMessage.error(e.message ?? '保存失败')
  } finally {
    saving.value = false
  }
}

async function loadAfterCreate(contentId: number) {
  await loadType()
  const byLang = await loadTranslations(contentId)
  currentType.value = types.value.find((t) => t.name === currentType.value?.name) ?? null
  activeTab.value = currentLang.value
  const entry = byLang[activeTab.value]
  form.value = { ...sharedValues.value, ...(entry?.fields ?? {}) }
}

async function togglePublish() {
  if (!id.value) return
  const entry = translations.value.find((t) => t.content.lang === activeTab.value)
  const target = entry?.content.status === 'published' ? 'unpublish' : 'publish'
  if (target === 'publish') await publishContent(id.value)
  else await unpublishContent(id.value)
  ElMessage.success(target === 'publish' ? '已发布' : '已撤回')
  await loadEdit(id.value)
}

onMounted(async () => {
  meta.value = await fetchMeta()
  await loadType()
  if (isNew.value) await initNew()
  else if (id.value) await loadEdit(id.value)
})
</script>

<template>
  <div v-if="currentType">
    <h2>{{ isNew ? '新建内容' : '编辑内容' }} · {{ currentType.label }}</h2>
    <el-tabs v-model="activeTab" @tab-change="switchTab">
      <el-tab-pane v-for="l in langTabs" :key="l" :label="l" :name="l" />
    </el-tabs>
    <DynamicForm v-model="form" :fields="fields" />
    <div class="actions">
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      <el-button v-if="!isNew && auth.isAdmin" @click="togglePublish">发布/撤回</el-button>
      <el-button @click="router.back()">返回</el-button>
    </div>
  </div>
  <el-empty v-else description="请先创建内容类型" />
</template>

<style scoped>
.actions { margin-top: 24px; display: flex; gap: 12px; }
</style>
```

> 说明：此实现覆盖 新建/编辑/多语言 Tab/翻译创建/发布 全流程。`switchTab` 对未翻译语言设置 `needCreate`，保存时 `createTranslation`。`sharedValues` 保证非 translatable 字段跨语言共享。`loadEdit` 后 activeTab 正确回填。

- [ ] **步骤 2：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vite build`
预期：构建成功

> 手动验证（dev）：`npm run dev`，登录 → 内容 → 新建 article → 填表单 → 保存 → 切 en Tab → 保存（创建翻译）→ 发布 → 前台 `http://localhost:8080/en/article/...` 渲染。**冒烟由任务 14 覆盖**。

---

### 任务 11：媒体库页面

**文件：**
- 创建：`admin/src/views/MediaLibraryView.vue`（填充）

- [ ] **步骤 1：MediaLibraryView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listMedia, uploadMedia, deleteMedia, type MediaItem } from '../api/media'

const items = ref<MediaItem[]>([])
const loading = ref(false)
const uploading = ref(false)

async function load() {
  loading.value = true
  try {
    const r = await listMedia()
    items.value = r.items
  } finally {
    loading.value = false
  }
}

async function onUpload(file: File) {
  uploading.value = true
  try {
    await uploadMedia(file)
    ElMessage.success('上传成功')
    await load()
  } catch (e: any) {
    ElMessage.error(e.message ?? '上传失败')
  } finally {
    uploading.value = false
  }
}

async function remove(m: MediaItem) {
  try { await ElMessageBox.confirm(`确认删除 ${m.filename}？`, '提示') } catch { return }
  await deleteMedia(m.id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <h2>媒体库</h2>
    <el-upload
      :show-file-list="false"
      :before-upload="(f: File) => { onUpload(f); return false }"
      :disabled="uploading"
    >
      <el-button type="primary" :loading="uploading">上传媒体</el-button>
    </el-upload>
    <div class="grid" v-loading="loading">
      <div v-for="m in items" :key="m.id" class="item">
        <img v-if="m.mime.startsWith('image/')" :src="m.url" :alt="m.filename" />
        <div v-else class="file-icon">{{ m.filename }}</div>
        <div class="meta">
          <div class="name">{{ m.filename }}</div>
          <el-button link type="danger" @click="remove(m)">删除</el-button>
        </div>
      </div>
      <el-empty v-if="!loading && items.length === 0" description="暂无媒体" style="grid-column: 1/-1" />
    </div>
  </div>
</template>

<style scoped>
.grid { display: grid; grid-template-columns: repeat(5, 1fr); gap: 12px; margin-top: 16px; }
.item { border: 1px solid #eee; border-radius: 4px; overflow: hidden; }
.item img { width: 100%; height: 120px; object-fit: cover; display: block; }
.file-icon { height: 120px; display: flex; align-items: center; justify-content: center; background: #fafafa; }
.meta { padding: 8px; display: flex; justify-content: space-between; align-items: center; }
.name { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
```

- [ ] **步骤 2：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vite build`
预期：构建成功

---

### 任务 12：菜单管理页面

**文件：**
- 创建：`admin/src/views/MenuManageView.vue`（填充）

- [ ] **步骤 1：MenuManageView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listMenus, createMenu, updateMenu, deleteMenu, type MenuItem } from '../api/menu'
import { fetchMeta, type MetaData } from '../api/meta'

const meta = ref<MetaData | null>(null)
const lang = ref('zh')
const items = ref<MenuItem[]>([])
const editing = ref<MenuItem | null>(null)
const dialogVisible = ref(false)
const formName = ref('')
const formItems = ref('')

interface NavItem { label: string; url: string }

function parseItems(raw: string): NavItem[] {
  try { return JSON.parse(raw || '[]') } catch { return [] }
}

async function load() {
  const r = await listMenus(lang.value)
  items.value = r.items
}

onMounted(async () => {
  meta.value = await fetchMeta()
  lang.value = meta.value.default_lang
  await load()
})

function openCreate() {
  editing.value = null
  formName.value = ''
  formItems.value = '[]'
  dialogVisible.value = true
}

function openEdit(m: MenuItem) {
  editing.value = m
  formName.value = m.name
  formItems.value = m.items
  dialogVisible.value = true
}

async function save() {
  const itemsArr = parseItems(formItems.value)
  const payload = { name: formName.value, lang: lang.value, items: itemsArr }
  if (editing.value) {
    await updateMenu(editing.value.id, payload)
  } else {
    await createMenu(payload)
  }
  ElMessage.success('已保存')
  dialogVisible.value = false
  await load()
}

async function remove(m: MenuItem) {
  try { await ElMessageBox.confirm(`确认删除菜单 ${m.name}？`, '提示') } catch { return }
  await deleteMenu(m.id)
  await load()
}
</script>

<template>
  <div>
    <h2>菜单管理</h2>
    <div class="bar">
      <el-select v-model="lang" style="width: 120px" @change="load">
        <el-option v-for="l in meta?.languages ?? []" :key="l" :label="l" :value="l" />
      </el-select>
      <el-button type="primary" @click="openCreate">新建菜单</el-button>
    </div>
    <el-table :data="items">
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="lang" label="语言" width="80" />
      <el-table-column label="导航项" min-width="200">
        <template #default="{ row }">
          <el-tag v-for="it in parseItems(row.items)" :key="it.url" size="small" style="margin-right: 4px">{{ it.label }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑菜单' : '新建菜单'" width="560px">
      <el-form label-width="80px">
        <el-form-item label="名称">
          <el-input v-model="formName" />
        </el-form-item>
        <el-form-item label="导航项">
          <el-input v-model="formItems" type="textarea" :rows="6" placeholder='[{"label":"首页","url":"/"}]' />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.bar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; }
</style>
```

- [ ] **步骤 2：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vite build`
预期：构建成功

---

### 任务 13：设置页面

**文件：**
- 创建：`admin/src/views/SettingsView.vue`（填充）

- [ ] **步骤 1：SettingsView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchSettings, updateSettings } from '../api/settings'
import { fetchMeta, type MetaData } from '../api/meta'

const meta = ref<MetaData | null>(null)
const settings = ref<Record<string, string>>({})
const loading = ref(false)
const saving = ref(false)

async function load() {
  meta.value = await fetchMeta()
  const r = await fetchSettings()
  settings.value = { ...r.settings }
  if (!settings.value.site_name) settings.value.site_name = meta.value.site_name
  if (!settings.value.description) settings.value.description = meta.value.description
  if (!settings.value.theme) settings.value.theme = meta.value.theme
}

async function save() {
  saving.value = true
  try {
    await updateSettings(settings.value)
    ElMessage.success('已保存')
  } catch (e: any) {
    ElMessage.error(e.message ?? '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div v-if="meta">
    <h2>站点设置</h2>
    <el-form label-width="100px" style="max-width: 520px">
      <el-form-item label="站点名称">
        <el-input v-model="settings.site_name" />
      </el-form-item>
      <el-form-item label="站点描述">
        <el-input v-model="settings.description" type="textarea" :rows="3" />
      </el-form-item>
      <el-form-item label="主题">
        <el-input v-model="settings.theme" placeholder="default" />
      </el-form-item>
      <el-form-item label="语言">
        <el-tag v-for="l in meta.languages" :key="l" style="margin-right: 4px">{{ l }}</el-tag>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>
```

- [ ] **步骤 2：验证**

运行：`cd admin && npx vue-tsc --noEmit && npx vite build`
预期：构建成功

---

### 任务 14：生产集成 + 全量验收

**文件：**
- 创建：`admin/build.sh`（或说明构建命令）
- 修改：`internal/server/dist/`（复制产物）
- 测试：全量验证

- [ ] **步骤 1：构建 SPA**

```bash
cd admin
npm run build
```

- [ ] **步骤 2：复制产物到 server dist**

```powershell
# 清空并复制
Remove-Item -Recurse -Force "internal\server\dist" -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path "internal\server\dist" -Force | Out-Null
Copy-Item -Path "admin\dist\*" -Destination "internal\server\dist\" -Recurse -Force
# 保留 .gitkeep 以便空目录可 build（若已被删则重建）
if (-not (Test-Path "internal\server\dist\.gitkeep")) {
  New-Item -ItemType File -Path "internal\server\dist\.gitkeep" -Force | Out-Null
}
```

- [ ] **步骤 3：Go 侧全量验证**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

- [ ] **步骤 4：前端测试**

运行：`cd admin && npx vitest run`
预期：全部通过

- [ ] **步骤 5：端到端冒烟**

```bash
# 1. 启动后端（含 admin 产物）
go run ./cmd/dulizhan seed
go run ./cmd/dulizhan --config config.yaml
# 2. 浏览器访问 http://localhost:8080/admin —— 应显示登录页
# 3. 登录 admin/admin123 → 仪表盘
# 4. 内容 → 新建 article → 填表单（含富文本正文）→ 保存 → 发布
# 5. 前台 http://localhost:8080/article/... 渲染出内容
# 6. 切 en 翻译 Tab → 保存 → 前台 /en/article/... 渲染
# 7. 媒体库上传 → 动态表单 image 字段选媒体
# 8. 菜单管理新建 → 设置页保存
```

预期：浏览器全程可视化操作内容成功；`/admin` 静态资源（JS/CSS）正确加载，无 404。

- [ ] **步骤 6：冒烟后清理**

清理 `dulizhan.db`、`data/` 产物与临时进程（8080 端口释放）。

---

## 自检记录

**1. 规格覆盖度：**
- 规格 §1 范围（登录/仪表盘/搜索/内容管理/媒体/菜单/设置）→ 任务 6（登录）、任务 9（仪表盘+列表+搜索）、任务 10（内容编辑+多语言+发布）、任务 11（媒体）、任务 12（菜单）、任务 13（设置）
- 规格 §2 集成（vite 代理/go:embed//admin 托管）→ 任务 5（脚手架）、任务 4（Go 嵌入）
- 规格 §3 认证（Bearer token/角色两级/守卫）→ 任务 6
- 规格 §4 动态表单（12 种字段/注册表/MediaPicker/富文本）→ 任务 7（基础控件+registry）、任务 8（richtext/image/file+MediaPicker）
- 规格 §5 页面流程 → 任务 9-13
- 规格 §6 后端配套（/api/search、/api/stats、最小修正 /api/meta）→ 任务 1/2/3
- 规格 §7 错误处理 → 任务 6（client 401 拦截）+ 各页面 ElMessage
- 规格 §8 测试 → 任务 1-3（Go 单测）、任务 6/7（Vitest）、任务 14（冒烟）

**2. 占位符扫描：** 无 TBD/TODO。每步含代码。注意若干"实现时核对"标注（router base 路径、updateContent body、FormData 头）是明确的实现决策提示，非占位符。

**3. 类型一致性：**
- `ContentEntry`（content/type_name/fields）在任务 9 定义，任务 10/11 一致使用
- `SchemaField`/`FormValues`/`FieldType` 任务 7 定义，任务 8/10 使用
- `MediaItem`（id/filename/url/mime/size/created_at）任务 8 定义，任务 9/11 使用
- 后端 `Stats`/`TypeStat`/`Search` 任务 2/3 定义，adminapi 一致
- `Deps.Cfg` 任务 1 定义，任务 1 装配一致
- `register.go` 新增路由（/meta、/search、/stats）与 handler 名一致
