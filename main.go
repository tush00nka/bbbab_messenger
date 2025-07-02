package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

const DSN string = "host=localhost user=bor password=bor dbname=bbbab sslmode=disable"

var db *gorm.DB
var Store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

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

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/user/{id:[0-9]+}", usersHandler)

	fs := http.FileServer(http.Dir("static"))
	router.Handle("/", fs)

	router.HandleFunc("/login", loginHandler).Methods("POST", "GET")
	router.HandleFunc("/register", registerHandler).Methods("POST", "GET")
	router.HandleFunc("/chats", chatsHandler).Methods("POST", "GET")
	router.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		session, err := Store.Get(r, "session")
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		if _, ok := session.Values["currentUser"]; ok {
			delete(session.Values, "currentUser")
			session.Save(r, w)
			http.Redirect(w, r, "/login", http.StatusFound)
		}
	})

	fmt.Println("Server is listening on 8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
