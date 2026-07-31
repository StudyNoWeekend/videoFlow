package asr

import "context"

// Segment 表示 ASR 转录得到的一个片段
// Start 与 End 单位为秒
type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// ASRProvider 定义 ASR 引擎统一接口
type ASRProvider interface {
	// Transcribe 对指定音频文件进行转录，返回带时间戳的文本片段
	Transcribe(ctx context.Context, audioPath string) ([]Segment, error)
}
