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
	router *mux.Router
}

func NewServer(userHandler *handler.UserHandler, chatHandler *handler.ChatHandler) *Server {
	router := mux.NewRouter()

	// TODO: Middleware
	// router.Use(middleware.Logging)

	// Routes
	userHandler.RegisterRoutes(router)
	chatHandler.RegisterRoutes(router)

	// Настройка Swagger
	swaggerHandler := httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"), // Важно: относительный путь
	)

	router.PathPrefix("/swagger/").Handler(swaggerHandler)

	// Явно обслуживаем doc.json
	router.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./docs/swagger.json")
	})

	return &Server{router: router}
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
