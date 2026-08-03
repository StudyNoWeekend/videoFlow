package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"video-captions/enum"
	"video-captions/internal/component"
	"video-captions/internal/dto/req"
	"video-captions/internal/logic"
	"video-captions/utils/response"
)

// ComponentController 组件管理控制器
type ComponentController struct {
	componentLogic *logic.ComponentLogic
}

// NewComponentController 创建组件管理控制器
func NewComponentController() *ComponentController {
	return &ComponentController{
		componentLogic: logic.NewComponentLogic(),
	}
}

// ListComponents GET /api/v1/components
func (ctl *ComponentController) ListComponents(c *gin.Context) {
	res, err := ctl.componentLogic.ListComponents(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// InstallComponent POST /api/v1/components/install
func (ctl *ComponentController) InstallComponent(c *gin.Context) {
	var installReq req.ComponentInstallReq
	if err := c.ShouldBindJSON(&installReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}
	res, err := ctl.componentLogic.InstallComponent(c.Request.Context(), &installReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// ReinstallComponent POST /api/v1/components/reinstall
func (ctl *ComponentController) ReinstallComponent(c *gin.Context) {
	var installReq req.ComponentInstallReq
	if err := c.ShouldBindJSON(&installReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}
	res, err := ctl.componentLogic.InstallComponent(c.Request.Context(), &installReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// UninstallComponent POST /api/v1/components/uninstall
func (ctl *ComponentController) UninstallComponent(c *gin.Context) {
	var uninstallReq req.ComponentUninstallReq
	if err := c.ShouldBindJSON(&uninstallReq); err != nil {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}
	res, err := ctl.componentLogic.UninstallComponent(c.Request.Context(), &uninstallReq)
	if err != nil {
		HandleError(c, err)
		return
	}
	response.Success(c, res)
}

// GetActiveSession GET /api/v1/components/active-session/:component_type
// 获取指定组件的运行中 session（用于刷新页面后重新连接 SSE）
func (ctl *ComponentController) GetActiveSession(c *gin.Context) {
	compType := c.Param("component_type")
	if compType == "" {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	session := ctl.componentLogic.GetActiveSessionByComponent(component.ComponentType(compType))
	if session == nil {
		response.Success(c, gin.H{"session_id": ""})
		return
	}
	response.Success(c, gin.H{"session_id": session.ID})
}

// InstallProgress SSE /api/v1/components/install/progress/:session_id
func (ctl *ComponentController) InstallProgress(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	session := ctl.componentLogic.GetSession(sessionID)
	if session == nil {
		response.FailByBizError(c, enum.ErrNotFound)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.FailByBizError(c, enum.ErrInternalServer)
		return
	}

	// Send initial event
	_, _ = fmt.Fprintf(c.Writer, "data: {\"session_id\":\"%s\",\"status\":\"connected\"}\n\n", sessionID)
	flusher.Flush()

	// Replay history events so reconnecting clients see the full log
	session.ReplayHistory(c.Writer, flusher)

	for {
		select {
		case event, ok := <-session.Events:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", event.ToJSON())
			flusher.Flush()

			if event.Status == "completed" || event.Status == "failed" {
				return
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}

// GetInstallHistory GET /api/v1/components/install/history/:component_type
func (ctl *ComponentController) GetInstallHistory(c *gin.Context) {
	compType := c.Param("component_type")
	if compType == "" {
		response.FailByBizError(c, enum.ErrInvalidParam)
		return
	}

	events := ctl.componentLogic.GetInstallHistory(component.ComponentType(compType))
	response.Success(c, gin.H{"events": events})
}
