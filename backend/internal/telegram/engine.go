// Package telegram 提供 Telegram（MTProto）下载能力，供视频下载模块在识别到 t.me 链接时调用。
//
// 底层复用 [iyear/tdl] 的 core 模块：客户端封装（tclient，含重试/恢复/限流中间件与代理支持）、
// DC 连接池（dcpool）、媒体解析（tmedia）、链接与消息工具（tutil）以及多线程下载引擎（downloader）。
//
// [iyear/tdl]: https://github.com/iyear/tdl
package telegram

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/telegram"
	gotdpeers "github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/iyear/tdl/core/dcpool"
	"github.com/iyear/tdl/core/logctx"
	"github.com/iyear/tdl/core/storage"
	"github.com/iyear/tdl/core/tclient"
)

// 引擎默认值与超时
const (
	defaultThreads          = 4
	maxThreads              = 16
	defaultPoolSize         = 4
	defaultReconnectTimeout = 5 * time.Minute
	// maxReconnectBackoff 引擎外层重连退避上限
	maxReconnectBackoff = 30 * time.Second
	// stableConnectionWindow 连接稳定运行超过该时长后重置重连退避
	stableConnectionWindow = time.Minute
	authStatusTimeout      = 15 * time.Second
	waitReadyPollInterval  = 200 * time.Millisecond
	// readyWaitTimeout 等待连接就绪的最长时间
	readyWaitTimeout = 30 * time.Second
	// stopTimeout 停止引擎时等待连接循环退出的最长时间
	stopTimeout = 5 * time.Second
)

// ErrNotReady 引擎尚未就绪（未配置凭据、正在重连或网络不可达）
var ErrNotReady = errors.New("Telegram 未就绪，请稍后重试")

// Config Telegram 引擎配置
type Config struct {
	// AppID/AppHash 来自 my.telegram.org 的应用凭据，未配置时引擎不会启动
	AppID   int
	AppHash string
	// Proxy 支持 socks5/socks5h/http/https，为空表示直连（由 tclient 解析）
	Proxy string
	// DataDir 会话文件存放目录
	DataDir string
	// Threads 单文件分片下载并发数（downloader 会按文件大小自动收敛）
	Threads int
	// PoolSize 每个 DC 的连接池大小
	PoolSize int
	// ReconnectTimeout 客户端重连退避上限
	ReconnectTimeout time.Duration
}

// Enabled 是否已配置可用的应用凭据
func (c Config) Enabled() bool {
	return c.AppID > 0 && c.AppHash != ""
}

// withDefaults 补齐未设置的字段并做范围收敛
func (c Config) withDefaults() Config {
	if c.Threads <= 0 {
		c.Threads = defaultThreads
	}
	if c.Threads > maxThreads {
		c.Threads = maxThreads
	}
	if c.PoolSize <= 0 {
		c.PoolSize = defaultPoolSize
	}
	if c.ReconnectTimeout <= 0 {
		c.ReconnectTimeout = defaultReconnectTimeout
	}
	if c.DataDir == "" {
		c.DataDir = filepath.Join("data", "telegram")
	}
	return c
}

// conn 一次连接周期内可用的运行时资源。
// gotd 的 Client 是一次性的（Run 返回后不可复用），断线后这些资源随连接一起重建。
type conn struct {
	ctx     context.Context
	pool    dcpool.Pool
	manager *gotdpeers.Manager
}

// Engine 维护 Telegram 客户端的长连接与登录状态
type Engine struct {
	// cfgMu 保护 cfg 与 store，支持运行时重新配置
	cfgMu sync.RWMutex
	cfg   Config
	store *fileSessionStore
	// kv 缓存 peers 解析结果，跨重连保留，避免重复发起解析请求
	kv *memKV

	log      *zap.Logger
	dispatch tg.UpdateDispatcher

	// mu 保护当前连接相关的运行时状态
	mu     sync.RWMutex
	client *telegram.Client
	cur    *conn
	// authChecked 表示本次连接的授权状态已查询过，避免把「未查询」误判为「未登录」
	authChecked bool

	// authMu 保护授权状态缓存，避免每个请求都发起 RPC
	authMu     sync.RWMutex
	authorized bool
	account    string

	login loginState

	// runMu 保护连接循环的启动状态
	runMu     sync.Mutex
	runCancel context.CancelFunc
	runDone   chan struct{}

	// reconnectCh 用于按需触发重连（非阻塞通知）
	reconnectCh chan struct{}
}

// NewEngine 创建引擎实例，此时尚未建立连接
func NewEngine(cfg Config, log *zap.Logger) *Engine {
	if log == nil {
		log = zap.NewNop()
	}
	cfg = cfg.withDefaults()

	return &Engine{
		cfg:         cfg,
		log:         log.Named("telegram"),
		store:       newFileSessionStore(cfg.DataDir),
		kv:          newMemKV(),
		dispatch:    tg.NewUpdateDispatcher(),
		reconnectCh: make(chan struct{}, 1),
	}
}

