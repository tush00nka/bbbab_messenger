package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"tush00nka/bbbab_messenger/internal/model"

	"github.com/go-redis/redis/v8"
)

type RedisStorage struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisStorage(addr, password string, db int) *RedisStorage {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		panic(fmt.Sprintf("Failed to connect to Redis: %v", err))
	}

	return &RedisStorage{
		client: client,
		ctx:    ctx,
	}
}

func (s *RedisStorage) SaveVerificationCode(phone, code string, expiresIn time.Duration) error {
	verification := model.VerificationCode{
		Phone:     phone,
		Code:      code,
		ExpiresAt: time.Now().Add(expiresIn),
	}

	data, err := json.Marshal(verification)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("verification:%s", phone)
	return s.client.Set(s.ctx, key, data, expiresIn).Err()
}

func (s *RedisStorage) GetVerificationCode(phone string) (*model.VerificationCode, error) {
	key := fmt.Sprintf("verification:%s", phone)
	data, err := s.client.Get(s.ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var verification model.VerificationCode
	if err := json.Unmarshal(data, &verification); err != nil {
		return nil, err
	}

	return &verification, nil
}

func (s *RedisStorage) DeleteVerificationCode(phone string) error {
	key := fmt.Sprintf("verification:%s", phone)
	return s.client.Del(s.ctx, key).Err()
}
