package model

import (
	"time"

	"gorm.io/gorm"
)

type Message struct {
	gorm.Model
	ChatID   uint   `gorm:"index;not null" json:"chat_id"`
	SenderID uint   `gorm:"index;not null" json:"sender_id"`
	Message  string `gorm:"type:text;not null" json:"message"`
	Type     string `gorm:"type:varchar(20);default:'text'" json:"type"`
	Status   string `gorm:"type:varchar(20);default:'sent'" json:"status"`

	Timestamp time.Time

	// Вложения и ссылки
	AttachmentURL *string `json:"attachment_url,omitempty"`
	ReplyToID     *uint   `gorm:"index" json:"reply_to_id,omitempty"`

	// Статистика
	IsEdited bool `gorm:"default:false" json:"is_edited"`

	// Связи
	Sender  User     `gorm:"foreignKey:SenderID" json:"sender"`
	ReplyTo *Message `gorm:"foreignKey:ReplyToID" json:"reply_to,omitempty"`
}

// Таблица для отслеживания прочитанных сообщений
type MessageRead struct {
	ID        uint           `gorm:"primarykey"`
	MessageID uint           `gorm:"index;not null"`
	UserID    uint           `gorm:"index;not null"`
	ReadAt    gorm.DeletedAt `gorm:"index"`

	// Составной уникальный индекс
	UniqueIndex string `gorm:"uniqueIndex:idx_message_user"`
}

func (MessageRead) TableName() string {
	return "message_reads"
}
