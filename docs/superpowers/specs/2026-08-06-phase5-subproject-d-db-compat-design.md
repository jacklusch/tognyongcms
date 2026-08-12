# Dulizhan CMS — 阶段 5 子项目 D：PG/MySQL 可移植性验证 设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`（§3 数据模型）、`2026-08-06-phase4-content-type-builder-design.md`
- 基线：阶段 4 已验收通过；SQLite 存储层（migrate.go + sqlite.go）

## 1. 目标与范围

审查现有 SQLite SQL 的 PG/MySQL 兼容差异，将不兼容写法改造为跨库兼容（保持 SQLite 测试绿），产出差异文档。

**包含**：SQL 可移植性审查、settings upsert 等不兼容 SQL 改造、`docs/database-compatibility.md` 差异文档。
**排除**：实现 PG/MySQL 驱动（本阶段是"验证"非"实现"）、Docker 集成测试（环境无 Docker 实测，文档说明验证方式）。

## 2. SQL 可移植性审查

### 2.1 审查清单（`internal/store/sqlite/sqlite.go` 全部 SQL）

| SQL 用法 | SQLite | PG | MySQL | 处置 |
|---|---|---|---|---|
| `INSERT INTO settings ... ON CONFLICT(key) DO UPDATE` | ✓ | ✓(9.5+) | ✗ | **改造**为先查后写 |
| `INSERT OR REPLACE` / `REPLACE INTO` | ✓ | ✗ | ✓(REPLACE) | 无（当前未用，文档记录） |
| `id INTEGER PRIMARY KEY AUTOINCREMENT` | ✓ | ✗(SERIAL/IDENTITY) | ✓(AUTO_INCREMENT) | 文档化 DDL 差异 |
| `?` 占位符 | ✓ | ✗(`$1`) | ✓ | 文档化（驱动差异） |
| `LIMIT ? OFFSET ?` | ✓ | ✓ | ✓ | 兼容 |
| `RETURNING` | ✗(旧版) | ✓ | ✓ | 当前未用，文档记录 |
| RFC3339 文本日期存储 | ✓ | ✓ | ✓ | 兼容 |
| `UNIQUE(lang, content_type_id, slug)` | ✓ | ✓ | ✓ | 兼容（索引/约束） |

### 2.2 改造点：settings upsert（`internal/store/sqlite/sqlite.go`）

`settingRepo.Set` 由 `INSERT ... ON CONFLICT(key) DO UPDATE` 改为跨库兼容的先查后写：

```go
func (r *settingRepo) Set(ctx context.Context, key, value string) error {
	// 三库兼容：存在则 UPDATE，否则 INSERT
	var exists int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings WHERE key = ?", key).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		_, err = r.db.ExecContext(ctx, "UPDATE settings SET value = ? WHERE key = ?", value, key)
	} else {
		_, err = r.db.ExecContext(ctx, "INSERT INTO settings (key, value) VALUES (?, ?)", key, value)
	}
	return err
}
```
> 竞态说明：单连接 `SetMaxOpenConns(1)` + 单写者下先查后写无并发覆盖问题（SQLite 场景）；PG/MySQL 未来可升级为 upsert 语法按驱动分支。

补测试：`TestSettingUpsert` 保持（先 Set 再 Set 覆盖 → Get 返回新值）。

## 3. 差异文档（`docs/database-compatibility.md`）

内容：
- **驱动接入**：`database/sql` + 驱动（PG：`lib/pq` 或 `pgx`；MySQL：`go-sql-driver/mysql`）。DSN 示例与连接串。
- **DDL 差异对照**：自增列（SQLite AUTOINCREMENT / PG GENERATED ALWAYS AS IDENTITY / MySQL AUTO_INCREMENT）；JSON 列（SQLite/MySQL 存 TEXT、PG 可 jsonb）；时间戳列。
- **占位符差异**：SQLite/MySQL `?`、PG `$1`（代码层用 `?` 需 driver 转换或逐库 SQL）。
- **upsert 差异**：SQLite `ON CONFLICT`、PG `ON CONFLICT DO UPDATE`、MySQL `ON DUPLICATE KEY UPDATE`（当前实现改先查后写避免差异）。
- **事务/锁**：SQLite 单写者、PG MVCC、MySQL InnoDB 行锁；隔离级别差异。
- **现有 SQL 审查结论**：逐条记录审查结果 + 改造记录。
- **验证方式**：各库集成测试连接串与 `go test` 标签（如 `//go:build integration`），留待未来环境具备时启用。

## 4. 测试

- 现有 SQLite 全量测试保持绿（settings upsert 改造后 `TestSettingUpsert`/`TestSettingRepo` 通过）
- 无 PG/MySQL 集成测试（文档说明连接串与标签）

## 5. 验收

- `go test -count=1 ./...` 全绿、build/vet/gofmt 干净
- `docs/database-compatibility.md` 完整覆盖审查清单

## 6. 工作流约定

- 不 git 提交；superpowers SDD 驱动
