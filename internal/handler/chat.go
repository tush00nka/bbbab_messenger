package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"tush00nka/bbbab_messenger/internal/model"
	"tush00nka/bbbab_messenger/internal/pkg/auth"
	"tush00nka/bbbab_messenger/internal/pkg/httputils"
	"tush00nka/bbbab_messenger/internal/service"
	"tush00nka/bbbab_messenger/internal/ws"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type createGroupRequest struct {
	Name    string `json:"name"`
	UserIDs []uint `json:"user_ids"`
}

type ChatHandler struct {
	chatService      service.ChatService
	chatCacheService *service.ChatCacheService
	s3Service        service.S3Service
	hub              *ws.Hub
}

func NewChatHandler(
	chatService service.ChatService,
	chatCacheService *service.ChatCacheService,
	s3Service service.S3Service,
	hub *ws.Hub,
) *ChatHandler {
	return &ChatHandler{
		chatService:      chatService,
		chatCacheService: chatCacheService,
		s3Service:        s3Service,
		hub:              hub,
	}
}

func (h *ChatHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/sendmessage", h.sendMessage).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/{id}", h.getMessages).Methods("GET", "OPTIONS")
	router.HandleFunc("/chat/create", h.createChat).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/list", h.listChats).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/{id}/ws", h.wsChat).Methods("GET", "OPTIONS")
	router.HandleFunc("/chat/{id}/messages", h.getMessages).Methods("GET", "OPTIONS")
	router.HandleFunc("/chat/join/{chat_id:[0-9]+}/{user_id:[0-9]+}", h.UserJoined).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/leave/{chat_id:[0-9]+}/{user_id:[0-9]+}", h.UserLeft).Methods("POST", "OPTIONS")
}

// helper: извлечь токен из заголовка Authorization: Bearer <token> или из заголовка Bearer
func extractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		authHeader = r.Header.Get("Bearer")
	}
	authHeader = strings.TrimSpace(authHeader)
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return authHeader
}

type sendMessageRequest struct {
	ReceiverID uint   `json:"receiver_id"`
	Message    string `json:"message"`
}

// @Summary Send message to user
// @Description Send messages between two users only (for now)
// @ID send-message
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Param msgData body sendMessageRequest true "Message Data"
// @Success 200
// @Failure 400 {object} response.ErrorResponse
// @Router /sendmessage [post]
func (h *ChatHandler) sendMessage(w http.ResponseWriter, r *http.Request) {
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

	var request sendMessageRequest
	if err = json.NewDecoder(r.Body).Decode(&request); err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid request format")
		return
	}
	defer r.Body.Close()

	if request.ReceiverID == 0 || strings.TrimSpace(request.Message) == "" {
		httputils.ResponseError(w, http.StatusBadRequest, "receiver_id and message are required")
		return
	}

	chat, err := h.chatService.GetChatForUsers(claims.UserID, request.ReceiverID)
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to get chat for users")
		return
	}

	if chat == nil {
		chat = &model.Chat{}
		if err = h.chatService.CreateChat(chat); err != nil {
			httputils.ResponseError(w, http.StatusInternalServerError, "failed to create chat")
			return
		}

		if err = h.chatService.AddUsersToChat(chat.ID, claims.UserID, request.ReceiverID); err != nil {
			httputils.ResponseError(w, http.StatusInternalServerError, "failed to add users to chat")
			return
		}
	}

	msg := model.Message{
		ChatID:   chat.ID,
		SenderID: claims.UserID,
		Message:  request.Message,
	}

	// persist to DB
	if err = h.chatService.SendMessageToChat(chat, msg); err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to send message to chat")
		return
	}

	// cache in redis (best-effort)
	if h.chatCacheService != nil {
		if err := h.chatCacheService.SendMessage(chat, msg); err != nil {
			log.Printf("warning: failed to cache message in redis: %v", err)
		}
	}

	if h.hub != nil {
		h.hub.BroadcastMessage(chat.ID, msg)
	}

	httputils.ResponseJSON(w, http.StatusCreated, msg)
}

type GetChatMessagesRequest struct {
	Cursor    string `json:"cursor" form:"cursor" query:"cursor"`
	Limit     int    `json:"limit" form:"limit" query:"limit"`
	Direction string `json:"direction" form:"direction" query:"direction"`
}

type PaginationInfo struct {
	NextCursor     *string `json:"nextCursor,omitempty"`
	PreviousCursor *string `json:"previousCursor,omitempty"`
	HasNext        bool    `json:"hasNext"`
	HasPrevious    bool    `json:"hasPrevious"`
	Limit          int     `json:"limit"`
	TotalCount     *int64  `json:"totalCount,omitempty"`
}

