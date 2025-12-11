package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"tush00nka/bbbab_messenger/internal/model"
	"tush00nka/bbbab_messenger/internal/repository"
)

// ChatStatistics статистика чата
type ChatStatistics struct {
	TotalMessages  int64     `json:"totalMessages"`
	ActiveUsers    int64     `json:"activeUsers"`
	LastMessageAt  time.Time `json:"lastMessageAt"`
	FirstMessageAt time.Time `json:"firstMessageAt"`
}

// chatService реализация ChatService
type chatService struct {
	chatRepo repository.ChatRepository
}

// NewChatService создает новый экземпляр ChatService
func NewChatService(chatRepo repository.ChatRepository) ChatService {
	return &chatService{chatRepo: chatRepo}
}

// CreateChat создает новый чат
func (s *chatService) CreateChat(ctx context.Context, chat *model.Chat) error {
	if chat == nil {
		return errors.New("chat cannot be nil")
	}

	// Валидация
	if strings.TrimSpace(chat.Name) == "" && chat.IsGroup {
		return errors.New("group chat name cannot be empty")
	}

	// Устанавливаем метаданные
	now := time.Now()
	chat.CreatedAt = now
	chat.UpdatedAt = now

	return s.chatRepo.Create(ctx, chat)
}

// GetChatByID возвращает чат по ID
func (s *chatService) GetChatByID(ctx context.Context, chatID uint) (*model.Chat, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	return s.chatRepo.GetByID(ctx, chatID)
}

// DeleteChat удаляет чат
func (s *chatService) DeleteChat(ctx context.Context, chatID uint) error {
	if chatID == 0 {
		return errors.New("chatID cannot be zero")
	}

	return s.chatRepo.Delete(ctx, chatID)
}

// UpdateChat обновляет информацию о чате
func (s *chatService) UpdateChat(ctx context.Context, chat *model.Chat) error {
	if chat == nil || chat.ID == 0 {
		return errors.New("invalid chat")
	}

	chat.UpdatedAt = time.Now()
	return s.chatRepo.Update(ctx, chat)
}

// AddUsersToChat добавляет пользователей в чат
func (s *chatService) AddUsersToChat(ctx context.Context, chatID uint, userIDs ...uint) error {
	if chatID == 0 {
		return errors.New("chatID cannot be zero")
	}

	if len(userIDs) == 0 {
		return errors.New("at least one userID is required")
	}

	// Проверяем уникальность пользователей
	userMap := make(map[uint]bool)
	for _, userID := range userIDs {
		if userID == 0 {
			return errors.New("userID cannot be zero")
		}
		if userMap[userID] {
			return fmt.Errorf("duplicate userID: %d", userID)
		}
		userMap[userID] = true
	}

	// Добавляем пользователей
	for _, userID := range userIDs {
		if err := s.chatRepo.AddUser(ctx, chatID, userID); err != nil {
			// Возвращаем ошибку, но не откатываем предыдущие добавления
			// Можно улучшить транзакцией, если репозиторий поддерживает
			return fmt.Errorf("failed to add user %d: %w", userID, err)
		}
	}

	return nil
}

// RemoveUserFromChat удаляет пользователя из чата
func (s *chatService) RemoveUserFromChat(ctx context.Context, chatID, userID uint) error {
	if chatID == 0 || userID == 0 {
		return errors.New("chatID and userID cannot be zero")
	}

	return s.chatRepo.RemoveUser(ctx, chatID, userID)
}

// GetChatUsers возвращает пользователей чата
func (s *chatService) GetChatUsers(ctx context.Context, chatID uint) ([]model.User, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	return s.chatRepo.GetChatUsers(ctx, chatID)
}

// IsUserInChat проверяет, является ли пользователь участником чата
func (s *chatService) IsUserInChat(ctx context.Context, chatID, userID uint) (bool, error) {
	if chatID == 0 || userID == 0 {
		return false, errors.New("chatID and userID cannot be zero")
	}

	return s.chatRepo.IsUserInChat(ctx, chatID, userID)
}

