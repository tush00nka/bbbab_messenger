package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"tush00nka/bbbab_messenger/internal/model"

	"github.com/redis/go-redis/v9"
)

type SMSRepository interface {
	SaveVerificationCode(phone, code string, expiresIn time.Duration) error
	GetVerificationCode(phone string) (*model.VerificationCode, error)
	DeleteVerificationCode(phone string) error
}

type smsRepository struct {
	rdb *redis.Client
	ctx context.Context
}

func NewSMSRepository(rdb *redis.Client) SMSRepository {
	ctx := context.Background()

	if err := rdb.Ping(ctx).Err(); err != nil {
		panic(fmt.Sprintf("Failed to connect to Redis: %v", err))
	}

	return &smsRepository{
		rdb: rdb,
		ctx: ctx,
	}
}

func (s *smsRepository) SaveVerificationCode(phone, code string, expiresIn time.Duration) error {
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
	return s.rdb.Set(s.ctx, key, data, expiresIn).Err()
}

func (s *smsRepository) GetVerificationCode(phone string) (*model.VerificationCode, error) {
	key := fmt.Sprintf("verification:%s", phone)
	data, err := s.rdb.Get(s.ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var verification model.VerificationCode
	if err := json.Unmarshal(data, &verification); err != nil {
		return nil, err
	}

	return &verification, nil
}

func (s *smsRepository) DeleteVerificationCode(phone string) error {
	key := fmt.Sprintf("verification:%s", phone)
	return s.rdb.Del(s.ctx, key).Err()
}