type GetChatMessagesResponse struct {
	Data       []model.Message `json:"data"`
	Pagination PaginationInfo  `json:"pagination"`
}

// GetChatMessages возвращает сообщения чата с пагинацией
// @Summary Получить сообщения чата
// @Description Получить сообщения чата с cursor-based пагинацией
// @Tags chat
// @Accept json
// @Produce json
// @Param id path int true "ID чата"
// @Param cursor query string false "Курсор для пагинации"
// @Param limit query int false "Лимит сообщений" minimum(1) maximum(100) default(20)
// @Param direction query string false "Направление пагинации" Enums(older, newer) default(older)
// @Success 200 {object} GetChatMessagesResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /chat/{id}/messages [get]
func (h *ChatHandler) getMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chatID, err := strconv.Atoi(vars["id"])
	if err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	queryParams := r.URL.Query()
	cursor := queryParams.Get("cursor")
	limitStr := queryParams.Get("limit")
	direction := queryParams.Get("direction")
	if direction == "" {
		direction = "older"
	}

	// Валидация direction
	if direction != "older" && direction != "newer" {
		httputils.ResponseError(w, http.StatusBadRequest, "Direction must be 'older' or 'newer'")
		return
	}

	// Парсим limit
	limit := 20
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 100 {
			httputils.ResponseError(w, http.StatusBadRequest, "Limit must be between 1 and 100")
			return
		}
	}

	messages, err := h.chatCacheService.GetMessages(uint(chatID))
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}

	var hasNext bool
	var hasPrevious bool
	var totalCount *int64

	if len(messages) == 0 {
		messages, hasNext, hasPrevious, totalCount, err = h.chatService.GetChatMessages(uint(chatID), cursor, limit, direction, r.Context())
		if err != nil {
			httputils.ResponseError(w, http.StatusInternalServerError, "failed to get messages from DB")
			return
		}
	} else {
		response := GetChatMessagesResponse{
			Data:       messages,
			Pagination: PaginationInfo{},
		}
		httputils.ResponseJSON(w, 200, response)
		return
	}

	if len(messages) == 0 {
		response := GetChatMessagesResponse{
			Data: []model.Message{},
			Pagination: PaginationInfo{
				Limit:       limit,
				HasNext:     false,
				HasPrevious: false,
			},
		}
		httputils.ResponseJSON(w, 200, response)
		return
	}

	// Формируем курсоры
	var nextCursor, previousCursor *string

	if hasNext {
		lastMessageTime := messages[len(messages)-1].CreatedAt.Format(time.RFC3339)
		nextCursor = &lastMessageTime
	}

	if hasPrevious {
		firstMessageTime := messages[0].CreatedAt.Format(time.RFC3339)
		previousCursor = &firstMessageTime
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

type createChatRequest struct {
	Name    string `json:"name"`
	UserIDs []uint `json:"user_ids"`
}

func (h *ChatHandler) createChat(w http.ResponseWriter, r *http.Request) {
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

	var req createChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid request format")
		return
	}
	defer r.Body.Close()

	chat := &model.Chat{Name: req.Name}
	if err := h.chatService.CreateChat(chat); err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to create chat")
		return
	}

	// ensure current user is in chat
	userIDs := req.UserIDs
	found := slices.Contains(userIDs, claims.UserID)
	if !found {
		userIDs = append(userIDs, claims.UserID)
	}

	if err := h.chatService.AddUsersToChat(chat.ID, userIDs...); err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to add users to chat")
		return
	}

	httputils.ResponseJSON(w, http.StatusCreated, map[string]uint{"chat_id": chat.ID})
}

func (h *ChatHandler) UserJoined(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chatID, err1 := strconv.Atoi(vars["chat_id"])
	userID, err2 := strconv.Atoi(vars["user_id"])
	if err1 != nil || err2 != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid chat or user id")
		return
	}

	if err := h.chatCacheService.UserJoined(uint(chatID), uint(userID)); err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to join chat")
		return
	}

	httputils.ResponseJSON(w, http.StatusOK, map[string]string{"status": "user joined"})
}

func (h *ChatHandler) UserLeft(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chatID, err1 := strconv.Atoi(vars["chat_id"])
	userID, err2 := strconv.Atoi(vars["user_id"])
	if err1 != nil || err2 != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid chat or user id")
		return
	}

	if err := h.chatCacheService.UserLeft(uint(chatID), uint(userID)); err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to leave chat")
		return
	}

	httputils.ResponseJSON(w, http.StatusOK, map[string]string{"status": "user left"})
}

