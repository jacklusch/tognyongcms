# 阶段 5 子项目 E：文档 + 单二进制打包 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 提供 build.sh/build.ps1 一步构建脚本（产出含 embed SPA 的单二进制）+ README.md + docs/ 详细手册（deployment/theme-development/api-reference）。

**架构：** 双平台构建脚本（bash + PowerShell，逻辑一致：npm run build → 同步 dist → go build）；README 概述 + docs 手册细化。

**技术栈：** bash、PowerShell、Markdown。

**前置基线：** 阶段 4 已验收通过；SPA 产物 go:embed 进二进制机制已就绪。规格：`docs/superpowers/specs/2026-08-06-phase5-subproject-e-docs-build-design.md`。

**环境约束：**
- **不 git 提交**
- build.ps1 需 Windows 实测（当前环境 win32）；build.sh 语法校验（无 bash 则逻辑核对）
- 文档内容与代码现状一致（端点/权限点/配置节核对）

---

### 任务 1：build.sh + build.ps1 + config.example.yaml

**文件：**
- 创建：`build.sh`
- 创建：`build.ps1`
- 创建：`config.example.yaml`

- [ ] **步骤 1：build.sh**

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

- [ ] **步骤 2：build.ps1**

```powershell
$ErrorActionPreference = "Stop"
Push-Location $PSScriptRoot

Write-Host "==> 构建 admin SPA"
Push-Location admin
npm run build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Pop-Location

Write-Host "==> 同步产物到 internal/server/dist"
if (Test-Path internal/server/dist) { Remove-Item -Recurse -Force internal/server/dist }
New-Item -ItemType Directory -Path internal/server/dist -Force | Out-Null
Copy-Item -Path admin/dist/* -Destination internal/server/dist/ -Recurse -Force
New-Item -ItemType File -Path internal/server/dist/.gitkeep -Force | Out-Null

Write-Host "==> 构建二进制"
go build -o dulizhan.exe ./cmd/dulizhan
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> 完成: ./dulizhan.exe (配置见 config.yaml)"
Pop-Location
```

- [ ] **步骤 3：config.example.yaml**

复制 `config.yaml` 内容（含注释说明各节用途）：
```yaml
# Dulizhan CMS 配置示例。复制为 config.yaml 后按需修改。
server:
  addr: ":8080"
  data_dir: "./data"
  debug: false              # true 时主题热重载 + DEBUG 日志
database:
  driver: "sqlite"          # sqlite（当前支持）；PG/MySQL 见 docs/database-compatibility.md
  dsn: "dulizhan.db"
site:
  name: "Dulizhan CMS"
  url: "http://localhost:8080"
  description: "一个通用的 Go CMS"
  default_lang: "zh"
  languages: ["zh", "en"]
  theme: "default"
  themes_dir: "./themes"
media:
  driver: "local"           # local | s3
  # s3:
  #   endpoint: ""                    # 自定义端点（MinIO/OSS），AWS 可留空
  #   region: "cn-hangzhou"
  #   bucket: "my-bucket"
  #   access_key: "xxx"
  #   secret_key: "yyy"
  #   public_base_url: ""             # 媒体公开 URL 前缀（CDN/自定义域名）
```

- [ ] **步骤 4：验证**

- Windows：`.\build.ps1` 跑通 → `dulizhan.exe` 生成 → 启动 `/admin` 可访问
- build.sh：`bash -n build.sh` 语法校验（若本机无 bash 则逻辑人工核对）
- 冒烟后清理（停服、删 exe/db/data）

---

### 任务 2：README.md

**文件：**
- 创建：`README.md`（仓库根）

- [ ] **步骤 1：写 README.md**

