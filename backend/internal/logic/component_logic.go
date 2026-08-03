package logic

import (
	"context"
	"sync"

	"video-captions/internal/component"
	"video-captions/internal/dto/req"
	"video-captions/internal/dto/res"
)

// ComponentLogic 组件管理业务逻辑
type ComponentLogic struct {
	detector       *component.Detector
	sessionManager *component.SessionManager
}

var (
	componentLogic     *ComponentLogic
	componentLogicOnce sync.Once
)

// NewComponentLogic 创建组件管理 logic 实例（单例）
func NewComponentLogic() *ComponentLogic {
	componentLogicOnce.Do(func() {
		componentLogic = &ComponentLogic{
			detector:       component.NewDetector(),
			sessionManager: component.NewSessionManager(),
		}
	})
	return componentLogic
}

// ListComponents 获取所有组件状态（合并活跃 session）
func (l *ComponentLogic) ListComponents(ctx context.Context) (*res.ComponentListRes, error) {
	infos := l.detector.DetectAll(ctx)

	// 将运行中 session 的状态覆盖到检测结果上
	activeSessions := l.sessionManager.GetActiveSessions()
	for _, s := range activeSessions {
		for i, info := range infos {
			if info.Type == s.Params.ComponentType {
				infos[i].Status = component.StatusInstalling
				break
			}
		}
	}

	components := make([]res.ComponentInfoRes, len(infos))
	for i, info := range infos {
		components[i] = res.ComponentInfoRes{
			Type:        string(info.Type),
			Name:        info.Name,
			Status:      string(info.Status),
			Version:     info.Version,
			ErrorMsg:    info.ErrorMsg,
			Description: info.Description,
			NeedsDocker: info.NeedsDocker,
		}
	}
	return &res.ComponentListRes{Components: components}, nil
}

// InstallComponent 安装组件
func (l *ComponentLogic) InstallComponent(ctx context.Context, req *req.ComponentInstallReq) (*res.ComponentInstallRes, error) {
	params := component.InstallParams{
		ComponentType: component.ComponentType(req.ComponentType),
		ASREngine:     req.ASREngine,
		ASRModel:      req.ASRModel,
		ASRDevice:     req.ASRDevice,
		HFToken:       req.HFToken,
	}
	session := l.sessionManager.CreateSession(params)
	return &res.ComponentInstallRes{SessionID: session.ID}, nil
}

// UninstallComponent 卸载组件
func (l *ComponentLogic) UninstallComponent(ctx context.Context, req *req.ComponentUninstallReq) (*res.ComponentInstallRes, error) {
	params := component.InstallParams{
		ComponentType: component.ComponentType(req.ComponentType),
		Action:        "uninstall",
	}
	session := l.sessionManager.CreateSession(params)
	return &res.ComponentInstallRes{SessionID: session.ID}, nil
}

// GetSession 获取指定 ID 的安装会话
func (l *ComponentLogic) GetSession(sessionID string) *component.InstallSession {
	return l.sessionManager.GetSession(sessionID)
}

// GetActiveSessionByComponent 获取指定组件正在运行的 session
func (l *ComponentLogic) GetActiveSessionByComponent(componentType component.ComponentType) *component.InstallSession {
	sessions := l.sessionManager.GetActiveSessions()
	for _, s := range sessions {
		if s.Params.ComponentType == componentType {
			return s
		}
	}
	return nil
}

// GetInstallHistory 获取指定组件类型的历史安装事件
func (l *ComponentLogic) GetInstallHistory(componentType component.ComponentType) []component.ProgressEvent {
	return l.sessionManager.GetHistory(componentType)
}
