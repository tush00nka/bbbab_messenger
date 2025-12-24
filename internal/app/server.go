package app

import (
	"log"
	"net/http"
	"time"
	"tush00nka/bbbab_messenger/internal/handler"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

type Server struct {
	router      *mux.Router
	userHandler *handler.UserHandler
	chatHandler *handler.ChatHandler
}

func NewServer(userHandler *handler.UserHandler, chatHandler *handler.ChatHandler) *Server {
	s := &Server{
		router:      mux.NewRouter(),
		userHandler: userHandler,
		chatHandler: chatHandler,
	}

	s.setupRoutes()

	return s
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Установите CORS заголовки
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		// Если это preflight OPTIONS запрос
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) setupRoutes() {
	// Сначала создаем основной роутер
	s.router = mux.NewRouter()

	s.router.Use(CORSMiddleware)

	// API роуты
	api := s.router.PathPrefix("/api").Subrouter()

	// Routes для пользователей
	s.userHandler.RegisterRoutes(api)

	// Routes для чатов
	s.chatHandler.RegisterRoutes(api)

	api.HandleFunc("/ping", handler.Ping)

	s.router.PathPrefix("/swagger/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Установите CORS заголовки
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		// Продолжить обработку Swagger
		httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
		).ServeHTTP(w, r)
	})

	s.router.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./docs/swagger.json")
	})
}

func (s *Server) Run(port string) {
	srv := &http.Server{
		Handler:      s.router,
		Addr:         ":" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(srv.ListenAndServeTLS("/etc/letsencrypt/live/amber.thatusualguy.ru/fullchain.pem", "/etc/letsencrypt/live/amber.thatusualguy.ru/privkey.pem"))
	// log.Fatal(srv.ListenAndServe())
}
