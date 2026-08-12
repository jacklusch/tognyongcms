package adminapi

import (
	"github.com/gin-gonic/gin"

	"dulizhan/internal/auth"
	"dulizhan/internal/content"
)

// HandleStats 仪表盘统计：GET /api/stats
func (d *Deps) HandleStats(c *gin.Context) {
	us, ok := auth.UserFromContext(c)
	if !ok || us == nil {
		unauthorized(c)
		return
	}
	st, err := d.Content.Stats(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	// C3 修复：recent 含内容全文，按用户可读类型过滤 by_type/recent（admin/editor 全过；author 无类型级权限则剔除）。
	filtered := content.Stats{ByType: make([]content.TypeStat, 0, len(st.ByType)), Recent: []content.Entry{}}
	for _, ts := range st.ByType {
		if us.HasPerm("content.read." + ts.TypeName) {
			filtered.ByType = append(filtered.ByType, ts)
		}
	}
	for _, r := range st.Recent {
		if us.HasPerm("content.read." + r.TypeName) {
			filtered.Recent = append(filtered.Recent, r)
		}
	}
	respondOK(c, filtered)
}
