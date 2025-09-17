package app

import (
	"fmt"
	"log"
	"tush00nka/bbbab_messenger/internal/config"
	"tush00nka/bbbab_messenger/internal/handler"
	"tush00nka/bbbab_messenger/internal/pkg/sms"
	"tush00nka/bbbab_messenger/internal/pkg/storage"
	"tush00nka/bbbab_messenger/internal/repository"
	"tush00nka/bbbab_messenger/internal/service"
)

type App struct {
}

func Run(cfg *config.Config) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.DBPort)
	db, err := repository.NewDB(dsn)
	if err != nil {
		log.Fatal(err)
	}

	storage := storage.NewRedisStorage(fmt.Sprintf("storage:%s", cfg.RedisPort), cfg.RedisPassword, 0) // TODO: get rid of magic number
	sms := sms.NewMockSMSProvider("SOMETOKEN")

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService, storage, sms)

	chatRepo := repository.NewChatRepository(db)
	chatService := service.NewChatService(chatRepo)
	chatHandler := handler.NewChatHandler(chatService)

	server := NewServer(userHandler, chatHandler)
	server.Run(cfg.ServerPort)
}
