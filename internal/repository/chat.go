package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"tush00nka/bbbab_messenger/internal/model"

	"gorm.io/gorm"
)

// ChatRepository интерфейс репозитория чатов
type ChatRepository interface {
	// Основные операции с чатами
	Create(ctx context.Context, chat *model.Chat) error
	GetByID(ctx context.Context, chatID uint) (*model.Chat, error)
	Update(ctx context.Context, chat *model.Chat) error
	Delete(ctx context.Context, chatID uint) error

	// Операции с участниками
	AddUser(ctx context.Context, chatID, userID uint) error
	RemoveUser(ctx context.Context, chatID, userID uint) error
	GetChatUsers(ctx context.Context, chatID uint) ([]model.User, error)
	IsUserInChat(ctx context.Context, chatID, userID uint) (bool, error)
	GetChatUsersCount(ctx context.Context, chatID uint) (int64, error)

	// Операции с сообщениями
	SendMessage(ctx context.Context, chat *model.Chat, message model.Message) error
	GetMessages(ctx context.Context, chatID uint) ([]model.Message, error)
	GetRecentMessages(ctx context.Context, chatID uint, limit int) ([]model.Message, error)
	GetMessageByID(ctx context.Context, messageID uint) (*model.Message, error)
	MarkMessageAsRead(ctx context.Context, messageID, userID uint) error
	DeleteMessage(ctx context.Context, messageID uint) error

	// Пагинация сообщений
	GetChatMessages(ctx context.Context, chatID uint, cursor string, limit int, direction string) (
		[]model.Message, bool, bool, *int64, error)

	// Чаты пользователя
	GetChatsForUser(ctx context.Context, userID uint) (*[]model.Chat, error)
	GetDirectChatsForUser(ctx context.Context, userID uint) ([]model.Chat, error)
	GetForUsers(ctx context.Context, user1ID, user2ID uint) (*model.Chat, error)

	// Групповые чаты
	CreateGroup(ctx context.Context, chat *model.Chat, userIDs []uint) error
	UpdateGroupInfo(ctx context.Context, chatID uint, name, description string) error
	GetGroupChatsForUser(ctx context.Context, userID uint) ([]model.Chat, error)

	// Статистика и поиск
	GetChatStatistics(ctx context.Context, chatID uint) (*ChatStats, error)
	SearchMessages(ctx context.Context, chatID uint, query string, limit int) ([]model.Message, error)
	GetUnreadCount(ctx context.Context, userID, chatID uint) (int64, error)
}

// ChatStats статистика чата
type ChatStats struct {
	TotalMessages  int64     `json:"totalMessages"`
	ActiveUsers    int64     `json:"activeUsers"`
	LastMessageAt  time.Time `json:"lastMessageAt"`
	FirstMessageAt time.Time `json:"firstMessageAt"`
}

// chatRepository реализация ChatRepository
type chatRepository struct {
	db *gorm.DB
}

// NewChatRepository создает новый экземпляр репозитория
func NewChatRepository(db *gorm.DB) ChatRepository {
	return &chatRepository{db: db}
}

// Create создает новый чат
func (r *chatRepository) Create(ctx context.Context, chat *model.Chat) error {
	if chat == nil {
		return errors.New("chat cannot be nil")
	}

	return r.db.WithContext(ctx).Create(chat).Error
}

// GetByID возвращает чат по ID
func (r *chatRepository) GetByID(ctx context.Context, chatID uint) (*model.Chat, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	var chat model.Chat
	err := r.db.WithContext(ctx).
		Preload("Users").
		Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.Order("messages.created_at DESC").Limit(20)
		}).
		First(&chat, chatID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &chat, err
}

// Update обновляет информацию о чате
func (r *chatRepository) Update(ctx context.Context, chat *model.Chat) error {
	if chat == nil || chat.ID == 0 {
		return errors.New("invalid chat")
	}

	return r.db.WithContext(ctx).Save(chat).Error
}

// Delete удаляет чат
func (r *chatRepository) Delete(ctx context.Context, chatID uint) error {
	if chatID == 0 {
		return errors.New("chatID cannot be zero")
	}

	// Используем мягкое удаление, если модель поддерживает soft delete
	return r.db.WithContext(ctx).Delete(&model.Chat{}, chatID).Error
}

