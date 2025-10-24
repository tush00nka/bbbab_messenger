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
	CreateGroup(chat *model.Chat, userIDs []uint) error
	IsUserInChat(chatID, userID uint) (bool, error)
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
	var chat model.Chat
	var chatID uint

	// Надёжно находим chat_id, где состоят оба пользователя
	err := r.db.Raw(`
		SELECT chat_id FROM chat_users
		WHERE chat_id IN (SELECT chat_id FROM chat_users WHERE user_id = ?)
		  AND chat_id IN (SELECT chat_id FROM chat_users WHERE user_id = ?)
		LIMIT 1
	`, user1ID, user2ID).Scan(&chatID).Error
	if err != nil {
		return nil, err
	}

	if chatID == 0 {
		return nil, nil
	}

	if err := r.db.Preload("Users").Preload("Messages").First(&chat, chatID).Error; err != nil {
		return nil, err
	}

	return &chat, nil
}

func (r *chatRepository) AddUser(chatID, userID uint) error {
	// Вставляем только если ещё нет записи
	return r.db.Exec(`
        INSERT INTO chat_users (chat_id, user_id)
        SELECT ?, ?
        WHERE NOT EXISTS (
            SELECT 1 FROM chat_users WHERE chat_id = ? AND user_id = ?
        )
    `, chatID, userID, chatID, userID).Error
}

func (r *chatRepository) SendMessage(chat *model.Chat, message model.Message) error {
	// Создаём message напрямую — надёжнее, чем пытаться Save() whole chat
	return r.db.Create(&message).Error
}

func (r *chatRepository) GetMessages(chatID uint) ([]model.Message, error) {
	var messages []model.Message

	err := r.db.Where("chat_id = ?", chatID).Order("created_at asc").Find(&messages).Error
	if err != nil {
		return nil, err
	}

	return messages, nil
}

func (r *chatRepository) CreateGroup(chat *model.Chat, userIDs []uint) error {
	if err := r.db.Create(chat).Error; err != nil {
		return err
	}

	for _, uid := range userIDs {
		if err := r.AddUser(chat.ID, uid); err != nil {
			return err
		}
	}

	return nil
}

func (r *chatRepository) IsUserInChat(chatID, userID uint) (bool, error) {
	var exists int64
	err := r.db.Table("chat_users").
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Count(&exists).Error
	return exists > 0, err
}
