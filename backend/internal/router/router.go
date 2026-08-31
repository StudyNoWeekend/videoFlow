package router

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"

	"video-captions/bootstrap"
	"video-captions/internal/logic"
	"video-captions/internal/middleware"
	"video-captions/utils/logger"
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

	// ---- 公开路由（无需认证） ----
	// 健康检查
	RegisterHealthRouter(r)
	// 认证相关（登录、初始化、验证码等）
	RegisterAuthRouter(r)
	// 版本号（公开接口）
	RegisterVersionRouter(r)
	// 组件安装进度 SSE（公开接口，EventSource 不支持自定义 header，无法传 token）
	RegisterComponentProgressRouter(r)

	// ---- 需认证路由 ----
	authLogic := logic.NewAuthLogic()
	api := r.Group("/api/v1")
	api.Use(middleware.AuthRequired(authLogic.ValidateToken))
	{
		RegisterVideoRouter(api)
		RegisterTaskRouter(api)
		RegisterSettingRouter(api)
		RegisterComponentRouter(api)
		RegisterDownloadRouter(api)
	}

	// 兜底 404
	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, 10040401, "接口不存在", http.StatusNotFound)
	})

	return r
}

// traceMiddleware 注入或透传 trace_id。
// 同时写入 gin.Keys（响应头、访问日志）与 request context（透传给 logic/基础设施层日志）。
func traceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = generateTraceID()
		}
		c.Set("trace_id", traceID)
		c.Writer.Header().Set("X-Trace-ID", traceID)
		c.Request = c.Request.WithContext(logger.ContextWithTraceID(c.Request.Context(), traceID))
		c.Next()
	}
}

// generateTraceID 生成简易 trace_id
func generateTraceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// corsMiddleware CORS 配置。
// 生产环境由 nginx 同源提供服务，不返回任何 CORS 头；
// 开发环境仅放行本机来源（前端 dev server），杜绝任意 Origin 反射。
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		isDev := bootstrap.Config != nil && bootstrap.Config.App.Env != "prod"
		if isDev && isLocalOrigin(origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Trace-ID")
			c.Writer.Header().Set("Vary", "Origin")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// isLocalOrigin 判断 Origin 是否为本机开发来源（localhost / 127.0.0.1 / ::1）
func isLocalOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
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
