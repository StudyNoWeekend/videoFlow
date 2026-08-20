package logic

import (
	"context"

	"video-captions/bootstrap"
	"video-captions/internal/dto/res"
)

// VersionLogic 版本号业务逻辑
type VersionLogic struct{}

// NewVersionLogic 创建版本号 logic 实例
func NewVersionLogic() *VersionLogic {
	return &VersionLogic{}
}

// GetVersion 获取应用版本号
func (l *VersionLogic) GetVersion(ctx context.Context) *res.VersionRes {
	return &res.VersionRes{
		Version: bootstrap.AppVersion,
	}
}
