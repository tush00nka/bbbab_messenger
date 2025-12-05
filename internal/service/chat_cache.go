package service

import (
	"context"
	"log"
	"time"
	"tush00nka/bbbab_messenger/internal/model"
	"tush00nka/bbbab_messenger/internal/repository"

	"gorm.io/gorm"
)

// ChatCacheService сервис для кеширования чатов
type ChatCacheService struct {
	cacheRepo repository.ChatCacheRepository
	chatRepo  repository.ChatRepository
}

// NewChatCacheService создает новый экземпляр ChatCacheService
func NewChatCacheService(
	cacheRepo repository.ChatCacheRepository,
	chatRepo repository.ChatRepository,
) *ChatCacheService {
	return &ChatCacheService{
		cacheRepo: cacheRepo,
		chatRepo:  chatRepo,
	}
}

// SendMessage сохраняет сообщение в кеше
func (s *ChatCacheService) SendMessage(ctx context.Context, chat *model.Chat, msg model.Message) error {
	if chat == nil || chat.ID == 0 {
		return nil // Невозможно кешировать без ID чата
	}

	// Добавляем Timestamp, если не установлен
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	return s.cacheRepo.SaveMessage(ctx, chat.ID, msg)
}

// GetMessages получает сообщения из кеша
func (s *ChatCacheService) GetMessages(ctx context.Context, chatID uint) ([]model.Message, error) {
	if chatID == 0 {
		return nil, nil
	}

	messages, err := s.cacheRepo.GetMessages(ctx, chatID)
	if err != nil {
		log.Printf("failed to get messages from cache: %v", err)
		return nil, err
	}

	return messages, nil
}

// CacheMessages сохраняет сообщения в кеше
func (s *ChatCacheService) CacheMessages(ctx context.Context, chatID uint, messages []model.Message) error {
	if chatID == 0 || len(messages) == 0 {
		return nil
	}

	for _, msg := range messages {
		if err := s.cacheRepo.SaveMessage(ctx, chatID, msg); err != nil {
			log.Printf("failed to cache message: %v", err)
			// Продолжаем кеширование остальных сообщений
		}
	}

	return nil
}

// UserJoined добавляет пользователя в кеш присутствия
func (s *ChatCacheService) UserJoined(ctx context.Context, chatID, userID uint) error {
	if chatID == 0 || userID == 0 {
		return nil
	}

	if err := s.cacheRepo.AddUserToChat(ctx, chatID, userID); err != nil {
		log.Printf("failed to add user to chat cache: %v", err)
		return err
	}

	return nil
}

// UserLeft удаляет пользователя из кеша присутствия
func (s *ChatCacheService) UserLeft(ctx context.Context, chatID, userID uint) error {
	if chatID == 0 || userID == 0 {
		return nil
	}

	count, err := s.cacheRepo.RemoveUserFromChat(ctx, chatID, userID)
	if err != nil {
		log.Printf("failed to remove user from chat cache: %v", err)
		return err
	}

	// Если никого не осталось в чате → сбрасываем сообщения в БД
	if count == 0 {
		if err := s.flushMessagesToDB(ctx, chatID); err != nil {
			log.Printf("failed to flush messages to DB: %v", err)
			return err
		}
	}

	return nil
}

// flushMessagesToDB сохраняет сообщения из кеша в БД и очищает кеш
func (s *ChatCacheService) flushMessagesToDB(ctx context.Context, chatID uint) error {
	// Получаем сообщения из кеша
	messages, err := s.cacheRepo.GetMessages(ctx, chatID)
	if err != nil {
		return err
	}

	// Если сообщений нет, просто очищаем кеш
	if len(messages) == 0 {
		return s.cacheRepo.ClearMessages(ctx, chatID)
	}

	// Сохраняем каждое сообщение в БД
	for _, msg := range messages {
		// Создаем минимальный объект чата для передачи в репозиторий
		chat := &model.Chat{Model: gorm.Model{ID: chatID}}

		// Сохраняем в БД
		if err := s.chatRepo.SendMessage(ctx, chat, msg); err != nil {
			log.Printf("failed to persist message from cache: %v", err)
			// Продолжаем сохранение остальных сообщений
		}
	}

	// Очищаем кеш после сохранения
	if err := s.cacheRepo.ClearMessages(ctx, chatID); err != nil {
		log.Printf("failed to clear cache: %v", err)
		return err
	}

	return nil
}

// GetActiveUsers возвращает активных пользователей чата
func (s *ChatCacheService) GetActiveUsers(ctx context.Context, chatID uint) ([]uint, error) {
	if chatID == 0 {
		return nil, nil
	}

	return s.cacheRepo.GetChatUsers(ctx, chatID)
}

// IsUserActive проверяет, активен ли пользователь в чате
func (s *ChatCacheService) IsUserActive(ctx context.Context, chatID, userID uint) (bool, error) {
	if chatID == 0 || userID == 0 {
		return false, nil
	}

	return s.cacheRepo.IsUserInChat(ctx, chatID, userID)
}

// GetActiveChatsCount возвращает количество активных чатов
func (s *ChatCacheService) GetActiveChatsCount(ctx context.Context) (int64, error) {
	return s.cacheRepo.GetActiveChatsCount(ctx)
}

// ClearChat очищает кеш чата
func (s *ChatCacheService) ClearChat(ctx context.Context, chatID uint) error {
	if chatID == 0 {
		return nil
	}

	return s.cacheRepo.ClearChat(ctx, chatID)
}

// GetChatStats возвращает статистику кеша чата
func (s *ChatCacheService) GetChatStats(ctx context.Context, chatID uint) (map[string]interface{}, error) {
	if chatID == 0 {
		return nil, nil
	}

	stats := make(map[string]interface{})

	// Количество сообщений в кеше
	messages, err := s.cacheRepo.GetMessages(ctx, chatID)
	if err == nil {
		stats["cached_messages"] = len(messages)
	}

	// Активные пользователи
	users, err := s.cacheRepo.GetChatUsers(ctx, chatID)
	if err == nil {
		stats["active_users"] = len(users)
	}

	return stats, nil
}

// Legacy методы для обратной совместимости
func (s *ChatCacheService) SendMessageLegacy(chat *model.Chat, msg model.Message) error {
	return s.SendMessage(context.Background(), chat, msg)
}

func (s *ChatCacheService) GetMessagesLegacy(chatID uint) ([]model.Message, error) {
	return s.GetMessages(context.Background(), chatID)
}

func (s *ChatCacheService) UserJoinedLegacy(chatID, userID uint) error {
	return s.UserJoined(context.Background(), chatID, userID)
}

func (s *ChatCacheService) UserLeftLegacy(chatID, userID uint) error {
	return s.UserLeft(context.Background(), chatID, userID)
}
