package app

import (
	"fmt"
	"log"
	"net/http"
	"tush00nka/bbbab_messenger/internal/config"
	"tush00nka/bbbab_messenger/internal/handler"
	"tush00nka/bbbab_messenger/internal/model"
	"tush00nka/bbbab_messenger/internal/pkg/sms"
	"tush00nka/bbbab_messenger/internal/pkg/tg"
	"tush00nka/bbbab_messenger/internal/repository"
	"tush00nka/bbbab_messenger/internal/service"
	"tush00nka/bbbab_messenger/internal/ws"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type App struct {
}

type simpleLogger struct{}

func (l *simpleLogger) Info(msg string, fields ...any) {
	log.Printf("[INFO] "+msg, fields...)
}

func (l *simpleLogger) Warn(msg string, fields ...any) {
	log.Printf("[WARN] "+msg, fields...)
}

func (l *simpleLogger) Error(msg string, fields ...any) {
	log.Printf("[ERROR] "+msg, fields...)
}

func (l *simpleLogger) Debug(msg string, fields ...any) {
	log.Printf("[DEBUG] "+msg, fields...)
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

	// Создаем Telegram-бота
	tgBot, err := tg.NewTelegramAdapter(cfg.TGBotAPI)
	if err != nil {
		log.Fatal("Failed to create S3 service", err)
	}

	// Запускаем канал обновлений типа))
	go tgBot.UpdateUserDatabase()

	// User
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService, smsRepo, sms, tgBot)

	db.Set("gorm:table_options", "CREATE TABLE chat_users (chat_id bigint, user_id bigint, PRIMARY KEY(chat_id, user_id))").AutoMigrate(&model.ChatUser{})

	// Chat
	chatRepo := repository.NewChatRepository(db)
	chatService := service.NewChatService(chatRepo)
	chatCacheService := service.NewChatCacheService(cacheRepo, chatRepo)
	s3Service, err := service.NewS3Service(cfg)
	if err != nil {
		log.Fatal("Failed to create S3 service", err)
	}

	// WS Hub
	hub := ws.NewHub()

	// WebSocket Upgrader с настройками
	wsUpgrader := &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// В development разрешаем все origins
			// В production нужно настроить правильные домены
			return true
		},
	}

	// Создаем логгер
	logger := &simpleLogger{}

	chatHandler := handler.NewChatHandler(chatService, chatCacheService, s3Service, hub, wsUpgrader, logger)
	server := NewServer(userHandler, chatHandler)
	server.Run(cfg.ServerPort)
}
