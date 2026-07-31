package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"video-captions/utils/logger"

	"go.uber.org/zap"
)

// OllamaConfig Ollama 配置
type OllamaConfig struct {
	URL            string
	Model          string
	PromptTemplate string
}

// OllamaExecutor Ollama 翻译执行器
type OllamaExecutor struct {
	config     OllamaConfig
	httpClient *http.Client
}

// ollamaRequest Ollama API 请求结构
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// ollamaResponse Ollama API 响应结构
type ollamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"` // 生成的文本
	Done      bool   `json:"done"`
	// 其他字段在实际响应中可能存在，但我们主要需要 response 字段
}

// NewOllamaExecutor 创建 Ollama 翻译执行器
func NewOllamaExecutor(config OllamaConfig) *OllamaExecutor {
	return &OllamaExecutor{
		config: config,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // 设置 5 分钟超时
		},
	}
}

// Translate 翻译 SRT 内容（保留以兼容旧调用）
func (e *OllamaExecutor) Translate(ctx context.Context, srtContent string) (string, error) {
	prompt := e.buildPrompt(srtContent)
	result, err := e.callOllama(ctx, prompt)
	if err != nil {
		return "", err
	}
	return result, nil
}

// TranslateTexts 批量翻译文本列表。
// 将所有文本用换行符拼接后一次性发送给模型，要求模型逐行翻译并保持行数一致。
// 返回的文本列表与输入列表一一对应。
func (e *OllamaExecutor) TranslateTexts(ctx context.Context, texts []string) ([]string, error) {
	if len(texts) == 0 {
		return []string{}, nil
	}

	// 用换行符拼接所有文本，每行一条
	joinedTexts := strings.Join(texts, "\n")

	// 构建提示词
	prompt := e.buildPrompt(joinedTexts)

	// 调用 Ollama
	translated, err := e.callOllama(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// 按换行符拆分翻译结果
	translatedLines := strings.Split(translated, "\n")

	// 如果翻译返回的行数与输入不一致，尝试尽力匹配
	if len(translatedLines) != len(texts) {
		logger.Logger.Warn("翻译行数不匹配",
			zap.Int("input_lines", len(texts)),
			zap.Int("output_lines", len(translatedLines)))
		// 如果翻译结果行数少于输入，用原文填充缺失的行
		for len(translatedLines) < len(texts) {
			translatedLines = append(translatedLines, texts[len(translatedLines)])
		}
		// 如果翻译结果行数多于输入，截断
		if len(translatedLines) > len(texts) {
			translatedLines = translatedLines[:len(texts)]
		}
	}

	// 清理每行末尾的空白字符
	for i, line := range translatedLines {
		translatedLines[i] = strings.TrimSpace(line)
	}

	logger.Logger.Info("批量翻译完成", zap.Int("text_count", len(texts)))
	return translatedLines, nil
}

// callOllama 调用 Ollama API 并返回生成的文本
func (e *OllamaExecutor) callOllama(ctx context.Context, prompt string) (string, error) {
	// 构建请求
	req := ollamaRequest{
		Model:  e.config.Model,
		Prompt: prompt,
		Stream: false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		logger.Logger.Error("序列化 Ollama 请求失败", zap.Error(err))
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.config.URL, bytes.NewReader(reqBody))
	if err != nil {
		logger.Logger.Error("创建 HTTP 请求失败", zap.Error(err))
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		logger.Logger.Error("发送请求到 Ollama 失败", zap.Error(err))
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Logger.Error("Ollama API 返回非 200 状态码",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)))
		return "", fmt.Errorf("ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		logger.Logger.Error("解析 Ollama 响应失败", zap.Error(err))
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// 清理响应内容（可能包含多余的空白字符）
	translated := strings.TrimSpace(ollamaResp.Response)
	if translated == "" {
		logger.Logger.Error("Ollama 返回空翻译结果")
		return "", fmt.Errorf("empty translation result")
	}

	return translated, nil
}

// buildPrompt 构建翻译提示词
func (e *OllamaExecutor) buildPrompt(srtContent string) string {
	// 如果提示词模板中有 {content} 占位符，则替换
	if strings.Contains(e.config.PromptTemplate, "{content}") {
		return strings.Replace(e.config.PromptTemplate, "{content}", srtContent, 1)
	}
	// 否则直接拼接
	return e.config.PromptTemplate + "\n\n" + srtContent
}
