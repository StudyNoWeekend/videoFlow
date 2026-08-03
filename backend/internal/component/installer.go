package component

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// InstallParams 安装参数
type InstallParams struct {
	ComponentType ComponentType `json:"component_type"`
	Action        string        `json:"action"` // install, reinstall, uninstall
	// Whisper ASR 参数
	ASREngine string `json:"asr_engine,omitempty"` // openai_whisper, faster_whisper, whisperx
	ASRModel  string `json:"asr_model,omitempty"`  // base, small, medium, large-v3...
	ASRDevice string `json:"asr_device,omitempty"` // cpu, cuda
	HFToken   string `json:"hf_token,omitempty"`   // for whisperx
}

// ProgressEvent 进度事件
type ProgressEvent struct {
	SessionID string `json:"session_id"`
	Step      string `json:"step"`
	Log       string `json:"log"`
	Status    string `json:"status"` // running, completed, failed
	Error     string `json:"error,omitempty"`
}

// InstallSession 安装会话
type InstallSession struct {
	ID        string
	Params    InstallParams
	Status    string          // running, completed, failed
	History   []ProgressEvent // 所有历史事件，用于重连时回放
	historyMu sync.RWMutex    // 保护 History 的并发读写
	Events    chan ProgressEvent
	Done      chan struct{}
	Cancel    context.CancelFunc
}

// AppendHistory 追加一个事件到历史记录（线程安全）
func (s *InstallSession) AppendHistory(e ProgressEvent) {
	s.historyMu.Lock()
	s.History = append(s.History, e)
	s.historyMu.Unlock()
}

// ReplayHistory 将历史事件通过 SSE 发送给客户端（用于重连时回放）
func (s *InstallSession) ReplayHistory(w io.Writer, f http.Flusher) {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()
	for _, e := range s.History {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", e.ToJSON())
	}
	if f != nil {
		f.Flush()
	}
}

// ToJSON 将 ProgressEvent 序列化为 JSON 字符串
func (e ProgressEvent) ToJSON() string {
	b, _ := json.Marshal(e)
	return string(b)
}
