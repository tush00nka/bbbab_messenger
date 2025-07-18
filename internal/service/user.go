package service

import (
	"errors"
	"tush00nka/bbbab_messenger/internal/model"
	"tush00nka/bbbab_messenger/internal/repository"
)

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) CreateUser(user *model.User) error {
	// Валидация данных перед созданием
	if user.Username == "" {
		return errors.New("username is required")
	}
	if user.Password == "" {
		return errors.New("password is required")
	}

	return s.userRepo.Create(user)
}

func (s *userService) GetUserByID(id uint) (*model.User, error) {
	if id == 0 {
		return nil, errors.New("invalid user ID")
	}

	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	// user.SanitizePassword() // Удаляем чувствительные данные перед возвратом

	return user, nil
}

func (s *userService) GetUserByUsername(username string) (*model.User, error) {
	if username == "" {
		return nil, errors.New("invalid username")
	}

	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}

	// user.SanitizePassword() // Удаляем чувствительные данные перед возвратом

	return user, nil
}

func (s *userService) UpdateUser(user *model.User) error {
	// Проверяем существование пользователя
	existingUser, err := s.userRepo.FindByID(user.ID)
	if err != nil {
		return err
	}

	// Обновляем только разрешенные поля
	existingUser.Password = user.Password

	return s.userRepo.Update(existingUser)
}

func (s *userService) UsernameExists(username string) (bool, error) {
	return s.userRepo.UsernameExists(username)
}

func (s *userService) SearchUsers(prompt string) ([]*model.User, error) {
	return s.userRepo.Search(prompt)
}
