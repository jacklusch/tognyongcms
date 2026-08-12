package adminapi

import (
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
	items, err := d.Store.MediaRepo().List(c.Request.Context(), 0, 100)
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"items": items})
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
