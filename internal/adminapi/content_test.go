package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"dulizhan/internal/config"
)

// fakeTranslator 固定返回翻译文本，便于 adminapi 注入测试自动翻译。
type fakeTranslator struct{}

func (f *fakeTranslator) TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	return "EN:" + text, nil
}

func (f *fakeTranslator) TranslateRichText(ctx context.Context, htmlStr, sourceLang, targetLang string) (string, error) {
	return "<p>EN</p>", nil
}

// failingTranslator 翻译始终失败，用于触发 fallback 降级路径。
type failingTranslator struct{}

func (f *failingTranslator) TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	return "", errors.New("翻译接口不可用")
}

func (f *failingTranslator) TranslateRichText(ctx context.Context, htmlStr, sourceLang, targetLang string) (string, error) {
	return "", errors.New("翻译接口不可用")
}

func TestContentCreateAutoTranslate(t *testing.T) {
	e := newEnv(t)
	// 注入翻译器（enabled）
	tr := &fakeTranslator{}
	cfg := &config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	e.svc.SetTranslator(tr, cfg)
	tok := e.login(t)

	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"自动翻译测试","slug":"auto-1","content":"<p>正文</p>","published_on":"2026-01-15","category":"1"}}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Content struct {
				ContentID string `json:"content_id"`
			} `json:"content"`
			AutoTranslate struct {
				Triggered bool   `json:"triggered"`
				Created   bool   `json:"created"`
				Status    string `json:"status"`
			} `json:"auto_translate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Data.AutoTranslate.Triggered || !resp.Data.AutoTranslate.Created || resp.Data.AutoTranslate.Status != "translated" {
		t.Errorf("auto_translate = %+v", resp.Data.AutoTranslate)
	}
}

func TestContentListByCategory(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// seed products 分类 id 从 /api/categories 拿
	w := e.do(t, http.MethodGet, "/api/categories", "", tok)
	var cl struct {
		Data struct {
			All []struct {
				ID   int64  `json:"id"`
				Path string `json:"path"`
			} `json:"all"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &cl)
	var productsID int64
	for _, a := range cl.Data.All {
		if a.Path == "产品" {
			productsID = a.ID
		}
	}
	if productsID == 0 {
		t.Fatal("未找到产品分类")
	}
	// 按分类筛选 zh
	w = e.do(t, http.MethodGet, "/api/content?type=article&lang=zh&category="+strconv.FormatInt(productsID, 10), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("content by category = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Items []struct {
				TypeName string `json:"type_name"`
				Fields   map[string]any
			} `json:"items"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Total != 2 {
		t.Errorf("products zh total = %d, want 2", resp.Data.Total)
	}
}

// helper：解析创建响应的 zh id。
func createdContentID(t *testing.T, w *httptest.ResponseRecorder) int64 {
	t.Helper()
	var resp struct {
		Data struct {
			Content struct {
				Content struct {
					ID int64 `json:"id"`
				} `json:"content"`
			} `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Content.Content.ID == 0 {
		t.Fatal("创建未回填 id")
	}
	return resp.Data.Content.Content.ID
}

// helper：查某翻译组中指定语言行的状态。
func translationStatus(t *testing.T, e *env, tok string, contentID int64, lang string) string {
	t.Helper()
	w := e.do(t, http.MethodGet, fmt.Sprintf("/api/content/%d/translations", contentID), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("translations = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Items []struct {
				Content struct {
					Lang   string `json:"lang"`
					Status string `json:"status"`
				} `json:"content"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for _, it := range resp.Data.Items {
		if it.Content.Lang == lang {
			return it.Content.Status
		}
	}
	t.Fatalf("翻译组 %d 中无 %s 语言行", contentID, lang)
	return ""
}

// helper：查某翻译组中指定语言行的 fields。
func translationFields(t *testing.T, e *env, tok string, contentID int64, lang string) map[string]any {
	t.Helper()
	w := e.do(t, http.MethodGet, fmt.Sprintf("/api/content/%d/translations", contentID), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("translations = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Items []struct {
				Content struct {
					Lang string `json:"lang"`
				} `json:"content"`
				Fields map[string]any `json:"fields"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for _, it := range resp.Data.Items {
		if it.Content.Lang == lang {
			return it.Fields
		}
	}
	t.Fatalf("翻译组 %d 中无 %s 语言行", contentID, lang)
	return nil
}

// 发现 1：发布 zh 后同组 en 状态同步 published。
func TestPublishSyncsTranslationStatus(t *testing.T) {
	e := newEnv(t)
	e.cfg.Translate = config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	e.svc.SetTranslator(&fakeTranslator{}, &e.cfg.Translate)
	tok := e.login(t)

	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"同步发布","slug":"sync-pub","content":"<p>正文</p>","published_on":"2026-01-15","category":"1"}}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	zhID := createdContentID(t, w)
	// 发布前 en 跟随 zh（draft）
	if s := translationStatus(t, e, tok, zhID, "en"); s != "draft" {
		t.Fatalf("发布前 en status = %q, want draft", s)
	}
	// 发布 zh
	w = e.do(t, http.MethodPost, fmt.Sprintf("/api/content/%d/publish", zhID), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}
	if s := translationStatus(t, e, tok, zhID, "en"); s != "published" {
		t.Errorf("发布 zh 后 en status = %q, want published", s)
	}
}

