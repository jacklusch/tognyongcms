# Dulizhan CMS — 阶段 1/2 源码恢复设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`（权威设计，从 git 索引取回磁盘）
- 审计线索：`.superpowers/sdd/2026-08-06-phase1-skeleton-frontend/`、`.superpowers/sdd/2026-08-06-phase2-admin-api/`（progress.md + 各 task brief/report + final-fix-report）

## 1. 目标

把阶段 1（前台骨架）与阶段 2（管理 API）恢复到账本记载的**契约状态**：

- 所有文档化测试通过（`go test -count=1 ./...`，uncached）
- `go build ./...`、`go vet ./...` 干净
- final-fix-reports 记载的冒烟步骤可复现
- 顺手解决账本列出的 deferred minor 项（按第 5 节分类处理）

验收以"账本记载的契约 + 测试 + 修复"为准，不追求与原盘代码逐字一致，行为等价即视为恢复成功。

## 2. 范围

### 2.1 恢复范围（包清单）

- **phase1**：`go.mod`/`go.sum`、`config.yaml`、`cmd/dulizhan/main.go`、`internal/{config,errs,store,store/sqlite,schema,content,i18n,seo,theme,server,seed}`、`themes/default`
- **phase2**：`internal/{auth,media,adminapi}`、store 扩展（users/roles/sessions/menus/media 仓库）

### 2.2 契约项（final-fix 轮已修复行为，按"必须实现"写入）

- **JSON-LD**：`JSONLDScript` 携带完整 `<script type="application/ld+json">...</script>` 片段，`base.html` 直接渲染（html/template 会把 `<script>` 上下文的值当 JS 转义，勿把 JSON 对象放 `<script>` 内）
- **hreflang**：前置 `x-default`（默认语言 URL），随后各语言条目
- **内容类型名与语言代码冲突**：`content.New` 增加 `reservedNames` 参数，`CreateType`/`UpdateType` 校验，冲突返回 `errs.ErrValidation`
- **renderList 错误映射**：`errors.Is(err, errs.ErrNotFound)` 才渲染 404，其余 500
- **登录/me/actor nil 防护**：`HandleLogin`/`HandleMe`/`actor` 失败返回 401，不裸解引用
- **CreateTranslation 归属**：增加 `Actor` 参数，校验父内容存在与归属（作者翻译他人内容→403）
- **重复 slug**：store 层检测 UNIQUE constraint，包装为 `errs.ErrValidation`（`wrapUnique`，消息 `slug 已存在`），HTTP 422
- **JSON tag**：store 模型（含 Content/Menu/Media）JSON tag 一律小写 snake_case；content Entry 返回 `type_name/fields`，`data.content` 小写契约
- **PUT /content-types/:id**：尊重路径 `:id` 参数

### 2.3 排除

阶段 3-5（另行头脑风暴）；不引入任何新功能。设计文档第 10 节阶段 3+ 不属本规格。

## 3. 环境与工具链

- Go 1.26 已装。官方 proxy 被墙，拉依赖走镜像：当前机器 `GOPROXY=https://goproxy.cn,direct`；如失败改用 `GOPROXY=https://mirrors.aliyun.com/goproxy,direct GOSUMDB=off`
- go.mod 锁定版本（Go 1.22 兼容，勿用 @latest，需 Go 1.25+）：
  - `github.com/gin-gonic/gin v1.10.0`
  - `modernc.org/sqlite v1.34.2`
  - `gopkg.in/yaml.v3 v3.0.1`
  - `golang.org/x/crypto v0.33.0`（phase2 引入）
- **勿运行 `go mod tidy`** 直到 gin/sqlite 被真实 import（会清掉 indirect 锁定）
- 冒烟前确认 8080 无残留 dulizhan 进程，否则首次请求 500 是端口占用，非代码缺陷

## 4. 恢复结构与验收关卡

| 步 | 内容 | 验收关卡 |
|---|---|---|
| 0 | 取回设计文档到磁盘；建 go.mod/go.sum（锁 gin/sqlite/yaml.v3）、config.yaml、internal/errs、占位 cmd/dulizhan/main.go | `go build ./...`；`go test ./internal/config ./internal/errs` |
| 1 | store 接口 + SQLite 基础（content_types/content/settings 仓库） | sqlite 包 brief 测试 + B 类补测 |
| 2 | schema + content + i18n + seo | 各包 brief 测试 + B 类补测 |
| 3 | theme + 默认主题 + server 装配 + seed + 前台路由 | server 测试 + 冒烟（/article/hello-zh、/sitemap.xml、/robots.txt） |
| 4 | phase2 仓库（users/roles/sessions/menus/media）+ auth + RBAC + media | 各包 brief 测试 + B 类补测 |
| 5 | adminapi 全量 + server `/api` 装配 + e2e | adminapi 测试 + 冒烟（login→CRUD→发布→前台渲染） |
| 6 | minor 修复批次（A/B 类）+ 全量验收 | `go test -count=1 ./...` + build + vet + final-fix 冒烟清单 |

