package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"tush00nka/bbbab_messenger/internal/model"
	"tush00nka/bbbab_messenger/internal/pkg/auth"
	"tush00nka/bbbab_messenger/internal/pkg/httputils"
	"tush00nka/bbbab_messenger/internal/service"
	"tush00nka/bbbab_messenger/internal/ws"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Constants
const (
	MaxMessageLength    = 5000
	DefaultMessageLimit = 20
	MaxMessageLimit     = 100
	ConnectionTimeout   = 10 * time.Second
	PresenceTimeout     = 3 * time.Second
)

// Request/Response structs

// SendMessageRequest запрос на отправку сообщения
type SendMessageRequest struct {
	ReceiverID uint   `json:"receiver_id" binding:"required"`
	ChatID     uint   `json:"chat_id"`
	Message    string `json:"message" binding:"required,min=1,max=5000"`
	Type       string `json:"type" binding:"oneof=text image file" default:"text"`
}

// CreateChatRequest запрос на создание чата
type CreateChatRequest struct {
	Name    string `json:"name" binding:"max=100"`
	UserIDs []uint `json:"user_ids" binding:"required,min=1"`
}

// CreateGroupRequest запрос на создание группового чата
type CreateGroupRequest struct {
	Name    string `json:"name" binding:"required,min=1,max=100"`
	UserIDs []uint `json:"user_ids" binding:"required,min=1"`
}

// GetChatMessagesRequest запрос на получение сообщений с пагинацией
type GetChatMessagesRequest struct {
	Cursor    string `json:"cursor" form:"cursor" query:"cursor"`
	Limit     int    `json:"limit" form:"limit" query:"limit" binding:"min=1,max=100" default:"20"`
	Direction string `json:"direction" form:"direction" query:"direction" binding:"oneof=older newer" default:"older"`
}

// PaginationInfo информация о пагинации
type PaginationInfo struct {
	NextCursor     *string `json:"nextCursor,omitempty"`
	PreviousCursor *string `json:"previousCursor,omitempty"`
	HasNext        bool    `json:"hasNext"`
	HasPrevious    bool    `json:"hasPrevious"`
	Limit          int     `json:"limit"`
	TotalCount     *int64  `json:"totalCount,omitempty"`
}

// GetChatMessagesResponse ответ с сообщениями и пагинацией
type GetChatMessagesResponse struct {
	Data       []model.Message `json:"data"`
	Pagination PaginationInfo  `json:"pagination"`
}

