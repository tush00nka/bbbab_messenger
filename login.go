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
	db := GetDB()
	session, err := Store.Get(r, "test")
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)

	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()
		var form LoginPost
		if err := decoder.Decode(&form); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			encoder.Encode(ErrorGet{
				ErrorMessage: "Bad Request",
			})
			return
		}

		username := form.Username
		password := form.Password

		if !hasUsername(username) {
			w.WriteHeader(http.StatusBadRequest)
			encoder.Encode(ErrorGet{
				ErrorMessage: "Пользователь с таким именем не зарегистрирован!",
			})
			return
		}

		var user User
		db.Where("username = ?", username).First(&user)

		if !CheckPasswordHash(password, user.Password) {
			w.WriteHeader(http.StatusBadRequest)
			encoder.Encode(ErrorGet{
				ErrorMessage: "Неверный пароль!",
			})
			return
		}

		session.Values["currentUser"] = user.ID
		session.Save(r, w)
	}

	if val, ok := session.Values["currentUser"]; ok {
		http.Redirect(w, r, fmt.Sprintf("/user/%d", val.(uint)), http.StatusFound)
		return
	}

	errorMessage := ""
	if val, ok := session.Values["error"]; ok {
		errorMessage = val.(string)
		delete(session.Values, "error")
		session.Save(r, w)
	}

	data := ErrorGet{
		ErrorMessage: errorMessage,
	}

	w.Header().Set("Content-Type", "application/json")
	encoder.Encode(data)
}
