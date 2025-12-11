package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/atomic"
)

// Константы
const (
	writeWait          = 10 * time.Second
	pongWait           = 60 * time.Second
	pingPeriod         = (pongWait * 9) / 10
	maxMessageSize     = 64 * 1024 // 64KB
	maxSendChannelSize = 256
	defaultRoomSize    = 100
)

// Типы событий
const (
	EventTypeMessage        = "message"
	EventTypeTyping         = "typing"
	EventTypeReadReceipt    = "read_receipt"
	EventTypeHistory        = "history"
	EventTypeError          = "error"
	EventTypeUserJoined     = "user_joined"
	EventTypeUserLeft       = "user_left"
	EventTypeMessageSent    = "message_sent"
	EventTypePresence       = "presence"
	EventTypeRoomInfo       = "room_info"
	EventTypeMessageDeleted = "message_deleted"
)

// OutEvent исходящее событие
type OutEvent struct {
	Type      string    `json:"type"`
	Message   any       `json:"message,omitempty"`
	Messages  any       `json:"messages,omitempty"`
	UserID    uint      `json:"user_id,omitempty"`
	ChatID    uint      `json:"chat_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	MessageID uint      `json:"message_id,omitempty"`
	Meta      any       `json:"meta,omitempty"`
}

// InEvent входящее событие
type InEvent struct {
	Type      string `json:"type"`
	Message   string `json:"message,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// HubOptions опции хаба
type HubOptions struct {
	MaxRoomSize           int
	MaxConnectionsPerUser int
	EnableMetrics         bool
	CleanupInterval       time.Duration
}

// Hub управляет комнатами и соединениями
type Hub struct {
	mu        sync.RWMutex
	rooms     map[uint]*Room
	userRooms map[uint]map[uint]bool // userID -> set of chatIDs
	options   HubOptions
	stats     HubStats
	shutdown  chan struct{}
	metrics   *Metrics
}

// HubStats статистика хаба
type HubStats struct {
	TotalRooms       int64
	TotalConnections int64
	ActiveUsers      int64
}

// Metrics метрики
type Metrics struct {
	MessagesSent     atomic.Int64
	MessagesReceived atomic.Int64
	Connections      atomic.Int64
	Errors           atomic.Int64
}

// NewHub создает новый хаб
func NewHub(options ...HubOptions) *Hub {
	opts := HubOptions{
		MaxRoomSize:           defaultRoomSize,
		MaxConnectionsPerUser: 10,
		EnableMetrics:         true,
		CleanupInterval:       5 * time.Minute,
	}

	if len(options) > 0 {
		opts = options[0]
	}

	hub := &Hub{
		rooms:     make(map[uint]*Room),
		userRooms: make(map[uint]map[uint]bool),
		options:   opts,
		shutdown:  make(chan struct{}),
	}

	if opts.EnableMetrics {
		hub.metrics = &Metrics{}
	}

	// Запускаем сборщик мусора
	go hub.cleanupLoop()

	return hub
}

// GetRoom возвращает комнату по ID чата
func (h *Hub) GetRoom(chatID uint) *Room {
	h.mu.RLock()
	room, exists := h.rooms[chatID]
	h.mu.RUnlock()

	if exists {
		return room
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Двойная проверка
	if room, exists := h.rooms[chatID]; exists {
		return room
	}

	room = NewRoom(chatID, h.options.MaxRoomSize)
	h.rooms[chatID] = room
	h.stats.TotalRooms++

	if h.metrics != nil {
		h.metrics.Connections.Inc()
	}

	return room
}

// GetRoomSafe возвращает комнату, если она существует
func (h *Hub) GetRoomSafe(chatID uint) (*Room, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	room, exists := h.rooms[chatID]
	return room, exists
}

// RegisterUserRoom регистрирует пользователя в комнате
func (h *Hub) RegisterUserRoom(userID, chatID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.userRooms[userID]; !exists {
		h.userRooms[userID] = make(map[uint]bool)
	}
	h.userRooms[userID][chatID] = true
}

// UnregisterUserRoom удаляет пользователя из комнаты
func (h *Hub) UnregisterUserRoom(userID, chatID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if rooms, exists := h.userRooms[userID]; exists {
		delete(rooms, chatID)
		if len(rooms) == 0 {
			delete(h.userRooms, userID)
		}
	}
}

// GetUserRooms возвращает комнаты пользователя
func (h *Hub) GetUserRooms(userID uint) []uint {
	h.mu.RLock()
	defer h.mu.RUnlock()

	rooms, exists := h.userRooms[userID]
	if !exists {
		return nil
	}

	roomIDs := make([]uint, 0, len(rooms))
	for chatID := range rooms {
		roomIDs = append(roomIDs, chatID)
	}

	return roomIDs
}

// BroadcastMessage отправляет сообщение во все комнаты пользователя
func (h *Hub) BroadcastMessage(chatID uint, payload any) {
	room := h.GetRoom(chatID)
	ev := OutEvent{
		Type:      EventTypeMessage,
		Message:   payload,
		ChatID:    chatID,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(ev)
	if err != nil {
		log.Printf("hub: failed to marshal broadcast message: %v", err)
		return
	}

	room.Broadcast(data)

	if h.metrics != nil {
		h.metrics.MessagesSent.Inc()
	}
}

// BroadcastTypingIndicator отправляет индикатор набора текста
func (h *Hub) BroadcastTypingIndicator(chatID, userID uint, isTyping bool) {
	room, exists := h.GetRoomSafe(chatID)
	if !exists {
		return
	}

	ev := OutEvent{
		Type:    EventTypeTyping,
		UserID:  userID,
		ChatID:  chatID,
		Message: isTyping,
	}

	data, _ := json.Marshal(ev)
	room.BroadcastToOthers(userID, data)
}

// BroadcastUserPresence отправляет информацию о присутствии пользователя
func (h *Hub) BroadcastUserPresence(chatID, userID uint, isOnline bool) {
	room, exists := h.GetRoomSafe(chatID)
	if !exists {
		return
	}

	eventType := EventTypeUserLeft
	if isOnline {
		eventType = EventTypeUserJoined
	}

	ev := OutEvent{
		Type:    eventType,
		UserID:  userID,
		ChatID:  chatID,
		Message: isOnline,
	}

	data, _ := json.Marshal(ev)
	room.BroadcastToOthers(userID, data)
}

// GetRoomInfo возвращает информацию о комнате
func (h *Hub) GetRoomInfo(chatID uint) *RoomInfo {
	room, exists := h.GetRoomSafe(chatID)
	if !exists {
		return nil
	}

	return room.GetInfo()
}

// GetStats возвращает статистику хаба
func (h *Hub) GetStats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.stats
}

// Shutdown останавливает хаб
func (h *Hub) Shutdown() {
	close(h.shutdown)

	h.mu.Lock()
	defer h.mu.Unlock()

	// Останавливаем все комнаты
	for _, room := range h.rooms {
		room.Shutdown()
	}

	h.rooms = make(map[uint]*Room)
	h.userRooms = make(map[uint]map[uint]bool)
}

// cleanupLoop периодически очищает неактивные комнаты
func (h *Hub) cleanupLoop() {
	ticker := time.NewTicker(h.options.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.shutdown:
			return
		case <-ticker.C:
			h.cleanupInactiveRooms()
		}
	}
}

// cleanupInactiveRooms очищает неактивные комнаты
func (h *Hub) cleanupInactiveRooms() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for chatID, room := range h.rooms {
		if room.IsEmpty() && room.IsInactive() {
			room.Shutdown()
			delete(h.rooms, chatID)
			h.stats.TotalRooms--

			// Удаляем комнату из записей пользователей
			for userID, rooms := range h.userRooms {
				delete(rooms, chatID)
				if len(rooms) == 0 {
					delete(h.userRooms, userID)
				}
			}
		}
	}
}

