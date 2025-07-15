package service

import "tush00nka/bbbab_messenger/internal/model"

type UserService interface {
	CreateUser(user *model.User) error
	GetUserByID(id uint) (*model.User, error)
	GetUserByUsername(username string) (*model.User, error)
	UpdateUser(user *model.User) error
	UsernameExists(username string) (bool, error)
	// DeleteUser(id uint) error
	// ListUsers() ([]model.User, error)
}
