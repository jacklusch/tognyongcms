# AGENTS.md

## 项目

dulizhan —— Go + Gin 通用 CMS（个人/公司官网）：动态内容类型、主题化前台 SSR、内容级多语言、SEO（sitemap/robots/hreflang/canonical/OG/JSON-LD）、RBAC 管理 API。单体单二进制，Vue 管理端 `go:embed` 进二进制。

权威设计文档（未落盘，仅在 git 索引中）：
`git show :docs/superpowers/specs/2026-08-05-cms-design.md`

## 当前仓库状态（重要）

- **全部 5 个阶段已完成**：阶段 1/2（骨架+管理 API，经 `2026-08-06-restore-phase1-phase2` 恢复）、阶段 3（Vue 管理端 SPA，`2026-08-06-phase3-admin-spa`）、阶段 4（可视化内容类型构建器 + 类型级权限，`2026-08-06-phase4-content-type-builder`）、阶段 5（用户/角色界面、S3 媒体、主题热重载+slog、PG/MySQL 可移植性文档、README/构建脚本，`2026-08-06-phase5-subproject-{a..e}`）。`go test -count=1 ./...` 全绿、build/vet/gofmt 干净、每阶段冒烟复现通过。
- 仍**不 git 提交**（用户约定：直接 master 开发，文件级审查）。`master` 无任何提交。
- 各阶段审计线索在 `.superpowers/sdd/` 下对应目录（progress.md + task brief/report + final-review/final-fix-report）。
- **注意**：phase2 原始 task-6..9（adminapi handler）brief/report 缺失，按恢复计划 Task 15 内联重建。
- **前端**：`admin/` 独立 Vite 工程（Vue3+TS+Element Plus+Pinia+wangEditor），构建产物同步到 `internal/server/dist/` 由 go:embed 嵌入。构建：`./build.sh` 或 `.\build.ps1`（npm run build → 同步 dist → go build）。
- **文档**：`README.md`、`docs/{deployment,theme-development,api-reference,database-compatibility}.md`、`config.example.yaml`。
- go.mod 已锁定：`gin@v1.10.0`、`modernc.org/sqlite@v1.34.2`（Go 1.22 兼容，勿用 @latest，需 Go 1.25+）、`gopkg.in/yaml.v3@v3.0.1`、`golang.org/x/crypto@v0.33.0`、aws-sdk-go-v2 系列（S3 媒体）。

## 环境与网络

- Go 1.26 已装。官方 proxy 被墙，拉依赖走镜像：当前机器 `GOPROXY=https://goproxy.cn,direct`；历史会话用 `GOPROXY=https://mirrors.aliyun.com/goproxy,direct GOSUMDB=off`。
- **勿运行 `go mod tidy`**（会清掉 indirect 锁定，如 gin/sqlite/aws 的 `// indirect` 标记）。新依赖用 `go get <pkg>@<精确版本>` 且验证 go.mod 既有锁定未变。
- Node 20+（本机 v23）+ npm 10。npm 慢/失败用 `--registry https://registry.npmmirror.com`。

## 验证命令（改动后必跑）

- Go：`go test -count=1 ./...`、`go build ./...`、`go vet ./...`、`gofmt -l .`
- 前端：`cd admin && npx vue-tsc --noEmit && npx vitest run`；改 SPA 后 `npx vite build` + 同步 `internal/server/dist/`
- 冒烟前先确认 8080 无残留 dulizhan 进程，否则首次请求 500 是端口占用，非代码缺陷。
- 冒烟用 PowerShell 发中文 JSON 会 GBK 乱码——用 ASCII body 或临时 httptest。

## 架构约定

- 分层：`internal/{config,server,schema,content,store,theme,i18n,auth,seo,media,adminapi}`、`cmd/dulizhan`（装配入口，媒体按 driver 选 LocalStore/S3Store）、`themes/default`（默认主题）。
- 模块间**只通过接口通信**（store/media/theme 可插拔），内部实现互不引用。
- 领域错误在 service 层返回 `errs.ErrNotFound / ErrForbidden / ErrValidation`，HTTP 层统一映射 404 / 403 / 422（JSON `{code,message}`）。SQLite UNIQUE 冲突须包装为 `ErrValidation`（sqlite 层 `wrapUnique`）。**users/roles 的 Create 也已走 wrapUnique**（子项目 A 修复）。
- store 模型 JSON tag 一律小写 snake_case；`data.content` 契约小写，返回 `type_name/fields`。
- 错误消息/文案用中文；日志用 slog（main + server 中间件；debug 时 LevelDEBUG + 主题热重载）。
- slug 唯一约束 `(lang, content_type_id, slug)`；内容多语言按 `content_id` 分组、一行一语言。
- 主题经 `type_map` 把内容类型映射到模板；**JSON-LD 由 `JSONLDScript` 携带完整 `<script>` 片段**、base.html 直接渲染——html/template 会把 `<script>` 上下文中的值当 JS 转义，勿把 JSON 对象直接放 `<script>` 标签内。
- RBAC：`HasPerm` 支持类型级前缀（`content.read.article` 命中 `content.read.*`/`content.*`/`*`）；`RequirePermType` 中间件；内容读操作按类型校验（写操作靠 `canManage` 归属）。角色 `admin`/`editor`/`author` 内置，内置角色不可改名/删；用户自我保护（删自己/内置 admin/最后 admin 403、改自己密码需旧密码）。

## 工作流约定

- 本仓库由 superpowers SDD（subagent-driven-development）驱动：先 `brainstorming`/`writing-plans` 产出计划，任务走 TDD（先写失败测试），派实现/审查子代理，审查意见记入 `progress.md`。
- **不 git 提交**（用户明确选择：直接 master 开发，文件级审查）。除非用户另行要求，勿执行 git add/commit。
- 设计文档 §10 的 5 个阶段已全部完成。若继续开发，新工作从新 brainstorming 开始。
