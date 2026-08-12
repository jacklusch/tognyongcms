# 阶段 1/2 源码恢复 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 把阶段 1（前台骨架）与阶段 2（管理 API）恢复到账本记载的契约状态：`go test -count=1 ./...` 全绿 + `go build`/`go vet` 干净 + final-fix-reports 的冒烟步骤可复现，并解决账本列出的 deferred minor 项（A 类改行为、B 类补测试、C 类保持原样并注明理由）。

**架构：** 单体单二进制 Go + Gin。分层 `internal/{config,errs,store,sqlite,schema,content,i18n,seo,theme,seed,server,auth,media,adminapi}` + `cmd/dulizhan`（入口）+ `themes/default`（默认主题）。模块间只通过接口通信。领域错误 service 层返回 `errs.ErrNotFound/ErrForbidden/ErrValidation`，HTTP 层统一映射 404/403/422（JSON `{code,message}`）。

**技术栈：** Go 1.26、gin@v1.10.0、modernc.org/sqlite@v1.34.2、gopkg.in/yaml.v3@v3.0.1、golang.org/x/crypto@v0.33.0（阶段 2 用）。

**权威素材（on-disk，逐字代码以此为源）：**
- 设计文档：`docs/superpowers/specs/2026-08-05-cms-design.md`（已从 git 索引恢复）
- 阶段 1 briefs：`.superpowers/sdd/2026-08-06-phase1-skeleton-frontend/task-{1..9}-brief.md`
- 阶段 2 briefs（仅 task-1..5、10 存在）：`.superpowers/sdd/2026-08-06-phase2-admin-api/task-{1..5,10}-brief.md`
- final-fix-reports（两阶段）：`.../final-fix-report.md`（含逐条契约修复）
- **缺口：阶段 2 task-6..9（adminapi handler）brief 缺失**，其代码在本计划 Task 15 内联给出

**环境约定（所有任务遵守）：**
- 官方 Go proxy 被墙。网络命令加 `GOPROXY=https://goproxy.cn,direct`；若失败改用 `GOPROXY=https://mirrors.aliyun.com/goproxy,direct GOSUMDB=off`
- go.mod 锁定版本，**勿用 `@latest`**（gin@latest 需 Go 1.25+）
- **勿运行 `go mod tidy`** 直到 gin/sqlite 被真实 import（会清掉 indirect 锁定）
- 每任务完成后：`go test -count=1 <相关包> -v`；阶段收尾才跑 `go test -count=1 ./...`
- **不 git 提交**（用户约定：直接 master 开发、文件级审查）。任务结束的"Commit"步骤改为"记录到 `.superpowers/.../progress.md`"

---

### 任务 1：脚手架（go.mod/config/errs/占位 main/config.yaml）

**文件：**
- 创建：`go.mod`、`go.sum`、`config.yaml`
- 创建：`internal/errs/errs.go`
- 创建：`internal/config/config.go`、`internal/config/config_test.go`
- 创建：`cmd/dulizhan/main.go`（占位）
- 测试：`internal/config/config_test.go`

**代码来源：** 逐字抄录 `phase1/task-1-brief.md` 的 Step 2/4/5/6/7，但 Step 1 用锁定版本（见下）。

- [ ] **步骤 1：初始化 go.mod 并拉依赖（锁定版本，勿用 @latest）**

```bash
go mod init dulizhan
$env:GOPROXY="https://goproxy.cn,direct"
go get gopkg.in/yaml.v3@v3.0.1
go get github.com/gin-gonic/gin@v1.10.0 modernc.org/sqlite@v1.34.2
```

验证 go.mod 中 gin= v1.10.0、sqlite= v1.34.2（均为 `// indirect`，正常——暂无包 import 它们）。
注意：`internal/config/config_test.go` 的 `TestEnvOverride` 用 `t.Setenv` 读写 `DULIZHAN_*` 变量。

- [ ] **步骤 2：编写失败测试**

抄录 `phase1/task-1-brief.md` Step 2 的 `internal/config/config_test.go` 全文（`sampleYAML`、`TestLoadFromYAML`、`TestEnvOverride`、`TestValidate`）。

- [ ] **步骤 3：运行确认失败**

运行：`go test -count=1 ./internal/config/ -run TestLoadFromYAML -v`
预期：FAIL（编译失败，`config.Load` 未定义）

- [ ] **步骤 4：实现 config/errs/main**

抄录 `phase1/task-1-brief.md` Step 4（`internal/config/config.go`）、Step 5（`internal/errs/errs.go`）、Step 6（`cmd/dulizhan/main.go` 占位）、Step 7（`config.yaml`）。

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/config/ -v`；`go build ./...`
预期：config 3/3 PASS；build OK
注意：若 build 报 gin/sqlite 相关错误属正常（尚未被 import），只要 config 包可编译即可。

---

### 任务 2：store 接口 + 模型 + SQLite 基础（含 A/B 类修复）

**文件：**
- 创建：`internal/store/store.go`、`internal/store/model.go`
- 创建：`internal/store/sqlite/sqlite.go`、`internal/store/sqlite/migrate.go`
- 创建：`internal/store/sqlite/sqlite_test.go`
- 测试：`internal/store/sqlite/sqlite_test.go`

**代码来源：** 逐字抄录 `phase1/task-2-brief.md`，然后应用下列修复。

- [ ] **步骤 1：编写失败测试**

抄录 `phase1/task-2-brief.md` Step 1 的 `sqlite_test.go` 全文（`newTestStore`、`TestContentTypeRepoCRUD`、`TestContentRepoCRUD`、`TestSettingRepo`）。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/store/sqlite/ -run TestContentTypeRepoCRUD -v`
预期：FAIL（`Open` 未定义）

- [ ] **步骤 3：实现模型与接口**

抄录 `phase1/task-2-brief.md` Step 3 的 `internal/store/model.go` 与 `internal/store/store.go`，**并应用 A 类修复——给模型加小写 JSON tag（final 契约）**：

```go
type ContentType struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Label  string `json:"label"`
	Fields string `json:"fields"`
	Config string `json:"config"`
}

type Content struct {
	ID            int64      `json:"id"`
	ContentTypeID int64      `json:"content_type_id"`
	ContentID     string     `json:"content_id"`
	Lang          string     `json:"lang"`
	Slug          string     `json:"slug"`
	Title         string     `json:"title"`
	Status        string     `json:"status"`
	CreatedBy     int64      `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	Payload       string     `json:"payload"`
}
```

接口照抄 brief。**扩展 ContentRepo 两个方法（final 契约，供翻译与后台列表用）**：

```go
type ContentRepo interface {
	Create(ctx context.Context, c *Content) error
	Update(ctx context.Context, c *Content) error
	GetByID(ctx context.Context, id int64) (Content, error)
	GetBySlugLangStatus(ctx context.Context, typeName, slug, lang, status string) (Content, error)
	ListByContentID(ctx context.Context, contentID string) ([]Content, error)
	ListByTypeLangStatus(ctx context.Context, typeName, lang, status string, offset, limit int) ([]Content, error)
	CountByTypeLangStatus(ctx context.Context, typeName, lang, status string) (int, error)
	Delete(ctx context.Context, id int64) error
}
```

- [ ] **步骤 4：实现 SQLite 存储（含 A/B 类修复）**

抄录 `phase1/task-2-brief.md` Step 4 的 `migrate.go` 与 `sqlite.go`，**应用以下 A 类修复**：

1. **去重 `SetMaxOpenConns(1)`**（brief 中重复出现两次，保留一次即可）：
```go
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写者，限制连接数
	db.SetConnMaxLifetime(0)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}
```

2. **`time.Parse` 错误不再静默丢弃**——`scanContent` 改为返回错误：
```go
func scanContent(row interface{ Scan(...any) error }) (store.Content, error) {
	var c store.Content
	var created, updated string
	var published *string
	err := row.Scan(&c.ID, &c.ContentTypeID, &c.ContentID, &c.Lang, &c.Slug, &c.Title,
		&c.Status, &c.CreatedBy, &created, &updated, &published, &c.Payload)
	if err != nil {
		return c, wrapErr(err)
	}
	if c.CreatedAt, err = time.Parse(tsLayout, created); err != nil {
		return c, fmt.Errorf("解析 created_at: %w", err)
	}
	if c.UpdatedAt, err = time.Parse(tsLayout, updated); err != nil {
		return c, fmt.Errorf("解析 updated_at: %w", err)
	}
	if published != nil {
		t, e := time.Parse(tsLayout, *published)
		if e != nil {
			return c, fmt.Errorf("解析 published_at: %w", e)
		}
		c.PublishedAt = &t
	}
	return c, nil
}
```

3. **`Update` 同时更新 `content_id`/`content_type_id`**：
```go
func (r *contentRepo) Update(ctx context.Context, c *store.Content) error {
	c.UpdatedAt = time.Now().UTC()
	var pub any
	if c.PublishedAt != nil {
		pub = c.PublishedAt.UTC().Format(tsLayout)
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE content SET content_type_id=?, content_id=?, lang=?, slug=?, title=?, status=?, created_by=?, updated_at=?, published_at=?, payload=? WHERE id=?",
		c.ContentTypeID, c.ContentID, c.Lang, c.Slug, c.Title, c.Status, c.CreatedBy,
		c.UpdatedAt.Format(tsLayout), pub, c.Payload, c.ID)
	return err
}
```

4. **`ListByContentID` 实现**（追加）：
```go
func (r *contentRepo) ListByContentID(ctx context.Context, contentID string) ([]store.Content, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+contentCols+" FROM content c WHERE c.content_id = ? ORDER BY c.id", contentID)
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
```

5. **`status == ""` 视为不过滤**（后台列表要同时看草稿/已发布）——改写 `ListByTypeLangStatus`/`CountByTypeLangStatus` 动态拼 WHERE：
```go
func (r *contentRepo) ListByTypeLangStatus(ctx context.Context, typeName, lang, status string, offset, limit int) ([]store.Content, error) {
	query := "SELECT " + contentCols + " FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=?"
	args := []any{typeName, lang}
	if status != "" {
		query += " AND c.status=?"
		args = append(args, status)
	}
	query += " ORDER BY c.published_at DESC, c.id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	// ...（同 brief，遍历 scanContent）
}

func (r *contentRepo) CountByTypeLangStatus(ctx context.Context, typeName, lang, status string) (int, error) {
	query := "SELECT COUNT(*) FROM content c JOIN content_types t ON t.id = c.content_type_id WHERE t.name=? AND c.lang=?"
	args := []any{typeName, lang}
	if status != "" {
		query += " AND c.status=?"
		args = append(args, status)
	}
	var n int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}
```

6. **UNIQUE 冲突包装（final 契约，`wrapUnique`）**——追加到 sqlite.go：
```go
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func wrapUnique(err error) error {
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: slug 已存在", errs.ErrValidation)
	}
	return err
}
```
并在 `contentRepo.Create`/`Update` 的 `INSERT`/`UPDATE` 出错路径改走 `wrapUnique`：
```go
	if err != nil {
		return wrapUnique(err)
	}
