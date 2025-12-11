package service

import (
	"context"
	"io"
	"time"
	"tush00nka/bbbab_messenger/internal/model"
)

type UserService interface {
	CreateUser(user *model.User) error
	GetUserByID(id uint) (*model.User, error)
	GetUserByUsername(username string) (*model.User, error)
	GetUserByPhone(phone string) (*model.User, error)
	UpdateUser(user *model.User) error
	UsernameExists(username string) (bool, error)
	PhoneExists(phone string) (bool, error)
	SearchUsers(prompt string) ([]*model.User, error)
	// DeleteUser(id uint) error
	// ListUsers() ([]model.User, error)
}

type MessageService interface {
	CreateMessage(message *model.Message) error
}

type ChatService interface {
	// Основные операции с чатами
	CreateChat(ctx context.Context, chat *model.Chat) error
	GetChatByID(ctx context.Context, chatID uint) (*model.Chat, error)
	DeleteChat(ctx context.Context, chatID uint) error
	UpdateChat(ctx context.Context, chat *model.Chat) error

	// Операции с участниками
	AddUsersToChat(ctx context.Context, chatID uint, userIDs ...uint) error
	RemoveUserFromChat(ctx context.Context, chatID, userID uint) error
	GetChatUsers(ctx context.Context, chatID uint) ([]model.User, error)
	IsUserInChat(ctx context.Context, chatID, userID uint) (bool, error)

	// Операции с сообщениями
	SendMessageToChat(ctx context.Context, chat *model.Chat, message *model.Message) error
	GetChatMessages(ctx context.Context, chatID uint, cursor string, limit int, direction string) (
		[]model.Message, bool, bool, *int64, error)
	GetRecentMessages(ctx context.Context, chatID uint, limit int) ([]model.Message, error)
	GetMessageByID(ctx context.Context, messageID uint) (*model.Message, error)
	MarkMessageAsRead(ctx context.Context, messageID, userID uint) error
	DeleteMessage(ctx context.Context, messageID uint) error

	// Операции с пользовательскими чатами
	GetChatsForUser(ctx context.Context, userID uint) (*[]model.Chat, error)
	GetDirectChatsForUser(ctx context.Context, userID uint) ([]model.Chat, error)
	GetChatForUsers(ctx context.Context, user1ID, user2ID uint) (*model.Chat, error)

	// Групповые чаты
	CreateGroupChat(ctx context.Context, name string, userIDs []uint) (*model.Chat, error)
	UpdateGroupInfo(ctx context.Context, chatID uint, name, description string) error

	// Статистика и утилиты
	GetChatStatistics(ctx context.Context, chatID uint) (*ChatStatistics, error)
	SearchMessages(ctx context.Context, chatID uint, query string, limit int) ([]model.Message, error)
	GetUnreadCount(ctx context.Context, userID, chatID uint) (int64, error)
}

type IS3Service interface {
	UploadFile(ctx context.Context, file io.Reader, filename, contentType, userID, chatID string) (*model.FileMetadata, error)
	GeneratePresignedURL(ctx context.Context, fileMetadata *model.FileMetadata, expires time.Duration) (string, error)
	HealthCheck(ctx context.Context) error
}
