# zh 保存自动生成 en 翻译 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 后台保存 zh 内容时，自动调用 OpenAI 兼容接口翻译生成 en 翻译（图片/分类等非可翻译字段保留、en 状态跟随 zh）；翻译失败降级为复制草稿，不阻断 zh 保存。

**架构：** 新增 `internal/translate` 包（可插拔 `Translator` 接口 + OpenAI 实现 + 富文本按块翻译）；`content.Service` 增加可选翻译器注入（`SetTranslator`，不破坏 `New`）与公开方法 `EnsureTranslation`；`adminapi.HandleContentCreate/Update` 在保存 zh 后调用 `EnsureTranslation` 并把 `TranslateStatus` 拼进响应；前端按返回状态提示。`translate.enabled` 默认 false，关闭时行为与现状完全一致。

**技术栈：** Go（gin、net/http、golang.org/x/net/html）、Vue3+TS（admin SPA）。

**前置基线：** 规格 `docs/superpowers/specs/2026-08-13-auto-translate-en-design.md` 已确认。现有 `content.Service.Create/Update/CreateTranslation`（service.go）、`schema.Field.Translatable`（schema.go:37）、`golang.org/x/net v0.25.0`（go.mod:51，indirect）、`config.Config` 结构。

**环境约束（所有任务遵守）：**
- **不 git 提交**（仓库约定，直接 master 文件级审查）；勿运行 `go mod tidy`
- Go 1.26.5（`C:\Program Files\Go\bin`，跑 go 前 `$env:Path = "C:\Program Files\Go\bin;" + $env:Path`）
- Go 验证：`go test -count=1 ./...`、`go vet ./...`、`gofmt -l .`
- 前端：`cd admin && npx vue-tsc --noEmit && npx vitest run`；改 SPA 后 `npx vite build` + 同步 `internal/server/dist/`
- 错误消息用中文
- `golang.org/x/net` 已是 indirect 依赖，新增直接 import 时不改 go.mod（勿 tidy）；若需要显式化用 `go get golang.org/x/net@v0.25.0`（锁定版本）

**验证基线（写任何代码前先跑一次）：**
```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
go test -count=1 ./...
```
预期：全绿。

---

### 任务 1：translate 配置（config）

**文件：**
- 修改：`internal/config/config.go`
- 测试：`internal/config/config_test.go`
- 修改：`config.example.yaml`

新增 `translate` 配置节（默认关闭）。

- [ ] **步骤 1：编写失败测试**（`internal/config/config_test.go` 追加）

```go
func TestTranslateConfigLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `
site:
  url: "https://example.com"
  default_lang: "zh"
  languages: ["zh", "en"]
translate:
  enabled: true
  api_key: "sk-test"
  base_url: "https://api.openai.com/v1"
  model: "gpt-4o-mini"
  source_lang: "zh"
  target_lang: "en"
`
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Translate.Enabled {
		t.Error("Translate.Enabled = false, want true")
	}
	if cfg.Translate.APIKey != "sk-test" || cfg.Translate.BaseURL != "https://api.openai.com/v1" || cfg.Translate.Model != "gpt-4o-mini" {
		t.Errorf("translate 配置错误: %+v", cfg.Translate)
	}
	if cfg.Translate.SourceLang != "zh" || cfg.Translate.TargetLang != "en" {
		t.Errorf("源/目标语言错误: %+v", cfg.Translate)
	}
}

func TestTranslateValidate(t *testing.T) {
	// enabled 但缺 api_key → 报错
	cfg := &Config{}
	cfg.Site.URL = "https://example.com"
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Languages = []string{"zh", "en"}
	cfg.Site.Theme = "default"
	cfg.Translate.Enabled = true
	if err := cfg.Validate(); err == nil {
		t.Error("enabled 缺 api_key 应报错")
	}
	cfg.Translate.APIKey = "sk"
	cfg.Translate.BaseURL = "https://api.openai.com/v1"
	cfg.Translate.Model = "gpt-4o-mini"
	if err := cfg.Validate(); err != nil {
		t.Errorf("补齐后应通过: %v", err)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/config/ -run TestTranslate -v`
预期：FAIL（编译错误：`cfg.Translate` 未定义）。

- [ ] **步骤 3：实现配置**

