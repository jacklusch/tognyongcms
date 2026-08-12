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

## 分类子分类（树形）升级
子分类功能为 `categories` 表新增 `parent_id` 列（0=顶级）。启动时自动迁移：
- 首次启动自动执行 `ALTER TABLE categories ADD COLUMN parent_id INTEGER NOT NULL DEFAULT 0`（PRAGMA 探测，列已存在则跳过，幂等）
- 既有顶级分类 `parent_id=0`，行为不变
- 无需手动操作；升级后即可在后台分类管理中添加子分类（如 产品 → 斩拌机/香肠机/拌馅机）

## seed 演示数据
`./dulizhan seed` 幂等初始化：新闻/关于/产品三个顶级分类，产品下预建 斩拌机(chopper)/香肠机(sausage-machine)/拌馅机(mixer) 三个子分类，各带 1 篇双语文章，main 菜单含子分类下拉项。`DULIZHAN_SEED_NO_DOWNLOAD=1` 可跳过封面图网络下载（离线/测试用）。
