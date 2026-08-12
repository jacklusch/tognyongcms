package media

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestLocalStoreSaveDeleteURL(t *testing.T) {
	dir := t.TempDir()
	ls := NewLocalStore(dir, "/media")

	key, err := RandomKey(".png")
	if err != nil {
		t.Fatal(err)
	}
	url, err := ls.Save(nil, key, bytes.NewReader([]byte("hello")), "image/png")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if url != "/media/"+key {
		t.Errorf("url = %q, want /media/%s", url, key)
	}
	full := filepath.Join(dir, key)
	b, err := os.ReadFile(full)
	if err != nil || string(b) != "hello" {
		t.Errorf("file content = %q, %v", b, err)
	}

	if err := ls.Delete(nil, key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(full); !os.IsNotExist(err) {
		t.Errorf("删除后文件应不存在, %v", err)
	}
	if got := ls.URL(key); got != "/media/"+key {
		t.Errorf("URL = %q", got)
	}
}

func TestRandomKey(t *testing.T) {
	k1, _ := RandomKey(".png")
	k2, _ := RandomKey(".png")
	if k1 == k2 {
		t.Error("两次 RandomKey 不应相同")
	}
	re := regexp.MustCompile(`^\d{6}/[0-9a-f]{24}\.png$`)
	if !re.MatchString(k1) {
		t.Errorf("key 格式异常: %q", k1)
	}
}

func TestLocalStoreSaveCreatesDirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "nested")
	ls := NewLocalStore(dir, "/media")
	key, _ := RandomKey(".jpg")
	if _, err := ls.Save(nil, key, io.LimitReader(bytes.NewReader([]byte("x")), 1), "image/jpeg"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, key)); err != nil {
		t.Errorf("文件未创建: %v", err)
	}
}

func TestLocalStoreKey(t *testing.T) {
	ls := NewLocalStore("/tmp/media", "/media")
	cases := []struct{ url, want string }{
		{"/media/202608/x.png", "202608/x.png"},
		{"/media/202608/x.png/", "202608/x.png/"},
		// 不以 BaseURL 开头：原样返回（防御）
		{"https://cdn.example.com/202608/x.png", "https://cdn.example.com/202608/x.png"},
		{"/other/202608/x.png", "/other/202608/x.png"},
	}
	for _, c := range cases {
		if got := ls.Key(c.url); got != c.want {
			t.Errorf("Key(%q) = %q, want %q", c.url, got, c.want)
		}
	}
	// 与 URL 互逆：URL(Key(url)) 还原
	key := "202608/y.png"
	if got := ls.Key(ls.URL(key)); got != key {
		t.Errorf("URL/Key 不可逆: Key(URL(%q)) = %q", key, got)
	}
}
