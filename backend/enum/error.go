package enum

import "fmt"

// BizError 业务错误，统一包含错误码、提示信息、HTTP 状态码
type BizError struct {
	Code     int
	Msg      string
	HttpCode int
}

// Error 实现 error 接口
func (e *BizError) Error() string {
	return fmt.Sprintf("code=%d, msg=%s", e.Code, e.Msg)
}

// WithMsg 复制当前业务错误并替换提示信息
func (e *BizError) WithMsg(msg string) *BizError {
	return &BizError{
		Code:     e.Code,
		Msg:      msg,
		HttpCode: e.HttpCode,
	}
}

// NewBizError 创建业务错误
func NewBizError(code int, msg string, httpCode int) *BizError {
	return &BizError{
		Code:     code,
		Msg:      msg,
		HttpCode: httpCode,
	}
}

// 通用错误码
var (
	ErrInvalidParam   = NewBizError(10040001, "请求参数错误", 400)
	ErrUnauthorized   = NewBizError(10040101, "未认证，请先登录", 401)
	ErrForbidden      = NewBizError(10040301, "无权限访问该资源", 403)
	ErrNotFound       = NewBizError(10040401, "资源不存在", 404)
	ErrInternalServer = NewBizError(10050001, "系统内部错误", 500)
	ErrDatabase       = NewBizError(10050002, "数据库操作失败", 500)
	ErrThirdParty     = NewBizError(10050003, "第三方服务调用失败", 500)

	// 视频/音频处理错误
	ErrFFmpegNotFound  = NewBizError(10060001, "未找到 ffmpeg，请安装后重试", 500)
	ErrFFmpegExecute   = NewBizError(10060002, "ffmpeg 执行失败", 500)
	ErrVideoNotFound   = NewBizError(10060003, "视频文件不存在", 400)
	ErrDurationParse   = NewBizError(10060004, "解析视频时长失败", 500)
	ErrAudioExtract    = NewBizError(10060005, "音频提取失败", 500)
	ErrCreateOutputDir = NewBizError(10060006, "创建输出目录失败", 500)
	ErrSubtitleBurn    = NewBizError(10060007, "字幕写入视频失败", 500)

	// 任务相关错误
	ErrTaskNotFound            = NewBizError(10070001, "任务不存在", 404)
	ErrTaskNotFailed           = NewBizError(10070002, "只有失败任务可以重试", 400)
	ErrTaskRunningCannotDelete = NewBizError(10070003, "运行中的任务不能删除", 400)
	ErrTaskNotCompleted        = NewBizError(10070004, "任务尚未完成，无法导出字幕", 400)
	ErrTaskNoResult            = NewBizError(10070005, "任务暂无可用字幕结果", 404)
	ErrSubtitleNotCompleted    = NewBizError(10040002, "请先完成字幕生成", 400)

	// 字幕导出相关错误
	ErrInvalidSubtitleFormat = NewBizError(10080001, "字幕格式参数错误，仅支持 srt/vtt/ass", 400)
)
