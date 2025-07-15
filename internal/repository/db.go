package repository

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Настройки логгирования
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Настройка пула соединений
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Конфигурация пула соединений
	sqlDB.SetMaxIdleConns(10)           // Максимальное количество бездействующих соединений
	sqlDB.SetMaxOpenConns(100)          // Максимальное количество открытых соединений
	sqlDB.SetConnMaxLifetime(time.Hour) // Максимальное время жизни соединения

	return db, nil
}
