# 数据库兼容性（SQLite / PostgreSQL / MySQL）

## 概述
dulizhan 默认 SQLite，存储层代码以 `?` 占位符与跨库 SQL 编写。本文档记录 PG/MySQL 接入差异与现有 SQL 审查结论。

## 驱动接入
- SQLite：`modernc.org/sqlite`（已用）
- PostgreSQL：`github.com/jackc/pgx/v5/stdlib` 或 `github.com/lib/pq`（database/sql 驱动）
  - DSN 示例：`postgres://user:pass@host:5432/db?sslmode=disable`
- MySQL：`github.com/go-sql-driver/mysql`
  - DSN 示例：`user:pass@tcp(host:3306)/db?parseTime=true&charset=utf8mb4`

## 占位符差异
- SQLite / MySQL：`?`
- PostgreSQL：`$1, $2, ...`
- 代码层统一 `?`：PG 需驱动转换（pgx 支持 `?`→`$n` 自动转换；lib/pq 不支持）→ 建议 pgx。

## DDL 差异
| 表/列 | SQLite | PostgreSQL | MySQL |
|---|---|---|---|
| 自增主键 | `INTEGER PRIMARY KEY AUTOINCREMENT` | `BIGSERIAL` / `GENERATED ALWAYS AS IDENTITY` | `BIGINT AUTO_INCREMENT` |
| JSON（content.payload 等） | `TEXT` | `JSONB` | `JSON` / `LONGTEXT` |
| 时间戳 | `TEXT`（RFC3339） | `TIMESTAMPTZ` | `DATETIME(3)` |
| TEXT 主键（settings.key、sessions.token） | `TEXT PRIMARY KEY`（合法） | `TEXT PRIMARY KEY`（合法） | 非法——TEXT 主键须限长（`VARCHAR(255) PRIMARY KEY`）；且 `key` 是 MySQL 保留字，须反引号 `` `key` `` |

## Upsert 差异
| 库 | 语法 |
|---|---|
| SQLite | `INSERT ... ON CONFLICT(key) DO UPDATE SET ...` |
| PostgreSQL | `INSERT ... ON CONFLICT (key) DO UPDATE SET ...` |
| MySQL | `INSERT ... ON DUPLICATE KEY UPDATE ...` |
- 当前实现（settings）：先查后写（三库兼容），避免语法差异；未来可按 driver 分支优化。

## LIKE 大小写语义
| 库 | 行为 |
|---|---|
| SQLite | 对 ASCII 大小写不敏感（`LIKE 'abc'` 命中 `ABC`） |
| PostgreSQL | `=` 区分大小写；`LIKE` 区分大小写、`ILIKE` 不区分 |
| MySQL | 默认 collation（如 `utf8mb4_general_ci`）下 `=` 与 `LIKE` 均不区分大小写 |
- 现状（SearchByTypeLang / CountSearch）：`LIKE` 搜索 title/slug，三库命中语义不一致，文档化；跨库统一需显式大小写规范化（如 lower()）或按库选择操作符。

## 事务与锁
- SQLite：单写者，`SetMaxOpenConns(1)` 串行
- PostgreSQL：MVCC，读不阻塞写
- MySQL（InnoDB）：行锁，`REPEATABLE READ` 默认隔离
- 项目用 `WithTx`（事务内两步）——跨库兼容。

## 现有 SQL 审查结论（2026-08-06）
| SQL | 位置 | 兼容性 | 处置 |
|---|---|---|---|
| `INSERT INTO settings ... ON CONFLICT(key) DO UPDATE` | sqlite.go Set | SQLite/PG 兼容、MySQL 不兼容 | 已改造为先查后写 |
| `?` 占位符 | 全部 | SQLite/MySQL 兼容、PG 需转换 | 文档化（建议 pgx） |
| `id INTEGER PRIMARY KEY AUTOINCREMENT` | migrate.go | 仅 SQLite | DDL 文档化 |
| `LIMIT ? OFFSET ?` | ListByTypeLangStatus 等 | 三库兼容 | 保持 |
| `UNIQUE(lang, content_type_id, slug)` | migrate.go | 三库兼容 | 保持 |
| RFC3339 文本日期 | Create/Update | 三库兼容 | 保持 |
| `res.LastInsertId()`（6 处：content_types / content / users / roles / menus / media 的 Create） | sqlite.go | SQLite/MySQL 支持；PG 的 lib/pq 返回 0 | PG 需改 `RETURNING id` 或换 pgx（pgx 经 stdlib 亦返回 0，需 `RETURNING`） |
| `LIKE` 搜索（title / slug） | SearchByTypeLang / CountSearch | 三库大小写语义不一致（见上节） | 文档化；跨库需规范化或按库选择操作符 |
| `TEXT PRIMARY KEY`（settings.key、sessions.token） | migrate.go | SQLite/PG 合法、MySQL 非法；`key` 为 MySQL 保留字 | DDL 文档化；MySQL 用 `VARCHAR(255) PRIMARY KEY` 且反引号保留字 |

## 子分类功能新增 SQL 兼容性（2026-08-12）
| SQL | 位置 | 兼容性 | 处置 |
|---|---|---|---|
| `ALTER TABLE categories ADD COLUMN parent_id INTEGER NOT NULL DEFAULT 0` | migrate.go `migrateCategoriesParent` | SQLite 支持 `ADD COLUMN`；PG 支持；MySQL 8+ 支持、MySQL <8 不支持单语句 ADD COLUMN 带默认值之外的部分 | 文档化；当前 SQLite 用 PRAGMA 探测列存在才 ALTER（幂等）；PG/MySQL 接入时需按库探测（information_schema）后执行对应 DDL |
| 递归 CTE `WITH RECURSIVE descs(id) AS (...)` | sqlite.go `Descendants` | SQLite 支持；PostgreSQL 支持；MySQL 8.0+ 支持、MariaDB 10.2+ 支持 | 文档化（需 MySQL 8+/MariaDB 10.2+；老版本需改迭代查询） |
| `payload LIKE '%"category":"<id>"%'`（OR 多值） | sqlite.go `ListByCategories`/`CountByCategories` | 三库兼容 | 保持；多分类时 OR 扫描，量级小可接受 |
| `idx_categories_parent` 索引 | migrate.go | 三库兼容（CREATE INDEX） | 保持 |

## 验证方式（未来）
集成测试标签 `//go:build integration` + 各库连接串；当前无 Docker 环境，未实测。