// Метод AddUser
func (r *chatRepository) AddUser(ctx context.Context, chatID, userID uint) error {
	if chatID == 0 || userID == 0 {
		return errors.New("chatID and userID cannot be zero")
	}

	// Используем модель ChatUser
	chatUser := model.ChatUser{
		ChatID: chatID,
		UserID: userID,
	}

	return r.db.WithContext(ctx).Create(&chatUser).Error
}

// Метод CreateGroup
func (r *chatRepository) CreateGroup(ctx context.Context, chat *model.Chat, userIDs []uint) error {
	if chat == nil {
		return errors.New("chat cannot be nil")
	}

	if len(userIDs) < 2 {
		return errors.New("group must have at least 2 users")
	}

	// Начинаем транзакцию
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Создаем чат
	chat.IsGroup = true
	if err := tx.Create(chat).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Добавляем пользователей через модель ChatUser
	for _, userID := range userIDs {
		chatUser := model.ChatUser{
			ChatID: chat.ID,
			UserID: userID,
		}

		if err := tx.Create(&chatUser).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to add user %d: %w", userID, err)
		}
	}

	return tx.Commit().Error
}

// RemoveUser удаляет пользователя из чата
func (r *chatRepository) RemoveUser(ctx context.Context, chatID, userID uint) error {
	if chatID == 0 || userID == 0 {
		return errors.New("chatID and userID cannot be zero")
	}

	return r.db.WithContext(ctx).Exec(`
		DELETE FROM chat_users
		WHERE chat_id = ? AND user_id = ?
	`, chatID, userID).Error
}

// GetChatUsers возвращает пользователей чата
func (r *chatRepository) GetChatUsers(ctx context.Context, chatID uint) ([]model.User, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	var users []model.User
	err := r.db.WithContext(ctx).Raw(`
		SELECT u.*
		FROM users u
		INNER JOIN chat_users cu ON u.id = cu.user_id
		WHERE cu.chat_id = ?
		ORDER BY u.username
	`, chatID).Scan(&users).Error

	return users, err
}

// GetChatUsersCount возвращает количество пользователей в чате
func (r *chatRepository) GetChatUsersCount(ctx context.Context, chatID uint) (int64, error) {
	if chatID == 0 {
		return 0, errors.New("chatID cannot be zero")
	}

	var count int64
	err := r.db.WithContext(ctx).Table("chat_users").
		Where("chat_id = ?", chatID).
		Count(&count).Error

	return count, err
}

// IsUserInChat проверяет, является ли пользователь участником чата
func (r *chatRepository) IsUserInChat(ctx context.Context, chatID, userID uint) (bool, error) {
	if chatID == 0 || userID == 0 {
		return false, errors.New("chatID and userID cannot be zero")
	}

	var exists int64
	err := r.db.WithContext(ctx).Table("chat_users").
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Count(&exists).Error

	return exists > 0, err
}

// SendMessage отправляет сообщение в чат
func (r *chatRepository) SendMessage(ctx context.Context, chat *model.Chat, message model.Message) error {
	if chat == nil || chat.ID == 0 {
		return errors.New("invalid chat")
	}

	if message.SenderID == 0 {
		return errors.New("senderID cannot be zero")
	}

	// Устанавливаем chat_id
	message.ChatID = chat.ID

	return r.db.WithContext(ctx).Create(&message).Error
}

// GetMessages возвращает все сообщения чата
func (r *chatRepository) GetMessages(ctx context.Context, chatID uint) ([]model.Message, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	var messages []model.Message
	err := r.db.WithContext(ctx).
		Where("chat_id = ?", chatID).
		Order("created_at ASC").
		Find(&messages).Error

	return messages, err
}