// @Summary Create group chat
// @Description Create a new group chat with users
// @ID create-group-chat
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Param groupData body createGroupRequest true "Group Data"
// @Success 201 {object} model.Chat
// @Failure 400 {object} response.ErrorResponse
// @Router /chat/create [post]
func (h *ChatHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.Header.Get("Authorization")
	if tokenStr == "" {
		httputils.ResponseError(w, http.StatusUnauthorized, "missing token")
		return
	}

	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")

	claims, err := auth.ValidateToken(tokenStr)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	var req createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Добавляем текущего пользователя в участников
	req.UserIDs = append(req.UserIDs, claims.UserID)

	chat, err := h.chatService.CreateGroupChat(req.Name, req.UserIDs)
	if err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "failed to create group chat")
		return
	}

	httputils.ResponseJSON(w, http.StatusCreated, chat)
}

func (h *ChatHandler) wsChat(w http.ResponseWriter, r *http.Request) {
	// 1) auth
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

	// 2) chat id
	vars := mux.Vars(r)
	chatID64, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil || chatID64 == 0 {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid chat id")
		return
	}
	chatID := uint(chatID64)

	// 3) membership
	ok, err := h.chatService.IsUserInChat(chatID, claims.UserID)
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to validate membership")
		return
	}
	if !ok {
		httputils.ResponseError(w, http.StatusForbidden, "user is not a member of this chat")
		return
	}

	// 4) upgrade
	conn, err := ws.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// 5) presence + история
	room := h.hub.GetRoom(chatID)
	ctx := r.Context()
	client := ws.NewClient(ctx, conn, claims.UserID, chatID)
	room.RegisterClient(client)

	_ = h.chatCacheService.UserJoined(chatID, claims.UserID)

	// история (redis->db)
	messages, err := h.chatCacheService.GetMessages(chatID)
	if err == nil && len(messages) == 0 {
		// todo: rework to support pagination

		// if msgs, err2 := h.chatService.GetMessagesOfChat(chatID); err2 == nil {
		// 	messages = msgs
		// }
	}
	client.SendJSON(ws.OutEvent{
		Type:     "history",
		Messages: messages,
	})

	// 6) pumps
	go client.WritePump()

	handleIncoming := func(c *ws.Client, ev ws.InEvent) {
		switch strings.ToLower(ev.Type) {
		case "message":
			txt := strings.TrimSpace(ev.Message)
			if txt == "" {
				c.SendJSON(ws.OutEvent{Type: "error", Message: "empty message"})
				return
			}
			msg := model.Message{
				ChatID:   c.ChatID,
				SenderID: c.UserID,
				Message:  txt,
			}
			if err := h.chatService.SendMessageToChat(&model.Chat{Model: gorm.Model{ID: c.ChatID}}, msg); err != nil {
				c.SendJSON(ws.OutEvent{Type: "error", Message: "failed to persist message"})
				return
			}
			if h.chatCacheService != nil {
				_ = h.chatCacheService.SendMessage(&model.Chat{Model: gorm.Model{ID: c.ChatID}}, msg)
			}
			h.hub.BroadcastMessage(c.ChatID, msg)
		default:
			// ignore
		}
	}

	client.ReadPump(handleIncoming)

	// 7) cleanup
	room.UnregisterClient(client)
	_ = h.chatCacheService.UserLeft(chatID, claims.UserID)
}

type ListChatsResponse struct {
	Name        string
	LastMessage model.Message
	gorm.Model
}

// @Summary List chats
// @Description List chats of user
// @ID chat-list
// @Tags chat
// @Accept json
// @Produce json
// @Param Bearer header string true "Auth Token"
// @Param groupData body createGroupRequest true "Group Data"
// @Success 200 {object} []ListChatsResponse
// @Failure 500 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /chat/list [post]
func (h *ChatHandler) listChats(w http.ResponseWriter, r *http.Request) {
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

	userID := claims.UserID

	chats, err := h.chatService.GetChatsForUser(userID)
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get chat list for user with ID: %d", userID))
		return
	}

	var responses []ListChatsResponse

	for _, chat := range *chats {
		responses = append(responses, ListChatsResponse{
			Name:        chat.Name,
			LastMessage: chat.Messages[len(chat.Messages)-1],
			Model:       chat.Model,
		})
	}

	httputils.ResponseJSON(w, http.StatusOK, responses)
}
