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

func (s *Server) setupRoutes() {
	api := s.router.PathPrefix("/api").Subrouter()

	// Routes для пользователей
	s.userHandler.RegisterRoutes(api)

	// Routes для чатов
	s.chatHandler.RegisterRoutes(api)
	api.HandleFunc("/chat/join/{chat_id:[0-9]+}/{user_id:[0-9]+}", s.chatHandler.UserJoined).Methods("POST")
	api.HandleFunc("/chat/leave/{chat_id:[0-9]+}/{user_id:[0-9]+}", s.chatHandler.UserLeft).Methods("POST")

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
	srv := &http.Server{
		Handler:      s.router,
		Addr:         ":" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(srv.ListenAndServe())
}
