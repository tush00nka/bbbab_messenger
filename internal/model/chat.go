package model

import "gorm.io/gorm"

type Chat struct {
	gorm.Model
	Name     string `json:"name"` // опционально — имя группового чата
	Users    []User `gorm:"many2many:chat_users;"`
	Messages []Message
}