// Config 返回当前生效的配置
func (e *Engine) Config() Config {
	return e.config()
}

// config 返回当前配置快照
func (e *Engine) config() Config {
	e.cfgMu.RLock()
	defer e.cfgMu.RUnlock()
	return e.cfg
}

// sessionStore 返回当前会话存储
func (e *Engine) sessionStore() *fileSessionStore {
	e.cfgMu.RLock()
	defer e.cfgMu.RUnlock()
	return e.store
}

// Configured 是否已配置应用凭据
func (e *Engine) Configured() bool {
	return e.config().Enabled()
}

// Start 启动连接循环；未配置凭据或已在运行时不做任何事
func (e *Engine) Start() {
	if !e.config().Enabled() {
		e.log.Info("未配置 Telegram api_id/api_hash，引擎不启动")
		return
	}

	e.runMu.Lock()
	if e.runCancel != nil {
		e.runMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	e.runCancel = cancel
	e.runDone = done
	e.runMu.Unlock()

	go func() {
		defer close(done)
		e.Run(ctx)
	}()
}

// Stop 停止连接循环并等待其退出
func (e *Engine) Stop() {
	e.runMu.Lock()
	cancel, done := e.runCancel, e.runDone
	e.runCancel, e.runDone = nil, nil
	e.runMu.Unlock()

	if cancel == nil {
		return
	}
	cancel()

	select {
	case <-done:
	case <-time.After(stopTimeout):
		e.log.Warn("等待 Telegram 连接循环退出超时")
	}
}

// Reconfigure 更新配置；连接参数发生变化时重启连接循环。
// 会话文件不受影响，重启后无需重新登录。
func (e *Engine) Reconfigure(cfg Config) {
	cfg = cfg.withDefaults()

	e.cfgMu.Lock()
	changed := e.cfg != cfg
	if changed {
		e.cfg = cfg
		e.store = newFileSessionStore(cfg.DataDir)
	}
	e.cfgMu.Unlock()

	if !changed {
		return
	}

	e.runMu.Lock()
	running := e.runCancel != nil
	e.runMu.Unlock()

	if !running {
		e.Start()
		return
	}

	e.Stop()
	e.Start()
}

// Run 连接 Telegram 并在断线后自动重连，直到 ctx 取消。该方法阻塞，请在独立 goroutine 中调用。
func (e *Engine) Run(ctx context.Context) {
	if !e.config().Enabled() {
		e.log.Info("未配置 Telegram api_id/api_hash，引擎不启动")
		return
	}

	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}

		startedAt := time.Now()
		err := e.runOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			e.log.Warn("Telegram 连接已断开，准备重连", zap.Error(err))
		}
		// 连接曾稳定运行则重置退避，避免长时间在线后重连仍等待过长
		if time.Since(startedAt) > stableConnectionWindow {
			backoff = time.Second
		}

		select {
		case <-ctx.Done():
			return
		case <-e.reconnectCh:
			e.log.Info("收到重连请求，立即重连 Telegram")
		case <-time.After(backoff):
		}

		if backoff < maxReconnectBackoff {
			backoff *= 2
		}
	}
}

// runOnce 建立一次客户端连接并运行至断开
func (e *Engine) runOnce(ctx context.Context) error {
	client, err := e.newClient(ctx)
	if err != nil {
		return err
	}

	return client.Run(ctx, func(runCtx context.Context) error {
		e.setRuntime(client, runCtx)
		defer e.clearRuntime()

		e.login.reset()
		e.refreshAuth(runCtx, client)
		e.log.Info("Telegram 已连接",
			zap.Bool("authorized", e.IsAuthenticated()),
			zap.String("account", e.Account()),
		)

		<-runCtx.Done()
		return runCtx.Err()
	})
}

// newClient 通过 tdl core 构造客户端（含重试/恢复/限流中间件、代理与设备信息）
func (e *Engine) newClient(ctx context.Context) (*telegram.Client, error) {
	cfg := e.config()

	return tclient.New(logctx.With(ctx, e.log), tclient.Options{
		AppID:            cfg.AppID,
		AppHash:          cfg.AppHash,
		Session:          e.sessionStore(),
		Proxy:            cfg.Proxy,
		ReconnectTimeout: cfg.ReconnectTimeout,
		UpdateHandler:    e.dispatch,
	})
}

// setRuntime 记录当前连接的运行时状态，并构建 DC 连接池与 peers 管理器
func (e *Engine) setRuntime(client *telegram.Client, runCtx context.Context) {
	cfg := e.config()
	pool := dcpool.NewPool(client, int64(cfg.PoolSize),
		tclient.NewDefaultMiddlewares(runCtx, cfg.ReconnectTimeout)...)
	manager := gotdpeers.Options{Storage: storage.NewPeers(e.kv)}.Build(pool.Default(runCtx))

	e.mu.Lock()
	e.client = client
	e.cur = &conn{ctx: runCtx, pool: pool, manager: manager}
	e.authChecked = false
	e.mu.Unlock()
}

