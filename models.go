package main

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Chats    []Chat `gorm:"many2many:chat_users;"`
	Username string
	Password string
}

type Chat struct {
	gorm.Model
	Users []User `gorm:"many2many:chat_users;"`
}

type Message struct {
	gorm.Model
	ChatID   uint
	SenderID uint
	Message  string
}
