package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type RegisterPost struct {
	Username        string
	Password        string
	ConfirmPassword string
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
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
		var form RegisterPost
		if err = decoder.Decode(&form); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			encoder.Encode(ErrorGet{
				ErrorMessage: "Error parsing post form",
			})

			return
		}

		username := form.Username
		password := form.Password
		confirm := form.ConfirmPassword

		if password != confirm {
			w.WriteHeader(http.StatusBadRequest)
			encoder.Encode(ErrorGet{
				ErrorMessage: "Пароли не совпадают",
			})
			return
		}

		if hasUsername(username) {
			w.WriteHeader((http.StatusBadRequest))
			encoder.Encode(ErrorGet{
				ErrorMessage: "Пользователь с таким именем уже существует!",
			})
			return
		}

		if username != "" && password != "" {
			hash, _ := HashPassword(password)
			db.Create(&User{Username: username, Password: hash})
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
	}

	if val, ok := session.Values["currentUser"]; ok {
		http.Redirect(w, r, fmt.Sprintf("/user/%d", val.(uint)), http.StatusFound)
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

	encoder.Encode(data)
}
