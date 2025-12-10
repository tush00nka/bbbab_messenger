package ws

import (
	"net/http"
	"os"
	"slices"

	"github.com/gorilla/websocket"
)

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Разрешаем все origins для WebSocket соединений
	// В production замените на конкретные домены
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// Список разрешенных origins
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://94.241.170.140:8080",
			"http://amber.thatusualguy.ru:8080",
		}

		// Для разработки разрешаем все
		if os.Getenv("ENVIRONMENT") == "development" {
			return true
		}

		return slices.Contains(allowedOrigins, origin)
	},
}
