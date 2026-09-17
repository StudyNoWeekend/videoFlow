package telegram

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"go.uber.org/zap"
)

// 登录状态
const (
	LoginStatusIdle    = ""
	LoginStatusPending = "pending"
	LoginStatusNeed2FA = "need_2fa"
	LoginStatusSuccess = "success"
	LoginStatusError   = "error"
)

// maxTwoFAAttempts 两步验证密码的最大尝试次数
const maxTwoFAAttempts = 3

// LoginSnapshot 登录状态的只读快照
type LoginSnapshot struct {
	Status string `json:"status"`
	QRURL  string `json:"qr_url,omitempty"`
	Error  string `json:"error,omitempty"`
}

// loginState 扫码登录状态机，内部自带锁
type loginState struct {
	mu     sync.RWMutex
	status string
	qrURL  string
	errMsg string
	// twoFA 承载用户提交的两步验证密码，缓冲 1 以允许先提交后等待
	twoFA chan string
}

// reset 清空登录状态（连接重建后二维码已失效）
func (s *loginState) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = LoginStatusIdle
	s.qrURL = ""
	s.errMsg = ""
	s.twoFA = nil
}

func (s *loginState) snapshot() LoginSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return LoginSnapshot{Status: s.status, QRURL: s.qrURL, Error: s.errMsg}
}

// begin 进入登录流程并返回密码通道
func (s *loginState) begin() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = LoginStatusPending
	s.qrURL = ""
	s.errMsg = ""
	s.twoFA = make(chan string, 1)
}

// inProgress 是否正处于登录流程中
func (s *loginState) inProgress() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.status == LoginStatusPending || s.status == LoginStatusNeed2FA
}

func (s *loginState) setQR(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.qrURL = url
}

func (s *loginState) setStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = status
}

// setNeed2FA 进入（或回到）等待两步验证密码的状态
func (s *loginState) setNeed2FA(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = LoginStatusNeed2FA
	if err != nil {
		s.errMsg = err.Error()
		return
	}
	s.errMsg = ""
}

func (s *loginState) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = LoginStatusError
	if err != nil {
		s.errMsg = err.Error()
	}
}

// waitPassword 阻塞等待用户提交两步验证密码
func (s *loginState) waitPassword(ctx context.Context) (string, error) {
	s.mu.RLock()
	ch := s.twoFA
	s.mu.RUnlock()

	if ch == nil {
		return "", errors.New("两步验证流程未初始化")
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case password := <-ch:
		return password, nil
	}
}

// LoginSnapshot 返回当前登录状态
func (e *Engine) LoginSnapshot() LoginSnapshot {
	return e.login.snapshot()
}

// StartQRLogin 开始扫码登录。
// 已登录或已在登录流程中时直接返回，保证接口幂等。
func (e *Engine) StartQRLogin() error {
	if !e.config().Enabled() {
		return errors.New("未配置 Telegram api_id/api_hash，请先在设置中填写")
	}
	if e.IsAuthenticated() || e.login.inProgress() {
		return nil
	}

	client, runCtx, err := e.runtimeClient()
	if err != nil {
		e.EnsureReconnect()
		return err
	}

	e.login.begin()
	go e.doQRLogin(runCtx, client)
	return nil
}

// Submit2FA 提交两步验证密码
func (e *Engine) Submit2FA(password string) error {
	e.login.mu.RLock()
	status := e.login.status
	ch := e.login.twoFA
	e.login.mu.RUnlock()

	if status != LoginStatusNeed2FA {
		return errors.New("当前不需要两步验证密码")
	}

	select {
	case ch <- password:
		return nil
	default:
		return errors.New("两步验证密码已提交，请稍候")
	}
}

// Logout 注销服务端会话并删除本地会话文件
func (e *Engine) Logout(ctx context.Context) error {
	e.mu.RLock()
	client := e.client
	e.mu.RUnlock()

	if client != nil {
		logoutCtx, cancel := context.WithTimeout(ctx, authStatusTimeout)
		if _, err := tg.NewClient(client).AuthLogOut(logoutCtx); err != nil {
			// 服务端注销失败不影响本地清理，重连后即为未登录状态
			e.log.Warn("注销 Telegram 服务端会话失败，仅清理本地会话", zap.Error(err))
		}
		cancel()
	}

	if err := e.store.Remove(); err != nil {
		return err
	}

	e.setAuth(false, "")
	e.login.reset()
	e.EnsureReconnect()
	return nil
}

// doQRLogin 执行扫码登录流程，并在需要时等待两步验证密码
func (e *Engine) doQRLogin(runCtx context.Context, client *telegram.Client) {
	loggedIn := qrlogin.OnLoginToken(e.dispatch)

	_, err := client.QR().Auth(runCtx, loggedIn, func(_ context.Context, token qrlogin.Token) error {
		e.login.setStatus(LoginStatusPending)
		e.login.setQR(token.URL())
		return nil
	})

	if err != nil {
		if isPasswordNeeded(err) {
			if pwdErr := e.await2FA(runCtx, client); pwdErr != nil {
				e.log.Warn("Telegram 两步验证失败", zap.Error(pwdErr))
				e.login.setError(pwdErr)
				return
			}
		} else {
			e.log.Warn("Telegram 扫码登录失败", zap.Error(err))
			e.login.setError(err)
			return
		}
	}

	e.refreshAuth(runCtx, client)
	e.login.setStatus(LoginStatusSuccess)
	e.log.Info("Telegram 登录成功", zap.String("account", e.Account()))
}

// await2FA 等待并校验两步验证密码，允许在密码错误时重试
func (e *Engine) await2FA(runCtx context.Context, client *telegram.Client) error {
	for attempt := 0; attempt < maxTwoFAAttempts; attempt++ {
		e.login.setNeed2FA(nil)

		password, err := e.login.waitPassword(runCtx)
		if err != nil {
			return err
		}

		if _, err := client.Auth().Password(runCtx, password); err != nil {
			if errors.Is(err, auth.ErrPasswordInvalid) {
				e.login.setNeed2FA(errors.New("两步验证密码错误，请重试"))
				continue
			}
			return fmt.Errorf("两步验证失败: %w", err)
		}
		return nil
	}

	return errors.New("两步验证密码错误次数过多，请重新扫码登录")
}

// isPasswordNeeded 判断错误是否为「需要两步验证密码」
func isPasswordNeeded(err error) bool {
	return tgerr.Is(err, "SESSION_PASSWORD_NEEDED") || errors.Is(err, auth.ErrPasswordAuthNeeded)
}
