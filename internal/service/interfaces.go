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
	CreateChat(chat *model.Chat) error
	GetChatForUsers(user1ID, user2ID uint) (*model.Chat, error)
	AddUsersToChat(chatID uint, userIDs ...uint) error
	SendMessageToChat(chat *model.Chat, message model.Message) error
	// GetMessagesOfChat(chatID uint) ([]model.Message, error)
	GetChatMessages(chatID uint, cursor string, limit int, direction string, ctx context.Context) ([]model.Message, bool, bool, *int64, error)
	CreateGroupChat(name string, userIDs []uint) (*model.Chat, error)
	IsUserInChat(chatID uint, userID uint) (bool, error)
}

type S3Service interface {
	UploadFile(ctx context.Context, file io.Reader, filename, contentType, userID, chatID string) (*model.FileMetadata, error)
	GeneratePresignedURL(ctx context.Context, fileMetadata *model.FileMetadata, expires time.Duration) (string, error)
	HealthCheck(ctx context.Context) error
}