// 发现 1：撤回 zh 后同组 en 状态同步 draft。
func TestUnpublishSyncsTranslationStatus(t *testing.T) {
	e := newEnv(t)
	e.cfg.Translate = config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	e.svc.SetTranslator(&fakeTranslator{}, &e.cfg.Translate)
	tok := e.login(t)

	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"同步撤回","slug":"sync-unpub","content":"<p>正文</p>","published_on":"2026-01-15","category":"1"}}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	zhID := createdContentID(t, w)
	// 先发布（en 跟随 published）
	w = e.do(t, http.MethodPost, fmt.Sprintf("/api/content/%d/publish", zhID), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}
	if s := translationStatus(t, e, tok, zhID, "en"); s != "published" {
		t.Fatalf("发布后 en status = %q, want published", s)
	}
	// 撤回 zh
	w = e.do(t, http.MethodPost, fmt.Sprintf("/api/content/%d/unpublish", zhID), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("unpublish = %d %s", w.Code, w.Body.String())
	}
	if s := translationStatus(t, e, tok, zhID, "en"); s != "draft" {
		t.Errorf("撤回 zh 后 en status = %q, want draft", s)
	}
}

// 发现 1：translate 未启用时 publish 不影响 en（行为不变）。
func TestPublishNoSyncWhenTranslateDisabled(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)

	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"禁用同步","slug":"no-sync","content":"<p>正文</p>","published_on":"2026-01-15","category":"1"}}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	zhID := createdContentID(t, w)
	// 未启用翻译：无 en 行，publish 应正常返回且不报错
	w = e.do(t, http.MethodPost, fmt.Sprintf("/api/content/%d/publish", zhID), "", tok)
	if w.Code != http.StatusOK {
		t.Errorf("publish（翻译关闭）= %d %s", w.Code, w.Body.String())
	}
}

// 最终修复：fallback 降级草稿（复制 zh 原文）带内部标记，发布 zh 时不被同步转正。
func TestPublishSkipsFallbackTranslationStatus(t *testing.T) {
	e := newEnv(t)
	e.cfg.Translate = config.TranslateConfig{Enabled: true, SourceLang: "zh", TargetLang: "en"}
	e.svc.SetTranslator(&failingTranslator{}, &e.cfg.Translate)
	tok := e.login(t)

	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"降级草稿","slug":"fallback-draft","content":"<p>原文</p>","published_on":"2026-01-15","category":"1"}}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	zhID := createdContentID(t, w)
	// fallback en 已生成且带内部标记
	if fb, ok := translationFields(t, e, tok, zhID, "en")["_auto_translate_fallback"].(bool); !ok || !fb {
		t.Errorf("fallback en 应带 _auto_translate_fallback 标记, fields = %v", translationFields(t, e, tok, zhID, "en"))
	}
	if s := translationStatus(t, e, tok, zhID, "en"); s != "draft" {
		t.Fatalf("fallback en status = %q, want draft", s)
	}
	// 发布 zh：fallback en 应保持 draft（不被同步转正）
	w = e.do(t, http.MethodPost, fmt.Sprintf("/api/content/%d/publish", zhID), "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}
	if s := translationStatus(t, e, tok, zhID, "en"); s != "draft" {
		t.Errorf("发布 zh 后 fallback en status = %q, want draft（不随 zh 转正）", s)
	}
}
