package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

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

type UserGet struct {
	CurrentUsersPage bool
	Username         string
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	session, err := Store.Get(r, "test")
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var username string
	err = db.QueryRow("SELECT username FROM users WHERE id=$1", id).Scan(&username)
	if err != nil {
		fmt.Fprintf(w, "no such user")
		return
	}

	data := UserGet{
		CurrentUsersPage: false,
		Username:         username,
	}

	if val, ok := session.Values["currentUser"]; ok {
		data.CurrentUsersPage = val.(int) == id
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}

func hasUsername(username string) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = $1", username).Scan(&count)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DB ERROR: %s", err)
	}

	return count > 0
}

type RegisterGet struct {
	HasError     bool
	ErrorMessage string
}

type RegisterPost struct {
	Username        string
	Password        string
	ConfirmPassword string
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	session, err := Store.Get(r, "test")
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()
		var form RegisterPost
		if err = decoder.Decode(&form); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		username := form.Username
		password := form.Password
		confirm := form.ConfirmPassword

		if password != confirm {
			session.Values["error"] = "Пароли не совпадают!"
			session.Save(r, w)
			http.Redirect(w, r, "/register", http.StatusSeeOther)
			return
		}

		if hasUsername(username) {
			session.Values["error"] = "Пользователь с таким именем уже существует!"
			session.Save(r, w)
			http.Redirect(w, r, "/register", http.StatusSeeOther)
			return
		}

		if username != "" && password != "" {
			hash, _ := HashPassword(password)
			_, err := db.Exec("INSERT INTO users (username, password) VALUES ($1, $2)", username, hash)
			if err != nil {
				fmt.Fprintf(os.Stderr, "DB INSERT ERROR: %s", err)
			}

			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
	}

	if val, ok := session.Values["currentUser"]; ok {
		http.Redirect(w, r, fmt.Sprintf("/user/%d", val.(int)), http.StatusFound)
	}

	errorMessage := ""
	if val, ok := session.Values["error"]; ok {
		errorMessage = val.(string)
		delete(session.Values, "error")
		session.Save(r, w)
	}

	data := RegisterGet{
		HasError:     errorMessage != "",
		ErrorMessage: errorMessage,
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}

type LoginPost struct {
	Username string
	Password string
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	session, err := Store.Get(r, "test")
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()
		var form LoginPost
		if err := decoder.Decode(&form); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		username := form.Username
		password := form.Password

		if !hasUsername(username) {
			session.Values["error"] = "Пользователь с таким именем не зарегистрирован!"
			session.Save(r, w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var hash string
		db.QueryRow("SELECT password FROM users WHERE username=$1", username).Scan(&hash)

		if !CheckPasswordHash(password, hash) {
			session.Values["error"] = "Неверный пароль!"
			session.Save(r, w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var id int
		db.QueryRow("SELECT id FROM users WHERE username=$1", username).Scan(&id)
		session.Values["currentUser"] = id
		session.Save(r, w)
	}

	if val, ok := session.Values["currentUser"]; ok {
		http.Redirect(w, r, fmt.Sprintf("/user/%d", val.(int)), http.StatusFound)
		return
	}

	errorMessage := ""
	if val, ok := session.Values["error"]; ok {
		errorMessage = val.(string)
		delete(session.Values, "error")
		session.Save(r, w)
	}

	data := RegisterGet{
		HasError:     errorMessage != "",
		ErrorMessage: errorMessage,
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
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
