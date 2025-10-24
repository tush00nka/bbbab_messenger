package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
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
	hub              *ws.Hub
}

func NewChatHandler(
	chatService service.ChatService,
	chatCacheService *service.ChatCacheService,
	hub *ws.Hub,
) *ChatHandler {
	return &ChatHandler{
		chatService:      chatService,
		chatCacheService: chatCacheService,
		hub:              hub,
	}
}

func (h *ChatHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/sendmessage", h.sendMessage).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/{id}", h.getMessages).Methods("GET", "OPTIONS")
	router.HandleFunc("/chat/create", h.createChat).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/{id}/ws", h.wsChat).Methods("GET")
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

// GetMessages (как было) — берём из Redis, если пусто — из БД
func (h *ChatHandler) getMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chatID, err := strconv.Atoi(vars["id"])
	if err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	messages, err := h.chatCacheService.GetMessages(uint(chatID))
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}

	if len(messages) == 0 {
		messages, err = h.chatService.GetMessagesOfChat(uint(chatID))
		if err != nil {
			httputils.ResponseError(w, http.StatusInternalServerError, "failed to get messages from DB")
			return
		}
	}

	httputils.ResponseJSON(w, http.StatusOK, messages)
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
		if msgs, err2 := h.chatService.GetMessagesOfChat(chatID); err2 == nil {
			messages = msgs
		}
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
