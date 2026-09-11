package adminapi

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/content"
	"dulizhan/internal/store"
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
	category := c.Query("category")
	var items []content.Entry
	var total int
	var err error
	if category != "" {
		cid, convErr := strconv.ParseInt(category, 10, 64)
		if convErr != nil {
			badRequest(c, "category 参数必须是分类 id")
			return
		}
		items, total, err = d.Content.SearchByTypeLangCategory(c.Request.Context(), typeName, lang, q, cid, page, perPage)
	} else {
		items, total, err = d.Content.Search(c.Request.Context(), typeName, lang, q, page, perPage)
	}
	if err != nil {
		fail(c, err)
		return
	}
	type outEntry struct {
		Content      store.Content  `json:"content"`
		TypeName     string         `json:"type_name"`
		Fields       map[string]any `json:"fields"`
		CategoryName string         `json:"category_name,omitempty"`
	}
	ctx := c.Request.Context()
	out := make([]outEntry, 0, len(items))
	for _, it := range items {
		out = append(out, outEntry{
			Content:      it.Content,
			TypeName:     it.TypeName,
			Fields:       it.Fields,
			CategoryName: d.categoryName(ctx, it),
		})
	}
	respondOK(c, gin.H{"items": out, "total": total})
}
