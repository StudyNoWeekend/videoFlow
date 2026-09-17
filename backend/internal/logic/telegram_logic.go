package logic

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"rsc.io/qr"

	"video-captions/enum"
	"video-captions/internal/dto/res"
	"video-captions/internal/telegram"
)

// TelegramLogic Telegram 登录与状态业务逻辑
type TelegramLogic struct{}

// NewTelegramLogic 创建 Telegram 业务逻辑实例
func NewTelegramLogic() *TelegramLogic {
	return &TelegramLogic{}
}

// Status 返回当前 Telegram 连接与登录状态
func (l *TelegramLogic) Status(_ context.Context) *res.TelegramStatusRes {
	engine := telegram.Global()
	if engine == nil {
		return &res.TelegramStatusRes{}
	}

	snapshot := engine.LoginSnapshot()
	return &res.TelegramStatusRes{
		Configured:    engine.Configured(),
		Ready:         engine.IsReady(),
		Authenticated: engine.IsAuthenticated(),
		Account:       engine.Account(),
		LoginStatus:   snapshot.Status,
		QRURL:         snapshot.QRURL,
		Error:         snapshot.Error,
	}
}

// StartQRLogin 发起扫码登录并返回最新状态
func (l *TelegramLogic) StartQRLogin(ctx context.Context) (*res.TelegramStatusRes, error) {
	engine := telegram.Global()
	if engine == nil || !engine.Configured() {
		return nil, enum.ErrTelegramNotConfigured
	}

	if err := engine.StartQRLogin(); err != nil {
		if errors.Is(err, telegram.ErrNotReady) {
			return nil, enum.ErrTelegramNotReady
		}
		return nil, enum.ErrInvalidParam.WithMsg(err.Error())
	}
	return l.Status(ctx), nil
}

// Submit2FA 提交两步验证密码
func (l *TelegramLogic) Submit2FA(_ context.Context, password string) error {
	engine := telegram.Global()
	if engine == nil || !engine.Configured() {
		return enum.ErrTelegramNotConfigured
	}

	if err := engine.Submit2FA(password); err != nil {
		return enum.ErrInvalidParam.WithMsg(err.Error())
	}
	return nil
}

// Logout 退出登录并清理本地会话
func (l *TelegramLogic) Logout(ctx context.Context) error {
	engine := telegram.Global()
	if engine == nil {
		return nil
	}

	if err := engine.Logout(ctx); err != nil {
		return enum.ErrInternalServer.WithMsg(err.Error())
	}
	return nil
}

// QRCode 生成当前登录二维码的图片。
// 二维码在服务端渲染成 PNG 并以 data URI 返回，前端无需引入二维码库。
func (l *TelegramLogic) QRCode(_ context.Context) (*res.TelegramQRRes, error) {
	engine := telegram.Global()
	if engine == nil || !engine.Configured() {
		return nil, enum.ErrTelegramNotConfigured
	}

	qrURL := engine.LoginSnapshot().QRURL
	if qrURL == "" {
		return nil, enum.ErrInvalidParam.WithMsg("当前没有可用的登录二维码，请先发起扫码登录")
	}

	code, err := qr.Encode(qrURL, qr.M)
	if err != nil {
		return nil, enum.ErrInternalServer.WithMsg(fmt.Sprintf("生成登录二维码失败: %v", err))
	}

	return &res.TelegramQRRes{
		QRURL:   qrURL,
		QRImage: "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG()),
	}, nil
}
