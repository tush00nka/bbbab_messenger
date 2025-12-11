package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"tush00nka/bbbab_messenger/internal/model"

	"github.com/redis/go-redis/v9"
)

// ChatCacheRepository интерфейс репозитория кеша чатов
type ChatCacheRepository interface {
	// Операции с сообщениями
	SaveMessage(ctx context.Context, chatID uint, msg model.Message) error
	GetMessages(ctx context.Context, chatID uint) ([]model.Message, error)
	ClearMessages(ctx context.Context, chatID uint) error
	GetMessageCount(ctx context.Context, chatID uint) (int64, error)
	TrimMessages(ctx context.Context, chatID uint, maxSize int64) error
	DeleteMessage(ctx context.Context, chatID, messageID uint) error

	// Операции с пользователями (присутствие)
	AddUserToChat(ctx context.Context, chatID, userID uint) error
	RemoveUserFromChat(ctx context.Context, chatID, userID uint) (int64, error)
	GetChatUsers(ctx context.Context, chatID uint) ([]uint, error)
	IsUserInChat(ctx context.Context, chatID, userID uint) (bool, error)
	GetUserChats(ctx context.Context, userID uint) ([]uint, error)

	// Операции с чатами
	ClearChat(ctx context.Context, chatID uint) error
	GetActiveChatsCount(ctx context.Context) (int64, error)
	SetChatTTL(ctx context.Context, chatID uint, ttl time.Duration) error

	// Статистика
	IncrementMessageCounter(ctx context.Context, chatID uint) (int64, error)
	GetChatStatistics(ctx context.Context, chatID uint) (map[string]interface{}, error)
}

// chatCacheRepository реализация ChatCacheRepository
type chatCacheRepository struct {
	rdb *redis.Client
}

// NewChatCacheRepository создает новый экземпляр репозитория кеша
func NewChatCacheRepository(rdb *redis.Client) ChatCacheRepository {
	return &chatCacheRepository{rdb: rdb}
}

// getMessageKey возвращает ключ для хранения сообщений чата
func (r *chatCacheRepository) getMessageKey(chatID uint) string {
	return fmt.Sprintf("chat:%d:messages", chatID)
}

// getUserKey возвращает ключ для хранения активных пользователей чата
func (r *chatCacheRepository) getUserKey(chatID uint) string {
	return fmt.Sprintf("chat:%d:users_online", chatID)
}

// getCounterKey возвращает ключ для счетчика сообщений
func (r *chatCacheRepository) getCounterKey(chatID uint) string {
	return fmt.Sprintf("chat:%d:msg_counter", chatID)
}

// getUserChatsKey возвращает ключ для хранения чатов пользователя
func (r *chatCacheRepository) getUserChatsKey(userID uint) string {
	return fmt.Sprintf("user:%d:active_chats", userID)
}

