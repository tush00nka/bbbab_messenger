package app

import (
	"log"
	"tush00nka/bbbab_messenger/internal/config"
	"tush00nka/bbbab_messenger/internal/handler"
	"tush00nka/bbbab_messenger/internal/repository"
	"tush00nka/bbbab_messenger/internal/service"
)

type App struct {
}

func Run(cfg *config.Config) {
	db, err := repository.NewDB(cfg.DSN)
	if err != nil {
		log.Fatal(err)
	}

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)
	chatRepo := repository.NewChatRepository(db)
	chatService := service.NewChatService(chatRepo)
	chatHandler := handler.NewChatHandler(chatService)

	server := NewServer(userHandler, chatHandler)
	server.Run(cfg.Port)
}
