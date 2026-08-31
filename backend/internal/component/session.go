package component

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// sessionRetention 会话进入终态后在内存中的保留时长，超时后回收，避免 sessions 无限增长
const sessionRetention = 10 * time.Minute

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
		if s.GetStatus() == "running" {
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
	if ok && session.GetStatus() == "running" {
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

	// 包装 events channel: 记录所有发送的事件到 History。
	// 转发到 session.Events 时带超时：无 SSE 消费者（或消费者卡住）时丢弃事件，
	// 保证安装流程不被阻塞；完整事件已写入 History，重连时可回放。
	events := make(chan ProgressEvent, 100)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for e := range events {
			session.AppendHistory(e)
			select {
			case session.Events <- e:
			case <-time.After(5 * time.Second):
			}
		}
	}()

	var err error
	switch session.Params.Action {
	case "uninstall":
		err = uninstallComponent(ctx, session.ID, session.Params.ComponentType, events)
	default:
		switch session.Params.ComponentType {
		case ComponentLada:
			err = installLada(ctx, session.ID, session.Params, events)
		case ComponentVideo2X:
			err = installVideo2X(ctx, session.ID, session.Params, events)
		case ComponentFFmpeg:
			err = installFFmpeg(ctx, session.ID, session.Params, events)
		default:
			err = fmt.Errorf("unsupported component type: %s", session.Params.ComponentType)
		}
	}

	// 关闭 events channel 通知转发协程退出
	close(events)
	// 等待转发协程处理完所有已发送的事件后再关闭 session.Events
	wg.Wait()

	if err != nil {
		session.SetStatus("failed")
	} else {
		session.SetStatus("completed")
	}

	// Session 完成后保存历史记录到 history map，按组件类型保留
	session.historyMu.RLock()
	historyCopy := make([]ProgressEvent, len(session.History))
	copy(historyCopy, session.History)
	session.historyMu.RUnlock()
	sm.mu.Lock()
	sm.history[session.Params.ComponentType] = historyCopy
	sm.mu.Unlock()

	// 终态保留一段时间供前端查询，之后从内存回收，避免 sessions 无限增长
	go func() {
		time.Sleep(sessionRetention)
		sm.mu.Lock()
		delete(sm.sessions, session.ID)
		sm.mu.Unlock()
	}()
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