// RoomInfo информация о комнате
type RoomInfo struct {
	ChatID        uint      `json:"chat_id"`
	ActiveClients int       `json:"active_clients"`
	CreatedAt     time.Time `json:"created_at"`
	LastActivity  time.Time `json:"last_activity"`
}

// Room управляет клиентами в чате
type Room struct {
	chatID      uint
	mu          sync.RWMutex
	clients     map[uint]*Client // userID -> Client
	broadcast   chan []byte
	register    chan *Client
	unregister  chan *Client
	shutdown    chan struct{}
	createdAt   time.Time
	lastActive  atomic.Time
	maxSize     int
	activeCount atomic.Int32
}

// NewRoom создает новую комнату
func NewRoom(chatID uint, maxSize int) *Room {
	room := &Room{
		chatID:     chatID,
		clients:    make(map[uint]*Client),
		broadcast:  make(chan []byte, maxSendChannelSize),
		register:   make(chan *Client, maxSize),
		unregister: make(chan *Client, maxSize),
		shutdown:   make(chan struct{}),
		createdAt:  time.Now(),
		maxSize:    maxSize,
	}

	room.lastActive.Store(time.Now())

	// Запускаем обработчик комнаты
	go room.run()

	return room
}

// run запускает обработчик комнаты
func (r *Room) run() {
	defer func() {
		// Закрываем все клиентские каналы при остановке
		r.mu.Lock()
		for _, client := range r.clients {
			client.Close()
		}
		r.mu.Unlock()
	}()

	for {
		select {
		case <-r.shutdown:
			return
		case client := <-r.register:
			r.handleRegister(client)
		case client := <-r.unregister:
			r.handleUnregister(client)
		case message := <-r.broadcast:
			r.handleBroadcast(message)
		}
	}
}

