package theme

import (
	"net/url"
	"strings"
)

// socialPlatform 从链接识别社交平台标识，用于页脚图标选择。
// 支持：tiktok / youtube / facebook / x / instagram；未知返回空串。
func socialPlatform(rawURL string) string {
	host := hostOf(rawURL)
	if host == "" {
		return ""
	}
	hasDomain := func(d string) bool {
		return host == d || strings.HasSuffix(host, "."+d)
	}
	switch {
	case hasDomain("tiktok.com"):
		return "tiktok"
	case hasDomain("youtube.com"), hasDomain("youtu.be"):
		return "youtube"
	case hasDomain("facebook.com"), hasDomain("fb.com"):
		return "facebook"
	case hasDomain("x.com"), hasDomain("twitter.com"):
		return "x"
	case hasDomain("instagram.com"):
		return "instagram"
	}
	return ""
}

// hostOf 解析链接主机名（小写、去 www. 前缀）；省略协议时补 https:// 再解析。
func hostOf(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		u, err = url.Parse("https://" + rawURL)
		if err != nil {
			return ""
		}
	}
	host := strings.ToLower(u.Hostname())
	return strings.TrimPrefix(host, "www.")
}
