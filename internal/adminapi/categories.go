package adminapi

import (
	"fmt"
	"regexp"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/errs"
	"dulizhan/internal/store"
)

type categoryReq struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

func (d *Deps) HandleCategories(c *gin.Context) {
	items, err := d.Store.CategoryRepo().List(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	// 带内容数
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		n, err := d.Store.CategoryRepo().CountContent(c.Request.Context(), it.ID)
		if err != nil {
			fail(c, err)
			return
		}
		out = append(out, gin.H{"id": it.ID, "name": it.Name, "slug": it.Slug, "description": it.Description, "content_count": n})
	}
	respondOK(c, gin.H{"items": out})
}

func (d *Deps) HandleCategoryCreate(c *gin.Context) {
	var req categoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	if err := validateCategory(req); err != nil {
		fail(c, err)
		return
	}
	cat := &store.Category{Name: req.Name, Slug: req.Slug, Description: req.Description}
	if err := d.Store.CategoryRepo().Create(c.Request.Context(), cat); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"category": cat})
}

func (d *Deps) HandleCategoryUpdate(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	var req categoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	if err := validateCategory(req); err != nil {
		fail(c, err)
		return
	}
	cat := &store.Category{ID: id, Name: req.Name, Slug: req.Slug, Description: req.Description}
	if err := d.Store.CategoryRepo().Update(c.Request.Context(), cat); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"category": cat})
}

func (d *Deps) HandleCategoryDelete(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	n, err := d.Store.CategoryRepo().CountContent(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	if n > 0 {
		fail(c, fmt.Errorf("%w: 该分类下仍有内容，请先移除", errs.ErrForbidden))
		return
	}
	if err := d.Store.CategoryRepo().Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"deleted": id})
}

func validateCategory(req categoryReq) error {
	if req.Name == "" {
		return fmt.Errorf("%w: 分类名称不能为空", errs.ErrValidation)
	}
	if !slugRe.MatchString(req.Slug) {
		return fmt.Errorf("%w: 分类 slug 需为小写字母数字连字符", errs.ErrValidation)
	}
	return nil
}
