package adminapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
	"dulizhan/internal/config"
	"dulizhan/internal/content"
	"dulizhan/internal/i18n"
	"dulizhan/internal/media"
	"dulizhan/internal/schema"
	"dulizhan/internal/seed"
	"dulizhan/internal/store"
	"dulizhan/internal/store/sqlite"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type env struct {
	g    *gin.Engine
	st   store.Store
	svc  *content.Service
	auth *auth.Service
	cfg  *config.Config
}

func newEnv(t *testing.T) *env {
	t.Helper()
	// 测试禁用 seed 封面图网络下载（离线/快速），cover 用 picsum 原 URL 兜底。
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1")
	st, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = i18n.New([]string{"zh", "en"}, "zh", false)
	authSvc := auth.New(st, time.Hour)
	svc := content.New(st, schema.NewRegistry(), []string{"zh", "en"})
	if err := seed.EnsureAuth(context.Background(), st, authSvc); err != nil {
		t.Fatal(err)
	}
	if err := seed.Run(context.Background(), st, svc, media.NewLocalStore(t.TempDir(), "/media")); err != nil {
		t.Fatal(err)
	}
	g := gin.New()
	cfg := &config.Config{}
	cfg.Site.Name = "测试站点"
	cfg.Site.URL = "https://example.com"
	cfg.Site.Description = "描述"
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Languages = []string{"zh", "en"}
	cfg.Site.Theme = "default"
	d := Deps{Store: st, Auth: authSvc, Content: svc, Media: media.NewLocalStore(t.TempDir(), "/media"), Cfg: cfg}
	Register(g.Group("/api"), d)
	return &env{g: g, st: st, svc: svc, auth: authSvc, cfg: cfg}
}

func (e *env) do(t *testing.T, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.g.ServeHTTP(w, req)
	return w
}

func (e *env) login(t *testing.T) string {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"admin123"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Data.Token
}

func TestLoginMe(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	w := e.do(t, http.MethodGet, "/api/auth/me", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("me = %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "admin") {
		t.Errorf("me 缺用户名: %s", w.Body.String())
	}
	// 未登录 → 401
	w = e.do(t, http.MethodGet, "/api/auth/me", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("未登录 me = %d, want 401", w.Code)
	}
}

func TestContentCRUDAndDuplicateSlug422(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 建类型
	w := e.do(t, http.MethodPost, "/api/content-types", `{"name":"note","label":"笔记","fields":[{"name":"title","label":"标题","type":"text","required":true}]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create type = %d %s", w.Code, w.Body.String())
	}
	// 建内容
	body := `{"type":"note","lang":"zh","data":{"title":"接口内容"}}`
	w = e.do(t, http.MethodPost, "/api/content", body, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create content = %d %s", w.Code, w.Body.String())
	}
	// 重复 slug → 422（seed 已有 article/hello-zh；这里建 note，无冲突——用 article 验证）
	w = e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"重复","slug":"hello-zh"}}`, tok)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("重复 slug = %d, want 422", w.Code)
	}
	// 缺必填 → 422
	w = e.do(t, http.MethodPost, "/api/content", `{"type":"note","lang":"zh","data":{}}`, tok)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("缺必填 = %d, want 422", w.Code)
	}
	// 未登录 → 401
	w = e.do(t, http.MethodPost, "/api/content", body, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("未登录 = %d, want 401", w.Code)
	}
}