每步按对应 brief 的测试实现（TDD：RED→GREEN），跑通该包测试再进下一步；遇到账本未记载的偏差**停下问用户**。

## 5. deferred minor 项处理策略

把账本中两个阶段约 40 条 minor 项按三档分类：

### A. 修复型（改代码行为）

- sqlite 层：不再静默丢弃 `time.Parse` 错误；去重 `SetMaxOpenConns(1)`；`Update` 也更新 `content_id`/`content_type_id`
- schema：multiselect/repeat 接受 `[]any`；`toFloat` 支持 `json.Number`/`float32`/`int64`；pattern 正则编译缓存
- i18n：`URLPath` 前置条件校验；`All()` 返回副本防外部修改；`New` 语言去重
- seo：`RobotsTXT` 尾斜杠规范化；`SitemapXML` 补 XML 声明；`json.Marshal` 错误不再静默
- adminapi：`HandleMenuUpdate` 部分 PUT 不再清空 Name/Lang；`menuReq.Items` 去掉双重编码；`actor()` 不吞 `UserFromContext` 错误；`SetStatus` 错误消息中文化
- 其他：`LocalStore` Save/Delete key 净化、Save 检查 Close 错误；`handleSitemap` 不吞 `ListPublished` 错误；main.go seed 子命令不依赖 flag 位置且不强制要求 config.yaml

### B. 补测型（补测试/断言，不改行为）

- sqlite：unique 约束、GetByID/Update/Delete/upsert 测试；Session.ExpiresAt 往返断言；DeleteByUser/Menu 各方法/RoleRepo.List 缺测项
- schema：表驱动 + datetime/slug/pattern/multiselect/IndexField/Slugify/DecodeFields/AllTypes 覆盖
- seo：title/description 回退、BuildList page>1、fmtTime nil、hrefLangs 容量
- 其余：过期会话删除断言、bcrypt 哈希断言、cookie flags 断言、越权/缺失 id 负面测试、permList 排序

### C. 保持原样（规格中写明理由）

- Task7 `raw` 的 XSS 风险：设计允许，仅 richtext 用（文档限定）
- Task8 `Fields` map 遍历顺序：无害
- `GET /media` 用 `media.upload` 权限：brief 既定
- `mustJSON`/`contains` 吞错、`..` 守卫：经评估安全（写明理由）

## 6. 错误处理与架构约定（恢复时须遵守）

- 领域错误在 service 层返回 `errs.ErrNotFound` / `ErrForbidden` / `ErrValidation`，HTTP 层统一映射 404 / 403 / 422（JSON `{code,message}`）；SQLite UNIQUE 冲突包装为 `ErrValidation`（`wrapUnique`）
- 模块间只通过接口通信（store/media/theme 可插拔），内部实现互不引用
- store 模型 JSON tag 一律小写 snake_case；`data.content` 契约小写，返回 `type_name/fields`
- 错误消息/文案用中文；日志用 slog
- slug 唯一约束 `(lang, content_type_id, slug)`；内容多语言按 `content_id` 分组、一行一语言
- 主题经 `type_map` 把内容类型映射到模板；JSON-LD 契约见 2.2

## 7. 最终验收（第 6 步）与交付物

**最终验收**：
1. `go test -count=1 ./...` 全绿（uncached）
2. `go build ./...`、`go vet ./...` 干净
3. 复现 final-fix-reports 两轮冒烟：
   - 前台：`/article/hello-zh` 返回有效 JSON-LD + x-default/zh/en hreflang
   - 管理端：login→me→建类型→建内容→发布→前台渲染；重复 slug 422；作者/翻译/发布越权 403
4. 冒烟前确认 8080 无残留进程

**交付物**：
- 可 `go build/test/vet` 全过的完整源码树（与 AGENTS.md 架构约定一致）
- 阶段 3（Vue 管理端 SPA）的明确开发基线

## 8. 工作流约定

- 按仓库约定：**不 git 提交**，直接 master 开发，文件级审查；除非用户另行要求，勿执行 git add/commit
- 由 superpowers SDD（subagent-driven-development）驱动：先 writing-plans 产出实现计划，任务走 TDD（先写失败测试）
- **验收为测试级**：以文档化测试 + build/vet + 冒烟为准，不强制派审查子代理；如某步发现契约不明或质量存疑，实现者可自行决定加一轮自审（记录于 `progress.md`）
