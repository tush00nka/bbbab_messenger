package model

import (
	"time"

	"gorm.io/gorm"
)

type Chat struct {
	gorm.Model
	Name     string `json:"name"` // опционально — имя группового чата
	Users    []User `gorm:"many2many:chat_users;"`
	Messages []Message
	IsGroup  bool
}

// ChatUser - промежуточная таблица для связи many-to-many
type ChatUser struct {
	ChatID    uint           `gorm:"primaryKey"`
	UserID    uint           `gorm:"primaryKey"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName задает имя таблицы
func (ChatUser) TableName() string {
	return "chat_users"
}