```
需新增 import `strings`。

- [ ] **步骤 5：追加 B 类测试**（追加到 `sqlite_test.go`）

```go
func TestContentDuplicateSlug(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	if err := types.Create(ctx, ct); err != nil {
		t.Fatal(err)
	}
	repo := st.ContentRepo()
	c := &store.Content{ContentTypeID: ct.ID, ContentID: "grp-1", Lang: "zh", Slug: "hello", Status: "published"}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	// 同 (lang, content_type_id, slug) 二次 Create → ErrValidation
	if err := repo.Create(ctx, &store.Content{ContentTypeID: ct.ID, ContentID: "grp-2", Lang: "zh", Slug: "hello", Status: "draft"}); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("重复 slug = %v, want ErrValidation", err)
	}
	// 同组不同 slug 仍可创建
	if err := repo.Create(ctx, &store.Content{ContentTypeID: ct.ID, ContentID: "grp-2", Lang: "zh", Slug: "other", Status: "draft"}); err != nil {
		t.Errorf("同组不同 slug 应成功: %v", err)
	}
}

func TestContentUpdateAndGetByID(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	types := st.ContentTypeRepo()
	ct := &store.ContentType{Name: "article", Label: "文章", Fields: `[]`}
	types.Create(ctx, ct)
	repo := st.ContentRepo()
	c := &store.Content{ContentTypeID: ct.ID, ContentID: "g", Lang: "zh", Slug: "a", Title: "旧", Status: "draft"}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, c.ID)
	if err != nil || got.Title != "旧" {
		t.Fatalf("GetByID = %+v, %v", got, err)
	}
	// Update 更新 content_id/content_type_id
	c.Title = "新"
	c.ContentID = "g2"
	if err := repo.Update(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetByID(ctx, c.ID)
	if got.Title != "新" || got.ContentID != "g2" {
		t.Errorf("Update 后 = %+v", got)
	}
	if err := repo.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, c.ID); err != errs.ErrNotFound {
		t.Errorf("删除后 = %v, want ErrNotFound", err)
	}
}

