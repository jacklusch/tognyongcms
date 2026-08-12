# 阶段 5 子项目 C：主题热重载 + slog 日志 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** config `server.debug: true` 时主题热重载（mtime 检测）；main + server 中间件改用 `log/slog`（debug 时 LevelDEBUG）。

**架构：** config 加 `ServerConfig.Debug`；`theme.Loader` 加 debug 字段 + mtime 检测热重载（`Get` 时检查）；main 建 slog.Logger；`server.New` 替换默认 gin.Logger/Recovery 为自定义 slog 中间件。

**技术栈：** Go（gin、log/slog）。

**前置基线：** 阶段 4 已验收通过。规格：`docs/superpowers/specs/2026-08-06-phase5-subproject-c-theme-reload-slog-design.md`。

**环境约束：**
- **不 git 提交**；**勿运行 `go mod tidy`**
- 错误消息用中文；依赖锁定勿动

**现状关键点：**
- `internal/config/config.go`：`ServerConfig{Addr, DataDir}`（无 Debug）
- `internal/theme/theme.go`：`Loader{ThemesDir, reg, cache, mu}`、`NewLoader(themesDir, reg)`、`Theme{...}` 无 loadedAt
- `internal/server/server.go`：`New` 用 `gin.New()` + `eng.Use(gin.Logger(), gin.Recovery())`；Server 无 log 字段
- `cmd/dulizhan/main.go`：`log.Fatalf`/`log.Printf`

---

### 任务 1：config server.debug + Loader 热重载

**文件：**
- 修改：`internal/config/config.go`（ServerConfig.Debug）
- 修改：`internal/theme/theme.go`（Loader.debug + loadedAt + Get mtime 检测）
- 修改：`internal/theme/theme_test.go`（热重载测试）
- 修改：`internal/config/config_test.go`（Debug 测试）

- [ ] **步骤 1：编写失败测试（config）**（追加到 `internal/config/config_test.go`）

```go
func TestServerDebugDefault(t *testing.T) {
	cfg := &Config{}
	cfg.Site.URL = "https://example.com"
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Languages = []string{"zh"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Debug {
		t.Error("Debug 默认应为 false")
	}
}
```

- [ ] **步骤 2：编写失败测试（theme 热重载）**（追加到 `internal/theme/theme_test.go`）

```go
func TestLoaderDebugReload(t *testing.T) {
	dir := writeFixtureTheme(t) // 已存在 helper，写入 dir/fixture
	reg, _ := i18n.New([]string{"zh", "en"}, "zh", false)
	loader := NewLoader(dir, reg, true) // debug 模式

	th, err := loader.Get("fixture")
	if err != nil {
		t.Fatal(err)
	}
	// 首次加载内容
	// 修改 theme.yaml 的 mtime
	oldPath := filepath.Join(dir, "fixture", "theme.yaml")
	// 改一个模板文件 mtime，验证 Get 重新加载
	singlePath := filepath.Join(dir, "fixture", "templates", "single.html")
	if err := os.Chtimes(singlePath, time.Now(), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	th2, err := loader.Get("fixture")
	if err != nil {
		t.Fatal(err)
	}
	if th2 == th {
		t.Error("debug 模式 mtime 变更后应重新加载新 Theme 实例")
	}
}

func TestLoaderNonDebugCache(t *testing.T) {
	dir := writeFixtureTheme(t)
	reg, _ := i18n.New([]string{"zh", "en"}, "zh", false)
	loader := NewLoader(dir, reg, false) // 非 debug
	th1, _ := loader.Get("fixture")
	th2, _ := loader.Get("fixture")
	if th1 != th2 {
		t.Error("非 debug 模式应返回缓存同一实例")
	}
}
```
> `writeFixtureTheme` 返回 dir（写入 dir/fixture/...）；`time` 需 import。

- [ ] **步骤 3：运行确认失败**

运行：`go test -count=1 ./internal/theme/ -run 'TestLoaderDebugReload|TestLoaderNonDebugCache' -v`
预期：FAIL（`NewLoader` 不接受 debug 参数）

- [ ] **步骤 4：config Debug**（`internal/config/config.go`）

```go
type ServerConfig struct {
	Addr    string `yaml:"addr"`
	DataDir string `yaml:"data_dir"`
	Debug   bool   `yaml:"debug"`
}
```
`applyEnv` 追加：
```go
	if v, ok := os.LookupEnv("DULIZHAN_SERVER_DEBUG"); ok && v == "true" {
		cfg.Server.Debug = true
	}
```

- [ ] **步骤 5：Loader 热重载**（`internal/theme/theme.go`）

```go
import "time"

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

// latestModTime 返回主题目录下 theme.yaml 与 templates/ 下文件的最大 ModTime。
func latestModTime(dir string) (time.Time, error) {
	var latest time.Time
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest, err
}
```

`Theme` 加字段 + Load 时设置：
```go
type Theme struct {
	// ...既有
	loadedAt time.Time
}
// Load 中 th 构造后：th.loadedAt = time.Now()
```
> 注意：`latestModTime` 用 `filepath.Walk`（需 import `os`、`time`）。`th.loadedAt` 在 Load 的 `th` 构造时设置（在 `l.cache[name] = th` 之前）。

- [ ] **步骤 6：运行测试验证通过**