```markdown
# Dulizhan CMS

Go + Gin 通用 CMS（个人/公司官网）。单体单二进制，Vue 管理端 `go:embed` 进二进制。

## 功能
- 动态内容类型：后台可视化定义（阶段 4 构建器），运行时生效
- 主题化前台 SSR：主题目录切换、type_map 映射
- 内容级多语言：URL 语言前缀、翻译 Tab
- SEO：sitemap / robots / hreflang（含 x-default）/ canonical / Open Graph / JSON-LD
- RBAC 管理 API：角色/权限点、类型级权限（content.read.<type>）
- 媒体：本地磁盘或 S3（MinIO/OSS 兼容）

## 快速开始

前置：Go 1.22+、Node 20+。

```bash
./build.sh        # Windows: .\build.ps1（构建 SPA + 同步 dist + 编译二进制）
./dulizhan seed   # 初始化数据（幂等：内容类型 + 示例内容 + 三角色 + admin/admin123）
./dulizhan        # 启动 http://localhost:8080
```

浏览器访问 http://localhost:8080/admin 登录（admin / admin123）。

## 配置

见 `config.example.yaml`。环境变量 `DULIZHAN_*` 覆盖（如 `DULIZHAN_SITE_URL`）。详见 `docs/deployment.md`。

## 开发

- 后端：`go run ./cmd/dulizhan`（8080）
- 前端：`cd admin && npm install && npm run dev`（vite 5173，代理 /api /media /themes 到 8080）
- 主题热重载：config `server.debug: true`

## 文档

- `docs/deployment.md` —— 部署
- `docs/theme-development.md` —— 主题开发
- `docs/api-reference.md` —— API 参考
- `docs/database-compatibility.md` —— 数据库兼容性
```

- [ ] **步骤 2：验证**

核对 README 内容与代码现状一致（端点/命令/配置节）。

---

### 任务 3：docs/ 详细手册

**文件：**
- 创建：`docs/deployment.md`
- 创建：`docs/theme-development.md`
- 创建：`docs/api-reference.md`

- [ ] **步骤 1：docs/deployment.md**

```markdown
# 部署

## 前置
单二进制 `dulizhan`（含 embed SPA）+ `config.yaml` + `themes/` 目录。

## 构建
```bash
./build.sh   # Windows: .\build.ps1
```

## 运行
```bash
./dulizhan seed   # 首次初始化（幂等）
./dulizhan        # 默认 :8080
```

## 配置覆盖（环境变量）
`DULIZHAN_SERVER_ADDR`、`DULIZHAN_SITE_URL`、`DULIZHAN_DATABASE_DSN`、`DULIZHAN_MEDIA_DRIVER` 等（前缀 `DULIZHAN_`）。

## 数据与媒体
- `data_dir`（默认 ./data）存媒体文件
- SQLite 数据库文件（默认 dulizhan.db）

## 媒体驱动
- local：`media.driver: local`，文件存 `data_dir/media`
- s3：`media.driver: s3` + `media.s3` 块（AWS/MinIO/OSS）

## 生产建议
- 反向代理（nginx/caddy）转发 80/443 → :8080
- 静态资源缓存（/themes、/media）
- 定时备份 SQLite 文件
- 多实例部署前确认存储（SQLite 单写者，多实例共享需 PG/MySQL——见 database-compatibility.md）
```

- [ ] **步骤 2：docs/theme-development.md**

```markdown
# 主题开发

## 目录结构
```
themes/<name>/
├── theme.yaml            # 主题清单
├── templates/            # 模板（index/list/single/page 等）
│   └── partials/         # 公共片段（nav/footer）
├── static/               # CSS/JS/图片
└── locales/              # zh.yaml / en.yaml 主题文案
```

## theme.yaml
```yaml
name: default
version: 1.0.0
templates:
  index: templates/index.html
  single: templates/single.html
  list: templates/list.html
partials:
  - templates/partials/nav.html
type_map:                 # 内容类型 → 模板 key
  article: single
  page: single
locales:
  zh: locales/zh.yaml
  en: locales/en.yaml
```

## 模板函数
- `t "lang" "key"` —— 界面文案翻译
- `url "lang" entry` —— 内容 URL（带语言前缀）
- `typeURL "lang" "typeName"` —— 类型列表 URL
- `home "lang"` —— 首页 URL
- `asset "/css/x.css"` —— 主题静态资源 URL
- `media value` —— 媒体 URL 解析
- `raw value` —— 原始 HTML（仅 richtext 字段）
- `pages total perPage` —— 分页页码数组
- `pagerURL data page` —— 分页 URL

## 数据（Data 结构）
`.Site.Name/.URL/.Description`、`.Lang`、`.Langs`、`.Entry.Content`、`.Entry.Fields.<name>`、`.Items`、`.Total/.Page/.PerPage`、`.Meta.*`（SEO）。

## 开发热重载
config `server.debug: true` 时改模板立即生效（mtime 检测）。

## 前台主题切换
改 config `site.theme` 或 settings.theme。
```

