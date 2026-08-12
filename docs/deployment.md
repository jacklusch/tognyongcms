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

## 分类系统升级（v1 已有 select 分类字段的库）
既有库的 article `category` 是 select 字段（options 存中文名），升级后需手动迁移到分类表：
1. 在分类管理中创建对应分类（如 新闻/产品/关于），记下 id
2. 用 SQL 把旧 payload 里的 select 值替换为分类 id（需先建分类）：
   UPDATE content SET payload = REPLACE(payload, '"category":"新闻"', '"category":"1"') WHERE payload LIKE '%"category":"新闻"%';
3. 在内容类型构建器把 article 的 category 字段改为 relation（目标 category）
4. 升级后新内容选分类存 id；旧内容替换后前台归档生效

空库无此风险；新库 seed 直接建 relation 字段。
