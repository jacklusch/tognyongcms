package i18n

import "testing"

func TestResolvePath(t *testing.T) {
	reg, err := New([]string{"zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path string
		lang string
		segs []string
	}{
		{"/", "zh", nil},
		{"/article/hello", "zh", []string{"article", "hello"}},
		{"/en/article/hello", "en", []string{"article", "hello"}},
		{"/en/", "en", nil},
		{"/fr/article/x", "zh", []string{"fr", "article", "x"}}, // 未知语言按默认处理
	}
	for _, c := range cases {
		lang, segs := reg.ResolvePath(c.path)
		if lang != c.lang {
			t.Errorf("%s lang = %q, want %q", c.path, lang, c.lang)
		}
		if len(segs) != len(c.segs) {
			t.Errorf("%s segs = %v, want %v", c.path, segs, c.segs)
		}
	}
}

func TestURLPath(t *testing.T) {
	reg, _ := New([]string{"zh", "en"}, "zh", false)
	if got := reg.URLPath("zh", "/article/x"); got != "/article/x" {
		t.Errorf("default no prefix, got %q", got)
	}
	if got := reg.URLPath("en", "/article/x"); got != "/en/article/x" {
		t.Errorf("prefixed, got %q", got)
	}
	if got := reg.URLPath("en", ""); got != "/en/" {
		t.Errorf("home prefixed, got %q", got)
	}
	reg2, _ := New([]string{"zh", "en"}, "zh", true)
	if got := reg2.URLPath("zh", "/article/x"); got != "/zh/article/x" {
		t.Errorf("prefix default on, got %q", got)
	}
}

func TestInvalidDefault(t *testing.T) {
	if _, err := New([]string{"zh"}, "en", false); err == nil {
		t.Error("默认语言不在语言列表中应报错")
	}
}

func TestResolvePathUnknownLangValue(t *testing.T) {
	reg, _ := New([]string{"zh", "en"}, "zh", false)
	lang, segs := reg.ResolvePath("/fr/article/x")
	if lang != "zh" {
		t.Errorf("未知语言 lang = %q, want zh", lang)
	}
	if len(segs) != 3 || segs[0] != "fr" {
		t.Errorf("未知语言 segs = %v", segs)
	}
}

func TestNewDedupe(t *testing.T) {
	reg, err := New([]string{"zh", "zh", "en"}, "zh", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reg.All()); got != 2 {
		t.Errorf("去重后语言数 = %d, want 2", got)
	}
}
