package seed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"dulizhan/internal/media"
)

// mockMediaStore 最小 MediaStore（记录 Save 的 URL）。
type mockMediaStore struct {
	media.MediaStore
	savedURL string
}

func (m *mockMediaStore) Save(ctx context.Context, key string, r io.Reader, contentType string) (string, error) {
	b, _ := io.ReadAll(r)
	if len(b) == 0 {
		return "", fmt.Errorf("空内容")
	}
	m.savedURL = "/media/" + key
	return m.savedURL, nil
}

func TestDownloadCoverSuccess(t *testing.T) {
	// httptest mock 图片端点（返回图片字节）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer ts.Close()

	mock := &mockMediaStore{}
	url := downloadCover(context.Background(), mock, ts.URL+"/img.jpg")
	if url == "" || url != mock.savedURL {
		t.Errorf("downloadCover 应返回媒体库 URL, got %q, saved=%q", url, mock.savedURL)
	}
}

func TestDownloadCoverFallback(t *testing.T) {
	// 网络失败（不可达）→ 降级返回原 URL
	mock := &mockMediaStore{}
	url := downloadCover(context.Background(), mock, "http://127.0.0.1:1/unreachable.jpg")
	if url != "http://127.0.0.1:1/unreachable.jpg" {
		t.Errorf("失败应降级原 URL, got %q", url)
	}
	// 非 200 → 降级
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	url = downloadCover(context.Background(), mock, ts.URL+"/err.jpg")
	if url != ts.URL+"/err.jpg" {
		t.Errorf("非 200 应降级原 URL, got %q", url)
	}
}

func TestDownloadCoverSkip(t *testing.T) {
	t.Setenv("DULIZHAN_SEED_NO_DOWNLOAD", "1")
	mock := &mockMediaStore{}
	url := downloadCover(context.Background(), mock, "https://picsum.photos/seed/x/800/500")
	if url != "https://picsum.photos/seed/x/800/500" {
		t.Errorf("NO_DOWNLOAD 应跳过下载返回原 URL, got %q", url)
	}
	if mock.savedURL != "" {
		t.Errorf("NO_DOWNLOAD 不应保存媒体, saved=%q", mock.savedURL)
	}
}
