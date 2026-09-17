# zh 保存自动生成 en 翻译 设计文档

- 日期：2026-08-13
- 状态：已确认（用户逐节审阅通过）

## 1. 目标

在后台内容管理编辑 zh 内容并保存时，**自动**调用翻译 API 生成对应的 en 翻译（中文→英文），图片/分类/日期等非可翻译字段保留，zh 与 en 一一对应（content_id 同组）。en 状态跟随 zh。

## 2. 设计决策（头脑风暴确认）

| 决策点 | 结论 |
|---|---|
| 翻译引擎 | OpenAI 兼容 `/chat/completions` 接口（config 配 api_key/base_url/model） |
| 触发时机 | zh（源语言）点击"保存"（Create/Update）时自动生成/覆盖 en |
| 已有 en | 重新翻译覆盖（en 可翻译字段用新翻译，非可翻译字段同步 zh） |
| en 状态 | 跟随 zh（zh published → en published；zh draft → en draft） |
| 失败处理 | 降级：en 复制 zh 值 + 图片，状态 draft，不阻断 zh 保存 |
| 翻译范围 | 仅 schema 标记 `translatable=true` 的文本字段；富文本按块翻译 |
| 开关 | `translate.enabled`，默认 false（关闭=行为与现状一致） |
| 架构 | 翻译器注入 content.Service（可选字段，不破坏现有 New）；调用方保持兼容 |

## 3. 配置（`internal/config`）

```yaml
translate:
  enabled: false          # 默认关闭
  provider: "openai"      # openai 兼容接口
  api_key: ""             # env: DULIZHAN_TRANSLATE_API_KEY
  base_url: "https://api.openai.com/v1"
  model: "gpt-4o-mini"
  source_lang: "zh"       # 触发翻译的源语言
  target_lang: "en"       # 自动生成的目标语言
```

- `TranslateConfig` 结构；`applyEnv` 加 env 覆盖；`Validate` 校验：enabled 时 api_key/base_url/model 必填、source_lang != target_lang、且都在 `site.languages` 内。
- `config.example.yaml` 加注释示例。

## 4. 翻译服务（`internal/translate`，可插拔）

```go
// Service 可插拔翻译器接口（便于测试 mock）
type Translator interface {
    TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error)
}

// OpenAI 实现
type Service struct{ apiKey, baseURL, model string; http *http.Client }
func New(apiKey, baseURL, model string) *Service
func (s *Service) TranslateText(ctx, text, source, target) (string, error)
```

- 调 `POST {base_url}/chat/completions`，system prompt 要求只翻译不解释、保留术语。
- 返回 content 字段的纯文本翻译；错误返回给调用方。
- HTTP 超时、非 2xx 返回错误。

### 富文本按块翻译

content 是 HTML（wangEditor 输出 `<p>...</p>` 等）。翻译策略：
- 用 HTML 解析把块级元素（p/li/h1-h6）拆成文本块，非文本标签（img/a/br）保留。
- 每个文本块单独翻译，再按原结构拼回 HTML（块间顺序、标签不丢失）。
- 简化实现：用正则/`golang.org/x/net/html` 解析 `<p>` 块内纯文本，逐块翻译，重组。

## 5. 内容服务集成（`internal/content`）

### 架构：翻译器作为可选依赖注入 + adminapi 编排

不破坏现有 `New(st, reg, reservedNames)`。给 `Service` 增加：
```go
type Service struct {
    store         store.Store
    reg           *schema.Registry
    reservedNames map[string]bool
    translator    Translator      // 可选，nil = 关闭自动翻译
    translateCfg  *config.TranslateConfig  // 可选
}
func (s *Service) SetTranslator(t Translator, cfg *config.TranslateConfig) // 后门注入，cmd/server 装配时可选调用
```

**翻译触发点放在 adminapi 编排，而非 Create/Update 内部**：
- `content.Service` 新增公开方法 `EnsureTranslation(ctx, typeName, lang, contentID string, data map[string]any, actor Actor) (TranslateStatus, error)`，封装 autoTranslate 全部逻辑，返回状态（见下）。
- `Create`/`Update` 保持纯净（不动签名、不自动触发）——现有调用点零影响。
- `adminapi.HandleContentCreate`/`HandleContentUpdate`：在保存 **zh** 成功后，调 `Content.EnsureTranslation(...)` 拿结果拼进响应。非 zh 语言或关闭时不调用。