// SendMessageToChat отправляет сообщение в чат
func (s *chatService) SendMessageToChat(ctx context.Context, chat *model.Chat, message *model.Message) error {
	if chat == nil || chat.ID == 0 {
		return errors.New("invalid chat")
	}

	if message == nil {
		return errors.New("message cannot be nil")
	}

	if message.SenderID == 0 {
		return errors.New("senderID cannot be zero")
	}

	if strings.TrimSpace(message.Message) == "" {
		return errors.New("message cannot be empty")
	}

	now := time.Now()

	// GORM всё равно проставит CreatedAt/UpdatedAt, но мы можем синхронизировать Timestamp
	if message.CreatedAt.IsZero() {
		message.CreatedAt = now
	}
	message.UpdatedAt = now

	if message.Timestamp.IsZero() {
		// Делаем Timestamp согласованным с CreatedAt, чтобы пагинация по времени была стабильной
		message.Timestamp = message.CreatedAt
	}

	return s.chatRepo.SendMessage(ctx, chat, message)
}

// GetChatMessages возвращает сообщения чата с пагинацией
func (s *chatService) GetChatMessages(
	ctx context.Context,
	chatID uint,
	cursor string,
	limit int,
	direction string,
) ([]model.Message, bool, bool, *int64, error) {
	if chatID == 0 {
		return nil, false, false, nil, errors.New("chatID cannot be zero")
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	if direction != "older" && direction != "newer" {
		direction = "older"
	}

	return s.chatRepo.GetChatMessages(ctx, chatID, cursor, limit, direction)
}

// GetRecentMessages возвращает последние сообщения чата
func (s *chatService) GetRecentMessages(ctx context.Context, chatID uint, limit int) ([]model.Message, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	return s.chatRepo.GetRecentMessages(ctx, chatID, limit)
}

// GetMessageByID возвращает сообщение по ID
func (s *chatService) GetMessageByID(ctx context.Context, messageID uint) (*model.Message, error) {
	if messageID == 0 {
		return nil, errors.New("messageID cannot be zero")
	}

	return s.chatRepo.GetMessageByID(ctx, messageID)
}

// MarkMessageAsRead отмечает сообщение как прочитанное
func (s *chatService) MarkMessageAsRead(ctx context.Context, messageID, userID uint) error {
	if messageID == 0 || userID == 0 {
		return errors.New("messageID and userID cannot be zero")
	}

	return s.chatRepo.MarkMessageAsRead(ctx, messageID, userID)
}

// DeleteMessage удаляет сообщение
func (s *chatService) DeleteMessage(ctx context.Context, messageID uint) error {
	if messageID == 0 {
		return errors.New("messageID cannot be zero")
	}

	return s.chatRepo.DeleteMessage(ctx, messageID)
}

// GetChatsForUser возвращает все чаты пользователя
func (s *chatService) GetChatsForUser(ctx context.Context, userID uint) (*[]model.Chat, error) {
	if userID == 0 {
		return nil, errors.New("userID cannot be zero")
	}

	return s.chatRepo.GetChatsForUser(ctx, userID)
}

// GetDirectChatsForUser возвращает личные чаты пользователя
func (s *chatService) GetDirectChatsForUser(ctx context.Context, userID uint) ([]model.Chat, error) {
	if userID == 0 {
		return nil, errors.New("userID cannot be zero")
	}

	return s.chatRepo.GetDirectChatsForUser(ctx, userID)
}

// GetChatForUsers возвращает чат между двумя пользователями
func (s *chatService) GetChatForUsers(ctx context.Context, user1ID, user2ID uint) (*model.Chat, error) {
	if user1ID == 0 || user2ID == 0 {
		return nil, errors.New("userIDs cannot be zero")
	}

	if user1ID == user2ID {
		return nil, errors.New("userIDs must be different")
	}

	return s.chatRepo.GetForUsers(ctx, user1ID, user2ID)
}

// CreateGroupChat создает групповой чат
func (s *chatService) CreateGroupChat(ctx context.Context, name string, userIDs []uint) (*model.Chat, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("group chat name cannot be empty")
	}

	if len(userIDs) < 2 {
		return nil, errors.New("group chat requires at least 2 users")
	}

	// Проверяем уникальность пользователей
	userMap := make(map[uint]bool)
	for _, userID := range userIDs {
		if userID == 0 {
			return nil, errors.New("userID cannot be zero")
		}
		if userMap[userID] {
			return nil, fmt.Errorf("duplicate userID: %d", userID)
		}
		userMap[userID] = true
	}

	chat := &model.Chat{
		Name:    name,
		IsGroup: true,
	}

	err := s.chatRepo.CreateGroup(ctx, chat, userIDs)
	if err != nil {
		return nil, err
	}

	return chat, nil
}

