package adminapi

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/media"
	"dulizhan/internal/store"
)

func (d *Deps) HandleMediaUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "缺少 file 字段")
		return
	}
	ext := ""
	if i := lastIndexByte(file.Filename, '.'); i >= 0 {
		ext = file.Filename[i:]
	}
	key, err := media.RandomKey(ext)
	if err != nil {
		fail(c, err)
		return
	}
	src, err := file.Open()
	if err != nil {
		fail(c, err)
		return
	}
	defer src.Close()
	url, err := d.Media.Save(c.Request.Context(), key, src, file.Header.Get("Content-Type"))
	if err != nil {
		fail(c, err)
		return
	}
	m := &store.Media{Filename: file.Filename, URL: url, Mime: file.Header.Get("Content-Type"), Size: file.Size}
	if err := d.Store.MediaRepo().Create(c.Request.Context(), m); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"key": key, "url": url})
}

func (d *Deps) HandleMediaList(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.Query("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	ctx := c.Request.Context()
	total, err := d.Store.MediaRepo().Count(ctx)
	if err != nil {
		fail(c, err)
		return
	}
	items, err := d.Store.MediaRepo().List(ctx, (page-1)*perPage, perPage)
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": items, "total": total})
}

type mediaBatchDeleteReq struct {
	IDs []int64 `json:"ids"`
}

// HandleMediaBatchDelete 批量删除媒体（逐个删文件 + 记录）；已不存在的 id 跳过。
func (d *Deps) HandleMediaBatchDelete(c *gin.Context) {
	var req mediaBatchDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	ctx := c.Request.Context()
	deleted := 0
	for _, id := range req.IDs {
		m, err := d.Store.MediaRepo().GetByID(ctx, id)
		if err != nil {
			continue
		}
		if err := d.Media.Delete(ctx, d.Media.Key(m.URL)); err != nil {
			continue
		}
		if err := d.Store.MediaRepo().Delete(ctx, id); err != nil {
			continue
		}
		deleted++
	}
	respondOK(c, gin.H{"deleted": deleted})
}

func (d *Deps) HandleMediaDelete(c *gin.Context) {
	id, ok := mustID(c)
	if !ok {
		return
	}
	m, err := d.Store.MediaRepo().GetByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	if err := d.Media.Delete(c.Request.Context(), d.Media.Key(m.URL)); err != nil {
		fail(c, err)
		return
	}
	if err := d.Store.MediaRepo().Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"deleted": id})
}

func lastIndexByte(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}
