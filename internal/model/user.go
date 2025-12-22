package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Chats             []Chat `gorm:"many2many:chat_users;"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	Phone             string `json:"phone"` // +79995552233
	DisplayName       string `json:"display_name"`
	ProfilePictureKey string `json:"profile_picture_key"`
}

func (u *User) SanitizePassword() {
	u.Password = ""
}

func (u *User) EnsureDisplayName() {
	if u.DisplayName == "" {
		u.DisplayName = u.Username
	}
}

type VerificationCode struct {
	Phone     string    `json:"phone"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}
