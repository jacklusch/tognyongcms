package i18n

import (
	"fmt"
	"strings"
)

type Lang struct {
	Code  string
	Label string
}

type Registry struct {
	langs         []Lang
	def           string
	prefixDefault bool
}

func New(codes []string, def string, prefixDefault bool) (*Registry, error) {
	if len(codes) == 0 {
		return nil, fmt.Errorf("语言列表不能为空")
	}
	seen := map[string]bool{}
	langs := make([]Lang, 0, len(codes))
	for _, c := range codes {
		if seen[c] {
			continue
		}
		seen[c] = true
		langs = append(langs, Lang{Code: c, Label: labelOf(c)})
	}
	found := false
	for _, l := range langs {
		if l.Code == def {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("默认语言 %q 不在语言列表中", def)
	}
	return &Registry{langs: langs, def: def, prefixDefault: prefixDefault}, nil
}

func labelOf(code string) string {
	switch code {
	case "zh":
		return "中文"
	case "en":
		return "English"
	case "ja":
		return "日本語"
	case "fr":
		return "Français"
	case "de":
		return "Deutsch"
	default:
		return code
	}
}

func (r *Registry) Default() string { return r.def }

// All 返回语言列表副本，防止调用方修改内部切片。
func (r *Registry) All() []Lang {
	out := make([]Lang, len(r.langs))
	copy(out, r.langs)
	return out
}

func (r *Registry) IsValid(code string) bool {
	for _, l := range r.langs {
		if l.Code == code {
			return true
		}
	}
	return false
}

// ResolvePath 解析请求路径，返回 (语言, 去掉语言前缀后的路径段)。
func (r *Registry) ResolvePath(path string) (string, []string) {
	trimmed := strings.Trim(path, "/")
	var segs []string
	if trimmed != "" {
		segs = strings.Split(trimmed, "/")
	}
	lang := r.def
	if len(segs) > 0 && r.IsValid(segs[0]) && (r.prefixDefault || segs[0] != r.def) {
		lang = segs[0]
		segs = segs[1:]
	}
	return lang, segs
}

// URLPath 生成某语言下的路径（rest 以 / 开头或空串）。
func (r *Registry) URLPath(lang, rest string) string {
	if !r.IsValid(lang) {
		lang = r.def
	}
	if lang == r.def && !r.prefixDefault {
		if rest == "" {
			return "/"
		}
		return rest
	}
	if rest == "" {
		return "/" + lang + "/"
	}
	return "/" + lang + rest
}
