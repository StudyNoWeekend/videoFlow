package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"video-captions/internal/subtitle"
	"video-captions/utils/logger"

	"go.uber.org/zap"
)

// ASRClient 直接对接 HTTP ASR 服务（Whisper ASR Webservice）
type ASRClient struct {
	url            string
	language       string
	vadFilter      bool
	task           string
	encode         bool
	initialPrompt  string
	wordTimestamps bool
	output         string
	client         *http.Client
}

// NewASRClient 创建 ASR 客户端
func NewASRClient(url, language string, vadFilter bool) *ASRClient {
	return &ASRClient{
		url:       url,
		language:  language,
		vadFilter: vadFilter,
		task:      "transcribe",
		encode:    true,
		output:    "json",
		client:    &http.Client{Timeout: 10 * time.Minute},
	}
}

// ASRClientOptions ASR 客户端可选参数
type ASRClientOptions struct {
	Task           string
	Encode         bool
	InitialPrompt  string
	WordTimestamps bool
	Output         string
}

// NewASRClientWithOpts 使用完整参数创建 ASR 客户端
func NewASRClientWithOpts(url, language string, vadFilter bool, opts ASRClientOptions) *ASRClient {
	c := &ASRClient{
		url:            url,
		language:       language,
		vadFilter:      vadFilter,
		task:           opts.Task,
		encode:         opts.Encode,
		initialPrompt:  opts.InitialPrompt,
		wordTimestamps: opts.WordTimestamps,
		output:         opts.Output,
		client:         &http.Client{Timeout: 10 * time.Minute},
	}
	if c.task == "" {
		c.task = "transcribe"
	}
	if c.output == "" {
		c.output = "json"
	}
	return c
}

// asrResponse ASR 服务响应结构（优先尝试）
type asrResponse struct {
	Segments []Segment `json:"segments"`
	Text     string    `json:"text"`
}

// Transcribe 通过 multipart/form-data 上传音频并获取转录结果
func (c *ASRClient) Transcribe(ctx context.Context, audioPath string) ([]Segment, error) {
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("打开音频文件失败: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("audio_file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("创建表单文件失败: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("写入表单文件失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 multipart writer 失败: %w", err)
	}

	reqURL, err := url.Parse(c.url)
	if err != nil {
		return nil, fmt.Errorf("解析 ASR URL 失败: %w", err)
	}
	query := reqURL.Query()
	query.Set("output", c.output)
	query.Set("language", c.language)
	query.Set("vad_filter", strconv.FormatBool(c.vadFilter))
	query.Set("encode", strconv.FormatBool(c.encode))
	if c.task != "" {
		query.Set("task", c.task)
	}
	if c.initialPrompt != "" {
		query.Set("initial_prompt", c.initialPrompt)
	}
	query.Set("word_timestamps", strconv.FormatBool(c.wordTimestamps))
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), &body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	logger.Logger.Info("调用 ASR 服务",
		zap.String("url", reqURL.String()),
		zap.String("audio", audioPath),
		zap.String("language", c.language),
		zap.Bool("vad_filter", c.vadFilter),
		zap.String("task", c.task),
		zap.Bool("encode", c.encode),
		zap.String("initial_prompt", c.initialPrompt),
		zap.Bool("word_timestamps", c.wordTimestamps),
		zap.String("output", c.output),
	)

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Logger.Error("ASR 请求失败", zap.Error(err), zap.String("url", reqURL.String()))
		return nil, fmt.Errorf("ASR 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Logger.Error("ASR 返回非 200", zap.Int("status", resp.StatusCode), zap.String("body", string(respBody)))
		return nil, fmt.Errorf("ASR 返回错误状态码: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return c.parseResponse(respBody)
}

// parseResponse 根据输出格式解析 ASR 响应
func (c *ASRClient) parseResponse(body []byte) ([]Segment, error) {
	bodyStr := string(body)

	// 根据 output 配置选择解析方式
	switch c.output {
	case "srt":
		segs := subtitle.ParseSRT(bodyStr)
		if len(segs) > 0 {
			result := subtitleSegsToASR(segs)
			logger.Logger.Info("ASR 转录成功（SRT 格式）", zap.Int("segments", len(result)))
			return result, nil
		}
		logger.Logger.Error("解析 SRT 响应失败", zap.String("body", bodyStr))
		return nil, fmt.Errorf("解析 SRT 响应失败")

	case "vtt":
		segs := subtitle.ParseSRT(bodyStr) // VTT 时间轴格式与 SRT 类似，复用解析
		if len(segs) > 0 {
			result := subtitleSegsToASR(segs)
			logger.Logger.Info("ASR 转录成功（VTT 格式）", zap.Int("segments", len(result)))
			return result, nil
		}
		logger.Logger.Error("解析 VTT 响应失败", zap.String("body", bodyStr))
		return nil, fmt.Errorf("解析 VTT 响应失败")

	case "txt":
		text := strings.TrimSpace(bodyStr)
		if text != "" {
			logger.Logger.Info("ASR 转录成功（TXT 格式）")
			return []Segment{{Start: 0, End: 0, Text: text}}, nil
		}
		return nil, fmt.Errorf("ASR 返回空文本")

	case "json":
		// JSON 格式可能返回 {"segments":[...], "text":"..."} 或直接返回 [...]
		var obj asrResponse
		if err := json.Unmarshal(body, &obj); err == nil {
			if len(obj.Segments) > 0 {
				logger.Logger.Info("ASR 转录成功（JSON 格式）", zap.Int("segments", len(obj.Segments)))
				return obj.Segments, nil
			}
			if obj.Text != "" {
				logger.Logger.Info("ASR 转录成功（JSON 整体文本）")
				return []Segment{{Start: 0, End: 0, Text: obj.Text}}, nil
			}
		}

		var segments []Segment
		if err := json.Unmarshal(body, &segments); err == nil && len(segments) > 0 {
			logger.Logger.Info("ASR 转录成功（JSON 数组）", zap.Int("segments", len(segments)))
			return segments, nil
		}

		// JSON 解析失败，尝试按 SRT 解析（服务端可能忽略了 output 参数）
		if segs := subtitle.ParseSRT(bodyStr); len(segs) > 0 {
			result := subtitleSegsToASR(segs)
			logger.Logger.Info("ASR 响应非 JSON，按 SRT 解析成功", zap.Int("segments", len(result)))
			return result, nil
		}

		logger.Logger.Error("解析 ASR JSON 响应失败", zap.String("body", bodyStr))
		return nil, fmt.Errorf("解析 ASR 响应失败")

	default:
		// 未知格式，自动检测
		if segs := subtitle.ParseSRT(bodyStr); len(segs) > 0 {
			result := subtitleSegsToASR(segs)
			logger.Logger.Info("ASR 转录成功（自动检测 SRT）", zap.Int("segments", len(result)))
			return result, nil
		}
		text := strings.TrimSpace(bodyStr)
		if text != "" {
			return []Segment{{Start: 0, End: 0, Text: text}}, nil
		}
		return nil, fmt.Errorf("无法解析 ASR 响应: %s", bodyStr)
	}
}

// subtitleSegsToASR 将 subtitle.Segment 转换为 asr.Segment
func subtitleSegsToASR(segs []subtitle.Segment) []Segment {
	result := make([]Segment, len(segs))
	for i, s := range segs {
		result[i] = Segment{Start: s.Start, End: s.End, Text: s.Text}
	}
	return result
}
