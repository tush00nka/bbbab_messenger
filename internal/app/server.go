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

	cors := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}), // Разрешить все источники
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-Requested-With"}),
		// handlers.AllowCredentials(),
	)

	s.router.Use(cors)

	api := s.router.PathPrefix("/api").Subrouter()

	// Routes для пользователей
	s.userHandler.RegisterRoutes(api)

	// Routes для чатов
	s.chatHandler.RegisterRoutes(api)
	api.HandleFunc("/ping", handler.Ping)

	// Swagger
	swaggerHandler := httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	)
	s.router.PathPrefix("/swagger/").Handler(swaggerHandler)

	// doc.json
	s.router.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./docs/swagger.json")
	})

}

func (s *Server) Run(port string) {
	// Apply CORS middleware to the router
	cors := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-Requested-With"}),
	)

	srv := &http.Server{
		Handler:      cors(s.router),
		Addr:         ":" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(srv.ListenAndServe())
}
