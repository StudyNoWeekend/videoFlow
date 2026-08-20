package logic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"video-captions/enum"
	"video-captions/internal/dto/req"
	"video-captions/internal/dto/res"
	"video-captions/internal/model"
	"video-captions/utils/logger"
)

const (
	jwtSecretKey = "jwt_secret"
	jwtExpiryKey = "jwt_expiry" // 小时，默认 24
)

const (
	defaultJWTExpiry    = 24
	tokenExpireDuration = 30 * time.Minute // 重置令牌有效期为 30 分钟
	resetTokenCoolDown  = 60 * time.Second // 通过接口触发生成重置令牌的冷却时间
)

// resetTokenCoolDownMap 记录每个用户上次通过接口生成重置令牌的时间，用于 60s 冷却
var resetTokenCoolDownMap = struct {
	sync.Mutex
	last map[string]time.Time
}{last: make(map[string]time.Time)}

// AuthLogic 认证业务逻辑
type AuthLogic struct{}

// NewAuthLogic 创建认证逻辑实例
func NewAuthLogic() *AuthLogic {
	return &AuthLogic{}
}

// ----- 初始化相关 -----

// CheckInitStatus 检查系统初始化状态
func (l *AuthLogic) CheckInitStatus(ctx context.Context) *res.AuthStatusRes {
	var count int64
	model.DB.WithContext(ctx).Model(&model.User{}).Count(&count)
	return &res.AuthStatusRes{
		Initialized: count > 0,
	}
}

// Init 系统初始化（创建首个管理员用户 + 生成 JWT secret）
func (l *AuthLogic) Init(ctx context.Context, r *req.InitReq) (*res.LoginRes, error) {
	// 检查是否已初始化
	status := l.CheckInitStatus(ctx)
	if status.Initialized {
		return nil, enum.ErrUserExists
	}

	// 校验用户名与密码格式（规则见 auth_validate.go）
	if err := validateUsername(r.Username); err != nil {
		return nil, err
	}
	if err := validatePassword(r.Password); err != nil {
		return nil, err
	}

	// bcrypt 加密密码
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(r.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, enum.ErrInternalServer.WithMsg("密码加密失败")
	}

	// 创建用户
	user := model.User{
		Username: r.Username,
		Password: string(hashedPwd),
	}
	user.ID = uuid.New().String()

	if err := model.DB.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, enum.ErrDatabase.WithMsg("创建用户失败")
	}

	// 生成 JWT secret 并持久化
	secret := l.generateSecret()
	if err := model.SettingSet(ctx, jwtSecretKey, secret); err != nil {
		return nil, enum.ErrDatabase.WithMsg("保存 JWT secret 失败")
	}

	// 签发 token
	token, err := l.generateToken(user.ID, user.Username, secret)
	if err != nil {
		return nil, enum.ErrInternalServer.WithMsg("生成 token 失败")
	}

	return &res.LoginRes{
		Token: token,
		User: res.UserInfo{
			ID:       user.ID,
			Username: user.Username,
		},
	}, nil
}

// ----- 登录 -----

// LoginByPassword 密码登录（支持用户名或邮箱）
func (l *AuthLogic) LoginByPassword(ctx context.Context, r *req.LoginPwdReq) (*res.LoginRes, error) {
	var user model.User
	err := model.DB.WithContext(ctx).
		Where("username = ?", r.Username).
		First(&user).Error
	if err != nil {
		return nil, enum.ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(r.Password)); err != nil {
		return nil, enum.ErrInvalidPassword
	}

	token, err := l.generateTokenFromUser(ctx, &user)
	if err != nil {
		return nil, enum.ErrInternalServer.WithMsg("生成 token 失败")
	}

	return &res.LoginRes{
		Token: token,
		User: res.UserInfo{
			ID:       user.ID,
			Username: user.Username,
		},
	}, nil
}

// ----- 密码修改 -----

// ChangePassword 修改密码（需已登录）
func (l *AuthLogic) ChangePassword(ctx context.Context, userID string, r *req.ChangePwdReq) error {
	var user model.User
	if err := model.DB.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		return enum.ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(r.OldPassword)); err != nil {
		return enum.ErrInvalidPassword
	}

	if r.OldPassword == r.NewPassword {
		return enum.ErrSamePassword
	}

	// 校验新密码格式（规则见 auth_validate.go）
	if err := validatePassword(r.NewPassword); err != nil {
		return err
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(r.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return enum.ErrInternalServer.WithMsg("密码加密失败")
	}

	if err := model.DB.WithContext(ctx).Model(&user).Update("password", string(hashedPwd)).Error; err != nil {
		return enum.ErrDatabase.WithMsg("更新密码失败")
	}

	return nil
}

// ----- 重置令牌相关 -----

// generateResetToken 生成密码重置令牌并持久化，返回原始令牌与有效期
func (l *AuthLogic) generateResetToken(ctx context.Context, username string) (rawToken string, expire string, err error) {
	var user model.User
	if err := model.DB.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return "", "", enum.ErrUserNotFound
	}

	// 生成 32 字节随机令牌
	rawToken = l.generateSecret() // 64 位 hex 字符串

	// 对令牌做 SHA256 hash 后存入数据库
	tokenHash := l.hashToken(rawToken)

	rt := model.ResetToken{
		UserID:    user.ID,
		Token:     tokenHash,
		ExpiresAt: time.Now().Add(tokenExpireDuration).UnixMilli(),
	}
	rt.ID = uuid.New().String()

	if err := model.DB.WithContext(ctx).Create(&rt).Error; err != nil {
		return "", "", enum.ErrDatabase.WithMsg("保存重置令牌失败")
	}

	return rawToken, tokenExpireDuration.String(), nil
}