func TestSettingUpsert(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.SettingRepo()
	if err := repo.Set(ctx, "k", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Set(ctx, "k", "v2"); err != nil { // upsert 覆盖
		t.Fatal(err)
	}
	v, err := repo.Get(ctx, "k")
	if err != nil || v != "v2" {
		t.Errorf("Get = %q, %v", v, err)
	}
}
```
需新增 import `errors`。

- [ ] **步骤 6：运行测试验证通过**

运行：`go test -count=1 ./internal/store/sqlite/ -v`
预期：全部 PASS

---

### 任务 3：schema 引擎 v1（含 A/B 类修复）

**文件：**
- 创建：`internal/schema/schema.go`、`internal/schema/validate.go`、`internal/schema/schema_test.go`
- 测试：`internal/schema/schema_test.go`

**代码来源：** 逐字抄录 `phase1/task-3-brief.md`，然后应用下列修复。

- [ ] **步骤 1：编写失败测试**

抄录 `phase1/task-3-brief.md` Step 1 的 `schema_test.go` 全文（`validCT`、`TestValidateContentType`、`TestValidateDocument`）。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/schema/ -run TestValidateContentType -v`
预期：FAIL（`NewRegistry` 未定义）

- [ ] **步骤 3：实现 schema 模型与校验（含 A 类修复）**

抄录 `phase1/task-3-brief.md` Step 3 的 `schema.go` 与 `validate.go`，**应用 A 类修复**：

1. **`schema.ContentType` 加 JSON tag**（final 契约，adminapi 需序列化）：
```go
type ContentType struct {
	ID     int64          `json:"id"`
	Name   string         `json:"name"`
	Label  string         `json:"label"`
	Fields []Field        `json:"fields"`
	Config map[string]any `json:"config,omitempty"`
}
```

2. **pattern 正则编译缓存**——`Registry` 增加 `patterns map[string]*regexp.Regexp`，`NewRegistry` 初始化，`validateFieldValue` 改为方法 `(r *Registry) validateFieldValue(f Field, v any) error`（`ValidateDocument` 内调用点同步改），pattern 命中缓存：
```go
func (r *Registry) getPattern(p string) (*regexp.Regexp, error) {
	if re, ok := r.patterns[p]; ok {
		return re, nil
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, err
	}
	r.patterns[p] = re
	return re, nil
}
```
（`ValidateDocument` 中 `if err := r.validateFieldValue(f, v); err != nil`。）

3. **multiselect/repeat 接受 `[]string` 或 `[]any`**（JSON 解码是 `[]any`，Go 直构可能 `[]string`）：
```go
func toStrSlice(v any) ([]string, bool) {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprint(item))
		}
		return out, true
	case []string:
		return t, true
	default:
		return nil, false
	}
}
```
`TypeMultiSelect`/`TypeRepeat` 分支改用：
```go
	case TypeMultiSelect:
		items, ok := toStrSlice(v)
		if !ok {
			return fmt.Errorf("字段 %q 需要字符串数组", f.Name)
		}
		for _, item := range items {
			if !contains(f.Options, item) {
				return fmt.Errorf("字段 %q 值 %q 不在选项内", f.Name, item)
			}
		}
	case TypeRepeat:
		if _, ok := toStrSlice(v); !ok {
			return fmt.Errorf("字段 %q 需要数组", f.Name)
		}
```

4. **`toFloat` 支持 `json.Number`/`float32`/`int64`**：
```go
func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		n, err := t.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(t, 64)
		return n, err == nil
	default:
		return 0, false
	}
}
```
需新增 import `encoding/json`。

- [ ] **步骤 4：追加 B 类测试（表驱动补齐覆盖）**（追加到 `schema_test.go`）

```go
func TestValidateDocumentCoverage(t *testing.T) {
	reg := NewRegistry()
	ct := ContentType{Name: "post", Label: "帖子", Fields: []Field{
		{Name: "title", Label: "标题", Type: TypeText, Required: true},
		{Name: "pub", Label: "时间", Type: TypeDateTime},
		{Name: "email", Label: "邮箱", Type: TypeText, Pattern: `^[^@]+@[^@]+$`},
		{Name: "tags", Label: "标签", Type: TypeMultiSelect, Options: []string{"go", "web"}},
	}}
	cases := []struct {
		name string
		data map[string]any
		want bool
	}{
		{"合法", map[string]any{"title": "t", "pub": "2026-08-06T10:00:00Z", "email": "a@b.com", "tags": []string{"go"}}, true},
		{"缺必填", map[string]any{}, false},
		{"datetime 非法", map[string]any{"title": "t", "pub": "2026/08/06"}, false},
		{"pattern 不匹配", map[string]any{"title": "t", "email": "nope"}, false},
		{"multiselect 值不在选项", map[string]any{"title": "t", "tags": []any{"rust"}}, false},
	}
	for _, c := range cases {
		err := reg.ValidateDocument(&ct, c.data)
		if (err == nil) != c.want {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello World":     "hello-world",
		"你好":              "",
		"Go & Web":        "go-web",
		"  spaced  ":      "spaced",
		"a--b":            "a-b",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIndexFieldAndDecode(t *testing.T) {
	ct := ContentType{Fields: []Field{
		{Name: "title", Type: TypeText, Indexed: true},
		{Name: "body", Type: TypeRichText},
	}}
	f := IndexField(&ct)
	if f == nil || f.Name != "title" {
		t.Errorf("IndexField = %+v", f)
	}
	if got := IndexField(&ContentType{}); got != nil {
		t.Errorf("无索引字段应为 nil")
	}
	fs, err := DecodeFields(`[{"name":"title","type":"text"}]`)
	if err != nil || len(fs) != 1 || fs[0].Name != "title" {
		t.Errorf("DecodeFields = %+v, %v", fs, err)
	}
}

func TestAllTypes(t *testing.T) {
	reg := NewRegistry()
	all := reg.AllTypes()
	if len(all) != 14 {
		t.Errorf("AllTypes = %d, want 14", len(all))
	}
	for _, ft := range all {
		if !reg.IsValid(ft) {
			t.Errorf("IsValid(%s) = false", ft)
		}
	}
	if reg.IsValid(FieldType("nope")) {
		t.Error("非法类型应无效")
	}
}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/schema/ -v`
预期：全部 PASS

---

### 任务 4：content 服务（含 reservedNames 契约与 A 类修复）

**文件：**
- 创建：`internal/content/service.go`、`internal/content/util.go`、`internal/content/service_test.go`
- 测试：`internal/content/service_test.go`

**代码来源：** 逐字抄录 `phase1/task-4-brief.md`，然后应用下列契约与修复。

- [ ] **步骤 1：编写失败测试**

抄录 `phase1/task-4-brief.md` Step 1 的 `service_test.go` 全文，**但 `newTestService` 适配 3 参 `New`（final 契约）**：

```go
func newTestService(t *testing.T) (*Service, store.Store) {
	t.Helper()
	st, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	reg := schema.NewRegistry()
	svc := New(st, reg, []string{"zh", "en"}) // reservedNames
	// ...（其余照抄，含 CreateType）
	return svc, st
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/content/ -run TestCreateAndGetPublished -v`
预期：FAIL（`New` 未定义）

- [ ] **步骤 3：实现 content 服务（含契约修复）**

抄录 `phase1/task-4-brief.md` Step 3 的 `service.go` 与 `util.go`，**应用 final 契约**：

1. **`Entry` 加 JSON tag（final 契约）**：
```go
type Entry struct {
	Content  store.Content  `json:"content"`
	TypeName string         `json:"type_name"`
	Fields   map[string]any `json:"fields"`
}
```

2. **`New` 增加 `reservedNames` 参数 + 类型名冲突校验（final 契约）**：
```go
type Service struct {
	store         store.Store
	reg           *schema.Registry
	reservedNames map[string]bool
}

func New(st store.Store, reg *schema.Registry, reservedNames []string) *Service {
	rn := make(map[string]bool, len(reservedNames))
	for _, n := range reservedNames {
		rn[n] = true
	}
	return &Service{store: st, reg: reg, reservedNames: rn}
}

func (s *Service) checkReservedName(name string) error {
	if s.reservedNames[name] {
		return fmt.Errorf("%w: 内容类型名 %q 与语言代码冲突", errs.ErrValidation, name)
	}
	return nil
}
```
`CreateType` 与 `UpdateType` 在 `ValidateContentType` 之后调用：
```go
	if err := s.checkReservedName(ct.Name); err != nil {
		return err
	}
```

3. **追加 `TestCreateTypeReservedName`（final 契约测试）** 到 `service_test.go`：
```go
func TestCreateTypeReservedName(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestService(t)
	ct := &schema.ContentType{Name: "zh", Label: "冲突", Fields: []schema.Field{{Name: "t", Type: schema.TypeText}}}
	if err := svc.CreateType(ctx, ct); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("创建 zh 类型 = %v, want ErrValidation", err)
	}
	// 未落库
	if _, err := st.ContentTypeRepo().GetByName(ctx, "zh"); err != errs.ErrNotFound {
		t.Errorf("冲突类型不应落库, got %v", err)
	}
}
```
需新增 import `errors`。

- [ ] **步骤 4：追加 B 类修复（TestUpdateAndDelete 删除断言加强）**

`phase1/task-4-brief.md` 的 `TestUpdateAndDelete` 删除后断言改为同时校验 update 结果：
```go
	if upd.Content.Title != "t2" {
		t.Errorf("Title = %q, want t2", upd.Content.Title)
	}
	if upd.Content.ContentID == "" {
		t.Error("Update 后 ContentID 不应为空")
	}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/content/ -v`
预期：全部 PASS

---

### 任务 5：i18n 语言注册表（含 A/B 类修复）

**文件：**
- 创建：`internal/i18n/i18n.go`、`internal/i18n/i18n_test.go`
- 测试：`internal/i18n/i18n_test.go`

**代码来源：** 逐字抄录 `phase1/task-5-brief.md`，然后应用下列修复。

- [ ] **步骤 1：编写失败测试**

抄录 `phase1/task-5-brief.md` Step 1 的 `i18n_test.go` 全文（`TestResolvePath`、`TestURLPath`、`TestInvalidDefault`）。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/i18n/ -run TestResolvePath -v`
预期：FAIL（`New` 未定义）

- [ ] **步骤 3：实现 i18n（含 A 类修复）**

抄录 `phase1/task-5-brief.md` Step 3 的 `i18n.go`，**应用 A 类修复**：

1. **`All()` 返回副本**（防调用方修改内部切片）：
```go
func (r *Registry) All() []Lang {
	out := make([]Lang, len(r.langs))
	copy(out, r.langs)
	return out
}
```

2. **`New` 语言去重**（重复 code 只保留一个）：
```go
func New(codes []string, def string, prefixDefault bool) (*Registry, error) {
	if len(codes) == 0 {
		return nil, fmt.Errorf("语言列表不能为空")
	}
	seen := map[string]bool{}
	langs := make([]Lang, 0, len(codes))
	for _, c := range codes {
		if seen[c] {
			continue
		}
		seen[c] = true
		langs = append(langs, Lang{Code: c, Label: labelOf(c)})
	}
	found := false
	for _, l := range langs {
		if l.Code == def {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("默认语言 %q 不在语言列表中", def)
	}
	return &Registry{langs: langs, def: def, prefixDefault: prefixDefault}, nil
}
```

3. **`URLPath` 前置条件校验**（`lang` 必须是注册语言，否则回退默认语言路径）：
```go
func (r *Registry) URLPath(lang, rest string) string {
	if !r.IsValid(lang) {
		lang = r.def
	}
	// ...（其余照抄）
}
```

- [ ] **步骤 4：追加 B 类测试**（追加到 `i18n_test.go`）：

```go
func TestResolvePathUnknownLangValue(t *testing.T) {
	reg, _ := New([]string{"zh", "en"}, "zh", false)
	lang, segs := reg.ResolvePath("/fr/article/x")
	if lang != "zh" {
		t.Errorf("未知语言 lang = %q, want zh", lang)
	}
	if len(segs) != 3 || segs[0] != "fr" {
		t.Errorf("未知语言 segs = %v", segs)
	}
}

func TestNewDedupe(t *testing.T) {
	reg, err := New([]string{"zh", "zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reg.All()); got != 2 {
		t.Errorf("去重后语言数 = %d, want 2", got)
	}
}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/i18n/ -v`
预期：全部 PASS

---

### 任务 6：SEO（Meta/hreflang/JSON-LD/sitemap/robots，含 final 契约）

**文件：**
- 创建：`internal/seo/seo.go`、`internal/seo/sitemap.go`、`internal/seo/seo_test.go`
- 测试：`internal/seo/seo_test.go`

**代码来源：** 逐字抄录 `phase1/task-6-brief.md`，然后应用 final-fix 契约与 A/B 修复。

- [ ] **步骤 1：编写失败测试**

抄录 `phase1/task-6-brief.md` Step 1 的 `seo_test.go` 全文，**但按 final 契约调整断言**：
- `TestBuildEntry` 中 `len(m.HrefLangs)` 改为 **3**（含 x-default）；新增断言存在 `x-default -> https://example.com/article/hello`
- 新增 `TestBuildEntryXDStartsWith`：`en` 页面下 x-default 为首条且指向默认语言 URL
- `TestBuildEntry` 的 JSON-LD 断言改为以 `<script type="application/ld+json">` 开头、包含未转义 `{"@context"`、不含 `\"` 或 `\u003c`

```go
func TestBuildEntry(t *testing.T) {
	b := testBuilder(t)
	now := time.Now().UTC()
	e := content.Entry{
		TypeName: "article",
		Content: store.Content{Slug: "hello", Title: "你好世界", Status: "published", PublishedAt: &now},
		Fields:  map[string]any{"excerpt": "一句话摘要"},
	}
	m := b.BuildEntry("zh", e)
	if !strings.Contains(m.Title, "你好世界") {
		t.Errorf("title = %q", m.Title)
	}
	if m.Canonical != "https://example.com/article/hello" {
		t.Errorf("canonical = %q", m.Canonical)
	}
	if len(m.HrefLangs) != 3 {
		t.Fatalf("hreflangs = %+v, want 3 (x-default/zh/en)", m.HrefLangs)
	}
	foundXD := false
	foundEn := false
	for _, h := range m.HrefLangs {
		if h.Lang == "x-default" && h.URL == "https://example.com/article/hello" {
			foundXD = true
		}
		if h.Lang == "en" && h.URL == "https://example.com/en/article/hello" {
			foundEn = true
		}
	}
	if !foundXD {
		t.Errorf("缺少 x-default hreflang: %+v", m.HrefLangs)
	}
	if !foundEn {
		t.Errorf("缺少 en hreflang: %+v", m.HrefLangs)
	}
	if m.OGTags["type"] != "article" {
		t.Errorf("og:type = %q", m.OGTags["type"])
	}
	if m.Description != "一句话摘要" {
		t.Errorf("description = %q", m.Description)
	}
	js := string(m.JSONLDScript)
	if !strings.HasPrefix(js, `<script type="application/ld+json">`) || !strings.Contains(js, `{"@context"`) {
		t.Errorf("JSON-LD 未包成完整 script 片段或对象被转义: %s", js)
	}
	if strings.Contains(js, `\"`) || strings.Contains(js, `\u003c`) {
		t.Errorf("JSON-LD 被转义: %s", js)
	}
}

func TestBuildEntryXDStartsWith(t *testing.T) {
	b := testBuilder(t)
	now := time.Now().UTC()
	e := content.Entry{TypeName: "article", Content: store.Content{Slug: "hello", Title: "t", PublishedAt: &now}}
	m := b.BuildEntry("en", e)
	if len(m.HrefLangs) == 0 || m.HrefLangs[0].Lang != "x-default" {
		t.Fatalf("x-default 应为首条: %+v", m.HrefLangs)
	}
	if m.HrefLangs[0].URL != "https://example.com/article/hello" {
		t.Errorf("x-default 应指向默认语言 URL: %+v", m.HrefLangs[0])
	}
}
```
其余测试（`TestBuildHome`、`TestSitemapXML`、`TestRobotsTXT`）照抄 brief。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/seo/ -run TestBuildEntry -v`
预期：FAIL（`NewBuilder` 未定义）

- [ ] **步骤 3：实现 seo 包（含 final 契约与 A 类修复）**

抄录 `phase1/task-6-brief.md` Step 3 的 `seo.go` 与 `sitemap.go`，**应用 final 契约与 A 类修复**：

1. **JSON-LD 包成完整 `<script>` 片段（final 契约）**——`jsonLDScript` 辅助函数，`BuildEntry`/`BuildHome` 使用：
```go
func jsonLDScript(ld []byte) template.HTML {
	return template.HTML(`<script type="application/ld+json">` + string(ld) + `</script>`)
}
```
`BuildEntry` 中 `JSONLDScript: jsonLDScript(ld)`；`BuildHome` 同。

2. **hrefLangs 前置 x-default（final 契约）**：
```go
func (b *Builder) hrefLangs(lang, path string) []HrefLang {
	out := make([]HrefLang, 0, len(b.reg.All())+1)
	out = append(out, HrefLang{Lang: "x-default", URL: b.siteURL + b.reg.URLPath(b.reg.Default(), path)})
	for _, l := range b.reg.All() {
		out = append(out, HrefLang{Lang: l.Code, URL: b.siteURL + b.reg.URLPath(l.Code, path)})
	}
	return out
}
```

3. **`SitemapXML` 补 XML 声明（A 类）**——不用 `xml.MarshalIndent` 直接输出，改为：
```go
func SitemapXML(entries []SitemapEntry) ([]byte, error) {
	items := make([]urlItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, urlItem{Loc: e.Loc, LastMod: e.LastMod})
	}
	body, err := xml.MarshalIndent(urlset{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: items}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}
```

4. **`RobotsTXT` 尾斜杠规范化（A 类）**：
```go
func RobotsTXT(siteURL string) []byte {
	base := strings.TrimRight(siteURL, "/")
	return []byte("User-agent: *\nAllow: /\n\nSitemap: " + base + "/sitemap.xml\n")
}
```
需新增 import `strings`。

5. **`json.Marshal` 错误不再静默（A 类）**——`BuildEntry`/`BuildHome` 改为返回 `(Meta, error)` 有破坏性；更稳妥做法：`ld, err := json.Marshal(...); if err != nil { ld = []byte(`{}`) }`，并把错误记入 `Meta.Description`？——为保持签名不变，采用"marshal 失败则放弃 JSON-LD 片段"：
```go
	ld, err := json.Marshal(map[string]any{...})
	if err != nil {
		ld = nil
	}
	...
	if len(ld) > 0 {
		m.JSONLDScript = jsonLDScript(ld)
	}
```
（`BuildEntry` 与 `BuildHome` 同样处理。）

- [ ] **步骤 4：追加 B 类测试**（追加到 `seo_test.go`）：

```go
func TestBuildHomeJSONLD(t *testing.T) {
	b := testBuilder(t)
	m := b.BuildHome("en")
	js := string(m.JSONLDScript)
	if !strings.HasPrefix(js, `<script type="application/ld+json">`) {
		t.Errorf("home JSON-LD 未包 script: %s", js)
	}
}

func TestRobotsTXTNoTrailingSlash(t *testing.T) {
	out := RobotsTXT("https://example.com/")
	if !strings.Contains(string(out), "Sitemap: https://example.com/sitemap.xml") {
		t.Errorf("robots 尾斜杠未规范化: %s", out)
	}
}

func TestSitemapXMLDeclaration(t *testing.T) {
	xml, err := SitemapXML([]SitemapEntry{{Loc: "https://example.com/x", LastMod: ""}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(xml), `<?xml version="1.0"`) {
		t.Errorf("sitemap 缺 XML 声明: %s", xml)
	}
}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/seo/ -v`
预期：全部 PASS

---

### 任务 7：主题系统（含并发缓存修复）

**文件：**
- 创建：`internal/theme/base.html`、`internal/theme/theme.go`、`internal/theme/funcs.go`、`internal/theme/theme_test.go`
- 测试：`internal/theme/theme_test.go`

**代码来源：** 逐字抄录 `phase1/task-7-brief.md`，然后应用下列修复。

- [ ] **步骤 1：编写失败测试**

抄录 `phase1/task-7-brief.md` Step 1 的 `theme_test.go` 全文（`writeFixtureTheme`、`testTheme`、`TestRenderIndex`、`TestRenderSingleUsesTypeMap`、`TestLookupLocale`、`TestRenderNotFound`）。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/theme/ -run TestRenderIndex -v`
预期：FAIL（`NewLoader` 未定义）

- [ ] **步骤 3：实现主题系统（含竞态修复）**

抄录 `phase1/task-7-brief.md` Step 3 的 `base.html`、`theme.go`、`funcs.go`，**应用竞态修复（progress 记录：renderSet/Loader 缓存并发竞态已加 sync.Mutex）**：

```go
type Theme struct {
	Name      string
	Version   string
	Dir       string
	Templates map[string]string
	TypeMap   map[string]string
	Locales   map[string]map[string]string
	reg       *i18n.Registry
	funcs     template.FuncMap
	cache     map[string]*template.Template
	mu        sync.Mutex
}

type Loader struct {
	ThemesDir string
	reg       *i18n.Registry
	cache     map[string]*Theme
	mu        sync.Mutex
}

func (l *Loader) Get(name string) (*Theme, error) {
	l.mu.Lock()
	th, ok := l.cache[name]
	l.mu.Unlock()
	if ok {
		return th, nil
	}
	return l.Load(name)
}

func (l *Loader) Load(name string) (*Theme, error) {
	// ...（同 brief）...
	l.mu.Lock()
	l.cache[name] = th
	l.mu.Unlock()
	return th, nil
}

func (t *Theme) renderSet(key string) (*template.Template, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if tpl, ok := t.cache[key]; ok {
		return tpl, nil
	}
	// ...（同 brief）...
	t.cache[key] = tpl
	return tpl, nil
}
```
需新增 import `sync`。

**base.html 关键点（final 契约）**：JSON-LD 在 HTML 上下文直接渲染完整片段（brief 的 base.html 第 185 行即为此式）：
```html
{{if .Meta.JSONLDScript}}{{.Meta.JSONLDScript}}{{end}}
```
（不要改成 `<script>` 标签包一个裸 JSON 值——html/template 的 `<script>` 上下文会把值当 JS 转义。）

- [ ] **步骤 4：追加 B 类测试（url 函数 en 前缀）**（追加到 `theme_test.go`）：

```go
func TestRenderIndexEnPrefix(t *testing.T) {
	th, reg := testTheme(t)
	buf := &bytes.Buffer{}
	entry := content.Entry{TypeName: "article", Content: store.Content{Slug: "hello", Title: "你好世界"}}
	err := th.Render(buf, "index", &Data{
		Site: SiteInfo{Name: "测试站点", URL: "https://example.com"},
		Lang: "en",
		Langs: reg.All(),
		Items: []content.Entry{entry},
		Meta: seo.Meta{Title: "x", Canonical: "https://example.com/en/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `href="/en/article/hello"`) {
		t.Errorf("en 前缀 URL 缺失: %s", buf.String())
	}
}
```

- [ ] **步骤 5：运行测试验证通过（含 -race）**

运行：`go test -count=1 -race ./internal/theme/ -v`
预期：全部 PASS，无数据竞态

---

### 任务 8：默认主题

**文件：**
- 创建：`themes/default/theme.yaml`、`templates/{index,list,single}.html`、`templates/partials/{nav,footer}.html`、`static/css/main.css`、`static/js/main.js`、`locales/{zh,en}.yaml`
- 创建：`themes/default/theme_test.go`

**代码来源：** 逐字抄录 `phase1/task-8-brief.md` 全部内容（无代码修复，C 类 minor 保持原样：`single.html` 嵌套 if 冗余、`Fields` map 遍历顺序、`raw` 仅 richtext 用，均照抄）。

- [ ] **步骤 1：写默认主题**

抄录 `phase1/task-8-brief.md` Step 1 的 `theme.yaml`、Step 2 的模板与静态资源、locales。

- [ ] **步骤 2：写验证测试**

抄录 `phase1/task-8-brief.md` Step 3 的 `themes/default/theme_test.go`（`TestDefaultThemeLoadsAndRenders`、`contains` helper）。

- [ ] **步骤 3：运行验证测试**

运行：`go test -count=1 ./themes/default/ -v`
预期：PASS

---

### 任务 9：server 装配 + 前台路由 + seed + 入口（含 final 契约）

**文件：**
- 创建：`internal/server/server.go`、`internal/server/frontend.go`、`internal/server/frontend_test.go`
- 创建：`internal/seed/seed.go`
- 修改：`cmd/dulizhan/main.go`
- 测试：`internal/server/frontend_test.go`

**代码来源：** 逐字抄录 `phase1/task-9-brief.md`，然后应用下列修复。

- [ ] **步骤 1：编写失败测试**

抄录 `phase1/task-9-brief.md` Step 1 的 `frontend_test.go` 全文，**适配 final 契约**：
- `buildTestServer` 中 `content.New(st, schema.NewRegistry())` → `content.New(st, schema.NewRegistry(), []string{"zh", "en"})`
- 新增断言（final-fix）：`/article/hello-zh` 响应含 `application/ld+json">{"@context"`（未转义 JSON-LD）与 x-default hreflang 链接
- 新增 `TestFrontendRoutes` 对 `/article/not-exist` 断言 404 页面含 "404"

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/server/ -run TestFrontendRoutes -v`
预期：FAIL（`seed.Run`/`New` 未定义）

- [ ] **步骤 3：实现 seed + server + 前台路由**

抄录 `phase1/task-9-brief.md` Step 3（`seed.go`）、Step 4（`server.go`+`frontend.go`）、Step 5（`main.go`），**应用 final 契约与 A 类修复**：

1. **gin NoRoute 预置 404 修复（task-9-report）**——`handleFrontend` 成功渲染前写 200：
```go
func (s *Server) handleFrontend(c *gin.Context) {
	lang, segs := s.i18n.ResolvePath(c.Request.URL.Path)
	th, err := s.themes.Get(s.cfg.Site.Theme)
	if err != nil {
		c.String(http.StatusInternalServerError, "主题加载失败: %v", err)
		return
	}
	c.Status(http.StatusOK) // gin NoRoute 预置 404，成功渲染前显式置 200
	data := &theme.Data{...}
	// ...（分发逻辑照抄）
}
```

2. **renderList 错误映射（final 契约）**——`GetType` 失败时 `errors.Is(errs.ErrNotFound)` 才 404，其余 500：
```go
func (s *Server) renderList(c *gin.Context, th *theme.Theme, lang, typeName string, data *theme.Data) {
	if _, err := s.content.GetType(c.Request.Context(), typeName); err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			s.render404(c, th, lang)
		} else {
			c.String(http.StatusInternalServerError, "查询失败: %v", err)
		}
		return
	}
	// ...（其余照抄）
}
```

3. **handleSitemap 不吞 `ListPublished` 错误（A 类）**——brief 中 `continue` 改为返回 500：
```go
	for _, ct := range types {
		for _, l := range s.i18n.All() {
			items, _, err := s.content.ListPublished(ctx, ct.Name, l.Code, 1, 1000)
			if err != nil && !errors.Is(err, errs.ErrNotFound) {
				c.String(http.StatusInternalServerError, "查询失败: %v", err)
				return
			}
			// ...（其余照抄）
		}
	}
```

4. **main.go seed 子命令不依赖 flag 位置且不强制 config.yaml（A 类）**——用 `flag.Args()` 检测子命令，seed 模式 config 可缺省：
```go
func main() {
	cfgPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	args := flag.Args()
	if len(args) > 0 && args[0] == "seed" {
		// seed 模式：config.yaml 可选，缺失时用默认值
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			cfg = &config.Config{}
			cfg.Database.Driver = "sqlite"
			cfg.Database.DSN = "dulizhan.db"
			cfg.Site.Languages = []string{"zh", "en"}
			cfg.Site.DefaultLang = "zh"
		}
		st, err := sqlite.Open(cfg.Database.DSN)
		if err != nil {
			log.Fatalf("打开数据库失败: %v", err)
		}
		reg, err := i18n.New(cfg.Site.Languages, cfg.Site.DefaultLang, cfg.Site.PrefixDefault)
		if err != nil {
			log.Fatalf("语言配置错误: %v", err)
		}
		svc := content.New(st, schema.NewRegistry(), cfg.Site.Languages)
		if err := seed.Run(context.Background(), svc); err != nil {
			log.Fatalf("初始化数据失败: %v", err)
		}
		fmt.Println("种子数据初始化完成")
		return
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	// ...（正常启动路径照抄 brief Step 5）
}
```

- [ ] **步骤 4：追加 B 类测试（主题静态 + ?page=）**（追加到 `frontend_test.go`）：

```go
func TestThemeStatic(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	code, body := get(t, srv, "/themes/default/static/css/main.css")
	if code != http.StatusOK || !strings.Contains(body, ":root") {
		t.Errorf("主题静态资源 = %d", code)
	}
	// 路径穿越防护
	code, _ = get(t, srv, "/themes/default/static/../../config.yaml")
	if code != http.StatusNotFound {
		t.Errorf("路径穿越 = %d, want 404", code)
	}
}

func TestListPagination(t *testing.T) {
	srv := buildTestServer(t, "../../themes")
	srv.cfg.Site.Theme = "default"
	code, body := get(t, srv, "/article?page=2")
	if code != http.StatusOK {
		t.Errorf("列表 page=2 = %d", code)
	}
	if !strings.Contains(body, "pager") {
		t.Errorf("分页缺失: %s", body)
	}
}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/server/ -v`
预期：全部 PASS（含 TestFrontendRoutes / TestSEOEndpoints / TestThemeStatic / TestListPagination）

- [ ] **步骤 6：全量测试 + 手动冒烟**

运行：`go test -count=1 ./...`
预期：全部 PASS

手动冒烟（final 契约验收）：
```bash
$env:GOPROXY="https://goproxy.cn,direct"
go run . seed
go run . --config config.yaml
# 另一终端
curl -s http://localhost:8080/article/hello-zh | Select-String -Pattern 'application/ld\+json">{"@context"' , 'hreflang="x-default"' , 'hreflang="en"'
curl -s http://localhost:8080/sitemap.xml | Select-String -Pattern 'hello-zh' , 'hello-en'
curl -s http://localhost:8080/robots.txt | Select-String -Pattern 'Sitemap:'
```
预期：JSON-LD 未转义、x-default/zh/en hreflang 齐备、sitemap 双语言、robots 含 Sitemap 行。冒烟前确认 8080 无残留进程（`Get-Process | Where-Object {$_.ProcessName -like "*dulizhan*"}` 杀掉）。

---

### 任务 10：phase2 仓库（users/roles/sessions/menus/media）

**文件：**
- 修改：`internal/store/model.go`、`internal/store/store.go`
- 修改：`internal/store/sqlite/sqlite.go`、`internal/store/sqlite/sqlite_test.go`
- 测试：`internal/store/sqlite/sqlite_test.go`

**代码来源：** 逐字抄录 `phase2/task-1-brief.md`，然后应用 A/B 修复。

- [ ] **步骤 1：编写失败测试**

抄录 `phase2/task-1-brief.md` Step 1 的测试全文（`TestUserRoleRepo`、`TestSessionRepo`、`TestMenuMediaRepo`）。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/store/sqlite/ -run TestUserRoleRepo -v`
预期：FAIL（接口未定义）

- [ ] **步骤 3：扩展模型与接口**

抄录 `phase2/task-1-brief.md` Step 3（`model.go` 追加 5 个模型）、Step 4（`store.go` 追加 5 个接口），**应用 final 契约——JSON tag**：

```go
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	RoleID       int64  `json:"role_id"`
}

type Role struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
}

type Session struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Menu struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Lang  string `json:"lang"`
	Items string `json:"items"`
}

type Media struct {
	ID        int64     `json:"id"`
	Filename  string    `json:"filename"`
	URL       string    `json:"url"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **步骤 4：实现 SQLite 仓库（含 A 类修复）**

抄录 `phase2/task-1-brief.md` Step 5（5 个 repo + 5 个访问器），**应用 A 类修复**：

1. **`time.Parse` 错误不再静默丢弃**——`sessionRepo.Get`/`mediaRepo.GetByID`/`mediaRepo.List` 的 `_ = time.Parse(...)` 改为返回错误：
```go
func (r *sessionRepo) Get(ctx context.Context, token string) (store.Session, error) {
	var s store.Session
	var exp string
	err := r.db.QueryRowContext(ctx, "SELECT token, user_id, expires_at FROM sessions WHERE token = ?", token).Scan(&s.Token, &s.UserID, &exp)
	if err != nil {
		return s, wrapErr(err)
	}
	if s.ExpiresAt, err = time.Parse(tsLayout, exp); err != nil {
		return s, fmt.Errorf("解析 expires_at: %w", err)
	}
	return s, nil
}
```
`mediaRepo.GetByID`/`mediaRepo.List` 同理（`created` 解析）。

- [ ] **步骤 5：追加 B 类测试**（追加到 `sqlite_test.go`）：

```go
func TestSessionExpiryRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	repo := st.SessionRepo()
	exp := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	if err := repo.Create(ctx, &store.Session{Token: "tok-r", UserID: 1, ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "tok-r")
	if err != nil {
		t.Fatal(err)
	}
	if !got.ExpiresAt.Equal(exp) {
		t.Errorf("ExpiresAt 往返 = %v, want %v", got.ExpiresAt, exp)
	}
	if err := repo.DeleteByUser(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, "tok-r"); err != errs.ErrNotFound {
		t.Errorf("DeleteByUser 后 = %v", err)
	}
}

func TestUserRoleMenuMediaRepoDetail(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	roles := st.RoleRepo()
	admin, _ := roles.Create(ctx, "admin")
	// RoleRepo.List
	list, err := roles.List(ctx)
	if err != nil || len(list) != 1 {
		t.Errorf("RoleRepo.List = %v, %v", list, err)
	}
	users := st.UserRepo()
	u := &store.User{Username: "u1", PasswordHash: "h", RoleID: admin.ID}
	if err := users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	// UserRepo.GetByID
	got, err := users.GetByID(ctx, u.ID)
	if err != nil || got.Username != "u1" {
		t.Errorf("GetByID = %+v, %v", got, err)
	}
	menus := st.MenuRepo()
	m := &store.Menu{Name: "主导航", Lang: "zh", Items: `[{"label":"首页"}]`}
	if err := menus.Create(ctx, m); err != nil {
		t.Fatal(err)
	}
	// Menu GetByID/Update/Delete
	gotM, err := menus.GetByID(ctx, m.ID)
	if err != nil || gotM.Items == "" {
		t.Errorf("Menu GetByID = %+v, %v", gotM, err)
	}
	m.Items = `[{"label":"关于"}]`
	if err := menus.Update(ctx, m); err != nil {
		t.Fatal(err)
	}
	gotM, _ = menus.GetByID(ctx, m.ID)
	if !strings.Contains(gotM.Items, "关于") {
		t.Errorf("Menu Update 后 = %+v", gotM)
	}
	if err := menus.Delete(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := menus.GetByID(ctx, m.ID); err != errs.ErrNotFound {
		t.Errorf("Menu Delete 后 = %v", err)
	}
}
```
需新增 import `strings`。

- [ ] **步骤 6：运行测试验证通过**

运行：`go test -count=1 ./internal/store/sqlite/ -v`
预期：全部 PASS

---

### 任务 11：auth 服务（bcrypt + 会话）

**文件：**
- 创建：`internal/auth/service.go`、`internal/auth/util.go`、`internal/auth/service_test.go`
- 测试：`internal/auth/service_test.go`

**代码来源：** 逐字抄录 `phase2/task-2-brief.md`，然后应用 B 类修复。

- [ ] **步骤 1：添加依赖**

```bash
$env:GOPROXY="https://goproxy.cn,direct"
go get golang.org/x/crypto@v0.33.0
# 确认 go.mod 中 gin/sqlite 版本未变
Select-String -Path go.mod -Pattern "gin-gonic|modernc"
```

- [ ] **步骤 2：编写失败测试**

抄录 `phase2/task-2-brief.md` Step 2 的 `service_test.go` 全文（`newTestService`、`seedAdmin`、`TestLoginAuthenticateLogout`、`TestLoginWrongPassword`、`TestAuthenticateExpired`、`TestCreateUserDuplicate`、`TestSetUserRoleAndPassword`）。

- [ ] **步骤 3：运行确认失败**

运行：`go test -count=1 ./internal/auth/ -run TestLoginAuthenticateLogout -v`
预期：FAIL（`auth.New` 未定义）

- [ ] **步骤 4：实现 auth 服务**

抄录 `phase2/task-2-brief.md` Step 4（`service.go` + `util.go`）。

- [ ] **步骤 5：追加 B 类测试**（追加到 `service_test.go`）：

```go
func TestAuthenticateExpiredDeletesSession(t *testing.T) {
	ctx := context.Background()
	st, _ := sqlite.Open(":memory:")
	s := New(st, -time.Hour)
	role, _ := st.RoleRepo().Create(ctx, "admin")
	u, _ := s.CreateUser(ctx, "admin", "secret123", role.ID)
	if err := st.SessionRepo().Create(ctx, &store.Session{Token: "tok-e", UserID: u.ID, ExpiresAt: time.Now().Add(-time.Minute).UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, "tok-e"); err != errs.ErrForbidden {
		t.Fatalf("过期会话 = %v, want ErrForbidden", err)
	}
	// 过期会话应被删除
	if _, err := st.SessionRepo().Get(ctx, "tok-e"); err != errs.ErrNotFound {
		t.Errorf("过期会话未删除: %v", err)
	}
}

func TestCreateUserStoresBcrypt(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	role, _ := s.store.RoleRepo().Create(ctx, "admin")
	u, err := s.CreateUser(ctx, "bob", "secret123", role.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.store.UserRepo().GetByID(ctx, u.ID)
	if got.PasswordHash == "secret123" {
		t.Error("密码未哈希存储")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("secret123")); err != nil {
		t.Errorf("哈希与密码不匹配: %v", err)
	}
}
```
需新增 import `golang.org/x/crypto/bcrypt`。

- [ ] **步骤 6：运行测试验证通过**

运行：`go test -count=1 ./internal/auth/ -v`
预期：全部 PASS

---

### 任务 12：RBAC Gin 中间件

**文件：**
- 创建：`internal/auth/middleware.go`、`internal/auth/middleware_test.go`
- 测试：`internal/auth/middleware_test.go`

**代码来源：** 逐字抄录 `phase2/task-3-brief.md`，然后应用修复。

- [ ] **步骤 1：编写失败测试**

抄录 `phase2/task-3-brief.md` Step 1 的 `middleware_test.go` 全文，**应用修复：删除测试中未使用的 `/open` 路由**（progress 记 minor "/open 路由未使用"）。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/auth/ -run TestRequireAuthAndPerm -v`
预期：FAIL（`RequireAuth` 未定义）

- [ ] **步骤 3：实现中间件**

抄录 `phase2/task-3-brief.md` Step 3 的 `middleware.go`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/auth/ -v`
预期：全部 PASS

---

### 任务 13：media 本地存储

**文件：**
- 创建：`internal/media/media.go`、`internal/media/media_test.go`
- 测试：`internal/media/media_test.go`

**代码来源：** 逐字抄录 `phase2/task-4-brief.md`，然后应用 A/B 修复。

- [ ] **步骤 1：编写失败测试**

抄录 `phase2/task-4-brief.md` Step 1 的 `media_test.go` 全文。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/media/ -run TestLocalStoreSaveDeleteURL -v`
预期：FAIL（`NewLocalStore` 未定义）

- [ ] **步骤 3：实现 media 包（含 A 类修复）**

抄录 `phase2/task-4-brief.md` Step 3 的 `media.go`，**应用 A 类修复**：

1. **Save/Delete key 净化**（拒绝 `..`、绝对路径、反斜杠）：
```go
func sanitizeKey(key string) (string, bool) {
	if key == "" || strings.Contains(key, "..") || strings.ContainsAny(key, `\/`[0:1]+"\\") || strings.HasPrefix(key, "/") {
		return "", false
	}
	return key, true
}
```
（简化：仅拦截 `..` 与路径分隔符。）
```go
func (s *LocalStore) Save(ctx context.Context, key string, r io.Reader, _ string) (string, error) {
	k, ok := sanitizeKey(key)
	if !ok {
		return "", fmt.Errorf("非法存储键 %q", key)
	}
	full := filepath.Join(s.Dir, filepath.FromSlash(k))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil { // A 类：Save 检查 Close 错误
		return "", err
	}
	return s.URL(k), nil
}
```
（`defer f.Close()` 与显式 `f.Close()` 重复——去掉 defer，仅保留显式 Close。）
`Delete` 同样先 `sanitizeKey`。

- [ ] **步骤 4：追加 B 类测试（TestRandomKey 时间前缀）**（追加到 `media_test.go`）：

```go
func TestRandomKeyFormat(t *testing.T) {
	k, err := RandomKey(".png")
	if err != nil {
		t.Fatal(err)
	}
	// 格式应为 YYYYMM/<hex>.png（替换原 "20" 前缀断言，避免 2100 年失效）
	re := regexp.MustCompile(`^\d{6}/[0-9a-f]{24}\.png$`)
	if !re.MatchString(k) {
		t.Errorf("key 格式异常: %q", k)
	}
}
```
需新增 import `regexp`。同时**删除** brief 中 `TestRandomKey` 的 `HasPrefix(k1, "20")` 断言（改用上面的正则断言），保留 `k1 != k2` 断言。

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/media/ -v`
预期：全部 PASS

---

### 任务 14：seed 角色与默认管理员

**文件：**
- 修改：`internal/seed/seed.go`
- 创建：`internal/seed/seed_test.go`
- 测试：`internal/seed/seed_test.go`

**代码来源：** 逐字抄录 `phase2/task-5-brief.md`（测试含 `TestEnsureAuth`、实现含 `EnsureAuth` + `mustJSON`、admin 权限 `{"*"}`——brief 已含）。

- [ ] **步骤 1：编写失败测试**

抄录 `phase2/task-5-brief.md` Step 1 的 `seed_test.go` 全文。

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/seed/ -run TestEnsureAuth -v`
预期：FAIL（`seed.EnsureAuth` 未定义）

- [ ] **步骤 3：实现 EnsureAuth**

抄录 `phase2/task-5-brief.md` Step 3 的追加代码（`EnsureAuth` + `mustJSON`），并按 brief 备注补 import（`encoding/json`、`dulizhan/internal/auth`、`dulizhan/internal/store`）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/seed/ -v`
预期：全部 PASS

---

### 任务 15：adminapi 全量 handler（缺口重建）

> **本任务为缺口重建**：阶段 2 task-6..9 brief 缺失。代码依据：task-10-brief 的 `register.go`（31 条路由 + handler 名）、final-fix-report（respond/auth/content/content_types 行为）、设计文档 §5（API 语义）、`content/service.go` 现有方法。

**文件：**
- 创建：`internal/adminapi/deps.go`、`internal/adminapi/respond.go`
- 创建：`internal/adminapi/auth.go`、`internal/adminapi/content_types.go`、`internal/adminapi/content.go`
- 创建：`internal/adminapi/media.go`、`internal/adminapi/menus.go`、`internal/adminapi/settings.go`、`internal/adminapi/users.go`、`internal/adminapi/roles.go`
- 创建：`internal/adminapi/adminapi_test.go`
- 测试：`internal/adminapi/adminapi_test.go`

**前置：** 本任务依赖 content 服务的 phase2 扩展方法（`Actor`、`GetByID`、`ListByContentID`、`CreateTranslation`、`ListAdmin`、`DeleteType`），**先完成下面步骤 0（content 扩展）再写 handler**。

- [ ] **步骤 0：content 服务阶段 2 扩展**（修改 `internal/content/service.go`、`internal/content/service_test.go`）

追加（final-fix Finding 2 契约）：
```go
type Actor struct {
	UserID      int64
	IsModerator bool
}

// GetByID 返回单条内容（含类型名与字段）。
func (s *Service) GetByID(ctx context.Context, id int64) (Entry, error) {
	row, err := s.store.ContentRepo().GetByID(ctx, id)
	if err != nil {
		return Entry{}, err
	}
	ct, err := s.GetTypeByID(ctx, row.ContentTypeID)
	if err != nil {
		return Entry{}, err
	}
	return s.entryFromStore(ctx, ct, row)
}

// ListByContentID 返回某翻译组全部语言变体。
func (s *Service) ListByContentID(ctx context.Context, contentID string) ([]Entry, error) {
	rows, err := s.store.ContentRepo().ListByContentID(ctx, contentID)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		ct, err := s.GetTypeByID(ctx, row.ContentTypeID)
		if err != nil {
			return nil, err
		}
		e, err := s.entryFromStore(ctx, ct, row)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// ListAdmin 后台内容列表：status 为空表示不过滤。
func (s *Service) ListAdmin(ctx context.Context, typeName, lang, status string, page, perPage int) ([]Entry, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if _, err := s.GetType(ctx, typeName); err != nil {
		return nil, 0, err
	}
	rows, err := s.store.ContentRepo().ListByTypeLangStatus(ctx, typeName, lang, status, (page-1)*perPage, perPage)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.ContentRepo().CountByTypeLangStatus(ctx, typeName, lang, status)
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

// CreateTranslation 给 contentID 翻译组新增一种语言变体。
// 作者只能给自己创建的内容加翻译，moderator 可翻译他人内容。
func (s *Service) CreateTranslation(ctx context.Context, typeName, lang, contentID string, data map[string]any, actor Actor) (Entry, error) {
	if _, err := s.GetType(ctx, typeName); err != nil {
		return Entry{}, err
	}
	parents, err := s.store.ContentRepo().ListByContentID(ctx, contentID)
	if err != nil {
		return Entry{}, err
	}
	if len(parents) == 0 {
		return Entry{}, errs.ErrNotFound
	}
	parent := parents[0]
	if !actor.IsModerator && parent.CreatedBy != actor.UserID {
		return Entry{}, errs.ErrForbidden
	}
	// 用父内容所在类型校验文档（保证必填/格式规则一致）
	ct, err := s.GetTypeByID(ctx, parent.ContentTypeID)
	if err != nil {
		return Entry{}, err
	}
	if err := s.reg.ValidateDocument(&ct, data); err != nil {
		return Entry{}, fmt.Errorf("%w: %v", errs.ErrValidation, err)
	}
	// 语言重复检查
	for _, p := range parents {
		if p.Lang == lang {
			return Entry{}, fmt.Errorf("%w: 该语言翻译已存在", errs.ErrValidation)
		}
	}
	slug, _ := data["slug"].(string)
	if slug == "" {
		slug = schema.Slugify(fmt.Sprint(data["title"]))
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return Entry{}, err
	}
	row := &store.Content{
		ContentTypeID: parent.ContentTypeID,
		ContentID:     contentID,
		Lang:          lang,
		Slug:          slug,
		Status:        "draft",
		CreatedBy:     actor.UserID,
		Payload:       string(payload),
	}
	if err := s.store.ContentRepo().Create(ctx, row); err != nil {
		return Entry{}, err
	}
	return s.entryFromStore(ctx, ct, *row)
}

// DeleteType 删除内容类型（不检查内容引用，Phase 4 增强）。
func (s *Service) DeleteType(ctx context.Context, name string) error {
	ct, err := s.GetType(ctx, name)
	if err != nil {
		return err
	}
	return s.store.ContentTypeRepo().Delete(ctx, ct.ID)
}
```
> 注意：`CreateTranslation` 必须用父内容所在类型（`GetTypeByID`）校验文档，而非空 `ContentType`——上述代码已内联处理。

追加测试（`service_test.go`）：
```go
func TestTranslationOwnership(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	moderator := Actor{UserID: 99, IsModerator: true}

	e, err := svc.Create(ctx, "article", "zh", map[string]any{"title": "原", "slug": "orig"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	// 作者给自己翻译 OK
	tr, err := svc.CreateTranslation(ctx, "article", "en", e.Content.ContentID, map[string]any{"title": "En"}, Actor{UserID: 1})
	if err != nil {
		t.Fatalf("作者自译应成功: %v", err)
	}
	if tr.Content.Lang != "en" {
		t.Errorf("lang = %q", tr.Content.Lang)
	}
	// 作者翻译他人内容 → Forbidden
	if _, err := svc.CreateTranslation(ctx, "article", "fr", e.Content.ContentID, map[string]any{"title": "Fr"}, Actor{UserID: 2}); err != errs.ErrForbidden {
		t.Errorf("作者翻译他人 = %v, want ErrForbidden", err)
	}
	// moderator 翻译他人 OK
	if _, err := svc.CreateTranslation(ctx, "article", "fr", e.Content.ContentID, map[string]any{"title": "Fr"}, moderator); err != nil {
		t.Errorf("moderator 翻译他人 = %v", err)
	}
	// 翻译不存在的组 → NotFound
	if _, err := svc.CreateTranslation(ctx, "article", "de", "nope", map[string]any{"title": "De"}, moderator); err != errs.ErrNotFound {
		t.Errorf("不存在组 = %v, want ErrNotFound", err)
	}
}
```
测试中 `CreateTranslation` 的类型校验用父内容类型（`article`，`title` required）——`data` 需含 title；若缺必填会返回 ErrValidation，上述用例均带 title，OK。

- [ ] **步骤 1：编写失败测试** `internal/adminapi/adminapi_test.go`

```go
package adminapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
	"dulizhan/internal/content"
	"dulizhan/internal/i18n"
	"dulizhan/internal/media"
	"dulizhan/internal/schema"
	"dulizhan/internal/seed"
	"dulizhan/internal/store"
	"dulizhan/internal/store/sqlite"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type env struct {
	g   *gin.Engine
	st  store.Store
	svc *content.Service
	auth *auth.Service
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	reg, _ := i18n.New([]string{"zh", "en"}, "zh", false)
	authSvc := auth.New(st, time.Hour)
	svc := content.New(st, schema.NewRegistry(), []string{"zh", "en"})
	if err := seed.EnsureAuth(context.Background(), st, authSvc); err != nil {
		t.Fatal(err)
	}
	if err := seed.Run(context.Background(), svc); err != nil {
		t.Fatal(err)
	}
	g := gin.New()
	d := Deps{Store: st, Auth: authSvc, Content: svc, Media: media.NewLocalStore(t.TempDir(), "/media")}
	Register(g.Group("/api"), d)
	return &env{g: g, st: st, svc: svc, auth: authSvc}
}

func (e *env) do(t *testing.T, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.g.ServeHTTP(w, req)
	return w
}

func (e *env) login(t *testing.T) string {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"admin123"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Data.Token
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/adminapi/ -run TestNone -v`
预期：编译失败（`Register`/`Deps` 未定义）

- [ ] **步骤 3：实现 Deps 与响应助手** `internal/adminapi/deps.go` + `internal/adminapi/respond.go`

```go
// internal/adminapi/deps.go
package adminapi

import (
	"dulizhan/internal/auth"
	"dulizhan/internal/content"
	"dulizhan/internal/media"
	"dulizhan/internal/store"
)

type Deps struct {
	Store   store.Store
	Auth    *auth.Service
	Content *content.Service
	Media   media.MediaStore
}
```

```go
// internal/adminapi/respond.go
package adminapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/errs"
)

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": data})
}

// fail 把领域错误映射为 HTTP 状态：ErrNotFound→404 / ErrForbidden→403 / ErrValidation→422 / 其他→500。
func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "资源不存在"})
	case errors.Is(err, errs.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "message": "无权限执行此操作"})
	case errors.Is(err, errs.ErrValidation):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": http.StatusUnprocessableEntity, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "服务器内部错误"})
	}
}

// unauthorized 统一 401（final-fix Finding 1）。
func unauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "未登录或会话已过期"})
}

// mustID 解析路径 :id，非法则写 422 并返回 false（final-fix Finding 5）。
func mustID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": http.StatusUnprocessableEntity, "message": "ID 不合法"})
		return 0, false
	}
	return id, true
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": msg})
}
```

- [ ] **步骤 4：实现认证 handler** `internal/adminapi/auth.go`

```go
// internal/adminapi/auth.go
package adminapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (d *Deps) HandleLogin(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	token, err := d.Auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil || token == "" { // final-fix：Authenticate 失败 nil 防护
		unauthorized(c)
		return
	}
	c.SetCookie(auth.SessionCookieName, token, 7*24*3600, "/", "", false, true)
	ok(c, gin.H{"token": token})
}

func (d *Deps) HandleLogout(c *gin.Context) {
	token := auth.TokenFromRequest(c)
	if err := d.Auth.Logout(c.Request.Context(), token); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"logged_out": true})
}

func (d *Deps) HandleMe(c *gin.Context) {
	us, ok := auth.UserFromContext(c)
	if !ok || us == nil { // final-fix：nil 防护
		unauthorized(c)
		return
	}
	ok(c, gin.H{"user": gin.H{"id": us.User.ID, "username": us.User.Username, "role": us.Role.Name}})
}
```

- [ ] **步骤 5：实现内容类型 handler** `internal/adminapi/content_types.go`

```go
// internal/adminapi/content_types.go
package adminapi

import (
	"github.com/gin-gonic/gin"

	"dulizhan/internal/schema"
)

type contentTypeReq struct {
	Name   string          `json:"name"`
	Label  string          `json:"label"`
	Fields []schema.Field  `json:"fields"`
}

func (d *Deps) HandleContentTypes(c *gin.Context) {
	types, err := d.Content.AllTypes(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": types})
}

func (d *Deps) HandleContentTypeCreate(c *gin.Context) {
	var req contentTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	ct := &schema.ContentType{Name: req.Name, Label: req.Label, Fields: req.Fields}
	if err := d.Content.CreateType(c.Request.Context(), ct); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"content_type": ct})
}

func (d *Deps) HandleContentTypeUpdate(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	var req contentTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	ct := &schema.ContentType{ID: id, Name: req.Name, Label: req.Label, Fields: req.Fields}
	if err := d.Content.UpdateType(c.Request.Context(), ct); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"content_type": ct})
}

func (d *Deps) HandleContentTypeDelete(c *gin.Context) {
	name := c.Param("name")
	if err := d.Content.DeleteType(c.Request.Context(), name); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": name})
}
```

- [ ] **步骤 6：实现内容 handler** `internal/adminapi/content.go`

```go
// internal/adminapi/content.go
package adminapi

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
	"dulizhan/internal/content"
	"dulizhan/internal/errs"
)

type contentReq struct {
	Type string         `json:"type"`
	Lang string         `json:"lang"`
	Data map[string]any `json:"data"`
}

// actor 从上下文取当前用户，缺会话返回零值（防御性，final-fix Finding 1）。
func (d *Deps) actor(c *gin.Context) content.Actor {
	us, ok := auth.UserFromContext(c)
	if !ok || us == nil {
		return content.Actor{}
	}
	return content.Actor{UserID: us.User.ID, IsModerator: us.HasPerm("content.publish") || us.Perms["*"]}
}

// canManage 校验操作权：moderator 可操作他人，作者只能操作自己创建的内容。
func (d *Deps) canManage(c *gin.Context, id int64) bool {
	a := d.actor(c)
	if a.IsModerator {
		return true
	}
	e, err := d.Content.GetByID(c.Request.Context(), id)
	if err != nil {
		return false
	}
	return e.Content.CreatedBy == a.UserID
}

func (d *Deps) HandleContentList(c *gin.Context) {
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
	status := c.Query("status")
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.Query("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	items, total, err := d.Content.ListAdmin(c.Request.Context(), typeName, lang, status, page, perPage)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": items, "total": total})
}

func (d *Deps) HandleContentCreate(c *gin.Context) {
	var req contentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	a := d.actor(c)
	if a.UserID == 0 {
		unauthorized(c)
		return
	}
	e, err := d.Content.Create(c.Request.Context(), req.Type, req.Lang, req.Data, a.UserID)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"content": e})
}

func (d *Deps) HandleContentGet(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	e, err := d.Content.GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"content": e})
}

func (d *Deps) HandleContentUpdate(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if !d.canManage(c, id) {
		fail(c, errs.ErrForbidden)
		return
	}
	var req contentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	e, err := d.Content.Update(c.Request.Context(), id, req.Data)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"content": e})
}

func (d *Deps) HandleContentDelete(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if !d.canManage(c, id) {
		fail(c, errs.ErrForbidden)
		return
	}
	if err := d.Content.Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": id})
}

func (d *Deps) HandleContentPublish(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if !d.canManage(c, id) {
		fail(c, errs.ErrForbidden)
		return
	}
	if err := d.Content.SetStatus(c.Request.Context(), id, "published"); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"id": id, "status": "published"})
}

func (d *Deps) HandleContentUnpublish(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if !d.canManage(c, id) {
		fail(c, errs.ErrForbidden)
		return
	}
	if err := d.Content.SetStatus(c.Request.Context(), id, "draft"); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"id": id, "status": "draft"})
}

func (d *Deps) HandleTranslations(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	e, err := d.Content.GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	entries, err := d.Content.ListByContentID(c.Request.Context(), e.Content.ContentID)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": entries})
}

func (d *Deps) HandleCreateTranslation(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	parent, err := d.Content.GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	var req contentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	e, err := d.Content.CreateTranslation(c.Request.Context(), req.Type, req.Lang, parent.Content.ContentID, req.Data, d.actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"content": e})
}
```

- [ ] **步骤 7：实现媒体/菜单/设置/用户/角色 handler**（5 个文件）

`internal/adminapi/media.go`：
```go
package adminapi

import (
	"github.com/gin-gonic/gin"

	"dulizhan/internal/media"
)

func (d *Deps) HandleMediaUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "缺少 file 字段")
		return
	}
	ext := ""
	if i := lastIndexByte(file.Filename, '.'); i >= 0 {
		ext = file.Filename[i:]
	}
	key, err := media.RandomKey(ext)
	if err != nil {
		fail(c, err)
		return
	}
	src, err := file.Open()
	if err != nil {
		fail(c, err)
		return
	}
	defer src.Close()
	url, err := d.Media.Save(c.Request.Context(), key, src, file.Header.Get("Content-Type"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"key": key, "url": url})
}

func (d *Deps) HandleMediaList(c *gin.Context) {
	items, err := d.Store.MediaRepo().List(c.Request.Context(), 0, 100)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": items})
}

func (d *Deps) HandleMediaDelete(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	m, err := d.Store.MediaRepo().GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	if err := d.Media.Delete(c.Request.Context(), trimPrefix(m.URL, "/media/")); err != nil {
		fail(c, err)
		return
	}
	if err := d.Store.MediaRepo().Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": id})
}

func lastIndexByte(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
```

`internal/adminapi/menus.go`：
```go
package adminapi

import (
	"github.com/gin-gonic/gin"

	"dulizhan/internal/store"
)

type menuReq struct {
	Name  string `json:"name"`
	Lang  string `json:"lang"`
	Items any    `json:"items"` // 数组或字符串
}

func (d *Deps) HandleMenus(c *gin.Context) {
	lang := c.Query("lang")
	if lang == "" {
		lang = "zh"
	}
	items, err := d.Store.MenuRepo().ListByLang(c.Request.Context(), lang)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": items})
}

func (d *Deps) HandleMenuCreate(c *gin.Context) {
	var req menuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	items, err := menuItems(req.Items)
	if err != nil {
		fail(c, err)
		return
	}
	m := &store.Menu{Name: req.Name, Lang: req.Lang, Items: items}
	if err := d.Store.MenuRepo().Create(c.Request.Context(), m); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"menu": m})
}

func (d *Deps) HandleMenuUpdate(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	var req menuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	cur, err := d.Store.MenuRepo().GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	// 部分 PUT：缺失字段保留原值（A 类修复）
	if req.Name != "" {
		cur.Name = req.Name
	}
	if req.Lang != "" {
		cur.Lang = req.Lang
	}
	if req.Items != nil {
		items, err := menuItems(req.Items)
		if err != nil {
			fail(c, err)
			return
		}
		cur.Items = items
	}
	if err := d.Store.MenuRepo().Update(c.Request.Context(), &cur); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"menu": cur})
}

func (d *Deps) HandleMenuDelete(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if err := d.Store.MenuRepo().Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": id})
}

// menuItems 把任意 items 统一为 JSON 字符串（避免双重编码，A 类修复）。
func menuItems(v any) (string, error) {
	if s, ok := v.(string); ok {
		return s, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
```
（需 import `encoding/json`。）

`internal/adminapi/settings.go`：
```go
package adminapi

import (
	"github.com/gin-gonic/gin"
)

func (d *Deps) HandleSettings(c *gin.Context) {
	all, err := d.Store.SettingRepo().All(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"settings": all})
}

func (d *Deps) HandleSettingsUpdate(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	for k, v := range req {
		if err := d.Store.SettingRepo().Set(c.Request.Context(), k, v); err != nil {
			fail(c, err)
			return
		}
	}
	ok(c, gin.H{"settings": req})
}
```

`internal/adminapi/users.go`：
```go
package adminapi

import (
	"github.com/gin-gonic/gin"

	"dulizhan/internal/errs"
)

type userReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	RoleID   int64  `json:"role_id"`
}

func (d *Deps) HandleUsers(c *gin.Context) {
	users, err := d.Auth.ListUsers(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": users})
}

func (d *Deps) HandleUserCreate(c *gin.Context) {
	var req userReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	u, err := d.Auth.CreateUser(c.Request.Context(), req.Username, req.Password, req.RoleID)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"user": u})
}

func (d *Deps) HandleUserDelete(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if err := d.Auth.DeleteUser(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": id})
}

func (d *Deps) HandleUserRole(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	var req struct {
		RoleID int64 `json:"role_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	if err := d.Auth.SetUserRole(c.Request.Context(), id, req.RoleID); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"id": id})
}

func (d *Deps) HandleUserPassword(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	if err := d.Auth.ChangePassword(c.Request.Context(), id, req.Password); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"id": id})
}
```

`internal/adminapi/roles.go`：
```go
package adminapi

