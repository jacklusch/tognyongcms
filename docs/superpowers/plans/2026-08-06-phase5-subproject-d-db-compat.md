# 阶段 5 子项目 D：PG/MySQL 可移植性验证 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 审查现有 SQLite SQL 的 PG/MySQL 兼容差异，将不兼容写法（settings upsert）改造为跨库兼容，产出 `docs/database-compatibility.md` 差异文档。

**架构：** 审查 `internal/store/sqlite/sqlite.go` 全部 SQL → 改造 `settingRepo.Set` 为先查后写 → 写差异文档（驱动接入/DDL/占位符/upsert/事务）。

**技术栈：** Go（database/sql）、SQLite/PG/MySQL 兼容性知识。

**前置基线：** 阶段 4 已验收通过。规格：`docs/superpowers/specs/2026-08-06-phase5-subproject-d-db-compat-design.md`。

**环境约束：**
- **不 git 提交**；**勿运行 `go mod tidy`**；不改依赖（无新依赖）
- 保持 SQLite 测试全绿

**现状关键点：**
- `internal/store/sqlite/sqlite.go`：`settingRepo.Set` 用 `INSERT ... ON CONFLICT(key) DO UPDATE`（MySQL 不兼容）
- 其余 SQL：`?` 占位符、`LIMIT ? OFFSET ?`、RFC3339 文本日期、`UNIQUE(lang, content_type_id, slug)`——大多跨库兼容或文档化差异
- `migrate.go`：`id INTEGER PRIMARY KEY AUTOINCREMENT`（PG 用 SERIAL/IDENTITY）

---

### 任务 1：settings upsert 改造为跨库兼容

**文件：**
- 修改：`internal/store/sqlite/sqlite.go`（settingRepo.Set）
- 修改：`internal/store/sqlite/sqlite_test.go`（TestSettingUpsert 保持/微调）

- [ ] **步骤 1：确认现有测试覆盖**（`internal/store/sqlite/sqlite_test.go` 的 TestSettingUpsert 已存在：Set v1 → Set v2 → Get 返回 v2）。改造后该测试应仍通过（语义不变）。

- [ ] **步骤 2：改造 settingRepo.Set**

```go
func (r *settingRepo) Set(ctx context.Context, key, value string) error {
	// 跨库兼容（SQLite/PG/MySQL）：存在则 UPDATE，否则 INSERT。
	// 注：SQLite 单连接单写者下无并发覆盖；PG/MySQL 未来可升级按驱动分支的 upsert 语法。
	var exists int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings WHERE key = ?", key).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		_, err := r.db.ExecContext(ctx, "UPDATE settings SET value = ? WHERE key = ?", value, key)
		return err
	}
	_, err := r.db.ExecContext(ctx, "INSERT INTO settings (key, value) VALUES (?, ?)", key, value)
	return err
}
```

- [ ] **步骤 3：运行测试验证通过**

运行：`go test -count=1 ./internal/store/sqlite/ -run 'TestSettingUpsert|TestSettingRepo' -v`
预期：全部 PASS

- [ ] **步骤 4：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 2：SQL 可移植性审查

**文件：**
- 审查：`internal/store/sqlite/sqlite.go`、`internal/store/sqlite/migrate.go`（逐条 SQL 记录）

- [ ] **步骤 1：审查全部 SQL**（实现者逐条核对）

审查清单（对照规格 §2.1 表格）：
- `settingRepo.Set`：已改造（任务 1）
- `content` 表 `id INTEGER PRIMARY KEY AUTOINCREMENT`（migrate.go）——DDL 差异文档化
- `?` 占位符：SQLite/MySQL 用 `?`，PG 用 `$1`——文档化（代码层保持 `?`，PG 驱动需 `?`→`$n` 转换或逐库 SQL）
- `INSERT INTO settings ... ON CONFLICT`：已消除（任务 1）
- `LIMIT ? OFFSET ?`、`UNIQUE(...)`、`COUNT(*)`、`ORDER BY`、`LIKE`：跨库兼容
- 日期存储：RFC3339 文本（`time.RFC3339`）——三库兼容

- [ ] **步骤 2：记录审查结论**（供任务 3 文档引用）

每类 SQL 记录：位置、写法、SQLite/PG/MySQL 兼容性、处置（兼容/已改造/文档化）。

---

### 任务 3：差异文档 docs/database-compatibility.md

**文件：**
- 创建：`docs/database-compatibility.md`

- [ ] **步骤 1：写差异文档**

```markdown
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

## Upsert 差异
| 库 | 语法 |
|---|---|
| SQLite | `INSERT ... ON CONFLICT(key) DO UPDATE SET ...` |
| PostgreSQL | `INSERT ... ON CONFLICT (key) DO UPDATE SET ...` |
| MySQL | `INSERT ... ON DUPLICATE KEY UPDATE ...` |
- 当前实现（settings）：先查后写（三库兼容），避免语法差异；未来可按 driver 分支优化。

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

## 验证方式（未来）
集成测试标签 `//go:build integration` + 各库连接串；当前无 Docker 环境，未实测。
```

- [ ] **步骤 2：验证**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿（仅新增文档 + settings 改造）

---

## 自检记录

**1. 规格覆盖度：**
- 规格 §2 审查清单 + §2.2 改造 → 任务 1/2
- 规格 §3 差异文档 → 任务 3
- 规格 §4 测试 → 任务 1 步骤 3

**2. 占位符扫描：** 无 TBD/TODO。文档内容完整。

**3. 类型一致性：** `settingRepo.Set` 签名不变（`(ctx, key, value) error`），仅内部实现；`TestSettingUpsert` 语义不变。