`internal/config/config.go` 加：
```go
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Site     SiteConfig     `yaml:"site"`
	Media    MediaConfig    `yaml:"media"`
	Translate TranslateConfig `yaml:"translate"`
}

type TranslateConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Provider   string `yaml:"provider"`
	APIKey     string `yaml:"api_key"`
	BaseURL    string `yaml:"base_url"`
	Model      string `yaml:"model"`
	SourceLang string `yaml:"source_lang"`
	TargetLang string `yaml:"target_lang"`
}
```

`applyEnv` 增加：
```go
if v, ok := os.LookupEnv("DULIZHAN_TRANSLATE_ENABLED"); ok && v == "true" {
	cfg.Translate.Enabled = true
}
set("TRANSLATE_API_KEY", &cfg.Translate.APIKey)
set("TRANSLATE_BASE_URL", &cfg.Translate.BaseURL)
set("TRANSLATE_MODEL", &cfg.Translate.Model)
set("TRANSLATE_SOURCE_LANG", &cfg.Translate.SourceLang)
set("TRANSLATE_TARGET_LANG", &cfg.Translate.TargetLang)
```

`Validate` 增加（在末尾）：
```go
if c.Translate.Enabled {
	if c.Translate.APIKey == "" {
		return fmt.Errorf("translate.api_key 必填（enabled=true 时）")
	}
	if c.Translate.BaseURL == "" {
		c.Translate.BaseURL = "https://api.openai.com/v1"
	}
	if c.Translate.Model == "" {
		return fmt.Errorf("translate.model 必填（enabled=true 时）")
	}
	if c.Translate.SourceLang == "" {
		c.Translate.SourceLang = c.Site.DefaultLang
	}
	if c.Translate.TargetLang == "" {
		return fmt.Errorf("translate.target_lang 必填（enabled=true 时）")
	}
	if c.Translate.SourceLang == c.Translate.TargetLang {
		return fmt.Errorf("translate.source_lang 与 target_lang 不能相同")
	}
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/config/ -v`
预期：PASS（含新用例）。

- [ ] **步骤 5：更新 config.example.yaml**

`config.example.yaml` 末尾加注释示例：
```yaml
# translate:
#   enabled: false                 # true 时保存源语言内容自动翻译生成目标语言
#   provider: "openai"             # openai 兼容接口
#   api_key: ""                    # env: DULIZHAN_TRANSLATE_API_KEY
#   base_url: "https://api.openai.com/v1"
#   model: "gpt-4o-mini"
#   source_lang: "zh"              # 触发翻译的源语言（默认站点默认语言）
#   target_lang: "en"              # 自动生成的目标语言
```

---

### 任务 2：translate 包（Translator + OpenAI + 富文本）

**文件：**
- 创建：`internal/translate/translate.go`
- 创建：`internal/translate/richtext.go`
- 测试：`internal/translate/translate_test.go`

定义 `Translator` 接口 + OpenAI 实现 + 富文本按块翻译。

- [ ] **步骤 1：创建 translate.go**（先写接口，测试随后）

`internal/translate/translate.go`：
```go
package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Translator 可插拔翻译器（便于测试 mock）。
type Translator interface {
	TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error)
	// TranslateRichText 按块翻译 HTML，非文本标签（img/a/br）保留。
	TranslateRichText(ctx context.Context, htmlStr, sourceLang, targetLang string) (string, error)
}

// Service OpenAI 兼容接口翻译实现。
type Service struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

func New(apiKey, baseURL, model string) *Service {
	return &Service{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// TranslateText 调用 chat/completions 翻译单段文本。
func (s *Service) TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}
	reqBody := map[string]any{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a professional translator. Translate the user's text from " + sourceLang + " to " + targetLang + ". Output only the translation, no explanations, no quotes."},
			{"role": "user", "content": text},
		},
		"temperature": 0.3,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("翻译接口返回 %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("翻译接口无结果")
	}
	return parsed.Choices[0].Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
```

- [ ] **步骤 2：创建 richtext.go（富文本按块翻译）**

