package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"video-captions/bootstrap"
	"video-captions/utils/response"
)

// SetupRouter 初始化并注册所有路由
func SetupRouter(cfg *bootstrap.AppConfigHTTP) *gin.Engine {
	if bootstrap.Config != nil && bootstrap.Config.App.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 全局中间件
	r.Use(gin.Recovery())
	r.Use(traceMiddleware())
	r.Use(corsMiddleware())
	r.Use(loggerMiddleware())

	// 注册健康检查路由
	RegisterHealthRouter(r)

	// 注册视频管理路由
	RegisterVideoRouter(r)

	// 注册任务管理路由
	RegisterTaskRouter(r)

	// 注册运行时配置路由
	RegisterSettingRouter(r)

	// 注册组件管理路由
	RegisterComponentRouter(r)

	// 兜底 404
	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, 10040401, "接口不存在", http.StatusNotFound)
	})

	return r
}

// traceMiddleware 注入或透传 trace_id
func traceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = generateTraceID()
		}
		c.Set("trace_id", traceID)
		c.Writer.Header().Set("X-Trace-ID", traceID)
		c.Next()
	}
}

// generateTraceID 生成简易 trace_id
func generateTraceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// corsMiddleware 开发环境 CORS 配置
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Trace-ID")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// loggerMiddleware 简易请求日志中间件
func loggerMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" \"%s\" trace_id=%s\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC3339),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
			param.Keys["trace_id"],
		)
	})
}
