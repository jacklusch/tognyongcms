// internal/theme/funcs.go
package theme

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"dulizhan/internal/content"
)

func (t *Theme) buildFuncs() template.FuncMap {
	return template.FuncMap{
		"t":         func(lang, key string) string { return t.LookupLocale(lang, key) },
		"url":       func(lang string, e content.Entry) string { return t.urlPath(lang, "/"+e.TypeName+"/"+e.Content.Slug) },
		"typeURL":   func(lang, typeName string) string { return t.urlPath(lang, "/"+typeName) },
		"home":      func(lang string) string { return t.urlPath(lang, "/") },
		"pagerURL":  func(d *Data, page int) string { return t.pagerURL(d, page) },
		"menu":      menuData,
		"firstMenu": firstMenu,
		"asset":     func(p string) string { return t.assetURL(p) },
		"entryCategoryURL": func(lang string, e content.Entry) string {
			if t.entryCatURL != nil {
				if u := t.entryCatURL(lang, e); u != "" {
					return u
				}
			}
			return t.urlPath(lang, "/"+e.TypeName+"/"+e.Content.Slug)
		},
		"media": mediaFunc,
		"raw":   func(v any) template.HTML { return template.HTML(fmt.Sprint(v)) },
		"pages": pagesFunc,
		// socialIcon 由链接识别社交平台标识（tiktok/youtube/facebook/x/instagram），供页脚图标选择。
		"socialIcon": socialPlatform,
	}
}

// assetURL 生成主题静态资源 URL，并附加文件 mtime 作为版本号，
// 资源变更后 URL 随之变化，浏览器立即重新拉取，避免旧缓存。
func (t *Theme) assetURL(p string) string {
	url := "/themes/" + t.Name + p
	full := filepath.Join(t.Dir, "static", filepath.FromSlash(p))
	if fi, err := os.Stat(full); err == nil {
		url += fmt.Sprintf("?v=%d", fi.ModTime().Unix())
	}
	return url
}

// menuData 按菜单名返回渲染就绪导航项（d 是模板根 Data）。
func menuData(d *Data, name string) []MenuItem {
	for _, m := range d.Menus {
		if m.Name == name {
			return m.Items
		}
	}
	return nil
}

// firstMenu 返回第一个有导航项的菜单（任意名字均可，供主导航渲染）。
func firstMenu(d *Data) []MenuItem {
	for _, m := range d.Menus {
		if len(m.Items) > 0 {
			return m.Items
		}
	}
	return nil
}

func (t *Theme) pagerURL(d *Data, page int) string {
	path := "/" + d.TypeName
	if page > 1 {
		path = fmt.Sprintf("/%s/page/%d", d.TypeName, page)
	}
	return t.urlPath(d.Lang, path)
}

func (t *Theme) urlPath(lang, rest string) string {
	if t.reg == nil {
		return rest
	}
	return t.reg.URLPath(lang, rest)
}

func mediaFunc(v any) string {
	switch s := v.(type) {
	case string:
		if s != "" && (strings.HasPrefix(s, "http") || strings.HasPrefix(s, "/")) {
			return s
		}
	}
	return ""
}

func pagesFunc(total, perPage int) []int {
	if perPage < 1 {
		perPage = 10
	}
	n := (total + perPage - 1) / perPage
	if n < 1 {
		n = 1
	}
	out := make([]int, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, i)
	}
	return out
}
