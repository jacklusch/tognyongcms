package translate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranslateText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"Hello world"}}]}`))
	}))
	defer srv.Close()

	s := New("sk-test", srv.URL, "gpt-4o-mini")
	got, err := s.TranslateText(context.Background(), "你好世界", "zh", "en")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hello world" {
		t.Errorf("got %q", got)
	}
}

func TestTranslateTextError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()

	s := New("sk", srv.URL, "m")
	if _, err := s.TranslateText(context.Background(), "x", "zh", "en"); err == nil {
		t.Error("期望错误")
	}
}

func TestTranslateRichText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"translated"}}]}`))
	}))
	defer srv.Close()

	s := New("sk", srv.URL, "m")
	html := `<p>中文段落一</p><p><img src="/a.png">中文段落二</p>`
	got, err := s.TranslateRichText(context.Background(), html, "zh", "en")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `src="/a.png"`) {
		t.Errorf("图片标签丢失: %s", got)
	}
	if !strings.Contains(got, "translated") {
		t.Errorf("翻译文本缺失: %s", got)
	}
}

// 发现 2：块级容器内联元素（strong/em/u/a/span/b/i）内的文本也应翻译，标签与属性保留。
func TestTranslateRichTextInlineElements(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"translated"}}]}`))
	}))
	defer srv.Close()

	s := New("sk", srv.URL, "m")
	html := `<p><strong>中文加粗</strong><em>斜体</em><a href="/link">链接文字</a></p>`
	got, err := s.TranslateRichText(context.Background(), html, "zh", "en")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<strong>translated</strong>") {
		t.Errorf("strong 内文本未翻译: %s", got)
	}
	if !strings.Contains(got, "<em>translated</em>") {
		t.Errorf("em 内文本未翻译: %s", got)
	}
	if !strings.Contains(got, `<a href="/link">translated</a>`) {
		t.Errorf("a 标签属性或内部文本丢失: %s", got)
	}
}
