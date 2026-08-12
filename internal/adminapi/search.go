package adminapi

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// HandleSearch 内容搜索：GET /api/search?type=&lang=&q=&page=&per_page=
func (d *Deps) HandleSearch(c *gin.Context) {
	typeName := c.Query("type")
	if typeName == "" {
		badRequest(c, "缺少 type 参数")
		return
	}
	// C3 修复：搜索含全文/草稿，须过类型级读权限，防止无类型权限用户枚举其他类型内容。
	if !d.requireTypePerm(c, "content.read", typeName) {
		return
	}
	lang := c.Query("lang")
	if lang == "" {
		badRequest(c, "缺少 lang 参数")
		return
	}
	q := c.Query("q")
	if q == "" {
		badRequest(c, "缺少 q 参数")
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.Query("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	items, total, err := d.Content.Search(c.Request.Context(), typeName, lang, q, page, perPage)
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": items, "total": total})
}
