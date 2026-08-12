package adminapi

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/errs"
	"dulizhan/internal/store"
)

type categoryReq struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ParentID    int64  `json:"parent_id"`
}

var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)
var slugCleanRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify 按规则生成 slug：小写、非 [a-z0-9] 替换为 -、去首尾 -；空输入返回空。
func slugify(name string) string {
	s := strings.ToLower(name)
	s = slugCleanRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func (d *Deps) HandleCategories(c *gin.Context) {
	ctx := c.Request.Context()
	items, err := d.Store.CategoryRepo().List(ctx)
	if err != nil {
		fail(c, err)
		return
	}
	counts := make(map[int64]int, len(items))
	for _, it := range items {
		n, err := d.Store.CategoryRepo().CountContent(ctx, it.ID)
		if err != nil {
			fail(c, err)
			return
		}
		counts[it.ID] = n
	}
	roots, all := buildCategoryTree(items, counts)
	respondOK(c, gin.H{"items": roots, "all": all})
}

// buildCategoryTree 把分类按 parent_id 挂成树返回 roots；all 为扁平 {id, path}（path 为父链拼接，顶级为 name）。
func buildCategoryTree(cats []store.Category, counts map[int64]int) (roots []gin.H, all []gin.H) {
	type node struct {
		id       int64
		parentID int64
		name     string
		slug     string
		desc     string
		count    int
		children []*node
	}
	byID := make(map[int64]*node, len(cats))
	for _, c := range cats {
		byID[c.ID] = &node{id: c.ID, parentID: c.ParentID, name: c.Name, slug: c.Slug, desc: c.Description, count: counts[c.ID]}
	}
	rootNodes := make([]*node, 0)
	for _, c := range cats {
		n := byID[c.ID]
		if p, ok := byID[n.parentID]; ok {
			p.children = append(p.children, n)
		} else {
			rootNodes = append(rootNodes, n) // 父缺失视为顶级
		}
	}
	var toH func(n *node, parentPath string) gin.H
	toH = func(n *node, parentPath string) gin.H {
		path := n.name
		if parentPath != "" {
			path = parentPath + "/" + n.name
		}
		all = append(all, gin.H{"id": n.id, "path": path})
		children := make([]gin.H, 0, len(n.children))
		for _, ch := range n.children {
			children = append(children, toH(ch, path))
		}
		return gin.H{
			"id": n.id, "parent_id": n.parentID, "name": n.name, "slug": n.slug,
			"description": n.desc, "content_count": n.count, "children": children,
		}
	}
	roots = make([]gin.H, 0, len(rootNodes))
	for _, r := range rootNodes {
		roots = append(roots, toH(r, ""))
	}
	return roots, all
}

// validateParent 校验上级分类：0 合法；不存在 → 422；选择自身或其子孙 → 422（防环）。
func (d *Deps) validateParent(ctx context.Context, id, parentID int64) error {
	if parentID == 0 {
		return nil
	}
	if _, err := d.Store.CategoryRepo().GetByID(ctx, parentID); err != nil {
		return fmt.Errorf("%w: 上级分类不存在", errs.ErrValidation)
	}
	if id == 0 {
		return nil // 创建时无自身，跳过防环
	}
	if id == parentID {
		return fmt.Errorf("%w: 不能选择自身或其子分类作为上级分类", errs.ErrValidation)
	}
	desc, err := d.Store.CategoryRepo().Descendants(ctx, id)
	if err != nil {
		return err
	}
	for _, c := range desc {
		if c.ID == parentID {
			return fmt.Errorf("%w: 不能选择自身或其子分类作为上级分类", errs.ErrValidation)
		}
	}
	return nil
}

// uniqueSlug 撞车加 -2/-3… 后缀直到空闲。
func (d *Deps) uniqueSlug(ctx context.Context, slug string) (string, error) {
	candidate := slug
	for i := 2; ; i++ {
		if _, err := d.Store.CategoryRepo().GetBySlug(ctx, candidate); err != nil {
			if errors.Is(err, errs.ErrNotFound) {
				return candidate, nil
			}
			return "", err
		}
		candidate = fmt.Sprintf("%s-%d", slug, i)
	}
}

func (d *Deps) HandleCategoryCreate(c *gin.Context) {
	ctx := c.Request.Context()
	var req categoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	if req.Slug == "" {
		req.Slug = slugify(req.Name)
		slug, err := d.uniqueSlug(ctx, req.Slug)
		if err != nil {
			fail(c, err)
			return
		}
		req.Slug = slug
	}
	if err := validateCategory(req); err != nil {
		fail(c, err)
		return
	}
	if err := d.validateParent(ctx, 0, req.ParentID); err != nil {
		fail(c, err)
		return
	}
	cat := &store.Category{Name: req.Name, Slug: req.Slug, Description: req.Description, ParentID: req.ParentID}
	if err := d.Store.CategoryRepo().Create(ctx, cat); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"category": cat})
}

func (d *Deps) HandleCategoryUpdate(c *gin.Context) {
	ctx := c.Request.Context()
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
	if err := d.validateParent(ctx, id, req.ParentID); err != nil {
		fail(c, err)
		return
	}
	cat := &store.Category{ID: id, Name: req.Name, Slug: req.Slug, Description: req.Description, ParentID: req.ParentID}
	if err := d.Store.CategoryRepo().Update(ctx, cat); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"category": cat})
}

func (d *Deps) HandleCategoryDelete(c *gin.Context) {
	ctx := c.Request.Context()
	id, ok := mustID(c)
	if !ok {
		return
	}
	kids, err := d.Store.CategoryRepo().ListChildren(ctx, id)
	if err != nil {
		fail(c, err)
		return
	}
	if len(kids) > 0 {
		fail(c, fmt.Errorf("%w: 该分类下仍有子分类，请先删除子分类", errs.ErrForbidden))
		return
	}
	n, err := d.Store.CategoryRepo().CountContent(ctx, id)
	if err != nil {
		fail(c, err)
		return
	}
	if n > 0 {
		fail(c, fmt.Errorf("%w: 该分类下仍有内容，请先移除", errs.ErrForbidden))
		return
	}
	if err := d.Store.CategoryRepo().Delete(ctx, id); err != nil {
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