### 触发逻辑（EnsureTranslation）

```
若 s.translator == nil 或 !cfg.enabled 或 lang != cfg.source_lang → 返回 TranslateStatus{Triggered:false}
1. 取类型 schema：遍历 translatable=true 的文本字段（title/excerpt/content 等）
2. 逐字段翻译：
   - 简单文本（text/textarea）：整段 TranslateText
   - 富文本（richtext）：按块翻译还原 HTML
3. 组装 en data：
   - 可翻译字段：翻译结果
   - 非可翻译字段（cover/category/日期/boolean/number/select 等）：复制 zh 的 data 值（图片保留）
4. 检查 en 是否已存在（ListByContentID 找 target_lang）：
   - 不存在 → CreateTranslation(typeName, target, contentID, enData, actor)
   - 存在 → Update(id, enData)（覆盖）
5. en 状态跟随 zh：SetStatus(enId, zh.status)
6. 任一步翻译失败 → 降级：enData = 复制 zh 全部字段（含图片），状态 draft，返回 TranslateStatus{Status:"fallback"}
   成功 → TranslateStatus{Status:"translated", Created: 新建}
```

### TranslateStatus

```go
type TranslateStatus struct {
    Triggered bool   `json:"triggered"`   // 是否触发翻译
    Created   bool   `json:"created"`     // 本次是否新建了 en
    Status    string `json:"status"`      // "translated" | "fallback"
}
```

### 返回给前端的翻译结果

`HandleContentCreate`/`HandleContentUpdate`（adminapi）在保存 zh 后调 `EnsureTranslation`，响应追加：
```json
{ "content": {...}, "auto_translate": { "triggered": true, "created": true, "status": "translated"|"fallback" } }
```
前端据此提示"已自动生成 en 翻译"或"en 已创建为草稿（翻译失败）"。

## 6. 前端（`admin/src`）

- `api/content.ts`：`ContentEntry`/响应类型加 `auto_translate?` 字段。
- `ContentEditView.vue` `save()`：保存 zh 后，根据返回的 `auto_translate` 提示：
  - `status: "translated"` → `ElMessage.success('已自动生成英文翻译')`
  - `status: "fallback"` → `ElMessage.warning('已创建英文草稿，请手动补充翻译')`
- 不改变编辑页 tab 行为（上一轮已修复的保存停留/隔离逻辑保留）。

## 7. 数据流与边界

- **幂等**：zh 重复保存 → 重新翻译覆盖 en（每次 zh 最新）。
- **en 单独保存**：不触发翻译（只 source_lang→target_lang 单向）。
- **非 zh 源语言**：source_lang 默认 zh，enabled 时可配置其他。
- **enabled=false**（默认）：`SetTranslator` 未调用或 cfg 关闭 → `EnsureTranslation` 直接返回 `Triggered:false`，行为与现状完全一致。
- **手动改过的 en**：下次 zh 保存（adminapi 调 EnsureTranslation）会被重新翻译覆盖（用户已确认）。
- **权限**：EnsureTranslation 用 actor 传 Content 服务（作者给翻译归属自己的内容，moderator 可翻译他人）——复用现有归属校验。

## 8. 测试

- `internal/translate`：mock HTTP server 测请求格式/响应解析/错误。
- `internal/translate`：富文本按块翻译还原测试（含 img/a 标签保留）。
- `internal/content`：`EnsureTranslation` 集成测试（mock Translator）：
  - zh 保存 → en 自动创建（翻译成功）
  - 已存在 en → 覆盖更新
  - 翻译失败 → 降级复制 + draft
  - enabled=false / translator=nil → Triggered:false
  - en 状态跟随 zh
- `internal/config`：translate 配置加载/校验/env。
- 前端：ContentEditView 提示逻辑（若有可测点）。

## 9. 验证

- `go test -count=1 ./...`、`go vet ./...`、`gofmt -l .` 干净。
- 前端 `npx vue-tsc --noEmit && npx vitest run`；`npx vite build` + 同步 dist。
- 冒烟（配置 translate.enabled=true + 假 API key → 失败降级路径；或真实 key 验证翻译路径）：
  - 后台新建 zh 文章保存 → en 自动生成、图片保留、状态跟随
  - 修改 zh 保存 → en 重新翻译覆盖
  - 未配置 translate → 行为不变