// handleRegister обрабатывает регистрацию клиента
func (r *Room) handleRegister(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Проверяем, не превышен ли лимит
	if len(r.clients) >= r.maxSize {
		client.SendJSON(OutEvent{
			Type:    EventTypeError,
			Message: "room is full",
		})
		client.Close()
		return
	}

	// Проверяем, не подключен ли уже пользователь
	if existingClient, exists := r.clients[client.UserID]; exists {
		// Закрываем старое соединение
		existingClient.Close()
		delete(r.clients, client.UserID)
		r.activeCount.Dec()
	}

	r.clients[client.UserID] = client
	r.activeCount.Inc()
	r.lastActive.Store(time.Now())

	// Отправляем информацию о комнате новому клиенту
	client.SendJSON(OutEvent{
		Type: EventTypeRoomInfo,
		Message: RoomInfo{
			ChatID:        r.chatID,
			ActiveClients: int(r.activeCount.Load()),
			CreatedAt:     r.createdAt,
			LastActivity:  r.lastActive.Load(),
		},
	})
}

// handleUnregister обрабатывает отключение клиента
func (r *Room) handleUnregister(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if storedClient, exists := r.clients[client.UserID]; exists && storedClient == client {
		delete(r.clients, client.UserID)
		r.activeCount.Dec()
		client.Close()
		r.lastActive.Store(time.Now())
	}
}

// handleBroadcast обрабатывает рассылку сообщений
func (r *Room) handleBroadcast(message []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, client := range r.clients {
		client.SendRaw(message)
	}

	r.lastActive.Store(time.Now())
}

// RegisterClient регистрирует клиента в комнате
func (r *Room) RegisterClient(client *Client) bool {
	select {
	case r.register <- client:
		return true
	default:
		return false // Комната перегружена
	}
}

// UnregisterClient отключает клиента от комнаты
func (r *Room) UnregisterClient(client *Client) {
	select {
	case r.unregister <- client:
	case <-r.shutdown:
	}
}

// Broadcast отправляет сообщение всем клиентам
func (r *Room) Broadcast(message []byte) {
	select {
	case r.broadcast <- message:
	case <-r.shutdown:
	}
}

// BroadcastToOthers отправляет сообщение всем, кроме указанного пользователя
func (r *Room) BroadcastToOthers(excludeUserID uint, message []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for userID, client := range r.clients {
		if userID != excludeUserID {
			client.SendRaw(message)
		}
	}

	r.lastActive.Store(time.Now())
}

// GetInfo возвращает информацию о комнате
func (r *Room) GetInfo() *RoomInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return &RoomInfo{
		ChatID:        r.chatID,
		ActiveClients: int(r.activeCount.Load()),
		CreatedAt:     r.createdAt,
		LastActivity:  r.lastActive.Load(),
	}
}

