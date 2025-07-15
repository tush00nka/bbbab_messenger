package model

import "gorm.io/gorm"

type Message struct {
	gorm.Model
	ChatID   uint
	SenderID uint
	Message  string
}
