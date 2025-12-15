package tg

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramSender interface {
	SendMessage(userID int64, message string) error
	GetID(phone string) int64
}

type TelegramAdapter struct {
	bot     *tgbotapi.BotAPI
	chatIDs map[string]int64
}

func NewTelegramAdapter(botToken string) (*TelegramAdapter, error) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, err
	}

	return &TelegramAdapter{bot: bot, chatIDs: make(map[string]int64)}, nil
}

// SendMessage отправляет текстовое сообщение в Telegram-чат.
// Принимает контекст (ctx) и текст сообщения (message).
// Возвращает ошибку, если сообщение не удалось отправить.
func (t *TelegramAdapter) SendMessage(userID int64, message string) error {
	// Создаём новое текстовое сообщение для отправки в указанный чат
	msg := tgbotapi.NewMessage(userID, message)

	log.Println(message)

	// Отправляем сообщение через API Telegram
	_, err := t.bot.Send(msg)

	// Возвращаем ошибку, если отправка не удалась
	return err
}

func (t *TelegramAdapter) GetID(phone string) int64 {
	if id, ok := t.chatIDs[phone]; ok {
		return id
	}

	return 0
}

func (t *TelegramAdapter) UpdateUserDatabase() {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := t.bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		id := update.Message.From.ID
		command := update.Message.Command()
		contact := update.Message.Contact

		if contact != nil {
			phone := contact.PhoneNumber
			if phone[0] != '+' {
				phone = fmt.Sprintf("+%s", phone)
			}
			log.Printf("Shared phone number via Telegram: %s\n", phone)
			t.chatIDs[phone] = id
			t.SendMessage(id, "Спасибо)")
			continue
		}

		if command != "" {
			if command == "start" {
				keyboard := tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButtonContact("Поделиться номером телефона")))
				msg := tgbotapi.NewMessage(id, "Поделитесь номером телефона")
				msg.ReplyMarkup = keyboard
				t.bot.Send(msg)
			} else {
				t.SendMessage(id, "Неизвестная команда")
			}
		} else {
			t.SendMessage(id, "Не команда!")
		}
	}
}