// IsEmpty проверяет, пуста ли комната
func (r *Room) IsEmpty() bool {
	return r.activeCount.Load() == 0
}

// IsInactive проверяет, неактивна ли комната
func (r *Room) IsInactive() bool {
	return time.Since(r.lastActive.Load()) > 1*time.Hour
}

// Shutdown останавливает комнату
func (r *Room) Shutdown() {
	close(r.shutdown)
}

// Client представляет WebSocket соединение
type Client struct {
	UserID    uint
	ChatID    uint
	ctx       context.Context
	cancel    context.CancelFunc
	conn      *websocket.Conn
	send      chan []byte
	mu        sync.RWMutex
	isClosed  bool
	rateLimit *RateLimiter
}

// RateLimiter ограничитель частоты сообщений
type RateLimiter struct {
	mu       sync.Mutex
	lastSent time.Time
	interval time.Duration
}

// NewRateLimiter создает новый ограничитель
func NewRateLimiter(interval time.Duration) *RateLimiter {
	return &RateLimiter{
		interval: interval,
		lastSent: time.Now().Add(-interval),
	}
}

// Allow проверяет, можно ли отправить сообщение
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastSent) >= rl.interval {
		rl.lastSent = now
		return true
	}

	return false
}

// NewClient создает нового клиента
func NewClient(ctx context.Context, conn *websocket.Conn, userID, chatID uint) *Client {
	ctx, cancel := context.WithCancel(ctx)

	return &Client{
		UserID:    userID,
		ChatID:    chatID,
		ctx:       ctx,
		cancel:    cancel,
		conn:      conn,
		send:      make(chan []byte, maxSendChannelSize),
		rateLimit: NewRateLimiter(100 * time.Millisecond), // 10 сообщений в секунду
	}
}

// SetRateLimit устанавливает лимит на частоту сообщений
func (c *Client) SetRateLimit(limitPerSecond int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	interval := time.Second / time.Duration(limitPerSecond)
	c.rateLimit = NewRateLimiter(interval)
}

// CheckRateLimit проверяет лимит частоты
func (c *Client) CheckRateLimit() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.rateLimit.Allow()
}

// ReadPump читает сообщения от клиента
func (c *Client) ReadPump(handleIncoming func(*Client, InEvent)) {
	defer c.Close()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			var ev InEvent
			if err := c.conn.ReadJSON(&ev); err != nil {
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure) {
					log.Printf("client read error: %v", err)
				}
				return
			}

			// Валидация времени
			if ev.Timestamp == 0 {
				ev.Timestamp = time.Now().UnixMilli()
			}

			handleIncoming(c, ev)
		}
	}
}

// WritePump отправляет сообщения клиенту
func (c *Client) WritePump() error {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return nil
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				// Канал закрыт
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return nil
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return err
			}

			if _, err := w.Write(message); err != nil {
				return err
			}

			// Обработка нескольких сообщений в одном writer
			n := len(c.send)
			for i := 0; i < n; i++ {
				if _, err := w.Write(<-c.send); err != nil {
					return err
				}
			}

			if err := w.Close(); err != nil {
				return err
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return err
			}
		}
	}
}

// SendJSON отправляет JSON сообщение
func (c *Client) SendJSON(v any) bool {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("client marshal error: %v", err)
		return false
	}

	return c.SendRaw(data)
}

// SendRaw отправляет сырые данные
func (c *Client) SendRaw(data []byte) bool {
	c.mu.RLock()
	if c.isClosed {
		c.mu.RUnlock()
		return false
	}

	select {
	case c.send <- data:
		c.mu.RUnlock()
		return true
	default:
		c.mu.RUnlock()
		// Перегруз - пропускаем сообщение
		return false
	}
}

// Close закрывает соединение
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed {
		return
	}

	c.isClosed = true
	c.cancel()
	close(c.send)
	c.conn.Close()
}

// IsClosed проверяет, закрыто ли соединение
func (c *Client) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isClosed
}
