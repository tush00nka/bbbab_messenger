package model

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Chats    []Chat `gorm:"many2many:chat_users;"`
	Username string `gorm:"uniqueIndex"`
	Password string
}

func (u *User) SanitizePassword() {
	u.Password = ""
}
