package adminapi

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/store"
)

type menuReq struct {
	Name  string `json:"name"`
	Lang  string `json:"lang"`
	Items any    `json:"items"` // 数组或字符串
}

func (d *Deps) HandleMenus(c *gin.Context) {
	lang := c.Query("lang")
	if lang == "" {
		lang = "zh"
	}
	items, err := d.Store.MenuRepo().ListByLang(c.Request.Context(), lang)
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": items})
}

func (d *Deps) HandleMenuCreate(c *gin.Context) {
	var req menuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	items, err := menuItems(req.Items)
	if err != nil {
		fail(c, err)
		return
	}
	m := &store.Menu{Name: req.Name, Lang: req.Lang, Items: items}
	if err := d.Store.MenuRepo().Create(c.Request.Context(), m); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"menu": m})
}

func (d *Deps) HandleMenuUpdate(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	var req menuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	cur, err := d.Store.MenuRepo().GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	// 部分 PUT：缺失字段保留原值（A 类修复）
	if req.Name != "" {
		cur.Name = req.Name
	}
	if req.Lang != "" {
		cur.Lang = req.Lang
	}
	if req.Items != nil {
		items, err := menuItems(req.Items)
		if err != nil {
			fail(c, err)
			return
		}
		cur.Items = items
	}
	if err := d.Store.MenuRepo().Update(c.Request.Context(), &cur); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"menu": cur})
}

func (d *Deps) HandleMenuDelete(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if err := d.Store.MenuRepo().Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"deleted": id})
}

// menuItems 把任意 items 统一为 JSON 字符串（避免双重编码，A 类修复）。
func menuItems(v any) (string, error) {
	if s, ok := v.(string); ok {
		return s, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
