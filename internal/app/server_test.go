package app

import (
	"net/http/httptest"
	"testing"

	"github.com/gorilla/handlers"
	"tush00nka/bbbab_messenger/internal/handler"
)

func TestCORSPreflightRequest(t *testing.T) {
	// Create a minimal server setup for testing
	userHandler := &handler.UserHandler{}
	chatHandler := &handler.ChatHandler{}
	server := NewServer(userHandler, chatHandler)

	// Create a test OPTIONS preflight request
	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Apply CORS middleware to the router (same as in Run method)
	cors := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-Requested-With"}),
	)
	corsHandler := cors(server.router)

	// Serve the request
	corsHandler.ServeHTTP(rr, req)

	// Check that CORS origin header is present
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %v, want *", got)
	}

	// For OPTIONS requests, gorilla/handlers sets the Allow-Headers based on request
	allowHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	if allowHeaders == "" {
		t.Error("Access-Control-Allow-Headers should not be empty for OPTIONS request")
	}
}

func TestCORSWithActualRequest(t *testing.T) {
	// Create a minimal server setup for testing
	userHandler := &handler.UserHandler{}
	chatHandler := &handler.ChatHandler{}
	server := NewServer(userHandler, chatHandler)

	// Create a test GET request
	req := httptest.NewRequest("GET", "/swagger/doc.json", nil)
	req.Header.Set("Origin", "http://example.com")

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Apply CORS middleware to the router
	cors := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-Requested-With"}),
	)
	corsHandler := cors(server.router)

	// Serve the request
	corsHandler.ServeHTTP(rr, req)

	// Check that CORS header is present
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %v, want *", got)
	}
}
