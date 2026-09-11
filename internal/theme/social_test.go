package theme

import "testing"

func TestSocialPlatform(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.tiktok.com/@jinbowei", "tiktok"},
		{"https://tiktok.com/@x", "tiktok"},
		{"https://youtu.be/abc123", "youtube"},
		{"https://www.youtube.com/c/Jinbowei", "youtube"},
		{"https://www.facebook.com/jinbowei", "facebook"},
		{"https://fb.com/jinbowei", "facebook"},
		{"https://x.com/jinbowei", "x"},
		{"https://twitter.com/jinbowei", "x"},
		{"https://www.instagram.com/jinbowei", "instagram"},
		{"www.tiktok.com/@no-scheme", "tiktok"}, // 省略协议
		{"https://example.com/about", ""},
		{"https://box.com/x", ""},     // 不得误判为 x.com
		{"https://netflix.com/x", ""}, // 不得误判为 x.com
		{"", ""},
		{"not a url", ""},
	}
	for _, c := range cases {
		if got := socialPlatform(c.url); got != c.want {
			t.Errorf("socialPlatform(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}