`internal/translate/richtext.go`：
```go
package translate

import (
	"context"
	"strings"

	"golang.org/x/net/html"
)

// TranslateRichText 按块翻译 HTML：提取 <p>/<li>/<h1-h6> 文本块逐个翻译，非文本标签（img/a/br）保留。
// 无法解析时退化为整体当纯文本翻译并包回 <p>。
func (s *Service) TranslateRichText(ctx context.Context, htmlStr, sourceLang, targetLang string) (string, error) {
	if strings.TrimSpace(htmlStr) == "" {
		return htmlStr, nil
	}
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return s.wrapParagraphFallback(ctx, htmlStr, sourceLang, targetLang)
	}
	var translateErr error
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode && n.Parent != nil {
			parent := n.Parent
			if parent.Data == "p" || parent.Data == "li" || parent.Data == "h1" || parent.Data == "h2" || parent.Data == "h3" ||
				parent.Data == "h4" || parent.Data == "h5" || parent.Data == "h6" || parent.Data == "span" {
				txt := strings.TrimSpace(n.Data)
				if txt != "" {
					tr, err := s.TranslateText(ctx, txt, sourceLang, targetLang)
					if err != nil {
						translateErr = err
						return
					}
					n.Data = tr
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if translateErr != nil {
		return s.wrapParagraphFallback(ctx, htmlStr, sourceLang, targetLang)
	}
	var b strings.Builder
	html.Render(&b, doc)
	return b.String(), nil
}

// wrapParagraphFallback 降级：整体当纯文本翻译，包回 <p>。
func (s *Service) wrapParagraphFallback(ctx context.Context, htmlStr, sourceLang, targetLang string) (string, error) {
	text := stripTags(htmlStr)
	tr, err := s.TranslateText(ctx, text, sourceLang, targetLang)
	if err != nil {
		return htmlStr, err
	}
	return "<p>" + tr + "</p>", nil
}

func stripTags(s string) string {
	var b strings.Builder
	r := strings.NewReader(s)
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt == html.TextToken {
			b.WriteString(z.Token().Data)
		}
	}
	return b.String()
}
```

- [ ] **步骤 3：编写测试**（`internal/translate/translate_test.go`）

```go
package translate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranslateText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"Hello world"}}]}`))
	}))
	defer srv.Close()

	s := New("sk-test", srv.URL, "gpt-4o-mini")
	got, err := s.TranslateText(context.Background(), "你好世界", "zh", "en")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hello world" {
		t.Errorf("got %q", got)
	}
}

func TestTranslateTextError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()

	s := New("sk", srv.URL, "m")
	if _, err := s.TranslateText(context.Background(), "x", "zh", "en"); err == nil {
		t.Error("期望错误")
	}
}

func TestTranslateRichText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"translated"}}]}`))
	}))
	defer srv.Close()

	s := New("sk", srv.URL, "m")
	html := `<p>中文段落一</p><p><img src="/a.png">中文段落二</p>`
	got, err := s.TranslateRichText(context.Background(), html, "zh", "en")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<img src="/a.png">`) {
		t.Errorf("图片标签丢失: %s", got)
	}
	if !strings.Contains(got, "translated") {
		t.Errorf("翻译文本缺失: %s", got)
	}
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：
```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
go test -count=1 ./internal/translate/ -v
```
预期：PASS。若 go.mod 报 x/net 需显式 require，运行 `go get golang.org/x/net@v0.25.0`（勿 tidy）。

---

### 任务 3：content.Service 注入 + EnsureTranslation

**文件：**
- 修改：`internal/content/service.go`
- 修改：`internal/content/util.go`
- 测试：`internal/content/service_test.go`

给 `Service` 加可选翻译器 + 公开方法 `EnsureTranslation`。

- [ ] **步骤 1：编写失败测试**（`internal/content/service_test.go` 追加）