func TestContentOwnershipForbidden(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	// 复用 seed 已建的 author 角色（无 content.publish 权限，含 content.delete 以便测 canManage 删除归属）
	authorRole, err := e.st.RoleRepo().GetByName(ctx, "author")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.RoleRepo().Update(ctx, authorRole.ID, "author", `["content.read","content.write","content.delete"]`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.CreateUser(ctx, "alice", "secret123", authorRole.ID); err != nil {
		t.Fatal(err)
	}
	authorTok, _ := e.auth.Login(ctx, "alice", "secret123")
	// author 建内容
	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"作者文章","slug":"alice-post","content":"<p>正文</p>","published_on":"2026-01-15","category":"1"}}`, authorTok)
	if w.Code != http.StatusOK {
		t.Fatalf("author create = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Content struct {
				Content struct {
					ID int64 `json:"id"`
				} `json:"content"`
			} `json:"content"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	id := resp.Data.Content.Content.ID
	// author 无法发布自己内容（无 content.publish 权限 → 403）
	w = e.do(t, http.MethodPost, fmt.Sprintf("/api/content/%d/publish", id), "", authorTok)
	if w.Code != http.StatusForbidden {
		t.Errorf("author publish = %d, want 403", w.Code)
	}

	// —— canManage 归属路径 ——
	// admin 登录建一篇他人内容
	adminTok := e.login(t)
	w = e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"他人文章","slug":"admin-post","content":"<p>正文</p>","published_on":"2026-01-15","category":"1"}}`, adminTok)
	if w.Code != http.StatusOK {
		t.Fatalf("admin create = %d %s", w.Code, w.Body.String())
	}
	var adminResp struct {
		Data struct {
			Content struct {
				Content struct {
					ID int64 `json:"id"`
				} `json:"content"`
			} `json:"content"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &adminResp)
	otherID := adminResp.Data.Content.Content.ID
	// author 修改他人内容 → 403（canManage 归属拒绝）
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/content/%d", otherID), `{"data":{"title":"改"}}`, authorTok)
	if w.Code != http.StatusForbidden {
		t.Errorf("author update 他人 = %d, want 403", w.Code)
	}
	// author 删除他人内容 → 403（canManage 归属拒绝）
	w = e.do(t, http.MethodDelete, fmt.Sprintf("/api/content/%d", otherID), "", authorTok)
	if w.Code != http.StatusForbidden {
		t.Errorf("author delete 他人 = %d, want 403", w.Code)
	}
	// 正向：author 修改自己内容 → 200（归属放行）
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/content/%d", id), `{"data":{"title":"改自","slug":"alice-post","content":"<p>正文2</p>","published_on":"2026-01-15","category":"1"}}`, authorTok)
	if w.Code != http.StatusOK {
		t.Errorf("author update 自己 = %d, want 200", w.Code)
	}
}

func TestContentTypeDuplicateName422(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// seed.Run 已建 article，重复同名 → 422（UNIQUE 冲突走 wrapUnique）
	w := e.do(t, http.MethodPost, "/api/content-types",
		`{"name":"article","label":"文章","fields":[{"name":"title","label":"标题","type":"text"}]}`, tok)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("重复类型名 = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "内容类型名已存在") {
		t.Errorf("消息不含预期文案: %s", w.Body.String())
	}
}