- [ ] **步骤 3：docs/api-reference.md**

```markdown
# API 参考

管理 API 前缀 `/api`，返回 JSON `{code, message, data?}`。认证：`POST /api/auth/login` 拿 token → `Authorization: Bearer <token>`。

## 认证
| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | /api/auth/login | 公开 | body {username,password} → {token} |
| POST | /api/auth/logout | 登录 | 失效会话 |
| GET | /api/auth/me | 登录 | {user:{id,username,role}, permissions} |

## 内容类型
| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | /api/content-types | content.read | 类型列表 |
| POST | /api/content-types | content_types.manage | 建类型 |
| PUT | /api/content-types/:id | content_types.manage | 改类型（name 不可变） |
| DELETE | /api/content-types/:name | content_types.manage | 级联删类型+内容 |

## 内容
| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | /api/content | content.read.<type> | 列表（type/lang/status/page） |
| POST | /api/content | content.write | 建内容 |
| GET | /api/content/:id | content.read.<type> | 详情 |
| PUT | /api/content/:id | content.write | 更新 |
| DELETE | /api/content/:id | content.delete | 删除 |
| POST | /api/content/:id/publish | content.publish | 发布 |
| POST | /api/content/:id/unpublish | content.publish | 撤回 |
| GET | /api/content/:id/translations | content.read.<type> | 翻译列表 |
| POST | /api/content/:id/translate | content.write | 建翻译 |

## 媒体 / 设置 / 菜单 / 用户 / 角色 / 搜索 / 统计
| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | /api/media/upload | media.upload | 上传 |
| GET | /api/media | media.upload | 列表 |
| DELETE | /api/media/:id | media.delete | 删除 |
| GET/PUT | /api/settings | settings.manage | 设置读写 |
| GET/POST | /api/menus | menus.manage | 菜单列表/新建 |
| PUT/DELETE | /api/menus/:id | menus.manage | 改/删菜单 |
| GET/POST | /api/users | users.manage | 用户列表/新建 |
| DELETE | /api/users/:id | users.manage | 删用户（自我保护） |
| PUT | /api/users/:id/role | users.manage | 改角色 |
| PUT | /api/users/:id/password | users.manage | 改密码（自己需旧密码） |
| GET | /api/roles | roles.manage | 角色列表 |
| POST | /api/roles | roles.manage | 建角色 |
| PUT/DELETE | /api/roles/:id | roles.manage | 改/删角色 |
| GET | /api/roles/perms | roles.manage | 权限点列表 |
| GET | /api/search | content.read.<type> | 搜索（type/lang/q） |
| GET | /api/stats | content.read | 仪表盘统计 |
| GET | /api/meta | 登录 | 站点配置（语言/主题） |

## 错误
`{code, message}`：401 未登录 / 403 无权限 / 404 不存在 / 422 校验失败 / 400 参数错误 / 500 服务器错误。

## 类型级权限
权限点支持前缀：`content.read.article` 命中 `content.read.*`、`content.*`、`*`（admin）。角色经 `GET /api/roles/perms` 查看全部权限点。
```

- [ ] **步骤 4：验证**

核对文档端点/权限点与 `internal/adminapi/register.go` 现状一致（含阶段 5 子项目 A 的角色 CRUD 扩展）。

---

## 自检记录

**1. 规格覆盖度：**
- 规格 §2 build 脚本 → 任务 1
- 规格 §3 README → 任务 2
- 规格 §4 docs/ 手册 → 任务 3
- 规格 §5 验收 → 任务 1 步骤 4

**2. 占位符扫描：** 无 TBD/TODO。文档内容完整。

**3. 类型一致性：** 文档端点/权限点与代码核对（register.go、子项目 A 角色扩展）；build 脚本命令与现有工作流一致。
