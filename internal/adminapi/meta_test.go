package adminapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestMetaEndpoint(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	w := e.do(t, http.MethodGet, "/api/meta", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("meta = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			SiteName    string   `json:"site_name"`
			Languages   []string `json:"languages"`
			DefaultLang string   `json:"default_lang"`
			Theme       string   `json:"theme"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.DefaultLang != "zh" {
		t.Errorf("default_lang = %q, want zh", resp.Data.DefaultLang)
	}
	if len(resp.Data.Languages) != 2 {
		t.Errorf("languages = %v", resp.Data.Languages)
	}
	if resp.Data.Theme == "" {
		t.Error("theme 为空")
	}
}
