# Dulizhan CMS — 阶段 5 子项目 E：文档 + 单二进制打包 设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`、`2026-08-06-phase4-content-type-builder-design.md`
- 基线：阶段 4 已验收通过；SPA 产物 go:embed 进二进制机制已就绪

## 1. 目标与范围

提供一步构建脚本（build.sh + build.ps1）产出含 embed SPA 的单二进制；编写 README 与 docs/ 详细手册。

**包含**：build.sh/build.ps1、README.md、docs/{deployment,theme-development,api-reference}.md。
**排除**：Dockerfile（用户确认不需要）、CI 配置、发布流程自动化。

## 2. build 脚本（双平台）

### 2.1 build.sh（bash，Linux/macOS）

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

echo "==> 构建 admin SPA"
(cd admin && npm run build)

echo "==> 同步产物到 internal/server/dist"
rm -rf internal/server/dist
mkdir -p internal/server/dist
cp -r admin/dist/* internal/server/dist/
touch internal/server/dist/.gitkeep

echo "==> 构建二进制"
go build -o dulizhan ./cmd/dulizhan

echo "==> 完成: ./dulizhan (配置见 config.yaml)"
```

### 2.2 build.ps1（Windows PowerShell）

```powershell
$ErrorActionPreference = "Stop"
Push-Location $PSScriptRoot

Write-Host "==> 构建 admin SPA"
Push-Location admin
npm run build
Pop-Location

Write-Host "==> 同步产物到 internal/server/dist"
Remove-Item -Recurse -Force internal/server/dist -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path internal/server/dist -Force | Out-Null
Copy-Item -Path admin/dist/* -Destination internal/server/dist/ -Recurse -Force
New-Item -ItemType File -Path internal/server/dist/.gitkeep -Force | Out-Null

Write-Host "==> 构建二进制"
go build -o dulizhan.exe ./cmd/dulizhan

Write-Host "==> 完成: ./dulizhan.exe (配置见 config.yaml)"
Pop-Location
```

### 2.3 验证

- `go build` 产物 `dulizhan`/`dulizhan.exe` 含 SPA（`/admin` 可访问）
- `config.example.yaml`：复制自 config.yaml，作运行模板
- 单二进制运行：二进制 + config.yaml + themes/ 目录（数据/媒体写 data_dir）即可

## 3. README.md（仓库根）

- **项目简介**：dulizhan —— Go + Gin 通用 CMS（个人/公司官网），单体单二进制，Vue 管理端 go:embed
- **功能**：动态内容类型、主题化前台 SSR、多语言、SEO（sitemap/robots/hreflang/JSON-LD）、RBAC 管理 API、媒体（本地/S3）
- **快速开始**：
  ```bash
  ./build.sh            # 或 build.ps1
  ./dulizhan seed       # 初始化数据（幂等）
  ./dulizhan            # 启动 :8080
  # 浏览器 http://localhost:8080/admin  (admin/admin123)
  ```
- **配置说明**：config.yaml 各节（server/database/site/media，含 media.driver local|s3）
- **开发**：admin dev（vite 代理 8080）+ 后端 dev（go run）；`server.debug: true` 主题热重载 + DEBUG 日志
- **API 概览**：/api 分组（auth/content-types/content/media/settings/menus/users/roles/search/stats/meta），Bearer token 认证

## 4. docs/ 详细手册

### 4.1 docs/deployment.md（部署）

- 前置：单二进制 + config.yaml + themes/；环境变量覆盖（DULIZHAN_*）
- 数据/媒体目录（data_dir）
- 媒体驱动选择（local | s3 配置示例）
- 生产建议：反向代理（nginx）静态资源缓存、时区、备份 SQLite

### 4.2 docs/theme-development.md（主题开发）

- 目录结构（theme.yaml/templates/static/locales）
- theme.yaml 配置（templates/partials/type_map/fields/locales）
- 模板函数（t/url/typeURL/home/pagerURL/asset/media/raw/pages）
- Data 结构（Site/Lang/Entry/Items/Meta 等）
- 开发热重载（server.debug: true）

### 4.3 docs/api-reference.md（API 参考）

- 认证（login → Bearer token / cookie）
- 完整端点表：方法/路径/权限点/请求示例/响应示例
  - auth（login/logout/me，me 返回 user+permissions）
  - content-types CRUD（含 perms 点）
  - content CRUD + 翻译 + publish/unpublish + search
  - media（upload/list/delete）
  - settings/menus
  - users/roles（阶段 5 子项目 A 扩展后）
  - stats/meta
- 错误格式（{code, message}，404/403/422/401）
- 类型级权限（content.read.<type> 前缀）

## 5. 验收

- build.sh/build.ps1 各跑通一次（Windows 验证 build.ps1；build.sh 语法校验），产物 `/admin` 可访问
- README/docs 内容与代码现状一致（端点、权限点、配置节核对）

## 6. 工作流约定

- 不 git 提交；superpowers SDD 驱动
