package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Chats    []Chat `gorm:"many2many:chat_users;"`
	Username string
	Password string
	Phone    string // такого формата мб 8-900-800-55-55
}

func (u *User) SanitizePassword() {
	u.Password = ""
}

type VerificationCode struct {
	Phone     string    `json:"phone"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}
