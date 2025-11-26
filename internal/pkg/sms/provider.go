package sms

import (
	"fmt"
	"math/rand"
)

type SMSProvider interface {
	SendSMS(phone, message string) error
}

type MockSMSProvider struct {
	Token string
}

func NewMockSMSProvider(token string) *MockSMSProvider {
	return &MockSMSProvider{Token: token}
}

func (p *MockSMSProvider) SendSMS(phone, message string) error {
	// Здесь будет интеграция с SMS-сервисом
	fmt.Printf("SMS to %s: %s\n", phone, message)
	return nil
}

func GenerateVerificationCode() string {
	return fmt.Sprintf("%05d", rand.Intn(100000))
}