运行：`go test -count=1 ./internal/theme/ ./internal/config/ -v`
预期：全部 PASS（含热重载测试）

- [ ] **步骤 7：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿（既有 NewLoader 调用点兼容 variadic）

---

### 任务 2：main.go slog 化

**文件：**
- 修改：`cmd/dulizhan/main.go`（slog 建 logger + 替换 log）
- 测试：无新增（行为验证 build/vet）

- [ ] **步骤 1：main.go slog 改造**

```go
import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	// ...既有
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// ...既有 config load / db open / i18n / seed 分支（保持 log.Fatalf 或换 slog）

	// 正常启动路径
	level := slog.LevelInfo
	if cfg.Server.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	// 启动日志：log.Printf → slog.Info
	slog.Info("Dulizhan CMS 启动", "addr", cfg.Server.Addr, "site", cfg.Site.Name, "theme", cfg.Site.Theme)
	// 错误：log.Fatalf → slog.Error + os.Exit(1)
}
```
> **实现细节**：`log.Fatalf` 调用点（加载配置失败、打开数据库失败等）在 seed 分支与启动路径前——logger 需在最早可用处建（config load 后）。**裁定**：config load 成功后即建 logger，后续 `log.Fatalf` 统一换 `slog.Error + os.Exit(1)`；或保留 `log` 用于启动前错误（更简单）。**实现时**：启动路径的错误用 slog.Error+os.Exit；启动前的 fatal（config load/db open 失败）因 logger 未建，保留 `log.Fatalf` 或用 `fmt.Fprintf(os.Stderr)+os.Exit`。**以最小改动为准**：保留 log 包用于早期 fatal，slog 用于正常启动日志与 server 中间件。

- [ ] **步骤 2：验证**

运行：`go build ./...`；`go vet ./...`；`go test -count=1 ./...`
预期：全绿（无行为变化，仅日志格式）

---

### 任务 3：server 自定义 slog 中间件

**文件：**
- 修改：`internal/server/server.go`（log 字段 + 自定义中间件）
- 修改：`internal/server/frontend_test.go`（适配 server.New 若签名变化——用现有 7 参签名不变化）

- [ ] **步骤 1：Server 加 log 字段 + 自定义中间件**（`internal/server/server.go`）

```go
import (
	// ...既有
	"log/slog"
)

type Server struct {
	// ...既有
	log *slog.Logger
}

func New(cfg *config.Config, st store.Store, svc *content.Service, reg *i18n.Registry, loader *theme.Loader, authSvc *auth.Service, medStore media.MediaStore) (*Server, error) {
	// ...既有
	gin.SetMode(gin.ReleaseMode)
	eng := gin.New()
	eng.Use(s.accessLog(cfg.Server.Debug), s.recovery())
	s := &Server{ /* ...既有 */ }
	// log 字段在 s 构造后赋值——但中间件需 s.log；**调整**：先建 logger 再建 engine
}
```
> **实现调整**：`New` 中先构造 `s`（含 log），再 `eng.Use`。重构：
```go
func New(...) (*Server, error) {
	// ...cfg defaults, gin mode
	level := slog.LevelInfo
	if cfg.Server.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	s := &Server{
		engine: nil, cfg: cfg, store: st, content: svc, i18n: reg,
		seo: seo.NewBuilder(...), themes: loader, auth: authSvc, media: medStore,
		log: logger,
	}
	eng := gin.New()
	eng.Use(s.accessLog(), s.recovery())
	s.engine = eng
	s.registerRoutes()
	return s, nil
}

// accessLog 访问日志中间件。
func (s *Server) accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		s.log.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
			"ip", c.ClientIP(),
		)
	}
}

// recovery 恢复中间件（记录 panic 到 slog）。
func (s *Server) recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("panic", "err", r, "path", c.Request.URL.Path)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
```
> 需 import `time`、`log/slog`。`time.Now()` 需 `time`（server.go 当前无 time import——检查）。gin 的 `c.Writer.Status()` 在 handler 后可用。

- [ ] **步骤 2：验证**

运行：`go build ./...`；`go vet ./...`；`go test -count=1 ./...`
预期：全绿（中间件行为等价，日志走 slog）

- [ ] **步骤 3：冒烟**

- 启动：日志输出含 `msg=http method=... path=... status=...`
- `server.debug: true`：日志 LevelDEBUG；改主题模板 → 刷新前台页面反映新模板

---

## 自检记录

**1. 规格覆盖度：**
- 规格 §2 config debug → 任务 1 步骤 4
- 规格 §3 Loader 热重载 → 任务 1 步骤 5
- 规格 §4 slog（main + server）→ 任务 2/3
- 规格 §5 测试 → 任务 1（theme/config）+ 任务 3 冒烟

**2. 占位符扫描：** 无 TBD/TODO。每步含代码。任务 2 的"实现时"标注（早期 fatal 处理）是明确决策提示。

**3. 类型一致性：**
- `NewLoader(themesDir, reg, debug ...bool)` 任务 1 定义，既有调用点（server/frontend_test）兼容
- `Loader.debug`/`Theme.loadedAt` 任务 1 定义，Get/Load 使用
- `Server.log *slog.Logger` 任务 3 定义，accessLog/recovery 使用
- `ServerConfig.Debug` 任务 1 定义，main/server 使用
