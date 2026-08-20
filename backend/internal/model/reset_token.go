package model

// ResetToken 密码重置令牌表
type ResetToken struct {
	BaseModel
	UserID    string `gorm:"index;size:36;not null" json:"user_id"`
	Token     string `gorm:"uniqueIndex;size:128;not null" json:"-"` // bcrypt hash of the raw token
	ExpiresAt int64  `gorm:"not null" json:"expires_at"`             // 过期时间戳（毫秒）
	Used      bool   `gorm:"default:false" json:"used"`
}

// TableName 指定表名
func (ResetToken) TableName() string {
	return "reset_tokens"
}