// GetRecentMessages возвращает последние сообщения чата
func (r *chatRepository) GetRecentMessages(ctx context.Context, chatID uint, limit int) ([]model.Message, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var messages []model.Message
	err := r.db.WithContext(ctx).
		Where("chat_id = ?", chatID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error

	// Возвращаем в правильном порядке (от старых к новым)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, err
}

// GetMessageByID возвращает сообщение по ID
func (r *chatRepository) GetMessageByID(ctx context.Context, messageID uint) (*model.Message, error) {
	if messageID == 0 {
		return nil, errors.New("messageID cannot be zero")
	}

	var message model.Message
	err := r.db.WithContext(ctx).First(&message, messageID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &message, err
}

// MarkMessageAsRead отмечает сообщение как прочитанное
func (r *chatRepository) MarkMessageAsRead(ctx context.Context, messageID, userID uint) error {
	if messageID == 0 || userID == 0 {
		return errors.New("messageID and userID cannot be zero")
	}

	// Проверяем, существует ли сообщение
	var message model.Message
	err := r.db.WithContext(ctx).First(&message, messageID).Error
	if err != nil {
		return err
	}

	// Проверяем, является ли пользователь участником чата
	isMember, err := r.IsUserInChat(ctx, message.ChatID, userID)
	if err != nil || !isMember {
		return errors.New("user is not a member of this chat")
	}

	// Добавляем запись о прочтении
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO message_reads (message_id, user_id, read_at)
		VALUES (?, ?, NOW())
		ON CONFLICT (message_id, user_id) DO UPDATE
		SET read_at = NOW()
	`, messageID, userID).Error
}

// DeleteMessage удаляет сообщение
func (r *chatRepository) DeleteMessage(ctx context.Context, messageID uint) error {
	if messageID == 0 {
		return errors.New("messageID cannot be zero")
	}

	return r.db.WithContext(ctx).Delete(&model.Message{}, messageID).Error
}

// GetChatMessages возвращает сообщения чата с пагинацией
func (r *chatRepository) GetChatMessages(
	ctx context.Context,
	chatID uint,
	cursor string,
	limit int,
	direction string,
) ([]model.Message, bool, bool, *int64, error) {
	if chatID == 0 {
		return nil, false, false, nil, errors.New("chatID cannot be zero")
	}

	// Валидация параметров
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	if direction != "older" && direction != "newer" {
		direction = "older"
	}

	// Считаем общее количество сообщений
	var totalCount int64
	err := r.db.WithContext(ctx).Model(&model.Message{}).
		Where("chat_id = ?", chatID).
		Count(&totalCount).Error
	if err != nil {
		return nil, false, false, nil, err
	}

	// Строим основной запрос
	query := r.db.WithContext(ctx).
		Model(&model.Message{}).
		Where("chat_id = ?", chatID).
		Preload("Sender").
		Preload("ReadBy")

	// Обрабатываем курсор
	var cursorTime time.Time
	if cursor != "" {
		var err error
		cursorTime, err = time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			// Пробуем другой формат
			cursorTime, err = time.Parse(time.RFC3339, cursor)
			if err != nil {
				return nil, false, false, nil, fmt.Errorf("invalid cursor format: %w", err)
			}
		}

		if direction == "older" {
			query = query.Where("created_at < ?", cursorTime)
		} else {
			query = query.Where("created_at > ?", cursorTime)
		}
	}

	// Определяем порядок сортировки
	if direction == "older" {
		query = query.Order("created_at DESC")
	} else {
		query = query.Order("created_at ASC")
	}

	// Получаем сообщения с лимитом +1 для проверки пагинации
	var messages []model.Message
	err = query.Limit(limit + 1).Find(&messages).Error
	if err != nil {
		return nil, false, false, nil, err
	}

	// Определяем наличие следующей/предыдущей страницы
	hasNext := false
	hasPrevious := false

	if direction == "older" {
		hasNext = len(messages) > limit
		if hasNext {
			messages = messages[:limit]
		}
		hasPrevious = cursor != "" // Если есть курсор, значит есть предыдущие страницы
	} else {
		hasPrevious = len(messages) > limit
		if hasPrevious {
			messages = messages[:limit]
		}
		hasNext = cursor != "" // Если есть курсор, значит есть следующие страницы

		// Для направления "newer" переворачиваем порядок
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}

	return messages, hasNext, hasPrevious, &totalCount, nil
}

// GetChatsForUser возвращает все чаты пользователя
func (r *chatRepository) GetChatsForUser(ctx context.Context, userID uint) (*[]model.Chat, error) {
	if userID == 0 {
		return nil, errors.New("userID cannot be zero")
	}

	var chats []model.Chat
	err := r.db.WithContext(ctx).Raw(`
		SELECT c.*,
		       (SELECT COUNT(*) FROM messages m WHERE m.chat_id = c.id) as message_count,
		       (SELECT m.message FROM messages m WHERE m.chat_id = c.id ORDER BY m.created_at DESC LIMIT 1) as last_message,
		       (SELECT m.created_at FROM messages m WHERE m.chat_id = c.id ORDER BY m.created_at DESC LIMIT 1) as last_message_time
		FROM chats c
		INNER JOIN chat_users cu ON c.id = cu.chat_id
		WHERE cu.user_id = ?
		ORDER BY last_message_time DESC NULLS LAST, c.updated_at DESC
	`, userID).Scan(&chats).Error

	if err != nil {
		return nil, err
	}

	// Загружаем пользователей для каждого чата
	for i := range chats {
		users, err := r.GetChatUsers(ctx, chats[i].ID)
		if err == nil {
			chats[i].Users = users
		}

		// Загружаем последние сообщения
		messages, err := r.GetRecentMessages(ctx, chats[i].ID, 1)
		if err == nil && len(messages) > 0 {
			chats[i].Messages = messages
		}
	}

	return &chats, nil
}

// GetDirectChatsForUser возвращает личные чаты пользователя
func (r *chatRepository) GetDirectChatsForUser(ctx context.Context, userID uint) ([]model.Chat, error) {
	if userID == 0 {
		return nil, errors.New("userID cannot be zero")
	}

	var chats []model.Chat
	err := r.db.WithContext(ctx).Raw(`
		SELECT c.*
		FROM chats c
		INNER JOIN chat_users cu ON c.id = cu.chat_id
		WHERE c.is_group = false
		  AND cu.user_id = ?
		ORDER BY c.updated_at DESC
	`, userID).Scan(&chats).Error

	return chats, err
}

// GetForUsers возвращает чат между двумя пользователями
func (r *chatRepository) GetForUsers(ctx context.Context, user1ID, user2ID uint) (*model.Chat, error) {
	if user1ID == 0 || user2ID == 0 {
		return nil, errors.New("userIDs cannot be zero")
	}

	// Находим чат, где оба пользователя являются участниками
	var chat model.Chat
	err := r.db.WithContext(ctx).Raw(`
		SELECT c.*
		FROM chats c
		WHERE c.id IN (
			SELECT cu1.chat_id
			FROM chat_users cu1
			INNER JOIN chat_users cu2 ON cu1.chat_id = cu2.chat_id
			WHERE cu1.user_id = ? AND cu2.user_id = ?
			GROUP BY cu1.chat_id
			HAVING COUNT(DISTINCT cu1.user_id) = 2
		)
		AND c.is_group = false
		LIMIT 1
	`, user1ID, user2ID).Scan(&chat).Error

	if err != nil {
		return nil, err
	}

	if chat.ID == 0 {
		return nil, nil
	}

	return &chat, nil
}

// UpdateGroupInfo обновляет информацию о групповом чате
func (r *chatRepository) UpdateGroupInfo(ctx context.Context, chatID uint, name, description string) error {
	if chatID == 0 {
		return errors.New("chatID cannot be zero")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("group name cannot be empty")
	}

	return r.db.WithContext(ctx).Exec(`
		UPDATE chats
		SET name = ?, description = ?, updated_at = NOW()
		WHERE id = ? AND is_group = true
	`, name, description, chatID).Error
}

// GetGroupChatsForUser возвращает групповые чаты пользователя
func (r *chatRepository) GetGroupChatsForUser(ctx context.Context, userID uint) ([]model.Chat, error) {
	if userID == 0 {
		return nil, errors.New("userID cannot be zero")
	}

	var chats []model.Chat
	err := r.db.WithContext(ctx).Raw(`
		SELECT c.*
		FROM chats c
		INNER JOIN chat_users cu ON c.id = cu.chat_id
		WHERE c.is_group = true
		  AND cu.user_id = ?
		ORDER BY c.updated_at DESC
	`, userID).Scan(&chats).Error

	return chats, err
}

// GetChatStatistics возвращает статистику чата
func (r *chatRepository) GetChatStatistics(ctx context.Context, chatID uint) (*ChatStats, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	var stats ChatStats

	// Количество сообщений
	err := r.db.WithContext(ctx).Model(&model.Message{}).
		Where("chat_id = ?", chatID).
		Count(&stats.TotalMessages).Error
	if err != nil {
		return nil, err
	}

	// Количество активных пользователей (отправлявших сообщения за последние 30 дней)
	err = r.db.WithContext(ctx).Raw(`
		SELECT COUNT(DISTINCT sender_id)
		FROM messages
		WHERE chat_id = ? 
		  AND created_at >= NOW() - INTERVAL '30 days'
	`, chatID).Scan(&stats.ActiveUsers).Error
	if err != nil {
		return nil, err
	}

	// Время последнего сообщения
	err = r.db.WithContext(ctx).Raw(`
		SELECT MAX(created_at)
		FROM messages
		WHERE chat_id = ?
	`, chatID).Scan(&stats.LastMessageAt).Error
	if err != nil {
		return nil, err
	}

	// Время первого сообщения
	err = r.db.WithContext(ctx).Raw(`
		SELECT MIN(created_at)
		FROM messages
		WHERE chat_id = ?
	`, chatID).Scan(&stats.FirstMessageAt).Error
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// SearchMessages ищет сообщения в чате
func (r *chatRepository) SearchMessages(ctx context.Context, chatID uint, query string, limit int) ([]model.Message, error) {
	if chatID == 0 {
		return nil, errors.New("chatID cannot be zero")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var messages []model.Message
	err := r.db.WithContext(ctx).
		Where("chat_id = ? AND message ILIKE ?", chatID, "%"+query+"%").
		Order("created_at DESC").
		Limit(limit).
		Preload("Sender").
		Find(&messages).Error

	return messages, err
}

// GetUnreadCount возвращает количество непрочитанных сообщений
func (r *chatRepository) GetUnreadCount(ctx context.Context, userID, chatID uint) (int64, error) {
	if userID == 0 {
		return 0, errors.New("userID cannot be zero")
	}

	var count int64

	if chatID == 0 {
		// Все непрочитанные сообщения пользователя
		err := r.db.WithContext(ctx).Raw(`
			SELECT COUNT(*)
			FROM messages m
			INNER JOIN chat_users cu ON m.chat_id = cu.chat_id
			LEFT JOIN message_reads mr ON m.id = mr.message_id AND mr.user_id = ?
			WHERE cu.user_id = ?
			  AND mr.message_id IS NULL
			  AND m.sender_id != ?
		`, userID, userID, userID).Scan(&count).Error
		return count, err
	}

	// Непрочитанные сообщения в конкретном чате
	err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(*)
		FROM messages m
		LEFT JOIN message_reads mr ON m.id = mr.message_id AND mr.user_id = ?
		WHERE m.chat_id = ?
		  AND mr.message_id IS NULL
		  AND m.sender_id != ?
	`, userID, chatID, userID).Scan(&count).Error

	return count, err
}

// Legacy методы для обратной совместимости
func (r *chatRepository) CreateLegacy(chat *model.Chat) error {
	return r.Create(context.Background(), chat)
}

func (r *chatRepository) GetForUsersLegacy(user1ID, user2ID uint) (*model.Chat, error) {
	return r.GetForUsers(context.Background(), user1ID, user2ID)
}

func (r *chatRepository) AddUserLegacy(chatID, userID uint) error {
	return r.AddUser(context.Background(), chatID, userID)
}

func (r *chatRepository) SendMessageLegacy(chat *model.Chat, message model.Message) error {
	return r.SendMessage(context.Background(), chat, message)
}

func (r *chatRepository) GetMessagesLegacy(chatID uint) ([]model.Message, error) {
	return r.GetMessages(context.Background(), chatID)
}

func (r *chatRepository) GetChatMessagesLegacy(chatID uint, cursor string, limit int, direction string, ctx context.Context) (
	[]model.Message, bool, bool, *int64, error) {
	return r.GetChatMessages(ctx, chatID, cursor, limit, direction)
}

func (r *chatRepository) CreateGroupLegacy(chat *model.Chat, userIDs []uint) error {
	return r.CreateGroup(context.Background(), chat, userIDs)
}

func (r *chatRepository) IsUserInChatLegacy(chatID, userID uint) (bool, error) {
	return r.IsUserInChat(context.Background(), chatID, userID)
}

func (r *chatRepository) GetChatsForUserLegacy(userID uint) (*[]model.Chat, error) {
	return r.GetChatsForUser(context.Background(), userID)
}

func (r *chatRepository) GetChatUsersLegacy(chatID uint) ([]model.User, error) {
	return r.GetChatUsers(context.Background(), chatID)
}
