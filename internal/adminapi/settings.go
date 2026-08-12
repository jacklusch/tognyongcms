package adminapi

import (
	"github.com/gin-gonic/gin"
)

func (d *Deps) HandleSettings(c *gin.Context) {
	all, err := d.Store.SettingRepo().All(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	respondOK(c, gin.H{"settings": all})
}

func (d *Deps) HandleSettingsUpdate(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体格式错误")
		return
	}
	for k, v := range req {
		if err := d.Store.SettingRepo().Set(c.Request.Context(), k, v); err != nil {
			fail(c, err)
			return
		}
	}
	respondOK(c, gin.H{"settings": req})
}
