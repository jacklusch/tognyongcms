package content

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dulizhan/internal/schema"
)

func timeNow() time.Time { return time.Now().UTC() }

// ensureSlug 生成合法 slug：优先显式 slug；为空则用标题 Slugify；仍为空（如纯中文标题）用时间戳兜底。
func ensureSlug(data map[string]any) string {
	slug, _ := data["slug"].(string)
	if slug == "" {
		slug = schema.Slugify(fmt.Sprint(data["title"]))
	}
	if slug == "" {
		slug = "post-" + strconv.FormatInt(timeNow().Unix(), 10)
	}
	return slug
}

func newContentID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(timeNow().String()))[:24]
	}
	return hex.EncodeToString(b)
}

// stripHTMLShell 剥离 html.Render 输出的 <html>/<head>/<body> 外壳，取 body 内内容。
func stripHTMLShell(s string) string {
	// 找 <body>...</body> 内的内容；找不到则返回原串
	low := strings.ToLower(s)
	i := strings.Index(low, "<body>")
	if i < 0 {
		return s
	}
	bodyStart := i + len("<body>")
	j := strings.Index(low[bodyStart:], "</body>")
	if j < 0 {
		return s
	}
	return s[bodyStart : bodyStart+j]
}
