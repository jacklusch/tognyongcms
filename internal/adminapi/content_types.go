package adminapi

import (
	"github.com/gin-gonic/gin"

	"dulizhan/internal/schema"
)

type contentTypeReq struct {
	Name   string         `json:"name"`
	Label  string         `json:"label"`
	Fields []schema.Field `json:"fields"`
}

func (d *Deps) HandleContentTypes(c *gin.Context) {
	types, err := d.Content.AllTypes(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": types})
}

func (d *Deps) HandleContentTypeCreate(c *gin.Context) {
	var req contentTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	ct := &schema.ContentType{Name: req.Name, Label: req.Label, Fields: req.Fields}
	if err := d.Content.CreateType(c.Request.Context(), ct); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"content_type": ct})
}

func (d *Deps) HandleContentTypeUpdate(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	var req contentTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	// C1 修复：name 不可变，前端更新请求不带 name——用现存类型回填，避免 ValidateContentType 拒空名。
	cur, err := d.Content.GetTypeByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	ct := &schema.ContentType{ID: id, Name: cur.Name, Label: req.Label, Fields: req.Fields, Config: cur.Config}
	if err := d.Content.UpdateType(c.Request.Context(), ct); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"content_type": ct})
}

func (d *Deps) HandleContentTypeDelete(c *gin.Context) {
	name := c.Param("name")
	if err := d.Content.DeleteType(c.Request.Context(), name); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"deleted": name})
}
