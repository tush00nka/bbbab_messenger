package model

import "gorm.io/gorm"

type Chat struct {
	gorm.Model
	Users []User `gorm:"many2many:chat_users;"`
}
