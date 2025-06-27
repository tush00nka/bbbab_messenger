package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

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

	data := ErrorGet{
		HasError:     errorMessage != "",
		ErrorMessage: errorMessage,
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}
