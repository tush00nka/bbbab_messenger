package service

import "tush00nka/bbbab_messenger/internal/model"

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
	GetMessagesOfChat(chatID uint) ([]model.Message, error)
	CreateGroupChat(name string, userIDs []uint) (*model.Chat, error)
	IsUserInChat(chatID uint, userID uint) (bool, error)
}