func TestContentTypeNameFormat(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 连字符名被拒（内容类型名是标识符，正则 ^[a-zA-Z][a-zA-Z0-9_]*$）
	w := e.do(t, http.MethodPost, "/api/content-types", `{"name":"article-copy","label":"副本","fields":[]}`, tok)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("连字符名 = %d, want 422", w.Code)
	}
	// 下划线名合法（前端复制用 name_copy）
	w = e.do(t, http.MethodPost, "/api/content-types", `{"name":"article_copy","label":"副本","fields":[]}`, tok)
	if w.Code != http.StatusOK {
		t.Errorf("下划线名 = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	// hex 名合法（前端新建用 newtype+hex）
	w = e.do(t, http.MethodPost, "/api/content-types", `{"name":"newtypemsolvjf9","label":"新建","fields":[]}`, tok)
	if w.Code != http.StatusOK {
		t.Errorf("hex 名 = %d, want 200, body=%s", w.Code, w.Body.String())
	}
}

func TestMediaUploadDelete(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 构造 multipart 上传
	var buf bytes.Buffer
	bw := multipart.NewWriter(&buf)
	fw, _ := bw.CreateFormFile("file", "a.png")
	fw.Write([]byte("hello"))
	bw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/media/upload", &buf)
	req.Header.Set("Content-Type", bw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	e.g.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("upload = %d %s", w.Code, w.Body.String())
	}
	// 列表
	w = e.do(t, http.MethodGet, "/api/media", "", tok)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "a.png") {
		t.Errorf("media list = %d %s", w.Code, w.Body.String())
	}
	// 从列表解析媒体 id
	var mediaList struct {
		Data struct {
			Items []struct {
				ID int64 `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &mediaList)
	if len(mediaList.Data.Items) == 0 {
		t.Fatal("media list 为空")
	}
	mid := mediaList.Data.Items[0].ID
	// 删除
	w = e.do(t, http.MethodDelete, fmt.Sprintf("/api/media/%d", mid), "", tok)
	if w.Code != http.StatusOK {
		t.Errorf("media delete = %d %s", w.Code, w.Body.String())
	}
}

func TestMenuPartialUpdate(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	w := e.do(t, http.MethodPost, "/api/menus", `{"name":"主导航","lang":"zh","items":[{"label":"首页"}]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("menu create = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Menu struct {
				ID int64 `json:"id"`
			} `json:"menu"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	id := resp.Data.Menu.ID
	// 部分更新只改 items，Name 保留（A 类修复）
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/menus/%d", id), `{"items":[{"label":"关于"}]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("menu update = %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "主导航") || !strings.Contains(w.Body.String(), "关于") {
		t.Errorf("部分更新丢字段: %s", w.Body.String())
	}
}

func TestTypePermScoped(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	// author 角色（content.read/content.write，无 content.*，无类型级）
	authorRole, err := e.st.RoleRepo().GetByName(ctx, "author")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.RoleRepo().Update(ctx, authorRole.ID, "author", `["content.read","content.write","media.upload"]`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.CreateUser(ctx, "alice", "secret123", authorRole.ID); err != nil {
		t.Fatal(err)
	}
	authorTok, _ := e.auth.Login(ctx, "alice", "secret123")
	// author 建内容（Create 走基础 content.write 门禁 + service canManage 归属，可过）
	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"作者文章","slug":"author-post","content":"<p>正文</p>","published_on":"2026-01-15","category":"1"}}`, authorTok)
	if w.Code != http.StatusOK {
		t.Fatalf("author create = %d %s", w.Code, w.Body.String())
	}
	// author 列表：RequirePerm("content.read") 过，但类型级 content.read.article 无（author 无 content.* 无 content.read.*）→ 403
	w = e.do(t, http.MethodGet, "/api/content?type=article&lang=zh", "", authorTok)
	if w.Code != http.StatusForbidden {
		t.Errorf("author 列表(无类型权限) = %d, want 403", w.Code)
	}
}

func TestContentTypeUpdateKeepsName(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 建类型
	w := e.do(t, http.MethodPost, "/api/content-types", `{"name":"note","label":"笔记","fields":[{"name":"title","label":"标题","type":"text"}]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("create type = %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			ContentType struct {
				ID int64 `json:"id"`
			} `json:"content_type"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.Data.ContentType.ID == 0 {
		t.Fatal("创建未回填 ID")
	}
	// C1：更新不带 name（模拟前端 updateContentType(id, {label, fields})）→ 200 且 name 保留
	w = e.do(t, http.MethodPut, fmt.Sprintf("/api/content-types/%d", created.Data.ContentType.ID), `{"label":"新标签","fields":[{"name":"title","label":"标题","type":"text"}]}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("update without name = %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"name":"note"`) {
		t.Errorf("更新后 name 应保留: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"label":"新标签"`) {
		t.Errorf("label 应更新: %s", w.Body.String())
	}
}

func TestContentListNoLang(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// C2：无 lang → 200，返回全部语言（seed 有 zh/en 两篇示例文章）
	w := e.do(t, http.MethodGet, "/api/content?type=article", "", tok)
	if w.Code != http.StatusOK {
		t.Fatalf("list without lang = %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello-zh") || !strings.Contains(w.Body.String(), "hello-en") {
		t.Errorf("应返回全部语言: %s", w.Body.String())
	}
}

func TestSearchAndStatsTypePermForAuthor(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	authorRole, err := e.st.RoleRepo().GetByName(ctx, "author")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.RoleRepo().Update(ctx, authorRole.ID, "author", `["content.read","content.write","media.upload"]`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.CreateUser(ctx, "bob", "secret123", authorRole.ID); err != nil {
		t.Fatal(err)
	}
	authorTok, _ := e.auth.Login(ctx, "bob", "secret123")
	// C3：author 无类型级读权限 → 搜索 403
	w := e.do(t, http.MethodGet, "/api/search?type=article&lang=zh&q=%E4%B8%96%E7%95%8C", "", authorTok)
	if w.Code != http.StatusForbidden {
		t.Errorf("author 搜索 = %d, want 403: %s", w.Code, w.Body.String())
	}
	// C3：stats 按权限过滤 → 200 且 by_type/recent 为空
	w = e.do(t, http.MethodGet, "/api/stats", "", authorTok)
	if w.Code != http.StatusOK {
		t.Fatalf("author stats = %d %s", w.Code, w.Body.String())
	}
	var st struct {
		Data struct {
			ByType []any `json:"by_type"`
			Recent []any `json:"recent"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &st)
	if len(st.Data.ByType) != 0 || len(st.Data.Recent) != 0 {
		t.Errorf("author stats 应按权限过滤为空: by_type=%d recent=%d", len(st.Data.ByType), len(st.Data.Recent))
	}
}

func TestAuthorSelfTranslation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	authorRole, err := e.st.RoleRepo().GetByName(ctx, "author")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.RoleRepo().Update(ctx, authorRole.ID, "author", `["content.read","content.write","media.upload"]`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.CreateUser(ctx, "carol", "secret123", authorRole.ID); err != nil {
		t.Fatal(err)
	}
	authorTok, _ := e.auth.Login(ctx, "carol", "secret123")
	// author 建内容
	w := e.do(t, http.MethodPost, "/api/content", `{"type":"article","lang":"zh","data":{"title":"原文","slug":"orig","content":"<p>x</p>","published_on":"2026-01-15","category":"1"}}`, authorTok)
	if w.Code != http.StatusOK {
		t.Fatalf("author create = %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Content struct {
				Content struct {
					ID int64 `json:"id"`
				} `json:"content"`
			} `json:"content"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	id := resp.Data.Content.Content.ID
	if id == 0 {
		t.Fatal("创建未回填 ID")
	}
	// I2：author 给自己内容加翻译 → 200（移除类型级读权限，归属由 service Actor 校验放行）
	w = e.do(t, http.MethodPost, fmt.Sprintf("/api/content/%d/translate", id), `{"type":"article","lang":"en","data":{"title":"En","content":"<p>e</p>","published_on":"2026-01-15","category":"1"}}`, authorTok)
	if w.Code != http.StatusOK {
		t.Errorf("author 自翻译 = %d, want 200: %s", w.Code, w.Body.String())
	}
}

func TestUserRoleAPIs(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	// 角色列表
	w := e.do(t, http.MethodGet, "/api/roles", "", tok)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "admin") {
		t.Errorf("roles = %d %s", w.Code, w.Body.String())
	}
	// 建用户
	w = e.do(t, http.MethodPost, "/api/users", `{"username":"u2","password":"secret123","role_id":1}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("user create = %d %s", w.Code, w.Body.String())
	}
	// 用户列表
	w = e.do(t, http.MethodGet, "/api/users", "", tok)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "u2") {
		t.Errorf("users = %d %s", w.Code, w.Body.String())
	}
}
