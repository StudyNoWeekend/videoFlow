package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"video-captions/enum"
)

// Response 统一接口响应结构
type Response struct {
	Data    interface{} `json:"data"`
	Code    int         `json:"code"`
	Msg     string      `json:"msg"`
	TraceID string      `json:"trace_id"`
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Data:    data,
		Code:    0,
		Msg:     "success",
		TraceID: c.GetString("trace_id"),
	})
}

// Fail 返回失败响应
func Fail(c *gin.Context, code int, msg string, httpCode int) {
	c.JSON(httpCode, Response{
		Data:    nil,
		Code:    code,
		Msg:     msg,
		TraceID: c.GetString("trace_id"),
	})
}

// FailByBizError 根据业务错误返回失败响应
func FailByBizError(c *gin.Context, err *enum.BizError) {
	Fail(c, err.Code, err.Msg, err.HttpCode)
}
