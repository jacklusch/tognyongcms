package adminapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestStatsEndpoint(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	w := e.do(t, http.MethodGet, "/api/stats", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("stats = %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "article") {
		t.Errorf("stats 缺类型: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "recent") {
		t.Errorf("stats 缺 recent: %s", w.Body.String())
	}
}
