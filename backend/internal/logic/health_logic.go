package logic

import (
	"context"

	"video-captions/internal/dto/res"
	"video-captions/internal/model"
)

// HealthLogic 健康检查业务逻辑
type HealthLogic struct{}

// NewHealthLogic 创建健康检查 logic 实例
func NewHealthLogic() *HealthLogic {
	return &HealthLogic{}
}

// Health 存活检查
func (l *HealthLogic) Health() *res.HealthRes {
	return &res.HealthRes{Status: "ok"}
}

// Ready 就绪检查，验证数据库连接可用性
func (l *HealthLogic) Ready(ctx context.Context) (*res.ReadyRes, error) {
	if err := model.CheckDBHealth(ctx); err != nil {
		return nil, err
	}
	return &res.ReadyRes{Status: "ready"}, nil
}
