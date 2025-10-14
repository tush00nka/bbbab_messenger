package app

import (
	"fmt"
	"log"
	"tush00nka/bbbab_messenger/internal/config"
	"tush00nka/bbbab_messenger/internal/handler"
	"tush00nka/bbbab_messenger/internal/pkg/sms"
	"tush00nka/bbbab_messenger/internal/repository"
	"tush00nka/bbbab_messenger/internal/service"

	"github.com/redis/go-redis/v9"
)

type App struct {
}

func Run(cfg *config.Config) {
	// Postgres
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.DBPort)
	db, err := repository.NewDB(dsn)
	if err != nil {
		log.Fatal(err)
	}

	// storage := storage.NewRedisStorage(fmt.Sprintf("storage:%s", cfg.RedisPort), cfg.RedisPassword, 0) // TODO: get rid of magic number
	sms := sms.NewMockSMSProvider("SOMETOKEN")

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("redis:%s", cfg.RedisPort),
		Password: cfg.RedisPassword,
	})
	cacheRepo := repository.NewChatCacheRepository(rdb)
	smsRepo := repository.NewSMSRepository(rdb)

	// User
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService, smsRepo, sms)

	// Chat
	chatRepo := repository.NewChatRepository(db)
	chatService := service.NewChatService(chatRepo)
	chatCacheService := service.NewChatCacheService(cacheRepo, chatRepo)
	chatHandler := handler.NewChatHandler(chatService, chatCacheService)

	server := NewServer(userHandler, chatHandler)
	server.Run(cfg.ServerPort)
}