// ListChatsResponse ответ со списком чатов
type ListChatsResponse struct {
	ID          uint           `json:"id"`
	Name        string         `json:"name"`
	LastMessage *model.Message `json:"lastMessage,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// StatusResponse ответ со статусом
type StatusResponse struct {
	Status string `json:"status"`
}

// ChatHandler обработчик чатов
type ChatHandler struct {
	chatService      service.ChatService
	chatCacheService *service.ChatCacheService
	s3Service        *service.S3Service
	hub              *ws.Hub
	wsUpgrader       *websocket.Upgrader
	logger           Logger
}

type Logger interface {
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	Debug(msg string, fields ...any)
}

// NewChatHandler создает новый экземпляр ChatHandler
func NewChatHandler(
	chatService service.ChatService,
	chatCacheService *service.ChatCacheService,
	s3Service *service.S3Service,
	hub *ws.Hub,
	wsUpgrader *websocket.Upgrader,
	logger Logger,
) *ChatHandler {
	if logger == nil {
		logger = &defaultLogger{}
	}

	return &ChatHandler{
		chatService:      chatService,
		chatCacheService: chatCacheService,
		s3Service:        s3Service,
		hub:              hub,
		wsUpgrader:       wsUpgrader,
		logger:           logger,
	}
}

// RegisterRoutes регистрирует маршруты
func (h *ChatHandler) RegisterRoutes(router *mux.Router) {
	authMiddleware := h.authMiddleware

	router.HandleFunc("/sendmessage", authMiddleware(h.sendMessage)).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/{id:[0-9]+}", authMiddleware(h.getChatInfo)).Methods("GET", "OPTIONS")
	router.HandleFunc("/chat/create", authMiddleware(h.createChat)).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/list", authMiddleware(h.listChats)).Methods("GET", "OPTIONS")
	router.HandleFunc("/chat/{id:[0-9]+}/ws", h.wsChat).Methods("GET", "OPTIONS")
	router.HandleFunc("/chat/{id:[0-9]+}/messages", authMiddleware(h.getMessages)).Methods("GET", "OPTIONS")
	router.HandleFunc("/chat/join/{chat_id:[0-9]+}/{user_id:[0-9]+}", authMiddleware(h.UserJoined)).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/leave/{chat_id:[0-9]+}/{user_id:[0-9]+}", authMiddleware(h.UserLeft)).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/group/create", authMiddleware(h.CreateGroup)).Methods("POST", "OPTIONS")
}

// authMiddleware middleware для аутентификации
func (h *ChatHandler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractTokenFromHeader(r)
		if tokenStr == "" {
			httputils.ResponseError(w, http.StatusUnauthorized, "missing auth token")
			return
		}

		claims, err := auth.ValidateToken(tokenStr)
		if err != nil {
			httputils.ResponseError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		// Добавляем claims в контекст
		ctx := context.WithValue(r.Context(), "userClaims", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// extractTokenFromHeader извлекает токен из заголовка
func extractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return parts[1]
	}

	return authHeader
}

// getClaimsFromContext получает claims из контекста
func getClaimsFromContext(r *http.Request) (*auth.Claims, error) {
	claims, ok := r.Context().Value("userClaims").(*auth.Claims)
	if !ok || claims == nil {
		return nil, fmt.Errorf("no claims in context")
	}
	return claims, nil
}

// SendMessage отправляет сообщение
// @Summary Send message
// @Description Send message to user or existing chat
// @ID send-message
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Param messageData body SendMessageRequest true "Message data"
// @Success 201 {object} model.Message
// @Failure 400 {object} httputils.ErrorResponse
// @Failure 401 {object} httputils.ErrorResponse
// @Failure 403 {object} httputils.ErrorResponse
// @Failure 500 {object} httputils.ErrorResponse
// @Router /sendmessage [post]
func (h *ChatHandler) sendMessage(w http.ResponseWriter, r *http.Request) {
	claims, err := getClaimsFromContext(r)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid request format")
		return
	}
	defer r.Body.Close()

	// Валидация
	if req.ReceiverID == 0 {
		httputils.ResponseError(w, http.StatusBadRequest, "receiver_id is required")
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" || len(req.Message) > MaxMessageLength {
		httputils.ResponseError(w, http.StatusBadRequest,
			fmt.Sprintf("message must be 1-%d characters", MaxMessageLength))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var chat *model.Chat

	// Если указан chat_id, используем существующий чат
	if req.ChatID > 0 {
		chat, err = h.chatService.GetChatByID(ctx, req.ChatID)
		if err != nil {
			httputils.ResponseError(w, http.StatusNotFound, "chat not found")
			return
		}

		// Проверяем, что пользователь является участником чата
		isMember, err := h.chatService.IsUserInChat(ctx, chat.ID, claims.UserID)
		if err != nil || !isMember {
			httputils.ResponseError(w, http.StatusForbidden, "user is not a member of this chat")
			return
		}
	} else {
		// Ищем или создаем личный чат
		chat, err = h.findOrCreateDirectChat(ctx, claims.UserID, req.ReceiverID)
		if err != nil {
			h.logger.Error("failed to find or create chat", "error", err)
			httputils.ResponseError(w, http.StatusInternalServerError, "failed to create chat")
			return
		}
	}

	// Создаем сообщение с Timestamp
	msg := model.Message{
		ChatID:    chat.ID,
		SenderID:  claims.UserID,
		Message:   html.EscapeString(req.Message),
		Type:      req.Type,
		Timestamp: time.Now(),
	}

	// Сохраняем сообщение
	if err := h.processMessage(ctx, chat, msg); err != nil {
		h.logger.Error("failed to process message", "error", err)
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to send message")
		return
	}

	// Возвращаем сообщение с ID и Timestamp
	httputils.ResponseJSON(w, http.StatusCreated, msg)
}

// findOrCreateDirectChat ищет или создает личный чат
func (h *ChatHandler) findOrCreateDirectChat(ctx context.Context, userID1, userID2 uint) (*model.Chat, error) {
	// Ищем существующий личный чат
	chats, err := h.chatService.GetDirectChatsForUser(ctx, userID1)
	if err != nil {
		return nil, err
	}

	for _, chat := range chats {
		users, err := h.chatService.GetChatUsers(ctx, chat.ID)
		if err != nil {
			continue
		}

		// Проверяем, есть ли второй пользователь в чате
		for _, user := range users {
			if user.ID == userID2 {
				return &chat, nil
			}
		}
	}

	// Создаем новый чат
	chat := &model.Chat{}
	if err := h.chatService.CreateChat(ctx, chat); err != nil {
		return nil, err
	}

	// Добавляем пользователей
	if err := h.chatService.AddUsersToChat(ctx, chat.ID, userID1, userID2); err != nil {
		// Откатываем создание чата при ошибке
		h.chatService.DeleteChat(ctx, chat.ID)
		return nil, err
	}

	return chat, nil
}

// processMessage обрабатывает отправку сообщения
func (h *ChatHandler) processMessage(ctx context.Context, chat *model.Chat, msg model.Message) error {
	// Сохраняем в БД
	if err := h.chatService.SendMessageToChat(ctx, chat, msg); err != nil {
		return err
	}

	// Кешируем в Redis (асинхронно)
	if h.chatCacheService != nil {
		go func() {
			ctxCache, cancel := context.WithTimeout(context.Background(), PresenceTimeout)
			defer cancel()

			if err := h.chatCacheService.SendMessage(ctxCache, chat, msg); err != nil {
				h.logger.Warn("failed to cache message", "error", err)
			}
		}()
	}

	// Отправляем через WebSocket
	if h.hub != nil {
		h.hub.BroadcastMessage(chat.ID, msg)
	}

	return nil
}

// GetMessages возвращает сообщения чата
// @Summary Get chat messages
// @Description Get messages from chat with pagination
// @ID get-messages
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Param id path int true "Chat ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Limit" minimum(1) maximum(100) default(20)
// @Param direction query string false "Direction" Enums(older, newer) default(older)
// @Success 200 {object} GetChatMessagesResponse
// @Failure 400 {object} httputils.ErrorResponse
// @Failure 403 {object} httputils.ErrorResponse
// @Failure 500 {object} httputils.ErrorResponse
// @Router /chat/{id}/messages [get]
func (h *ChatHandler) getMessages(w http.ResponseWriter, r *http.Request) {
	claims, err := getClaimsFromContext(r)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Получаем chat ID
	vars := mux.Vars(r)
	chatID, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	// Проверяем членство в чате
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	isMember, err := h.chatService.IsUserInChat(ctx, uint(chatID), claims.UserID)
	if err != nil {
		h.logger.Error("failed to check membership", "error", err)
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to validate membership")
		return
	}
	if !isMember {
		httputils.ResponseError(w, http.StatusForbidden, "user is not a member of this chat")
		return
	}

	// Парсим параметры запроса
	queryParams := r.URL.Query()
	cursor := queryParams.Get("cursor")

	limit := DefaultMessageLimit
	if limitStr := queryParams.Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			if parsedLimit >= 1 && parsedLimit <= MaxMessageLimit {
				limit = parsedLimit
			}
		}
	}

	direction := queryParams.Get("direction")
	if direction != "older" && direction != "newer" {
		direction = "older"
	}

	// Получаем сообщения
	messages, hasNext, hasPrevious, totalCount, err := h.chatService.GetChatMessages(
		ctx, uint(chatID), cursor, limit, direction)
	if err != nil {
		h.logger.Error("failed to get messages", "error", err)
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}

	// Формируем курсоры на основе Timestamp
	var nextCursor, previousCursor *string
	if hasNext && len(messages) > 0 {
		lastMsgTime := messages[len(messages)-1].Timestamp.Format(time.RFC3339)
		nextCursor = &lastMsgTime
	}

	if hasPrevious && len(messages) > 0 {
		firstMsgTime := messages[0].Timestamp.Format(time.RFC3339)
		previousCursor = &firstMsgTime
	}

	// Формируем ответ
	response := GetChatMessagesResponse{
		Data: messages,
		Pagination: PaginationInfo{
			NextCursor:     nextCursor,
			PreviousCursor: previousCursor,
			HasNext:        hasNext,
			HasPrevious:    hasPrevious,
			Limit:          limit,
			TotalCount:     totalCount,
		},
	}

	httputils.ResponseJSON(w, http.StatusOK, response)
}

// CreateChat создает чат
// @Summary Create chat
// @Description Create a new chat (personal or group)
// @ID create-chat
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Param chatData body CreateChatRequest true "Chat data"
// @Success 201 {object} model.Chat
// @Failure 400 {object} httputils.ErrorResponse
// @Failure 500 {object} httputils.ErrorResponse
// @Router /chat/create [post]
func (h *ChatHandler) createChat(w http.ResponseWriter, r *http.Request) {
	claims, err := getClaimsFromContext(r)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid request format")
		return
	}
	defer r.Body.Close()

	// Валидация
	if len(req.UserIDs) == 0 {
		httputils.ResponseError(w, http.StatusBadRequest, "at least one user is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Создаем чат
	chat := &model.Chat{Name: strings.TrimSpace(req.Name)}
	if err := h.chatService.CreateChat(ctx, chat); err != nil {
		h.logger.Error("failed to create chat", "error", err)
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to create chat")
		return
	}

	// Добавляем текущего пользователя, если его нет в списке
	userIDs := req.UserIDs
	hasCurrentUser := false
	for _, id := range userIDs {
		if id == claims.UserID {
			hasCurrentUser = true
			break
		}
	}
	if !hasCurrentUser {
		userIDs = append(userIDs, claims.UserID)
	}

	// Добавляем пользователей
	if err := h.chatService.AddUsersToChat(ctx, chat.ID, userIDs...); err != nil {
		// Откатываем создание чата
		h.chatService.DeleteChat(ctx, chat.ID)
		h.logger.Error("failed to add users to chat", "error", err)
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to add users to chat")
		return
	}

	httputils.ResponseJSON(w, http.StatusCreated, chat)
}

// UserJoined отмечает пользователя как подключенного
// @Summary User joined chat
// @Description Notify that user joined chat (for presence tracking)
// @ID user-joined
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Param chat_id path int true "Chat ID"
// @Param user_id path int true "User ID"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} httputils.ErrorResponse
// @Failure 403 {object} httputils.ErrorResponse
// @Failure 500 {object} httputils.ErrorResponse
// @Router /chat/join/{chat_id}/{user_id} [post]
func (h *ChatHandler) UserJoined(w http.ResponseWriter, r *http.Request) {
	claims, err := getClaimsFromContext(r)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	vars := mux.Vars(r)
	chatID, err1 := strconv.ParseUint(vars["chat_id"], 10, 64)
	userID, err2 := strconv.ParseUint(vars["user_id"], 10, 64)
	if err1 != nil || err2 != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid chat or user id")
		return
	}

	// Проверяем, что пользователь может отмечать только себя
	if claims.UserID != uint(userID) {
		httputils.ResponseError(w, http.StatusForbidden, "cannot update presence for other users")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), PresenceTimeout)
	defer cancel()

	if err := h.chatCacheService.UserJoined(ctx, uint(chatID), uint(userID)); err != nil {
		h.logger.Error("failed to update user presence", "error", err)
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to join chat")
		return
	}

	httputils.ResponseJSON(w, http.StatusOK, StatusResponse{Status: "user joined"})
}

// UserLeft отмечает пользователя как отключившегося
// @Summary User left chat
// @Description Notify that user left chat (for presence tracking)
// @ID user-left
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Param chat_id path int true "Chat ID"
// @Param user_id path int true "User ID"
// @Success 200 {object} StatusResponse
// @Failure 400 {object} httputils.ErrorResponse
// @Failure 403 {object} httputils.ErrorResponse
// @Failure 500 {object} httputils.ErrorResponse
// @Router /chat/leave/{chat_id}/{user_id} [post]
func (h *ChatHandler) UserLeft(w http.ResponseWriter, r *http.Request) {
	claims, err := getClaimsFromContext(r)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	vars := mux.Vars(r)
	chatID, err1 := strconv.ParseUint(vars["chat_id"], 10, 64)
	userID, err2 := strconv.ParseUint(vars["user_id"], 10, 64)
	if err1 != nil || err2 != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid chat or user id")
		return
	}

	// Проверяем, что пользователь может отмечать только себя
	if claims.UserID != uint(userID) {
		httputils.ResponseError(w, http.StatusForbidden, "cannot update presence for other users")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), PresenceTimeout)
	defer cancel()

	if err := h.chatCacheService.UserLeft(ctx, uint(chatID), uint(userID)); err != nil {
		h.logger.Error("failed to update user presence", "error", err)
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to leave chat")
		return
	}

	httputils.ResponseJSON(w, http.StatusOK, StatusResponse{Status: "user left"})
}

// CreateGroup создает групповой чат
// @Summary Create group chat
// @Description Create a new group chat
// @ID create-group-chat
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Param groupData body CreateGroupRequest true "Group data"
// @Success 201 {object} model.Chat
// @Failure 400 {object} httputils.ErrorResponse
// @Failure 500 {object} httputils.ErrorResponse
// @Router /chat/group/create [post]
func (h *ChatHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	claims, err := getClaimsFromContext(r)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	defer r.Body.Close()

	// Валидация
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httputils.ResponseError(w, http.StatusBadRequest, "group name is required")
		return
	}

	if len(req.UserIDs) == 0 {
		httputils.ResponseError(w, http.StatusBadRequest, "at least one user is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Добавляем текущего пользователя
	req.UserIDs = append(req.UserIDs, claims.UserID)

	// Создаем групповой чат
	chat, err := h.chatService.CreateGroupChat(ctx, req.Name, req.UserIDs)
	if err != nil {
		h.logger.Error("failed to create group chat", "error", err)
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to create group chat")
		return
	}

	httputils.ResponseJSON(w, http.StatusCreated, chat)
}

// WSChat устанавливает WebSocket соединение
func (h *ChatHandler) wsChat(w http.ResponseWriter, r *http.Request) {
	// Аутентификация
	tokenStr := extractTokenFromHeader(r)
	if tokenStr == "" {
		httputils.ResponseError(w, http.StatusUnauthorized, "missing auth token")
		return
	}

	claims, err := auth.ValidateToken(tokenStr)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	// Получение chat ID
	vars := mux.Vars(r)
	chatID64, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil || chatID64 == 0 {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid chat id")
		return
	}
	chatID := uint(chatID64)

	// Проверка членства в чате с таймаутом
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	ok, err := h.chatService.IsUserInChat(ctx, chatID, claims.UserID)
	if err != nil {
		h.logger.Error("failed to validate membership", "error", err)
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to validate membership")
		return
	}
	if !ok {
		httputils.ResponseError(w, http.StatusForbidden, "user is not a member of this chat")
		return
	}

	// Upgrade WebSocket соединения
	conn, err := ws.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("websocket upgrade failed", "error", err)
		return
	}

	// Создаем контекст для клиента
	clientCtx, clientCancel := context.WithCancel(r.Context())

	// Создаем и регистрируем клиента
	client := ws.NewClient(clientCtx, conn, claims.UserID, chatID)
	client.SetRateLimit(10) // 10 сообщений в секунду

	room := h.hub.GetRoom(chatID)
	if !room.RegisterClient(client) {
		clientCancel()
		conn.Close()
		httputils.ResponseError(w, http.StatusTooManyRequests, "too many connections for this chat")
		return
	}

	// Гарантированная очистка ресурсов
	defer func() {
		clientCancel()
		room.UnregisterClient(client)
		conn.Close()

		// Асинхронное обновление статуса
		go func() {
			ctxCleanup, cancelCleanup := context.WithTimeout(context.Background(), PresenceTimeout)
			defer cancelCleanup()
			_ = h.chatCacheService.UserLeft(ctxCleanup, chatID, claims.UserID)
		}()
	}()

	// Обновляем статус присутствия
	ctxPresence, cancelPresence := context.WithTimeout(clientCtx, PresenceTimeout)
	defer cancelPresence()

	if err := h.chatCacheService.UserJoined(ctxPresence, chatID, claims.UserID); err != nil {
		h.logger.Warn("failed to update user presence", "error", err)
	}

	// Асинхронно отправляем историю чата
	go h.sendChatHistory(client, chatID)

	// Запускаем обработку сообщений
	errChan := make(chan error, 2)
	var wg sync.WaitGroup

	// Write pump
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := client.WritePump(); err != nil {
			select {
			case errChan <- fmt.Errorf("write pump error: %w", err):
			default:
			}
		}
	}()

	// Read pump
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Просто запускаем ReadPump, ошибки обрабатываются внутри
		client.ReadPump(h.handleIncomingMessage)
	}()

	// Ждем завершения обеих горутин
	go func() {
		wg.Wait()
		// Закрываем канал ошибок после завершения всех горутин
		close(errChan)
	}()

	// Ожидаем завершения
	select {
	case <-clientCtx.Done():
		h.logger.Debug("websocket connection closed by context", "user_id", claims.UserID)
	case err, ok := <-errChan:
		if ok && err != nil {
			h.logger.Debug("websocket connection error", "error", err, "user_id", claims.UserID)
		} else if !ok {
			h.logger.Debug("websocket connection closed", "user_id", claims.UserID)
		}
	}
}

// sendChatHistory отправляет историю сообщений
func (h *ChatHandler) sendChatHistory(client *ws.Client, chatID uint) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Пробуем получить из кеша
	messages, err := h.chatCacheService.GetMessages(ctx, chatID)
	if err != nil {
		h.logger.Warn("failed to get messages from cache", "error", err)
		messages = nil
	}

	// Если в кеше пусто, пробуем БД
	if len(messages) == 0 {
		dbMessages, err := h.chatService.GetRecentMessages(ctx, chatID, 50)
		if err == nil && len(dbMessages) > 0 {
			messages = dbMessages

			// Асинхронно кешируем
			if h.chatCacheService != nil {
				go func() {
					ctxCache, cancelCache := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancelCache()
					if err := h.chatCacheService.CacheMessages(ctxCache, chatID, messages); err != nil {
						h.logger.Warn("failed to cache messages", "error", err)
					}
				}()
			}
		}
	}

	if len(messages) > 0 {
		client.SendJSON(ws.OutEvent{
			Type:     "history",
			Messages: messages,
			Meta: map[string]any{
				"count":    len(messages),
				"has_more": len(messages) == 50,
			},
		})
	}
}

// handleIncomingMessage обрабатывает входящие сообщения WebSocket
func (h *ChatHandler) handleIncomingMessage(c *ws.Client, ev ws.InEvent) {
	ev.Type = strings.ToLower(strings.TrimSpace(ev.Type))
	if ev.Type == "" {
		c.SendJSON(ws.OutEvent{Type: "error", Message: "empty event type"})
		return
	}

	switch ev.Type {
	case "message":
		h.handleChatMessage(c, ev)
	case "typing":
		h.handleTypingIndicator(c, ev)
	case "read_receipt":
		h.handleReadReceipt(c, ev)
	default:
		c.SendJSON(ws.OutEvent{
			Type:    "error",
			Message: fmt.Sprintf("unknown event type: %s", ev.Type),
		})
	}
}

// handleChatMessage обрабатывает текстовые сообщения
func (h *ChatHandler) handleChatMessage(c *ws.Client, ev ws.InEvent) {
	// Валидация
	txt := strings.TrimSpace(ev.Message)

	if len(txt) == 0 {
		c.SendJSON(ws.OutEvent{Type: "error", Message: "message cannot be empty"})
		return
	}

	if len(txt) > MaxMessageLength {
		c.SendJSON(ws.OutEvent{
			Type:    "error",
			Message: fmt.Sprintf("message too long (max %d characters)", MaxMessageLength),
		})
		return
	}

	// Проверка rate limit
	if !c.CheckRateLimit() {
		c.SendJSON(ws.OutEvent{
			Type:    "error",
			Message: "rate limit exceeded. please wait before sending more messages",
		})
		return
	}

	// Экранирование HTML
	txt = html.EscapeString(txt)

	// Создание сообщения с Timestamp
	msg := model.Message{
		ChatID:    c.ChatID,
		SenderID:  c.UserID,
		Message:   txt,
		Timestamp: time.Now(),
	}

	// Асинхронная обработка
	go h.processWebSocketMessage(c, msg)
}

// processWebSocketMessage обрабатывает сообщение из WebSocket
func (h *ChatHandler) processWebSocketMessage(c *ws.Client, msg model.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Сохраняем в БД
	chat := &model.Chat{}
	chat.ID = msg.ChatID

	if err := h.chatService.SendMessageToChat(ctx, chat, msg); err != nil {
		h.logger.Error("failed to save message", "error", err)

		select {
		case <-ctx.Done():
		default:
			c.SendJSON(ws.OutEvent{
				Type:    "error",
				Message: "failed to save message",
			})
		}
		return
	}

	// Кешируем
	if h.chatCacheService != nil {
		go func() {
			ctxCache, cancelCache := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancelCache()

			chat := &model.Chat{}
			chat.ID = msg.ChatID

			if err := h.chatCacheService.SendMessage(ctxCache, chat, msg); err != nil {
				h.logger.Warn("failed to cache message", "error", err)
			}
		}()
	}

	// Рассылаем
	if h.hub != nil {
		h.hub.BroadcastMessage(msg.ChatID, msg)
	}

	// Подтверждение с ID и Timestamp
	select {
	case <-ctx.Done():
	default:
		c.SendJSON(ws.OutEvent{
			Type:      "message_sent",
			MessageID: msg.ID,
			Timestamp: msg.Timestamp,
		})
	}
}

// handleTypingIndicator обрабатывает индикатор набора текста
func (h *ChatHandler) handleTypingIndicator(c *ws.Client, ev ws.InEvent) {
	isTyping := strings.ToLower(strings.TrimSpace(ev.Message)) == "true"
	if h.hub != nil {
		h.hub.BroadcastTypingIndicator(c.ChatID, c.UserID, isTyping)
	}
}

// handleReadReceipt обрабатывает подтверждение прочтения
func (h *ChatHandler) handleReadReceipt(c *ws.Client, ev ws.InEvent) {
	messageID, err := strconv.ParseUint(strings.TrimSpace(ev.Message), 10, 64)
	if err != nil {
		c.SendJSON(ws.OutEvent{Type: "error", Message: "invalid message id"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := h.chatService.MarkMessageAsRead(ctx, uint(messageID), c.UserID); err != nil {
		h.logger.Warn("failed to mark message as read", "error", err)
	}
}

// GetChatInfo возвращает информацию о чате
// @Summary Get chat info
// @Description Get chat information by ID
// @ID get-chat-info
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Param id path int true "Chat ID"
// @Success 200 {object} model.Chat
// @Failure 400 {object} httputils.ErrorResponse
// @Failure 403 {object} httputils.ErrorResponse
// @Failure 404 {object} httputils.ErrorResponse
// @Router /chat/{id} [get]
func (h *ChatHandler) getChatInfo(w http.ResponseWriter, r *http.Request) {
	claims, err := getClaimsFromContext(r)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	vars := mux.Vars(r)
	chatID, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// Проверяем членство
	isMember, err := h.chatService.IsUserInChat(ctx, uint(chatID), claims.UserID)
	if err != nil || !isMember {
		httputils.ResponseError(w, http.StatusForbidden, "user is not a member of this chat")
		return
	}

	// Получаем информацию о чате
	chat, err := h.chatService.GetChatByID(ctx, uint(chatID))
	if err != nil {
		httputils.ResponseError(w, http.StatusNotFound, "chat not found")
		return
	}

	httputils.ResponseJSON(w, http.StatusOK, chat)
}

// ListChats возвращает список чатов пользователя
// @Summary List user chats
// @Description Get list of chats for current user
// @ID list-chats
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Success 200 {object} []ListChatsResponse
// @Failure 500 {object} httputils.ErrorResponse
// @Router /chat/list [get]
func (h *ChatHandler) listChats(w http.ResponseWriter, r *http.Request) {
	claims, err := getClaimsFromContext(r)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	chats, err := h.chatService.GetChatsForUser(ctx, claims.UserID)
	if err != nil {
		h.logger.Error("failed to get chat list", "error", err)
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to get chat list")
		return
	}

	var responses []ListChatsResponse
	for _, chat := range *chats {
		response := ListChatsResponse{
			ID:        chat.ID,
			Name:      chat.Name,
			CreatedAt: chat.CreatedAt,
			UpdatedAt: chat.UpdatedAt,
		}

		// Добавляем последнее сообщение, если есть
		if len(chat.Messages) > 0 {
			response.LastMessage = &chat.Messages[len(chat.Messages)-1]
		}

		responses = append(responses, response)
	}

	httputils.ResponseJSON(w, http.StatusOK, responses)
}

// defaultLogger простой логгер по умолчанию
type defaultLogger struct{}

func (l *defaultLogger) Info(msg string, fields ...any) { log.Printf("INFO: "+msg, fields...) }
func (l *defaultLogger) Warn(msg string, fields ...any) { log.Printf("WARN: "+msg, fields...) }
func (l *defaultLogger) Error(msg string, fields ...any) {
	log.Printf("ERROR: "+msg, fields...)
}
func (l *defaultLogger) Debug(msg string, fields ...any) {
	log.Printf("DEBUG: "+msg, fields...)
}