// SaveMessage сохраняет сообщение в кеше
func (r *chatCacheRepository) SaveMessage(ctx context.Context, chatID uint, msg model.Message) error {
	if chatID == 0 {
		return fmt.Errorf("chatID cannot be zero")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	key := r.getMessageKey(chatID)

	// Сохраняем сообщение в список
	if err := r.rdb.RPush(ctx, key, data).Err(); err != nil {
		return fmt.Errorf("failed to save message to redis: %w", err)
	}

	// Ограничиваем размер списка (последние 1000 сообщений)
	if err := r.rdb.LTrim(ctx, key, -1000, -1).Err(); err != nil {
		return fmt.Errorf("failed to trim message list: %w", err)
	}

	// Устанавливаем TTL на ключ (24 часа)
	if err := r.rdb.Expire(ctx, key, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to set TTL: %w", err)
	}

	return nil
}

// GetMessages получает сообщения из кеша
func (r *chatCacheRepository) GetMessages(ctx context.Context, chatID uint) ([]model.Message, error) {
	if chatID == 0 {
		return nil, fmt.Errorf("chatID cannot be zero")
	}

	key := r.getMessageKey(chatID)
	values, err := r.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return []model.Message{}, nil
		}
		return nil, fmt.Errorf("failed to get messages from redis: %w", err)
	}

	messages := make([]model.Message, 0, len(values))
	for _, v := range values {
		var msg model.Message
		if err := json.Unmarshal([]byte(v), &msg); err != nil {
			// Пропускаем некорректные сообщения
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// ClearMessages очищает сообщения из кеша
func (r *chatCacheRepository) ClearMessages(ctx context.Context, chatID uint) error {
	if chatID == 0 {
		return fmt.Errorf("chatID cannot be zero")
	}

	key := r.getMessageKey(chatID)
	if err := r.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to clear messages: %w", err)
	}

	return nil
}

// GetMessageCount возвращает количество сообщений в кеше
func (r *chatCacheRepository) GetMessageCount(ctx context.Context, chatID uint) (int64, error) {
	if chatID == 0 {
		return 0, fmt.Errorf("chatID cannot be zero")
	}

	key := r.getMessageKey(chatID)
	count, err := r.rdb.LLen(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get message count: %w", err)
	}

	return count, nil
}

// TrimMessages обрезает список сообщений
func (r *chatCacheRepository) TrimMessages(ctx context.Context, chatID uint, maxSize int64) error {
	if chatID == 0 {
		return fmt.Errorf("chatID cannot be zero")
	}

	if maxSize <= 0 {
		return fmt.Errorf("maxSize must be positive")
	}

	key := r.getMessageKey(chatID)
	if err := r.rdb.LTrim(ctx, key, -maxSize, -1).Err(); err != nil {
		return fmt.Errorf("failed to trim messages: %w", err)
	}

	return nil
}

// AddUserToChat добавляет пользователя в кеш присутствия
func (r *chatCacheRepository) AddUserToChat(ctx context.Context, chatID, userID uint) error {
	if chatID == 0 || userID == 0 {
		return fmt.Errorf("chatID and userID cannot be zero")
	}

	userKey := r.getUserKey(chatID)
	userChatsKey := r.getUserChatsKey(userID)

	// Добавляем пользователя в множество пользователей чата
	if err := r.rdb.SAdd(ctx, userKey, userID).Err(); err != nil {
		return fmt.Errorf("failed to add user to chat: %w", err)
	}

	// Добавляем чат в множество активных чатов пользователя
	if err := r.rdb.SAdd(ctx, userChatsKey, chatID).Err(); err != nil {
		return fmt.Errorf("failed to add chat to user: %w", err)
	}

	// Устанавливаем TTL для обоих ключей
	for _, key := range []string{userKey, userChatsKey} {
		if err := r.rdb.Expire(ctx, key, 30*time.Minute).Err(); err != nil {
			return fmt.Errorf("failed to set TTL for key %s: %w", key, err)
		}
	}

	return nil
}

// RemoveUserFromChat удаляет пользователя из кеша присутствия
func (r *chatCacheRepository) RemoveUserFromChat(ctx context.Context, chatID, userID uint) (int64, error) {
	if chatID == 0 || userID == 0 {
		return 0, fmt.Errorf("chatID and userID cannot be zero")
	}

	userKey := r.getUserKey(chatID)
	userChatsKey := r.getUserChatsKey(userID)

	// Удаляем пользователя из множества пользователей чата
	if err := r.rdb.SRem(ctx, userKey, userID).Err(); err != nil {
		return 0, fmt.Errorf("failed to remove user from chat: %w", err)
	}

	// Удаляем чат из множества активных чатов пользователя
	if err := r.rdb.SRem(ctx, userChatsKey, chatID).Err(); err != nil {
		return 0, fmt.Errorf("failed to remove chat from user: %w", err)
	}

	// Получаем количество оставшихся пользователей
	count, err := r.rdb.SCard(ctx, userKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get user count: %w", err)
	}

	return count, nil
}

// GetChatUsers возвращает активных пользователей чата
func (r *chatCacheRepository) GetChatUsers(ctx context.Context, chatID uint) ([]uint, error) {
	if chatID == 0 {
		return nil, fmt.Errorf("chatID cannot be zero")
	}

	key := r.getUserKey(chatID)
	members, err := r.rdb.SMembers(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return []uint{}, nil
		}
		return nil, fmt.Errorf("failed to get chat users: %w", err)
	}

	users := make([]uint, 0, len(members))
	for _, member := range members {
		var userID uint
		if _, err := fmt.Sscanf(member, "%d", &userID); err == nil {
			users = append(users, userID)
		}
	}

	return users, nil
}

// IsUserInChat проверяет, активен ли пользователь в чате
func (r *chatCacheRepository) IsUserInChat(ctx context.Context, chatID, userID uint) (bool, error) {
	if chatID == 0 || userID == 0 {
		return false, fmt.Errorf("chatID and userID cannot be zero")
	}

	key := r.getUserKey(chatID)
	isMember, err := r.rdb.SIsMember(ctx, key, userID).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, fmt.Errorf("failed to check user membership: %w", err)
	}

	return isMember, nil
}

// GetUserChats возвращает активные чаты пользователя
func (r *chatCacheRepository) GetUserChats(ctx context.Context, userID uint) ([]uint, error) {
	if userID == 0 {
		return nil, fmt.Errorf("userID cannot be zero")
	}

	key := r.getUserChatsKey(userID)
	members, err := r.rdb.SMembers(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return []uint{}, nil
		}
		return nil, fmt.Errorf("failed to get user chats: %w", err)
	}

	chats := make([]uint, 0, len(members))
	for _, member := range members {
		var chatID uint
		if _, err := fmt.Sscanf(member, "%d", &chatID); err == nil {
			chats = append(chats, chatID)
		}
	}

	return chats, nil
}

// ClearChat полностью очищает кеш чата
func (r *chatCacheRepository) ClearChat(ctx context.Context, chatID uint) error {
	if chatID == 0 {
		return fmt.Errorf("chatID cannot be zero")
	}

	keys := []string{
		r.getMessageKey(chatID),
		r.getUserKey(chatID),
		r.getCounterKey(chatID),
	}

	if err := r.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to clear chat cache: %w", err)
	}

	return nil
}

