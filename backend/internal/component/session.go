package component

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// SessionManager 管理安装会话
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*InstallSession
	// 已完成 session 的历史记录，按组件类型保留最近一次
	history map[ComponentType][]ProgressEvent
}

// NewSessionManager 创建新的会话管理器
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*InstallSession),
		history:  make(map[ComponentType][]ProgressEvent),
	}
}

// CreateSession 创建新的安装会话
func (sm *SessionManager) CreateSession(params InstallParams) *InstallSession {
	id := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())

	session := &InstallSession{
		ID:      id,
		Params:  params,
		Status:  "running",
		History: make([]ProgressEvent, 0),
		Events:  make(chan ProgressEvent, 100),
		Done:    make(chan struct{}),
		Cancel:  cancel,
	}

	sm.mu.Lock()
	sm.sessions[id] = session
	sm.mu.Unlock()

	// Start installation/uninstallation in background
	go sm.runInstallation(ctx, session)

	return session
}

// GetSession 获取指定 ID 的会话
func (sm *SessionManager) GetSession(id string) *InstallSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[id]
}

// GetActiveSessions 返回所有运行中的 session
func (sm *SessionManager) GetActiveSessions() []*InstallSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var active []*InstallSession
	for _, s := range sm.sessions {
		if s.Status == "running" {
			active = append(active, s)
		}
	}
	return active
}

// CancelSession 取消指定 ID 的会话
func (sm *SessionManager) CancelSession(id string) {
	sm.mu.RLock()
	session, ok := sm.sessions[id]
	sm.mu.RUnlock()
	if ok && session.Status == "running" {
		session.Cancel()
	}
}

// GetHistory 获取指定组件类型的历史安装事件
func (sm *SessionManager) GetHistory(componentType ComponentType) []ProgressEvent {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	events := sm.history[componentType]
	if events == nil {
		return []ProgressEvent{}
	}
	return events
}

func (sm *SessionManager) runInstallation(ctx context.Context, session *InstallSession) {
	defer func() {
		close(session.Events)
		close(session.Done)
	}()

	// 包装 events channel: 记录所有发送的事件到 History
	events := make(chan ProgressEvent, 100)
	go func() {
		for e := range events {
			session.AppendHistory(e)
			session.Events <- e
		}
	}()

	var err error
	switch session.Params.Action {
	case "uninstall":
		err = uninstallComponent(ctx, session.ID, session.Params.ComponentType, events)
	default:
		switch session.Params.ComponentType {
		case ComponentWhisperASR:
			err = installWhisper(ctx, session.ID, session.Params, events)
		case ComponentLada:
			err = installLada(ctx, session.ID, session.Params, events)
		case ComponentFFmpeg:
			err = installFFmpeg(ctx, session.ID, session.Params, events)
		default:
			err = fmt.Errorf("unsupported component type: %s", session.Params.ComponentType)
		}
	}

	if err != nil {
		session.Status = "failed"
	} else {
		session.Status = "completed"
	}

	// Session 完成后保存历史记录到 history map，按组件类型保留
	session.historyMu.RLock()
	historyCopy := make([]ProgressEvent, len(session.History))
	copy(historyCopy, session.History)
	session.historyMu.RUnlock()
	sm.mu.Lock()
	sm.history[session.Params.ComponentType] = historyCopy
	sm.mu.Unlock()
}

// sendEvent 发送进度事件
func sendEvent(sessionID string, events chan<- ProgressEvent, step string, log, status string) {
	events <- ProgressEvent{
		SessionID: sessionID,
		Step:      step,
		Log:       log,
		Status:    status,
	}
}

// sendError 发送错误事件
func sendError(sessionID string, events chan<- ProgressEvent, step string, err error) {
	events <- ProgressEvent{
		SessionID: sessionID,
		Step:      step,
		Status:    "failed",
		Error:     err.Error(),
	}
}
