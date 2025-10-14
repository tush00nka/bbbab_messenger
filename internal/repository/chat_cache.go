package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"tush00nka/bbbab_messenger/internal/model"

	"github.com/redis/go-redis/v9"
)

type ChatCacheRepository interface {
	SaveMessage(chatID uint, msg model.Message) error
	GetMessages(chatID uint) ([]model.Message, error)
	ClearMessages(chatID uint) error
	AddUserToChat(chatID, userID uint) error
	RemoveUserFromChat(chatID, userID uint) (int64, error)
}

type chatCacheRepository struct {
	rdb *redis.Client
	ctx context.Context
}

func NewChatCacheRepository(rdb *redis.Client) ChatCacheRepository {
	return &chatCacheRepository{
		rdb: rdb,
		ctx: context.Background(),
	}
}

func (r *chatCacheRepository) SaveMessage(chatID uint, msg model.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return r.rdb.RPush(r.ctx, fmt.Sprintf("chat:%d:messages", chatID), data).Err()
}

func (r *chatCacheRepository) GetMessages(chatID uint) ([]model.Message, error) {
	key := fmt.Sprintf("chat:%d:messages", chatID)
	values, err := r.rdb.LRange(r.ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	var messages []model.Message
	for _, v := range values {
		var msg model.Message
		if err := json.Unmarshal([]byte(v), &msg); err == nil {
			messages = append(messages, msg)
		}
	}
	return messages, nil
}

func (r *chatCacheRepository) ClearMessages(chatID uint) error {
	return r.rdb.Del(r.ctx, fmt.Sprintf("chat:%d:messages", chatID)).Err()
}

func (r *chatCacheRepository) AddUserToChat(chatID, userID uint) error {
	return r.rdb.SAdd(r.ctx, fmt.Sprintf("chat:%d:users_online", chatID), userID).Err()
}

func (r *chatCacheRepository) RemoveUserFromChat(chatID, userID uint) (int64, error) {
	key := fmt.Sprintf("chat:%d:users_online", chatID)
	if err := r.rdb.SRem(r.ctx, key, userID).Err(); err != nil {
		return 0, err
	}
	return r.rdb.SCard(r.ctx, key).Result()
}
