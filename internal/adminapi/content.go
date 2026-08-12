package adminapi

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
	"dulizhan/internal/content"
	"dulizhan/internal/errs"
)

type contentReq struct {
	Type string         `json:"type"`
	Lang string         `json:"lang"`
	Data map[string]any `json:"data"`
}

// actor 从上下文取当前用户，缺会话返回零值（防御性，final-fix Finding 1）。
func (d *Deps) actor(c *gin.Context) content.Actor {
	us, ok := auth.UserFromContext(c)
	if !ok || us == nil {
		return content.Actor{}
	}
	return content.Actor{UserID: us.User.ID, IsModerator: us.HasPerm("content.publish") || us.Perms["*"]}
}

// canManage 校验操作权：moderator 可操作他人，作者只能操作自己创建的内容。
func (d *Deps) canManage(c *gin.Context, id int64) bool {
	a := d.actor(c)
	if a.IsModerator {
		return true
	}
	e, err := d.Content.GetByID(c.Request.Context(), id)
	if err != nil {
		return false
	}
	return e.Content.CreatedBy == a.UserID
}

// requireTypePerm 校验当前用户对某内容类型的操作权限；失败写 403 返回 false。
func (d *Deps) requireTypePerm(c *gin.Context, action, typeName string) bool {
	us, ok := auth.UserFromContext(c)
	if !ok || us == nil {
		unauthorized(c)
		return false
	}
	if !us.HasPerm(action + "." + typeName) {
		fail(c, errs.ErrForbidden)
		return false
	}
	return true
}

func (d *Deps) HandleContentList(c *gin.Context) {
	typeName := c.Query("type")
	if typeName == "" {
		badRequest(c, "缺少 type 参数")
		return
	}
	if !d.requireTypePerm(c, "content.read", typeName) {
		return
	}
	lang := c.Query("lang") // C2 修复：lang 为空=不过滤（store ListByTypeLangStatus 已支持），供 relation 选择器跨语言拉取
	status := c.Query("status")
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.Query("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	items, total, err := d.Content.ListAdmin(c.Request.Context(), typeName, lang, status, page, perPage)
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": items, "total": total})
}

func (d *Deps) HandleContentCreate(c *gin.Context) {
	var req contentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	a := d.actor(c)
	if a.UserID == 0 {
		unauthorized(c)
		return
	}
	e, err := d.Content.Create(c.Request.Context(), req.Type, req.Lang, req.Data, a.UserID)
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"content": e})
}

func (d *Deps) HandleContentGet(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	cur, err := d.Content.GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	if !d.requireTypePerm(c, "content.read", cur.TypeName) {
		return
	}
	respondOK(c, gin.H{"content": cur})
}

func (d *Deps) HandleContentUpdate(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if !d.canManage(c, id) {
		fail(c, errs.ErrForbidden)
		return
	}
	var req contentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	e, err := d.Content.Update(c.Request.Context(), id, req.Data)
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"content": e})
}

func (d *Deps) HandleContentDelete(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if !d.canManage(c, id) {
		fail(c, errs.ErrForbidden)
		return
	}
	if err := d.Content.Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"deleted": id})
}

func (d *Deps) HandleContentPublish(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if !d.canManage(c, id) {
		fail(c, errs.ErrForbidden)
		return
	}
	if err := d.Content.SetStatus(c.Request.Context(), id, "published"); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"id": id, "status": "published"})
}

func (d *Deps) HandleContentUnpublish(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	if !d.canManage(c, id) {
		fail(c, errs.ErrForbidden)
		return
	}
	if err := d.Content.SetStatus(c.Request.Context(), id, "draft"); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"id": id, "status": "draft"})
}

func (d *Deps) HandleTranslations(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	e, err := d.Content.GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	if !d.requireTypePerm(c, "content.read", e.TypeName) {
		return
	}
	entries, err := d.Content.ListByContentID(c.Request.Context(), e.Content.ContentID)
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": entries})
}

func (d *Deps) HandleCreateTranslation(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	parent, err := d.Content.GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	// I2 修复：翻译创建移除类型级读权限——author 给自己内容加翻译放行，归属校验由 service Actor 完成。
	var req contentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	e, err := d.Content.CreateTranslation(c.Request.Context(), req.Type, req.Lang, parent.Content.ContentID, req.Data, d.actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"content": e})
}