```go
// mockTranslator 固定返回翻译文本。
type mockTranslator struct{ translated string }

func (m *mockTranslator) TranslateText(ctx context.Context, text, source, target string) (string, error) {
	if m.translated == "" {
		return "", errors.New("翻译失败")
	}
	return m.translated, nil
}

func TestEnsureTranslationCreatesEn(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestService(t)
	// 注入翻译器（enabled）
	cfg := &config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	svc.SetTranslator(&mockTranslator{translated: "EN-TRANSLATED"}, cfg)

	e, err := svc.Create(ctx, "article", "zh", map[string]any{
		"title":   "你好世界", "slug": "hello",
		"content": "<p>正文内容</p>",
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	st, err2 := svc.EnsureTranslation(ctx, "article", "zh", e.Content.ContentID, e.Fields, content.Actor{UserID: 1, IsModerator: true})
	if err2 != nil {
		t.Fatalf("EnsureTranslation: %v", err2)
	}
	if !st.Triggered || !st.Created || st.Status != "translated" {
		t.Errorf("status = %+v", st)
	}
	// en 已生成
	en, err := svc.GetPublishedBySlugLang(ctx, "article", "hello", "en")
	if err != nil {
		t.Fatalf("en 未生成: %v", err)
	}
	if en.Content.Title != "EN-TRANSLATED" {
		t.Errorf("en title = %q", en.Content.Title)
	}
	if en.Fields["content"] != "<p>EN-TRANSLATED</p>" {
		t.Errorf("en content = %v", en.Fields["content"])
	}
	// 非可翻译字段（无 cover/category 在此类型）跳过；此处验证 title/content 已翻译
}

func TestEnsureTranslationDisabled(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	// 未注入翻译器 → Triggered:false
	e, _ := svc.Create(ctx, "article", "zh", map[string]any{"title": "x", "slug": "a", "content": "<p>x</p>"}, 1)
	st, err := svc.EnsureTranslation(ctx, "article", "zh", e.Content.ContentID, e.Fields, content.Actor{UserID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if st.Triggered {
		t.Error("未注入翻译器应 Triggered:false")
	}
}
```

> 说明：`newTestService` 的 article 类型（service_test.go:23-35）含 title/slug/content/excerpt 字段，**无 translatable 标记**（Field 未设 Translatable）。测试需先给字段加 Translatable:true（在测试内先 UpdateType 或用独立的类型定义）。实现者应在测试里建一个带 `Translatable: true` 的 article 类型（title/content），确保翻译只作用于可翻译字段。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/content/ -run TestEnsureTranslation -v`
预期：FAIL（`SetTranslator`/`EnsureTranslation` 未定义，或翻译逻辑未生效）。

- [ ] **步骤 3：实现**

`internal/content/service.go`：
- import 加 `"dulizhan/internal/config"`、`"dulizhan/internal/translate"`。
- `Service` 结构加字段：
```go
type Service struct {
	store         store.Store
	reg           *schema.Registry
	reservedNames map[string]bool
	translator    translate.Translator
	translateCfg  *config.TranslateConfig
}
```
- 加方法：
```go
// SetTranslator 注入翻译器与配置（可选；nil 或 Enabled=false 时关闭自动翻译）。
func (s *Service) SetTranslator(t translate.Translator, cfg *config.TranslateConfig) {
	s.translator = t
	s.translateCfg = cfg
}

// TranslateStatus 自动翻译结果状态。
type TranslateStatus struct {
	Triggered bool   `json:"triggered"`
	Created   bool   `json:"created"`
	Status    string `json:"status"` // "translated" | "fallback"
}

