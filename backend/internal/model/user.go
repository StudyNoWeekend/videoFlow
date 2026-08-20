package model

// User 用户表
type User struct {
	BaseModel
	Username string `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Email    string `gorm:"uniqueIndex;size:128" json:"email"` // 可选，兼容旧库
	Password string `gorm:"size:256;not null" json:"-"`        // bcrypt hash
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
