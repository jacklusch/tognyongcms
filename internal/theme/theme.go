// internal/theme/theme.go
package theme

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"dulizhan/internal/content"
	"dulizhan/internal/i18n"
	"dulizhan/internal/seo"
)

//go:embed base.html
var baseFS embed.FS

var baseHTML string

func init() {
	b, _ := baseFS.ReadFile("base.html")
	baseHTML = string(b)
}

type SiteInfo struct {
	Name        string
	URL         string
	Description string
}

// MenuItem 渲染就绪的导航项。
type MenuItem struct {
	Label    string     `json:"label"`
	URL      string     `json:"url"`
	Children []MenuItem `json:"children,omitempty"`
}

// Menu 一个具名菜单（如主导航 main / 页脚 footer）。
type Menu struct {
	Name  string     `json:"name"`
	Items []MenuItem `json:"items"`
}

// CategoryInfo 子分类导航项。
type CategoryInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	URL  string `json:"url"`
}

type Data struct {
	Site     SiteInfo
	Lang     string
	Langs    []i18n.Lang
	Entry    *content.Entry
	Items    []content.Entry
	Total    int
	Page     int
	PerPage  int
	TypeName string
	Meta     seo.Meta
	Menus    []Menu `json:"menus"`

	EntryCategory    string         `json:"entry_category"`
	SubCategories    []CategoryInfo `json:"sub_categories"`
	EntryCategoryURL string         `json:"entry_category_url"`
}

type Config struct {
	Name      string            `yaml:"name"`
	Version   string            `yaml:"version"`
	Templates map[string]string `yaml:"templates"`
	Partials  []string          `yaml:"partials"`
	TypeMap   map[string]string `yaml:"type_map"`
	Fields    []string          `yaml:"fields"`
	Locales   map[string]string `yaml:"locales"`
}

type Theme struct {
	Name      string
	Version   string
	Dir       string
	Templates map[string]string
	Partials  []string
	TypeMap   map[string]string
	Locales   map[string]map[string]string
	reg       *i18n.Registry
	funcs     template.FuncMap
	cache     map[string]*template.Template
	mu        sync.Mutex
	loadedAt  time.Time
}

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

func (l *Loader) Load(name string) (*Theme, error) {
	dir := filepath.Join(l.ThemesDir, name)
	data, err := os.ReadFile(filepath.Join(dir, "theme.yaml"))
	if err != nil {
		return nil, fmt.Errorf("读取主题 %q 配置: %w", name, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析主题 %q 配置: %w", name, err)
	}
	th := &Theme{
		Name:      name,
		Version:   cfg.Version,
		Dir:       dir,
		Templates: cfg.Templates,
		Partials:  cfg.Partials,
		TypeMap:   cfg.TypeMap,
		Locales:   map[string]map[string]string{},
		cache:     map[string]*template.Template{},
	}
	for lang, locPath := range cfg.Locales {
		loc := map[string]string{}
		raw, err := os.ReadFile(filepath.Join(dir, locPath))
		if err != nil {
			return nil, fmt.Errorf("读取主题语言包 %q: %w", lang, err)
		}
		if err := yaml.Unmarshal(raw, &loc); err != nil {
			return nil, fmt.Errorf("解析主题语言包 %q: %w", lang, err)
		}
		th.Locales[lang] = loc
	}
	th.reg = l.reg
	th.funcs = th.buildFuncs()
	th.loadedAt = time.Now()
	l.mu.Lock()
	l.cache[name] = th
	l.mu.Unlock()
	return th, nil
}

// TemplateFor 内容类型 → 模板 key，未映射回退 "single"。
func (t *Theme) TemplateFor(typeName string) string {
	if key, ok := t.TypeMap[typeName]; ok {
		return key
	}
	return "single"
}

func (t *Theme) LookupLocale(lang, key string) string {
	if m, ok := t.Locales[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	for _, m := range t.Locales {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}

func (t *Theme) Render(w io.Writer, key string, d *Data) error {
	tpl, err := t.renderSet(key)
	if err != nil {
		return err
	}
	return tpl.ExecuteTemplate(w, "base", d)
}

func (t *Theme) RenderNotFound(w io.Writer) error {
	_, err := io.WriteString(w, notFoundHTML)
	return err
}

func (t *Theme) renderSet(key string) (*template.Template, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if tpl, ok := t.cache[key]; ok {
		return tpl, nil
	}
	file, ok := t.Templates[key]
	if !ok {
		return nil, fmt.Errorf("主题 %q 未定义模板 %q", t.Name, key)
	}
	tpl := template.New(key).Funcs(t.funcs)
	var err error
	if tpl, err = tpl.Parse(baseHTML); err != nil {
		return nil, err
	}
	if tpl, err = tpl.ParseFiles(filepath.Join(t.Dir, file)); err != nil {
		return nil, err
	}
	for _, p := range t.Partials {
		if tpl, err = tpl.ParseFiles(filepath.Join(t.Dir, p)); err != nil {
			return nil, err
		}
	}
	t.cache[key] = tpl
	return tpl, nil
}

const notFoundHTML = `<!DOCTYPE html>
<html lang="zh"><head><meta charset="utf-8"><title>404 页面未找到</title></head>
<body><h1>404</h1><p>页面未找到</p></body></html>`
