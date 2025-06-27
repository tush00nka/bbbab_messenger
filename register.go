package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

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

	data := ErrorGet{
		HasError:     errorMessage != "",
		ErrorMessage: errorMessage,
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}