// GenerateResetTokenForAPI 通过接口触发生成密码重置令牌。
// 令牌只输出到终端日志，不随接口返回，方便不懂命令行的用户到日志里查找。
// 同一用户 60s 内只能触发一次。
func (l *AuthLogic) GenerateResetTokenForAPI(ctx context.Context, username string) error {
	// 冷却检查
	now := time.Now()
	resetTokenCoolDownMap.Lock()
	lastTime, exists := resetTokenCoolDownMap.last[username]
	if exists && now.Sub(lastTime) < resetTokenCoolDown {
		remain := int64(resetTokenCoolDown.Seconds() - now.Sub(lastTime).Seconds())
		if remain < 1 {
			remain = 1
		}
		resetTokenCoolDownMap.Unlock()
		return enum.ErrResetTokenTooFrequent.WithMsg(
			fmt.Sprintf("请求过于频繁，请在 %d 秒后重试", remain))
	}
	resetTokenCoolDownMap.Unlock()

	// 生成令牌（用户不存在时返回错误，不消耗冷却时间）
	rawToken, expire, err := l.generateResetToken(ctx, username)
	if err != nil {
		return err
	}

	// 生成成功后再记录冷却时间
	resetTokenCoolDownMap.Lock()
	resetTokenCoolDownMap.last[username] = time.Now()
	resetTokenCoolDownMap.Unlock()

	// 令牌输出到日志（stdout + 日志文件），例如通过 docker compose logs 查看
	logger.Logger.Info("密码重置令牌已生成 (reset token generated)",
		zap.String("username", username),
		zap.String("token", rawToken),
		zap.String("expire", expire),
		zap.String("hint", "请使用该令牌重置密码：POST /api/v1/auth/reset-password"))

	return nil
}

// ResetPassword 通过重置令牌重置密码
func (l *AuthLogic) ResetPassword(ctx context.Context, r *req.ResetPasswordReq) error {
	// 对传入的 token 做 hash 后查询
	tokenHash := l.hashToken(r.Token)

	var rt model.ResetToken
	err := model.DB.WithContext(ctx).
		Where("token = ? AND used = ? AND expires_at > ?", tokenHash, false, time.Now().UnixMilli()).
		First(&rt).Error
	if err != nil {
		return enum.ErrInvalidToken
	}

	// 查找用户
	var user model.User
	if err := model.DB.WithContext(ctx).First(&user, "id = ?", rt.UserID).Error; err != nil {
		return enum.ErrUserNotFound
	}

	// 校验新密码格式（规则见 auth_validate.go）
	if err := validatePassword(r.NewPassword); err != nil {
		return err
	}

	// 加密新密码
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(r.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return enum.ErrInternalServer.WithMsg("密码加密失败")
	}

	// 更新密码
	if err := model.DB.WithContext(ctx).Model(&user).Update("password", string(hashedPwd)).Error; err != nil {
		return enum.ErrDatabase.WithMsg("更新密码失败")
	}

	// 标记令牌已使用
	model.DB.WithContext(ctx).Model(&rt).Update("used", true)

	return nil
}

// ----- Token 相关 -----

// ValidateToken 验证并解析 JWT token
func (l *AuthLogic) ValidateToken(tokenString string) (map[string]interface{}, error) {
	secret := model.SettingGet(nil, jwtSecretKey)
	if secret == "" {
		return nil, errors.New("JWT secret 未配置")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("不支持的签名方法: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, enum.ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, enum.ErrTokenInvalid
	}

	exp, ok := claims["exp"].(float64)
	if ok {
		if time.Now().Unix() > int64(exp) {
			return nil, enum.ErrTokenExpired
		}
	}

	return map[string]interface{}{
		"user_id":  claims["user_id"],
		"username": claims["username"],
	}, nil
}

// ----- 私有辅助方法 -----

func (l *AuthLogic) generateSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (l *AuthLogic) hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (l *AuthLogic) generateRandomCode() (string, error) {
	code := ""
	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code += fmt.Sprintf("%d", n.Int64())
	}
	return code, nil
}

func (l *AuthLogic) generateTokenFromUser(ctx context.Context, user *model.User) (string, error) {
	secret := model.SettingGet(ctx, jwtSecretKey)
	if secret == "" {
		return "", errors.New("JWT secret 未配置")
	}
	return l.generateToken(user.ID, user.Username, secret)
}

func (l *AuthLogic) generateToken(userID, username, secret string) (string, error) {
	expiryStr := model.SettingGet(nil, jwtExpiryKey)
	expiry := defaultJWTExpiry
	if expiryStr != "" {
		if v, err := fmt.Sscanf(expiryStr, "%d", &expiry); err != nil || v != 1 {
			expiry = defaultJWTExpiry
		}
	}

	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(time.Duration(expiry) * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
