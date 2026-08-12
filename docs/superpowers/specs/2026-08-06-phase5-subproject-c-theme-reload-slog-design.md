# Dulizhan CMS — 阶段 5 子项目 C：主题热重载 + slog 日志 设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`（§4 主题系统、§9 错误处理/日志）、`2026-08-06-phase4-content-type-builder-design.md`
- 基线：阶段 4 已验收通过；主题系统（Loader/Theme 缓存）、gin 默认 Logger/Recovery 中间件

## 1. 目标与范围

- 主题热重载：`config server.debug: true` 时主题模板/资源变更无需重启即生效
- slog 日志：main + server 中间件改用 `log/slog`（替换 `log`），debug 时 LevelDEBUG

**包含**：config `server.debug`、Loader 热重载（mtime 检测）、main 建 slog、server 自定义 slog 中间件。
**排除**：全仓 slog 化（仅 main + server 中间件）、热重载触发式推送（无 WebSocket，请求时检测）。

## 2. config 扩展

`ServerConfig` 加 `Debug bool`（yaml:"debug"，env `DULIZHAN_SERVER_DEBUG`，默认 false）。`Validate()` 无新增校验（debug 非必需）。

## 3. 主题热重载（`internal/theme/theme.go` + `internal/config/config.go`）

### 3.1 Loader 加 debug + mtime 检测

```go
type Loader struct {
	ThemesDir string
	reg       *i18n.Registry
	cache     map[string]*Theme
	mu        sync.Mutex
	debug     bool
}

func NewLoader(themesDir string, reg *i18n.Registry, debug ...bool) *Loader {
	d := false
	if len(debug) > 0 {
		d = debug[0]
	}
	return &Loader{ThemesDir: themesDir, reg: reg, cache: map[string]*Theme{}, debug: d}
}

func (l *Loader) Get(name string) (*Theme, error) {
	if !l.debug {
		l.mu.Lock()
		th, ok := l.cache[name]
		l.mu.Unlock()
		if ok {
			return th, nil
		}
		return l.Load(name)
	}
	// debug 热重载：mtime 变更则重载
	l.mu.Lock()
	th, ok := l.cache[name]
	l.mu.Unlock()
	if ok {
		mod, err := latestModTime(filepath.Join(l.ThemesDir, name))
		if err != nil || !mod.After(th.loadedAt) {
			return th, nil
		}
	}
	return l.Load(name)
}

// latestModTime 返回主题目录下 theme.yaml + templates 的最新 mtime。
func latestModTime(dir string) (time.Time, error) {
	// filepath.Walk 找 theme.yaml 与 templates/ 下文件的最大 ModTime
}
```

`Theme` 加 `loadedAt time.Time`（Load 时 `time.Now()`）。

### 3.2 装配

`server.New` 传 `cfg.Server.Debug`：`theme.NewLoader(cfg.Site.ThemesDir, reg, cfg.Server.Debug)`。现有 `NewLoader(themesDir, reg)` 调用点（server/frontend_test）兼容（variadic bool）。

## 4. slog 日志

### 4.1 main.go 建 logger（`cmd/dulizhan/main.go`）

```go
level := slog.LevelInfo
if cfg.Server.Debug {
	level = slog.LevelDebug
}
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
slog.SetDefault(logger)
```
- `log.Fatalf(...)` → `slog.Error(...)` + `os.Exit(1)`（或 `log` 保留为 fallback，main 内统一）
- 启动日志 `slog.Info("Dulizhan CMS 启动", "addr", cfg.Server.Addr, "site", cfg.Site.Name, "theme", cfg.Site.Theme)`

### 4.2 server 自定义中间件（`internal/server/server.go`）

- `Server` 加 `log *slog.Logger` 字段
- `gin.New()` 后不用默认 `gin.Logger()`/`gin.Recovery()`，改为自定义：
  - 访问日志中间件：`logger.Info("http", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "latency", time.Since(start), "ip", c.ClientIP())`
  - Recovery：`defer func(){ if r := recover(); r != nil { logger.Error("panic", "err", r, "path", c.Request.URL.Path); c.AbortWithStatus(500) } }()`

## 5. 测试

- `theme_test.go`：debug 模式 Get 检测 mtime（改 theme.yaml 后 Get 返回新内容）、非 debug 走缓存
- `config_test.go`：Debug 默认 false、env 覆盖
- 全量回归：`go test -count=1 ./...` 全绿（server 中间件改造后既有测试适配）

## 6. 验收

- `go test -count=1 ./...`、`go build ./...`、`go vet ./...`、`gofmt -l` 全绿干净
- 冒烟：`server.debug: true` 启动，改 themes/default 模板 → 刷新前台页面反映新模板；日志输出含 http 访问行

## 7. 工作流约定

- 不 git 提交；superpowers SDD 驱动