// clearRuntime 清理运行时状态并释放 DC 连接池
func (e *Engine) clearRuntime() {
	e.mu.Lock()
	cur := e.cur
	e.client = nil
	e.cur = nil
	e.authChecked = false
	e.mu.Unlock()

	if cur != nil {
		if err := cur.pool.Close(); err != nil {
			e.log.Warn("关闭 Telegram DC 连接池失败", zap.Error(err))
		}
	}
	e.setAuth(false, "")
}

// runtime 返回当前连接的运行时资源
func (e *Engine) runtime() (*conn, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.cur == nil || e.cur.ctx.Err() != nil {
		return nil, ErrNotReady
	}
	return e.cur, nil
}

// runtimeClient 返回当前连接的客户端与上下文，扫码登录需要用到 telegram.Client
func (e *Engine) runtimeClient() (*telegram.Client, context.Context, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.cur == nil || e.client == nil || e.cur.ctx.Err() != nil {
		return nil, nil, ErrNotReady
	}
	return e.client, e.cur.ctx, nil
}

// IsReady 是否已建立连接
func (e *Engine) IsReady() bool {
	_, err := e.runtime()
	return err == nil
}

// IsAuthenticated 是否已登录（基于最近一次连接时刷新的缓存）
func (e *Engine) IsAuthenticated() bool {
	e.authMu.RLock()
	defer e.authMu.RUnlock()
	return e.authorized
}

// Account 当前登录账号的展示名，未登录时为空
func (e *Engine) Account() string {
	e.authMu.RLock()
	defer e.authMu.RUnlock()
	return e.account
}

// SessionExists 本地是否已有会话文件（不代表该会话仍然有效）
func (e *Engine) SessionExists() bool {
	return e.store.Exists()
}

// EnsureReconnect 非阻塞地请求引擎立即重连
func (e *Engine) EnsureReconnect() {
	select {
	case e.reconnectCh <- struct{}{}:
	default:
	}
}

// WaitReady 等待连接就绪且授权状态已知，返回运行时资源与是否已登录
func (e *Engine) WaitReady(ctx context.Context, timeout time.Duration) (*conn, bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		if cur, err := e.readyState(); err == nil {
			return cur, e.IsAuthenticated(), nil
		}

		if time.Now().After(deadline) {
			return nil, false, ErrNotReady
		}

		// 连接可能已断开，顺带请求一次重连
		e.EnsureReconnect()

		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(waitReadyPollInterval):
		}
	}
}

// ReadyState 等待连接就绪并返回登录状态，供任务创建前的依赖校验使用
func (e *Engine) ReadyState(ctx context.Context, timeout time.Duration) (ready bool, authorized bool, err error) {
	_, authorized, err = e.WaitReady(ctx, timeout)
	if err != nil {
		return false, false, err
	}
	return true, authorized, nil
}

// readyState 连接是否就绪且授权状态已查询过
func (e *Engine) readyState() (*conn, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.cur == nil || e.cur.ctx.Err() != nil || !e.authChecked {
		return nil, ErrNotReady
	}
	return e.cur, nil
}

// refreshAuth 重新查询并缓存授权状态
func (e *Engine) refreshAuth(runCtx context.Context, client *telegram.Client) {
	ctx, cancel := context.WithTimeout(runCtx, authStatusTimeout)
	defer cancel()

	authorized, account := false, ""
	status, err := client.Auth().Status(ctx)
	if err != nil {
		e.log.Warn("获取 Telegram 授权状态失败", zap.Error(err))
	} else if status.Authorized {
		authorized, account = true, displayName(status.User)
	}

	e.setAuth(authorized, account)

	e.mu.Lock()
	e.authChecked = true
	e.mu.Unlock()
}

func (e *Engine) setAuth(authorized bool, account string) {
	e.authMu.Lock()
	e.authorized = authorized
	e.account = account
	e.authMu.Unlock()
}

// displayName 生成账号的展示名
func displayName(u *tg.User) string {
	if u == nil {
		return ""
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	if name := strings.TrimSpace(u.FirstName + " " + u.LastName); name != "" {
		return name
	}
	return fmt.Sprintf("id:%d", u.ID)
}

// globalEngine 全局引擎实例，由启动流程注入，供下载模块与组件检测复用
var (
	globalMu     sync.RWMutex
	globalEngine *Engine
)

// SetGlobal 注入全局引擎实例
func SetGlobal(engine *Engine) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalEngine = engine
}

// Global 返回全局引擎实例，未注入时返回 nil
func Global() *Engine {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalEngine
}
