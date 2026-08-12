package media

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MediaStore 媒体存储抽象：本地磁盘或对象存储（S3/MinIO/OSS）。
type MediaStore interface {
	Save(ctx context.Context, key string, r io.Reader, contentType string) (string, error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
	// Key 从 URL 还原存储 key（删除时用）。
	Key(url string) string
}

// LocalStore 本地磁盘实现：{dir}/{key}，URL 为 {baseURL}/{key}。
type LocalStore struct {
	Dir     string
	BaseURL string
}

func NewLocalStore(dir, baseURL string) *LocalStore {
	return &LocalStore{Dir: dir, BaseURL: baseURL}
}

// sanitizeKey 校验并净化存储键：拒绝空键、`..`、反斜杠与以 `/` 开头的绝对路径。
func sanitizeKey(key string) (string, bool) {
	if key == "" || strings.Contains(key, "..") || strings.ContainsAny(key, `\/`[0:1]+"\\") || strings.HasPrefix(key, "/") {
		return "", false
	}
	return key, true
}

func (s *LocalStore) Save(ctx context.Context, key string, r io.Reader, _ string) (string, error) {
	k, ok := sanitizeKey(key)
	if !ok {
		return "", fmt.Errorf("非法存储键 %q", key)
	}
	full := filepath.Join(s.Dir, filepath.FromSlash(k))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(full)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil { // A 类：Save 检查 Close 错误
		return "", err
	}
	return s.URL(k), nil
}

func (s *LocalStore) Delete(ctx context.Context, key string) error {
	k, ok := sanitizeKey(key)
	if !ok {
		return fmt.Errorf("非法存储键 %q", key)
	}
	return os.Remove(filepath.Join(s.Dir, filepath.FromSlash(k)))
}

func (s *LocalStore) URL(key string) string {
	return s.BaseURL + "/" + key
}

// Key 从 URL 还原存储键。URL 形如 /media/<key>，去掉 BaseURL 前缀。
func (s *LocalStore) Key(url string) string {
	return trimPrefixURL(url, s.BaseURL)
}

// trimPrefixURL 去掉 baseURL（如 /media）前缀 + 前导 /；若不以 baseURL 开头返回原样。
func trimPrefixURL(url, baseURL string) string {
	rest := strings.TrimPrefix(url, baseURL)
	if rest == url {
		return url
	}
	return strings.TrimPrefix(rest, "/")
}

// RandomKey 生成 YYYYMM/随机hex{ext} 的存储键。
func RandomKey(ext string) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	prefix := time.Now().UTC().Format("200601")
	return prefix + "/" + hex.EncodeToString(b) + ext, nil
}
