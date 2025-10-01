package service

import (
	"tush00nka/bbbab_messenger/internal/model"
	"tush00nka/bbbab_messenger/internal/repository"
)

type chatService struct {
	chatRepo repository.ChatRepository
}

func NewChatService(chatRepo repository.ChatRepository) ChatService {
	return &chatService{chatRepo: chatRepo}
}

func (s *chatService) CreateChat(chat *model.Chat) error {
	return s.chatRepo.Create(chat)
}

func (s *chatService) GetChatForUsers(user1ID, user2ID uint) (*model.Chat, error) {
	return s.chatRepo.GetForUsers(user1ID, user2ID)
}

func (s *chatService) AddUsersToChat(chatID uint, userIDs ...uint) error {
	for _, userID := range userIDs {
		err := s.chatRepo.AddUser(chatID, userID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *chatService) SendMessageToChat(chat *model.Chat, message model.Message) error {
	return s.chatRepo.SendMessage(chat, message)
}

func (s *chatService) GetMessagesOfChat(chatID uint) ([]model.Message, error) {
	return s.chatRepo.GetMessages(chatID)
}

func (s *chatService) CreateGroupChat(name string, userIDs []uint) (*model.Chat, error) {
	chat := &model.Chat{Name: name}
	err := s.chatRepo.CreateGroup(chat, userIDs)
	if err != nil {
		return nil, err
	}
	return chat, nil
}