// UpdateGroupInfo обновляет информацию о групповом чате
func (s *chatService) UpdateGroupInfo(ctx context.Context, chatID uint, name, description string) error {
	if chatID == 0 {
		return errors.New("chatID cannot be zero")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("group name cannot be empty")
	}

	return s.chatRepo.UpdateGroupInfo(ctx, chatID, name, description)
}

// GetChatStatistics возвращает статистику чата
func (s *chatService) GetChatStatistics(ctx context.Context, chatID uint) (*ChatStatistics, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	stats, err := s.chatRepo.GetChatStatistics(ctx, chatID)
	if err != nil {
		return nil, err
	}

	return &ChatStatistics{
		TotalMessages:  stats.TotalMessages,
		ActiveUsers:    stats.ActiveUsers,
		LastMessageAt:  stats.LastMessageAt,
		FirstMessageAt: stats.FirstMessageAt,
	}, nil
}

// SearchMessages ищет сообщения в чате
func (s *chatService) SearchMessages(ctx context.Context, chatID uint, query string, limit int) ([]model.Message, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return s.chatRepo.SearchMessages(ctx, chatID, query, limit)
}

// GetUnreadCount возвращает количество непрочитанных сообщений
func (s *chatService) GetUnreadCount(ctx context.Context, userID, chatID uint) (int64, error) {
	if userID == 0 {
		return 0, errors.New("userID cannot be zero")
	}

	return s.chatRepo.GetUnreadCount(ctx, userID, chatID)
}

// Legacy методы для обратной совместимости
func (s *chatService) CreateChatLegacy(chat *model.Chat) error {
	return s.CreateChat(context.Background(), chat)
}

func (s *chatService) GetChatForUsersLegacy(user1ID, user2ID uint) (*model.Chat, error) {
	return s.GetChatForUsers(context.Background(), user1ID, user2ID)
}

func (s *chatService) AddUsersToChatLegacy(chatID uint, userIDs ...uint) error {
	return s.AddUsersToChat(context.Background(), chatID, userIDs...)
}

func (s *chatService) SendMessageToChatLegacy(chat *model.Chat, message model.Message) error {
	return s.SendMessageToChat(context.Background(), chat, &message)
}

func (s *chatService) GetChatMessagesLegacy(chatID uint, cursor string, limit int, direction string, ctx context.Context) (
	[]model.Message, bool, bool, *int64, error) {
	return s.GetChatMessages(ctx, chatID, cursor, limit, direction)
}

func (s *chatService) CreateGroupChatLegacy(name string, userIDs []uint) (*model.Chat, error) {
	return s.CreateGroupChat(context.Background(), name, userIDs)
}

func (s *chatService) IsUserInChatLegacy(chatID, userID uint) (bool, error) {
	return s.IsUserInChat(context.Background(), chatID, userID)
}

func (s *chatService) GetChatsForUserLegacy(userID uint) (*[]model.Chat, error) {
	return s.GetChatsForUser(context.Background(), userID)
}

func (s *chatService) GetChatUsersLegacy(chatID uint) ([]model.User, error) {
	return s.GetChatUsers(context.Background(), chatID)
}