import (
	"github.com/gin-gonic/gin"
)

func (d *Deps) HandleRoles(c *gin.Context) {
	roles, err := d.Store.RoleRepo().List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": roles})
}
```

- [ ] **步骤 8：实现 Register** `internal/adminapi/register.go`

抄录 `phase2/task-10-brief.md` Step 3 的 `register.go`，**删除 vestigial 桩**（`authG`/`hasPerm`/`_ = auth.RequireAuth` 不保留），保留完整 31 条路由与 `auth.RequireAuth(d.Auth)` + `auth.RequirePerm(...)` 包装。

- [ ] **步骤 9：追加 adminapi 行为测试**（追加到 `adminapi_test.go`）

```go
func TestLoginMe(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	w := e.do(t, http.MethodGet, "/api/auth/me", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("me = %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "admin") {
		t.Errorf("me 缺用户名: %s", w.Body.String())
	}
	// 未登录 → 401
	w = e.do(t, http.MethodGet, "/api/auth/me", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("未登录 me = %d, want 401", w.Code)
	}
}

func TestContentCRUDAndDuplicateSlug422(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 建类型
	w := e.do(t, http.MethodPost, "/api/content-types", `{"name":"note","label":"笔记","fields":[{"name":"title","label":"标题","type":"text","required":true}]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create type = %d %s", w.Code, w.Body.String())
	}
	// 建内容
	body := `{"type":"note","lang":"zh","data":{"title":"接口内容"}}`
	w = e.do(t, http.MethodPost, "/api/content", body, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create content = %d %s", w.Code, w.Body.String())
	}
	// 重复 slug → 422（seed 已有 article/hello-zh；这里建 note，无冲突——用 article 验证）
	w = e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"重复","slug":"hello-zh"}}`, tok)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("重复 slug = %d, want 422", w.Code)
	}
	// 缺必填 → 422
	w = e.do(t, http.MethodPost, "/api/content", `{"type":"note","lang":"zh","data":{}}`, tok)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("缺必填 = %d, want 422", w.Code)
	}
	// 未登录 → 401
	w = e.do(t, http.MethodPost, "/api/content", body, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("未登录 = %d, want 401", w.Code)
	}
}

