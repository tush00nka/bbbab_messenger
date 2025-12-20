package app

import (
	"log"
	"net/http"
	"time"
	"tush00nka/bbbab_messenger/internal/handler"

	"github.com/gorilla/handlers"
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

func (s *Server) setupRoutes() {
	// Сначала создаем основной роутер
	s.router = mux.NewRouter()

	swaggerHandler := httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.UIConfig(map[string]string{
			"defaultModelsExpandDepth": "0",
		}),
	)
	s.router.PathPrefix("/swagger/").Handler(swaggerHandler)

	s.router.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./docs/swagger.json")
	})

	// API роуты
	api := s.router.PathPrefix("/api").Subrouter()

	// Routes для пользователей
	s.userHandler.RegisterRoutes(api)

	// Routes для чатов
	s.chatHandler.RegisterRoutes(api)

	api.HandleFunc("/ping", handler.Ping)

	// CORS middleware должен оборачивать ВСЕ роуты
	corsMiddleware := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}),
		handlers.AllowedHeaders([]string{
			"Content-Type",
			"Authorization",
			"X-Requested-With",
			"Origin",
			"Accept",
			"X-CSRF-Token",
		}),
		handlers.ExposedHeaders([]string{
			"Content-Length",
			"Access-Control-Allow-Origin",
			"Access-Control-Allow-Headers",
		}),
		handlers.AllowCredentials(),
		handlers.MaxAge(86400),
		handlers.OptionStatusCode(200), // Важно для OPTIONS preflight
	)

	// Обернуть весь роутер в CORS
	s.router.Use(func(next http.Handler) http.Handler {
		return corsMiddleware(next)
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
	// log.Fatal(srv.ListenAndServeTLS("./certs/localhost+2.pem", "./certs/localhost+2-key.pem"))
	log.Fatal(srv.ListenAndServe())
}
