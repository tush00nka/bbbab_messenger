package main

import (
	"encoding/json"
	"net/http"
)

type RegisterPost struct {
	Username        string
	Password        string
	ConfirmPassword string
}

// @Description Register an account
// @ID register
// @Accept json
// @Produce  json
// @Success 200 "Редирект на профиль"
// @Failure 400 {object} ErrorGet
// @Failure 500 {object} ErrorGet
// @Param registerData body RegisterPost true "Register data"
// @Router /register [post]
func registerHandler(w http.ResponseWriter, r *http.Request) {
	db := GetDB()

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)

	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()
		var form RegisterPost
		if err := decoder.Decode(&form); err != nil {
			ResponseError(w, encoder, http.StatusBadRequest, "Error parsing form")
			return
		}

		username := form.Username
		password := form.Password
		confirm := form.ConfirmPassword

		if password != confirm {
			ResponseError(w, encoder, http.StatusBadRequest, "Пароли не совпадают")
			return
		}

		if hasUsername(username) {
			ResponseError(w, encoder, http.StatusBadRequest, "Пользователь с таким именем уже существует!")
			return
		}

		if username != "" && password != "" {
			hash, _ := HashPassword(password)
			db.Create(&User{Username: username, Password: hash})
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		} else {
			ResponseError(w, encoder, http.StatusBadRequest, "Имя пользователя и пароль обязательны к заполнению!")
		}

		return
	}

	if !RedirectLoggedIn(w, r, encoder) {
		w.WriteHeader(http.StatusOK)
	}
}
