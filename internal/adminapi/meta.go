package adminapi

import "github.com/gin-gonic/gin"

// HandleMeta 返回 SPA 需要的站点配置信息（语言、默认语言、主题、站点名等）。
func (d *Deps) HandleMeta(c *gin.Context) {
	respondOK(c, gin.H{
		"site_name":    d.Cfg.Site.Name,
		"site_url":     d.Cfg.Site.URL,
		"description":  d.Cfg.Site.Description,
		"languages":    d.Cfg.Site.Languages,
		"default_lang": d.Cfg.Site.DefaultLang,
		"theme":        d.Cfg.Site.Theme,
	})
}