// EnsureTranslation 保存源语言内容后调用：为 target_lang 自动生成/覆盖翻译。
func (s *Service) EnsureTranslation(ctx context.Context, typeName, lang, contentID string, data map[string]any, actor Actor) (TranslateStatus, error) {
	if s.translator == nil || s.translateCfg == nil || !s.translateCfg.Enabled {
		return TranslateStatus{}, nil
	}
	if lang != s.translateCfg.SourceLang {
		return TranslateStatus{}, nil
	}
	ct, err := s.GetType(ctx, typeName)
	if err != nil {
		return TranslateStatus{}, err
	}
	// 组装 en data：可翻译文本字段翻译，其余复制
	enData := make(map[string]any, len(data))
	for _, f := range ct.Fields {
		val, ok := data[f.Name]
		if !ok {
			continue
		}
		if f.Translatable && isTextType(f.Type) {
			str, ok := val.(string)
			if !ok || str == "" {
				enData[f.Name] = val
				continue
			}
			var tr string
			var terr error
			if f.Type == schema.TypeRichText {
				tr, terr = s.translator.TranslateRichText(ctx, str, s.translateCfg.SourceLang, s.translateCfg.TargetLang)
			} else {
				tr, terr = s.translator.TranslateText(ctx, str, s.translateCfg.SourceLang, s.translateCfg.TargetLang)
			}
			if terr != nil {
				// 降级：复制 zh
				enData[f.Name] = val
				return s.fallbackCopy(ctx, typeName, lang, contentID, data, actor)
			}
			enData[f.Name] = tr
		} else {
			enData[f.Name] = val
		}
	}
	// 写 en（存在覆盖 / 不存在新建）
	var enEntry Entry
	parents, err := s.store.ContentRepo().ListByContentID(ctx, contentID)
	if err != nil {
		return TranslateStatus{}, err
	}
	existing := int64(0)
	zhStatus := ""
	for _, p := range parents {
		if p.Lang == s.translateCfg.TargetLang {
			existing = p.ID
		}
		if p.Lang == lang {
			zhStatus = p.Status
		}
	}
	created := false
	if existing == 0 {
		ne, err := s.CreateTranslation(ctx, typeName, s.translateCfg.TargetLang, contentID, enData, actor)
		if err != nil {
			return TranslateStatus{}, err
		}
		enEntry = ne
		created = true
	} else {
		ue, err := s.Update(ctx, existing, enData)
		if err != nil {
			return TranslateStatus{}, err
		}
		enEntry = ue
	}
	// en 状态跟随 zh
	if zhStatus != "" && enEntry.Content.Status != zhStatus {
		if err := s.SetStatus(ctx, enEntry.Content.ID, zhStatus); err != nil {
			return TranslateStatus{}, err
		}
	}
	return TranslateStatus{Triggered: true, Created: created, Status: "translated"}, nil
}

// fallbackCopy 降级：en 复制 zh 全部字段，状态 draft。
func (s *Service) fallbackCopy(ctx context.Context, typeName, lang, contentID string, data map[string]any, actor Actor) (TranslateStatus, error) {
	enData := make(map[string]any, len(data))
	for k, v := range data {
		enData[k] = v
	}
	enData["slug"] = ensureSlug(enData)
	parents, err := s.store.ContentRepo().ListByContentID(ctx, contentID)
	if err != nil {
		return TranslateStatus{}, err
	}
	existing := int64(0)
	for _, p := range parents {
		if p.Lang == s.translateCfg.TargetLang {
			existing = p.ID
		}
	}
	created := false
	if existing == 0 {
		ne, err := s.CreateTranslation(ctx, typeName, s.translateCfg.TargetLang, contentID, enData, actor)
		if err != nil {
			return TranslateStatus{}, err
		}
		_ = s.SetStatus(ctx, ne.Content.ID, "draft")
		created = true
	} else {
		ue, err := s.Update(ctx, existing, enData)
		if err != nil {
			return TranslateStatus{}, err
		}
		_ = s.SetStatus(ctx, ue.Content.ID, "draft")
	}
	return TranslateStatus{Triggered: true, Created: created, Status: "fallback"}, nil
}

// isTextType 是否文本类字段类型。
func isTextType(t schema.FieldType) bool {
	switch t {
	case schema.TypeText, schema.TypeTextarea, schema.TypeRichText:
		return true
	}
	return false
}
```

> 说明：`Translator` 接口含 `TranslateText` 与 `TranslateRichText` 两个方法（任务 2 定义），这里直接调用 `s.translator.TranslateRichText(...)`，无需类型断言。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/content/ -run TestEnsureTranslation -v`
预期：PASS。同时 `go test -count=1 ./internal/content/ ./internal/translate/ -v` 无回归。

---

### 任务 4：adminapi 返回 auto_translate + main.go 装配

**文件：**
- 修改：`internal/adminapi/content.go`
- 修改：`cmd/dulizhan/main.go`
- 测试：`internal/adminapi/content_test.go`

`HandleContentCreate`/`HandleContentUpdate` 保存 zh 后调 `EnsureTranslation` 并把状态拼进响应；`main.go` 装配 SetTranslator。

- [ ] **步骤 1：编写失败测试**（`internal/adminapi/content_test.go` 追加）

