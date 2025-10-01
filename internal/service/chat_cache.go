package service

import (
	"log"
	"tush00nka/bbbab_messenger/internal/model"
	"tush00nka/bbbab_messenger/internal/repository"
)

type ChatCacheService struct {
	cacheRepo repository.ChatCacheRepository
	chatRepo  repository.ChatRepository
}

func NewChatCacheService(cacheRepo repository.ChatCacheRepository, chatRepo repository.ChatRepository) *ChatCacheService {
	return &ChatCacheService{
		cacheRepo: cacheRepo,
		chatRepo:  chatRepo,
	}
}

func (s *ChatCacheService) SendMessage(chat *model.Chat, msg model.Message) error {
	return s.cacheRepo.SaveMessage(chat.ID, msg)
}

func (s *ChatCacheService) GetMessages(chatID uint) ([]model.Message, error) {
	return s.cacheRepo.GetMessages(chatID)
}

func (s *ChatCacheService) UserJoined(chatID, userID uint) error {
	return s.cacheRepo.AddUserToChat(chatID, userID)
}

func (s *ChatCacheService) UserLeft(chatID, userID uint) error {
	count, err := s.cacheRepo.RemoveUserFromChat(chatID, userID)
	if err != nil {
		return err
	}

	// Если никого не осталось → сбрасываем сообщения в БД
	if count == 0 {
		messages, err := s.cacheRepo.GetMessages(chatID)
		if err != nil {
			return err
		}

		for _, msg := range messages {
			if err := s.chatRepo.SendMessage(&model.Chat{Model: msg.Model}, msg); err != nil {
				log.Printf("failed to persist message: %v", err)
			}
		}

		// Очистка
		if err := s.cacheRepo.ClearMessages(chatID); err != nil {
			return err
		}
	}

	return nil
}