func TestContentOwnershipForbidden(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	// 建 author 角色与用户
	authorRole, err := e.st.RoleRepo().Create(ctx, "author")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.RoleRepo().Update(ctx, authorRole.ID, "author", `["content.read","content.write"]`); err != nil {
		t.Fatal(err)
	}
	author, err := e.auth.CreateUser(ctx, "alice", "secret123", authorRole.ID)
	if err != nil {
		t.Fatal(err)
	}
	authorTok, _ := e.auth.Login(ctx, "alice", "secret123")
	// author 建内容
	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"作者文章","slug":"alice-post"}}`, authorTok)
	if w.Code != http.StatusOK {
		t.Fatalf("author create = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Content struct {
				Content struct {
					ID int64 `json:"id"`
				} `json:"content"`
			} `json:"content"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	id := resp.Data.Content.Content.ID
	_ = author
	_ = authorTok
	// author 无法发布自己内容（无 content.publish 权限 → 403）
	w = e.do(t, http.MethodPost, fmt.Sprintf("/api/content/%d/publish", id), "", authorTok)
	if w.Code != http.StatusForbidden {
		t.Errorf("author publish = %d, want 403", w.Code)
	}
}

func TestMediaUploadDelete(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 构造 multipart 上传
	var buf bytes.Buffer
	bw := multipart.NewWriter(&buf)
	fw, _ := bw.CreateFormFile("file", "a.png")
	fw.Write([]byte("hello"))
	bw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/media/upload", &buf)
	req.Header.Set("Content-Type", bw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.g.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("upload = %d %s", w.Code, w.Body.String())
	}
	// 列表
	w = e.do(t, http.MethodGet, "/api/media", "", tok)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "a.png") {
		t.Errorf("media list = %d %s", w.Code, w.Body.String())
	}
}

