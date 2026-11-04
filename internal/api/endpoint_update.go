package api

import (
	"net/http"
	"strconv"

	"NodePassDash/internal/nodepass"

	"github.com/gin-gonic/gin"
)

// endpointUpdateID 解析路由里的 endpoint id，失败时直接写出 400。
func endpointUpdateID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid endpoint ID",
		})
		return 0, false
	}
	return id, true
}

// HandleCheckEndpointUpdate 让主控检查自己的最新版本。
// GET /api/endpoints/:id/update/check
func (h *EndpointHandler) HandleCheckEndpointUpdate(c *gin.Context) {
	id, ok := endpointUpdateID(c)
	if !ok {
		return
	}

	result, err := nodepass.CheckEndpointUpdate(id)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// HandleApplyEndpointUpdate 触发主控下载并替换自身二进制。主控立即返回，
// 前端随后轮询 /update/status。
// POST /api/endpoints/:id/update/apply
func (h *EndpointHandler) HandleApplyEndpointUpdate(c *gin.Context) {
	id, ok := endpointUpdateID(c)
	if !ok {
		return
	}

	if err := nodepass.ApplyEndpointUpdate(id); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "update started",
	})
}

// HandleGetEndpointUpdateStatus 返回主控的自更新进度。
// GET /api/endpoints/:id/update/status
func (h *EndpointHandler) HandleGetEndpointUpdateStatus(c *gin.Context) {
	id, ok := endpointUpdateID(c)
	if !ok {
		return
	}

	status, err := nodepass.GetEndpointUpdateStatus(id)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}
