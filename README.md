# Dulizhan CMS

Go + Gin 通用 CMS（个人/公司官网）。单体单二进制，Vue 管理端 `go:embed` 进二进制。

## 功能
- 动态内容类型：后台可视化定义（阶段 4 构建器），运行时生效
- 主题化前台 SSR：主题目录切换、type_map 映射
- 内容级多语言：URL 语言前缀、翻译 Tab
- SEO：sitemap / robots / hreflang（含 x-default）/ canonical / Open Graph / JSON-LD
- RBAC 管理 API：用户/角色管理、权限点、类型级权限（content.read.<type>）
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