```go
func TestContentCreateAutoTranslate(t *testing.T) {
	e := newEnv(t)
	// 注入翻译器（enabled）
	tr := &fakeTranslator{}
	cfg := &config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	e.svc.SetTranslator(tr, cfg)
	tok := e.login(t)

	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"自动翻译测试","slug":"auto-1","content":"<p>正文</p>"}}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Content struct {
				ContentID string `json:"content_id"`
			} `json:"content"`
			AutoTranslate struct {
				Triggered bool   `json:"triggered"`
				Created   bool   `json:"created"`
				Status    string `json:"status"`
			} `json:"auto_translate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Data.AutoTranslate.Triggered || !resp.Data.AutoTranslate.Created || resp.Data.AutoTranslate.Status != "translated" {
		t.Errorf("auto_translate = %+v", resp.Data.AutoTranslate)
	}
}
```

> 说明：`newEnv` 返回 `*env`，含 `svc *content.Service`（adminapi_test.go:67）。`fakeTranslator` 测试内定义（实现 TranslateText/TranslateRichText 返回固定值）。`config` import 需加。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test -count=1 ./internal/adminapi/ -run TestContentCreateAutoTranslate -v`
预期：FAIL（`auto_translate` 未返回）。

- [ ] **步骤 3：实现**

`internal/adminapi/content.go`：
- import 加 `"dulizhan/internal/content"`（已有）、`"dulizhan/internal/config"`（若需要类型，其实不需要——用 d.Content.EnsureTranslation 即可）。

`HandleContentCreate` 保存后追加：
```go
	e, err := d.Content.Create(c.Request.Context(), req.Type, req.Lang, req.Data, a.UserID)
	if err != nil {
		fail(c, err)
		return
	}
	at, _ := d.Content.EnsureTranslation(c.Request.Context(), req.Type, req.Lang, e.Content.ContentID, e.Fields, d.actor(c))
	respondOK(c, gin.H{"content": e, "auto_translate": at})
```

`HandleContentUpdate` 保存后追加（id 更新后取 e 的 ContentID/Lang）：
```go
	e, err := d.Content.Update(c.Request.Context(), id, req.Data)
	if err != nil {
		fail(c, err)
		return
	}
	at, _ := d.Content.EnsureTranslation(c.Request.Context(), e.TypeName, e.Content.Lang, e.Content.ContentID, e.Fields, d.actor(c))
	respondOK(c, gin.H{"content": e, "auto_translate": at})
```

> 说明：`auto_translate` 字段 JSON 由 `TranslateStatus` 的 json tag 控制（triggered/created/status）。非 zh 或未启用时返回 `{}`（Triggered:false 的零值）。

`cmd/dulizhan/main.go` 在 `svc := content.New(...)` 之后、`server.New` 之前：
```go
	if cfg.Translate.Enabled {
		tr := translate.New(cfg.Translate.APIKey, cfg.Translate.BaseURL, cfg.Translate.Model)
		svc.SetTranslator(tr, &cfg.Translate)
	}
```
需 import `"dulizhan/internal/translate"`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/adminapi/ -run TestContentCreateAutoTranslate -v`
预期：PASS。同时 `go test -count=1 ./internal/adminapi/ -v` 确认现有测试无回归（现有 create 测试不检查 auto_translate，`{}` 不影响）。

---

### 任务 5：前端提示

**文件：**
- 修改：`admin/src/api/content.ts`
- 修改：`admin/src/views/ContentEditView.vue`

前端根据 `auto_translate` 状态提示。

- [ ] **步骤 1：修改 content.ts**

`createContent`/`updateContent` 返回类型加 `auto_translate`：
```ts
export interface AutoTranslateStatus {
  triggered?: boolean
  created?: boolean
  status?: string
}
export const createContent = (type: string, lang: string, data: Record<string, unknown>) =>
  request<{ content: ContentEntry; auto_translate?: AutoTranslateStatus }>('/content', { method: 'POST', body: { type, lang, data } })
export const updateContent = (id: number, data: Record<string, unknown>) =>
  request<{ content: ContentEntry; auto_translate?: AutoTranslateStatus }>(`/content/${id}`, { method: 'PUT', body: { data } })
