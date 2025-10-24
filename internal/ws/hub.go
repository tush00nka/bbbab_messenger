package ws

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"sync"

	"github.com/gorilla/websocket"
)

type OutEvent struct {
	Type     string      `json:"type"`
	Message  interface{} `json:"message,omitempty"`
	Messages interface{} `json:"messages,omitempty"`
	UserID   uint        `json:"user_id,omitempty"`
}

type InEvent struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

type Hub struct {
	mu    sync.RWMutex
	rooms map[uint]*Room
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[uint]*Room)}
}

// Экспортируемый доступ к комнате
func (h *Hub) GetRoom(chatID uint) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[chatID]; ok {
		return r
	}
	r := NewRoom(chatID)
	h.rooms[chatID] = r
	go r.run()
	return r
}

// Бродкаст из REST/где угодно
func (h *Hub) BroadcastMessage(chatID uint, payload any) {
	r := h.GetRoom(chatID)
	ev := OutEvent{Type: "message", Message: payload}
	data, _ := json.Marshal(ev)
	r.Broadcast(data)
}

// ---- Room ----

type Room struct {
	chatID     uint
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
}

func NewRoom(chatID uint) *Room {
	return &Room{
		chatID:     chatID,
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
	}
}

func (r *Room) run() {
	for {
		select {
		case c := <-r.register:
			r.clients[c] = true
		case c := <-r.unregister:
			if _, ok := r.clients[c]; ok {
				delete(r.clients, c)
				close(c.send)
			}
		case msg := <-r.broadcast:
			for c := range r.clients {
				select {
				case c.send <- msg:
				default:
					close(c.send)
					delete(r.clients, c)
				}
			}
		}
	}
}

// Экспортируемые обёртки, чтобы не трогать внутренние каналы снаружи
func (r *Room) RegisterClient(c *Client)   { r.register <- c }
func (r *Room) UnregisterClient(c *Client) { r.unregister <- c }
func (r *Room) Broadcast(msg []byte)       { r.broadcast <- msg }

// ---- Client ----

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 64 * 1024
)

type Client struct {
	UserID uint
	ChatID uint

	conn *websocket.Conn
	send chan []byte

	ctx context.Context
}

func NewClient(ctx context.Context, conn *websocket.Conn, userID, chatID uint) *Client {
	return &Client{
		UserID: userID,
		ChatID: chatID,
		conn:   conn,
		send:   make(chan []byte, 256),
		ctx:    ctx,
	}
}

// Экспортируем
func (c *Client) ReadPump(handleIncoming func(*Client, InEvent)) {
	defer func() { _ = c.conn.Close() }()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		var ev InEvent
		if err := c.conn.ReadJSON(&ev); err != nil {
			break
		}
		handleIncoming(c, ev)
	}
}

// Экспортируем
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				return
			}
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) SendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("ws marshal err: %v", err)
		return
	}
	select {
	case c.send <- data:
	default:
		// перегруз — дропнем
	}
}
