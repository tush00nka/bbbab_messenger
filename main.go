package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const CONN_STR string = "user=bor password=bor dbname=bbbab sslmode=disable"

var db *sql.DB
var Store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

func GetDB() *sql.DB {
	var err error

	if db == nil {
		db, err = sql.Open("postgres", CONN_STR)
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
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = $1", username).Scan(&count)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DB ERROR: %s", err)
	}

	return count > 0
}

type ErrorGet struct {
	HasError     bool
	ErrorMessage string
}

func main() {
	db := GetDB()
	defer db.Close()

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