func TestMenuPartialUpdate(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	w := e.do(t, http.MethodPost, "/api/menus", `{"name":"主导航","lang":"zh","items":[{"label":"首页"}]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("menu create = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Menu struct {
				ID int64 `json:"id"`
			} `json:"menu"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	id := resp.Data.Menu.ID
	// 部分更新只改 items，Name 保留（A 类修复）
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/menus/%d", id), `{"items":[{"label":"关于"}]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("menu update = %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "主导航") || !strings.Contains(w.Body.String(), "关于") {
		t.Errorf("部分更新丢字段: %s", w.Body.String())
	}
}

func TestUserRoleAPIs(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 角色列表
	w := e.do(t, http.MethodGet, "/api/roles", "", tok)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "admin") {
		t.Errorf("roles = %d %s", w.Code, w.Body.String())
	}
	// 建用户
	w = e.do(t, http.MethodPost, "/api/users", `{"username":"u2","password":"secret123","role_id":1}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("user create = %d %s", w.Code, w.Body.String())
	}
	// 用户列表
	w = e.do(t, http.MethodGet, "/api/users", "", tok)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "u2") {
		t.Errorf("users = %d %s", w.Code, w.Body.String())
	}
}
```
需新增 import `mime/multipart`。`TestContentOwnershipForbidden` 中 `author` 变量未用，删除 `_ = author` 两行或去掉该变量赋值。

- [ ] **步骤 10：运行测试验证通过**

运行：`go test -count=1 ./internal/adminapi/ -v`
预期：全部 PASS

---

### 任务 16：server /api 装配 + 端到端测试 + main 更新

**文件：**
- 修改：`internal/server/server.go`、`internal/server/frontend.go`（媒体静态路由）
- 修改：`internal/server/frontend_test.go`、`cmd/dulizhan/main.go`
- 测试：`internal/server/frontend_test.go`

**代码来源：** 逐字抄录 `phase2/task-10-brief.md` 与 `phase2/task-10-report.md` 的 Fix Report（`TestAdminAPIE2E` 适配 indexed 字段 + 前台断言 + 删死代码）。

- [ ] **步骤 1：编写失败测试**

抄录 `phase2/task-10-brief.md` Step 1 的 `TestAdminAPIE2E` 与 `rebuildServer` 追加到 `frontend_test.go`，**应用 task-10-report Fix Report**：
- 建类型 payload 的 title 字段加 `"indexed":true`：`{"name":"note","label":"笔记","fields":[{"name":"title","label":"标题","type":"text","required":true,"indexed":true}]}`
- `/note/` 断言改为同时校验 200 且响应含 `"接口发布的内容"`
- 删除第一个立即被 `rebuildServer` 覆盖的 `New(...)` 调用（只构建一次 server）

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/server/ -run TestAdminAPIE2E -v`
预期：FAIL（`New` 签名不符，编译失败）

- [ ] **步骤 3：改造 server.New 并装配 /api**

抄录 `phase2/task-10-brief.md` Step 3 的 `server.go` 改造与 `register.go`（已建），`server.New` 签名改为 7 参，新增 `/media/*key` 静态服务（含 `..` 防护 + `Cache-Control: public, max-age=3600`），`handleThemeStatic` 从内联闭包抽取为方法（task-10-report）。

- [ ] **步骤 4：更新 main.go**

抄录 `phase2/task-10-brief.md` Step 4 的 `main.go`（新增 import：`path/filepath`、`time`、`auth`、`media`；构造 `authSvc`/`med`；seed 分支先 `EnsureAuth` 再 `Run`；`server.New` 传新参数）。同时 `frontend_test.go` 的 `buildTestServer` 适配新签名（构造 authSvc + LocalStore，先 EnsureAuth 再 seed.Run）。

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/server/ -v`
预期：全部 PASS（TestFrontendRoutes / TestSEOEndpoints / TestThemeStatic / TestListPagination / TestAdminAPIE2E）

- [ ] **步骤 6：全量测试 + 手动冒烟**

运行：`go test -count=1 ./...`、`go build ./...`、`go vet ./...`
预期：全部干净

手动冒烟（phase2 final-fix 验收）：
```bash
go run . seed
go run . --config config.yaml
# 登录拿 token
$token = (Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/auth/login -ContentType 'application/json' -Body '{"username":"admin","password":"admin123"}').data.token
# 建类型/建内容/发布
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/content-types -ContentType 'application/json' -Headers @{Authorization="Bearer $token"} -Body '{"name":"note","label":"笔记","fields":[{"name":"title","label":"标题","type":"text","required":true,"indexed":true}]}'
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/content -ContentType 'application/json' -Headers @{Authorization="Bearer $token"} -Body '{"type":"note","lang":"zh","data":{"title":"API 冒烟内容"}}'
# 取内容 id 后发布（或直接发布 id=…）
# 重复 slug → 422
# /note/ 前台渲染 "API 冒烟内容"
```
预期：前台 `/note/` 渲染出 API 发布的内容；重复 slug 返回 422；`/api/auth/me` 返回 admin 与权限。冒烟前清理 8080 残留进程。

---

### 任务 17：全量验收（spec 第 7 节）

**文件：** 无新增。

- [ ] **步骤 1：全量测试（uncached）**

运行：`go test -count=1 ./...`
预期：全部 ok（config/errs/store/sqlite/schema/content/i18n/seo/theme/themes/default/server/seed/auth/media/adminapi）

- [ ] **步骤 2：build 与 vet**

运行：`go build ./...`、`go vet ./...`、`gofmt -l .`
预期：build/vet 无输出；gofmt 空

- [ ] **步骤 3：复现 final-fix-reports 冒烟清单**

1. 前台：`/article/hello-zh` 返回有效 JSON-LD（未转义 `{"@context"`）+ x-default/zh/en hreflang
2. 管理端：login→me→建类型→建内容→发布→前台渲染；重复 slug 422；作者建内容 200、翻译他人 403、发布他人 403
3. 冒烟前确认 8080 无残留进程

- [ ] **步骤 4：登记验收结果**

把各任务完成状态与最终验收结果记入 `.superpowers/sdd/2026-08-06-phase2-admin-api/progress.md`（或新建恢复专用 ledger），注明：不 git 提交、依赖锁定版本、adminapi 为缺口重建。

---

## 自检记录

**1. 规格覆盖度：**
- 规格 §2.1 包清单 → 任务 1-16 全覆盖（config/errs=1、store+sqlite=2/10、schema=3、content=4/15、i18n=5、seo=6、theme=7、默认主题=8、server/seed/main=9/16、auth=11/12、media=13、adminapi=15、seed auth=14）
- 规格 §2.2 契约项 → JSON-LD=6/7、x-default=6、reservedNames=4、renderList 404=9、nil 防护=15、CreateTranslation 归属=15、重复 slug=2/15、JSON tag=2/10/4/15、PUT content-types=15
- 规格 §5 minor → A 类：sqlite time.Parse/SetMaxOpenConns/Update=2、multiselect/toFloat/pattern=3、URLPath/All/New=5、RobotsTXT/SitemapXML/json.Marshal=6、adminapi menu/actor/SetStatus=15、LocalStore=13、handleSitemap/main seed=9；B 类补测=各任务步骤 4/5；C 类=任务 8 注明
- 规格 §6 架构约定 → 贯穿全部任务
- 规格 §7 验收 → 任务 17

**2. 占位符扫描：** 无 TBD/TODO。每步含代码或明确指向 on-disk brief 文件。adminapi（缺口）代码全部内联。无"适当处理错误"式占位。

**3. 类型一致性：**
- `content.New(st, reg, reservedNames)` 3 参在任务 4 定义，任务 9/15/16 一致使用
- `server.New` 5 参（任务 9）→ 7 参（任务 16），任务 16 同步全部调用点
- `CreateTranslation(ctx, typeName, lang, contentID, data, actor)` 在任务 15 定义，adminapi 与测试一致
- `ContentRepo.ListByContentID` 任务 2 定义，任务 15 使用
- `store` 模型 JSON tag：Content/ContentType（任务 2）、User/Role/Session/Menu/Media（任务 10）
- `Entry` JSON tag（content/type_name/fields）：任务 4 定义，任务 15 序列化契约一致
- `seed.EnsureAuth(ctx, st, authSvc)` 任务 14 定义，任务 15/16 使用