// GetActiveChatsCount возвращает количество активных чатов
func (r *chatCacheRepository) GetActiveChatsCount(ctx context.Context) (int64, error) {
	// Ищем все ключи с пользователями чатов
	pattern := "chat:*:users_online"

	var cursor uint64
	var count int64

	for {
		var keys []string
		var err error
		keys, cursor, err = r.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return 0, fmt.Errorf("failed to scan keys: %w", err)
		}

		for _, key := range keys {
			// Проверяем, что множество не пустое
			card, err := r.rdb.SCard(ctx, key).Result()
			if err == nil && card > 0 {
				count++
			}
		}

		if cursor == 0 {
			break
		}
	}

	return count, nil
}

// SetChatTTL устанавливает TTL для всех ключей чата
func (r *chatCacheRepository) SetChatTTL(ctx context.Context, chatID uint, ttl time.Duration) error {
	if chatID == 0 {
		return fmt.Errorf("chatID cannot be zero")
	}

	keys := []string{
		r.getMessageKey(chatID),
		r.getUserKey(chatID),
		r.getCounterKey(chatID),
	}

	for _, key := range keys {
		if err := r.rdb.Expire(ctx, key, ttl).Err(); err != nil {
			return fmt.Errorf("failed to set TTL for key %s: %w", key, err)
		}
	}

	return nil
}

// IncrementMessageCounter увеличивает счетчик сообщений
func (r *chatCacheRepository) IncrementMessageCounter(ctx context.Context, chatID uint) (int64, error) {
	if chatID == 0 {
		return 0, fmt.Errorf("chatID cannot be zero")
	}

	key := r.getCounterKey(chatID)
	count, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment counter: %w", err)
	}

	// Устанавливаем TTL
	if err := r.rdb.Expire(ctx, key, 24*time.Hour).Err(); err != nil {
		return count, fmt.Errorf("failed to set TTL for counter: %w", err)
	}

	return count, nil
}

// GetChatStatistics возвращает статистику кеша чата
func (r *chatCacheRepository) GetChatStatistics(ctx context.Context, chatID uint) (map[string]interface{}, error) {
	if chatID == 0 {
		return nil, fmt.Errorf("chatID cannot be zero")
	}

	stats := make(map[string]interface{})

	// Количество сообщений
	msgCount, err := r.GetMessageCount(ctx, chatID)
	if err == nil {
		stats["cached_messages"] = msgCount
	}

	// Активные пользователи
	users, err := r.GetChatUsers(ctx, chatID)
	if err == nil {
		stats["active_users"] = len(users)
		stats["users"] = users
	}

	// Счетчик сообщений
	counterKey := r.getCounterKey(chatID)
	counter, err := r.rdb.Get(ctx, counterKey).Int64()
	if err == nil || err == redis.Nil {
		stats["message_counter"] = counter
	}

	// TTL для ключей
	for _, key := range []string{r.getMessageKey(chatID), r.getUserKey(chatID)} {
		ttl, err := r.rdb.TTL(ctx, key).Result()
		if err == nil {
			stats[key+"_ttl"] = ttl.Seconds()
		}
	}

	return stats, nil
}

func (r *chatCacheRepository) DeleteMessage(ctx context.Context, chatID, messageID uint) error {
	if chatID == 0 || messageID == 0 {
		return fmt.Errorf("chatID and messageID cannot be zero")
	}

	key := r.getMessageKey(chatID)

	values, err := r.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("failed to get messages from redis: %w", err)
	}

	for _, v := range values {
		var msg model.Message
		if err := json.Unmarshal([]byte(v), &msg); err != nil {
			continue
		}

		if msg.ID == messageID {
			if err := r.rdb.LRem(ctx, key, 0, v).Err(); err != nil {
				return fmt.Errorf("failed to remove message from redis: %w", err)
			}
			// Если вдруг есть дубли — уберём все
		}
	}

	return nil
}

// Legacy методы для обратной совместимости
func (r *chatCacheRepository) SaveMessageLegacy(chatID uint, msg model.Message) error {
	return r.SaveMessage(context.Background(), chatID, msg)
}

func (r *chatCacheRepository) GetMessagesLegacy(chatID uint) ([]model.Message, error) {
	return r.GetMessages(context.Background(), chatID)
}

func (r *chatCacheRepository) ClearMessagesLegacy(chatID uint) error {
	return r.ClearMessages(context.Background(), chatID)
}

func (r *chatCacheRepository) AddUserToChatLegacy(chatID, userID uint) error {
	return r.AddUserToChat(context.Background(), chatID, userID)
}

func (r *chatCacheRepository) RemoveUserFromChatLegacy(chatID, userID uint) (int64, error) {
	return r.RemoveUserFromChat(context.Background(), chatID, userID)
}

func (r *chatCacheRepository) DeleteMessageLegacy(chatID, messageID uint) error {
	return r.DeleteMessage(context.Background(), chatID, messageID)
}
