package repository

import (
	"tush00nka/bbbab_messenger/internal/model"

	"gorm.io/gorm"
)

type ChatRepository interface {
	Create(chat *model.Chat) error
	GetForUsers(user1ID, user2ID uint) (*model.Chat, error)
	AddUser(chatID, userID uint) error
	SendMessage(chat *model.Chat, message model.Message) error
	GetMessages(chatID uint) ([]model.Message, error)
}

type chatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) Create(chat *model.Chat) error {
	return r.db.Create(chat).Error
}

func (r *chatRepository) GetForUsers(user1ID, user2ID uint) (*model.Chat, error) {
	var count int64
	var chatID uint
	var chat *model.Chat

	// Проверяем, есть ли чат, где оба пользователя состоят в одном чате
	err := r.db.Table("chat_users").
		Joins("JOIN chat_users as cu2 on chat_users.chat_id = cu2.chat_id").
		Where("chat_users.user_id = ? AND cu2.user_id = ?", user1ID, user2ID).
		Select("chat_users.chat_id").
		Count(&count).
		Scan(&chatID).Error

	if err != nil {
		return nil, err
	}

	r.db.First(chat, chatID)

	return chat, nil
}

func (r *chatRepository) AddUser(chatID, userID uint) error {
	return r.db.Exec(`
        INSERT INTO chat_users (chat_id, user_id) 
        VALUES (?, ?)
    `, chatID, userID).Error
}

func (r *chatRepository) SendMessage(chat *model.Chat, message model.Message) error {
	chat.Messages = append(chat.Messages, message)
	return r.db.Save(chat).Error
}

func (r *chatRepository) GetMessages(chatID uint) ([]model.Message, error) {
	var messages []model.Message

	err := r.db.Where("chat_id = ?", chatID).Find(&messages).Error
	if err != nil {
		return nil, err
	}

	return messages, nil
}
