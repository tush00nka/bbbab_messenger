package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"tush00nka/bbbab_messenger/internal/model"
	"tush00nka/bbbab_messenger/internal/pkg/auth"
	"tush00nka/bbbab_messenger/internal/pkg/httputils"
	"tush00nka/bbbab_messenger/internal/service"

	"github.com/gorilla/mux"
)

type ChatHandler struct {
	chatService service.ChatService
}

func NewChatHandler(chatService service.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

func (h *ChatHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/sendmessage", h.sendMessage).Methods("POST", "OPTIONS")
	router.HandleFunc("/chat/{id}", h.getMessages).Methods("GET", "OPTIONS")
}

// @Summary Send message
// @Description Send message to chat
// @ID send-message
// @Accept json
// @Produce  json
// @Param Bearer header string true "Auth Token"
// @Param MessageData body sendMessageRequest true "Message Data"
// @Success 200
// @Failure 401 {object} response.ErrorResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /sendmessage [post]
func (h *ChatHandler) sendMessage(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.Header["Bearer"][0]
	claims, err := auth.ValidateToken(tokenStr)
	if err != nil {
		httputils.ResponseError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	var request sendMessageRequest
	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "invalid request format")
		return
	}

	chat, err := h.chatService.GetChatForUsers(claims.UserID, request.ReceiverID)
	if err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "failed to get chat for users")
		return
	}

	if chat == nil {
		chat = &model.Chat{}
		if err = h.chatService.CreateChat(chat); err != nil {
			httputils.ResponseError(w, http.StatusBadRequest, "failed to create chat")
			return
		}

		if err = h.chatService.AddUsersToChat(chat.ID, claims.UserID, request.ReceiverID); err != nil {
			httputils.ResponseError(w, http.StatusBadRequest, "failed to add users to chat")
			return
		}
	}

	msg := model.Message{
		ChatID:   chat.ID,
		SenderID: claims.UserID,
		Message:  request.Message,
	}

	if err = h.chatService.SendMessageToChat(chat, msg); err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "failed to send message to chat")
		return
	}
}

type sendMessageRequest struct {
	ReceiverID uint   `json:"receiver_id"`
	Message    string `json:"message"`
}

// @Summary Get messages
// @Description Get messages for chat
// @ID get-messages
// @Accept json
// @Produce  json
// @Param id path int true "Chat ID"
// @Success 200 {object} []model.Message
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /chat/{id} [get]
func (h *ChatHandler) getMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chatID, err := strconv.Atoi(vars["id"])
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "Failed to parse chat ID")
		return
	}

	messages, err := h.chatService.GetMessagesOfChat(uint(chatID))
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "failed to get messages for chat")
		return
	}

	httputils.ResponseJSON(w, http.StatusOK, messages)
}