```

- [ ] **步骤 2：修改 ContentEditView.vue**

`save()` 中 create 与 update 分支，捕获返回值并提示：
```ts
if (isNew.value) {
  const r = await createContent(currentType.value.name, activeTab.value, payload)
  router.replace(`/content/${r.content.content.id}`)
  ElMessage.success('已创建')
  notifyAutoTranslate(r.auto_translate)
  await loadAfterCreate(r.content.content.id)
} else if (id.value) {
  if (needCreate.value || !currentEntryId.value) {
    const r = await createTranslation(id.value, activeTab.value, payload, currentType.value.name)
    currentEntryId.value = r.content.content.id
    needCreate.value = false
    ElMessage.success('已保存')
    // createTranslation 是新增语言版本（en→?），不触发 zh→en 自动翻译，无 auto_translate
  } else {
    const r = await updateContent(currentEntryId.value, payload)
    ElMessage.success('已保存')
    notifyAutoTranslate(r.auto_translate)
  }
  await loadEdit(id.value, activeTab.value)
}
```
新增辅助：
```ts
function notifyAutoTranslate(at?: { triggered?: boolean; status?: string }) {
  if (!at?.triggered) return
  if (at.status === 'translated') ElMessage.success('已自动生成英文翻译')
  else if (at.status === 'fallback') ElMessage.warning('已创建英文草稿，请手动补充翻译')
}
```
> 说明：自动翻译仅在**保存 zh**（`createContent` 新建 zh、`updateContent` 更新 zh）时触发；`createTranslation`（新增其他语言版本）不触发、无 `auto_translate`。

- [ ] **步骤 3：验证**

运行：
```powershell
cd admin
npx vue-tsc --noEmit
npx vitest run
```
预期：无类型错误、测试全绿。

---

### 任务 6：构建同步 + 全量验证

**文件：**（无代码逻辑改动）

- [ ] **步骤 1：构建 SPA 并同步 dist**

```powershell
cd admin
npx vite build
```
同步 `admin/dist/*` → `internal/server/dist/`（参照 build.ps1：清空重建 + .gitkeep）。

- [ ] **步骤 2：Go 全量验证**

运行：
```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
go test -count=1 ./...
go vet ./...
gofmt -l .
```
预期：全绿、vet 干净、gofmt 无输出。

- [ ] **步骤 3：冒烟**

重启服务。验证：
- 未配置 translate（enabled=false）→ 新建 zh 文章保存，无 en 生成、无提示，行为与之前一致
- config 配置 `translate.enabled: true` + 假 api_key → 新建 zh 保存 → en 草稿生成（fallback 降级），前端提示"已创建英文草稿"
- 若用户提供真实 key → en 翻译成功、状态跟随 zh、图片保留
- 清理测试产生的数据

预期：全部正确。

---

## 自检记录

**规格覆盖度：**
- §3 配置 → 任务 1 ✔
- §4 翻译服务（Translator + OpenAI + 富文本）→ 任务 2 ✔
- §5 内容服务集成（SetTranslator/EnsureTranslation/降级）→ 任务 3 ✔
- §5 adminapi 编排 + main 装配 → 任务 4 ✔
- §6 前端提示 → 任务 5 ✔
- §9 验证 → 任务 6 ✔

**占位符扫描：** 无 TODO/待定。所有代码块完整可照抄。

**类型一致性：**
- `config.TranslateConfig`（任务 1）字段 `Enabled/Provider/APIKey/BaseURL/Model/SourceLang/TargetLang` 在任务 3/4/6 使用，命名一致。
- `translate.Translator` 接口：`TranslateText(ctx, text, source, target)`（任务 2）；**含 `TranslateRichText(ctx, html, source, target)`**（计划修正已注明）。`translate.Service` 实现两者。
- `content.Service.SetTranslator(t Translator, cfg *config.TranslateConfig)`（任务 3）在任务 4 main.go 调用，签名一致。
- `content.TranslateStatus{Triggered, Created, Status}`（任务 3）在任务 4 adminapi 响应、任务 5 前端使用，json tag `triggered/created/status` 一致。
- `EnsureTranslation(ctx, typeName, lang, contentID, data, actor)` 签名在任务 3 定义、任务 4 adminapi 调用，一致。
- `newTestService`（service_test.go:14）与 `newEnv`（adminapi_test.go:40）复用；测试中 article 类型字段无 Translatable——任务 3 测试需先建带 `Translatable:true` 的类型（计划已注明）。
- `golang.org/x/net/html` 用于富文本解析（任务 2 richtext.go），go.mod 已有 v0.25.0。
