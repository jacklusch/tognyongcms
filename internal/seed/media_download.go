// internal/seed/media_download.go
package seed

import (
	"context"
	"net/http"
	"os"
	"time"

	"dulizhan/internal/media"
)

// downloadCover 从网络下载图片存媒体库；失败降级返回原 URL（前台 media 函数仍可渲染）。
// 设置环境变量 DULIZHAN_SEED_NO_DOWNLOAD=1 时跳过网络下载（测试/离线场景直接返回原 URL）。
func downloadCover(ctx context.Context, med media.MediaStore, url string) string {
	if os.Getenv("DULIZHAN_SEED_NO_DOWNLOAD") == "1" {
		return url
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return url
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return url
	}
	defer resp.Body.Close()
	key, err := media.RandomKey(".jpg")
	if err != nil {
		return url
	}
	u, err := med.Save(ctx, key, resp.Body, "image/jpeg")
	if err != nil {
		return url
	}
	return u
}
