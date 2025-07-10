// @title BBBAB Messenger
// @version 0.1
// @description This is a sample server.

// @host localhost:8080
// @BasePath /
// @query.collection.format multi

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "tush00nka/bbbab_messenger/docs"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
	"golang.org/x/crypto/bcrypt"
)

const DSN string = "host=localhost user=bor password=bor dbname=bbbab sslmode=disable"

var db *gorm.DB

func GetDB() *gorm.DB {
	var err error

	if db == nil {
		db, err = gorm.Open(postgres.Open(DSN), &gorm.Config{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "CONNECTION ERROR: %s", err)
			os.Exit(-1)
		}
	}

	return db
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func hasUsername(username string) bool {
	var count int64
	db.Table("users").Where("username = ?", username).Count(&count)
	return count > 0
}

type ErrorGet struct {
	ErrorMessage string
}

type TokenResponse struct {
	Token string
}

func ResponseError(w http.ResponseWriter, encoder *json.Encoder, errorCode int, errorMessage string) {
	w.WriteHeader(errorCode)
	encoder.Encode(ErrorGet{
		ErrorMessage: errorMessage,
	})
}

func RedirectLoggedIn(w http.ResponseWriter, r *http.Request, encoder *json.Encoder) bool {
	c, err := r.Cookie("token")
	if err != nil {
		if err == http.ErrNoCookie {
			// ResponseError(w, encoder, http.StatusUnauthorized, "No Token")
			return false
		}
		// ResponseError(w, encoder, http.StatusBadRequest, "Bad Request")
		return false
	}

	tokenStr := c.Value
	claims, err := ValidateToken(tokenStr)
	if err != nil {
		// ResponseError(w, encoder, http.StatusUnauthorized, "Invalid Token")
		return false
	}

	http.Redirect(w, r, fmt.Sprintf("/user/%s", claims.Username), http.StatusFound)
	return true
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/user/{username}", usersHandler)

	fs := http.FileServer(http.Dir("static"))
	router.Handle("/", fs)

	router.HandleFunc("/login", loginHandler).Methods("POST", "GET")
	router.HandleFunc("/register", registerHandler).Methods("POST", "GET")
	router.HandleFunc("/chats", chatsHandler).Methods("POST", "GET")
	router.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:    "token",
			Expires: time.Now().Add(-7 * 24 * time.Hour),
		})
		http.Redirect(w, r, "/login", http.StatusFound)
	})

	// Настройка Swagger
	swaggerHandler := httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"), // Важно: относительный путь
	)

	router.PathPrefix("/swagger/").Handler(swaggerHandler)

	// Явно обслуживаем doc.json
	router.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./docs/swagger.json")
	})

	fmt.Println("Server is listening on 8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
