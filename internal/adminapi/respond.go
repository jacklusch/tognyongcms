package adminapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"dulizhan/internal/errs"
)

func respondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": data})
}

// fail 把领域错误映射为 HTTP 状态：ErrNotFound→404 / ErrForbidden→403 / ErrValidation→422 / 其他→500。
func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "资源不存在"})
	case errors.Is(err, errs.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "message": "无权限执行此操作"})
	case errors.Is(err, errs.ErrValidation):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": http.StatusUnprocessableEntity, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "服务器内部错误"})
	}
}

// unauthorized 统一 401（final-fix Finding 1）。
func unauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "未登录或会话已过期"})
}

// mustID 解析路径 :id，非法则写 422 并返回 false（final-fix Finding 5）。
func mustID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": http.StatusUnprocessableEntity, "message": "ID 不合法"})
		return 0, false
	}
	return id, true
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": msg})
}
