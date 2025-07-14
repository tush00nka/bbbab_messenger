package main

import (
	"encoding/json"
	"net/http"
	"time"
)

type LoginPost struct {
	Username string
	Password string
}

// @Summary Login
// @Description Log into account
// @ID login
// @Accept json
// @Produce  json
// @Success 200 {object} TokenResponse
// @Failure 400 {object} ErrorGet
// @Failure 500 {object} ErrorGet
// @Param loginData body LoginPost true "Login data"
// @Router /login [post]
func loginHandler(w http.ResponseWriter, r *http.Request) {
	db := GetDB()

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)

	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()
		var form LoginPost
		if err := decoder.Decode(&form); err != nil {
			ResponseError(w, encoder, http.StatusBadRequest, "Bad Request")
			return
		}

		username := form.Username
		password := form.Password

		if !hasUsername(username) {
			ResponseError(w, encoder, http.StatusBadRequest, "Пользователь с таким именем не зарегистрирован!")
			return
		}

		var user User
		db.Where("username = ?", username).First(&user)

		if !CheckPasswordHash(password, user.Password) {
			ResponseError(w, encoder, http.StatusBadRequest, "Неверный пароль!")
			return
		}

		token, err := GenerateToken(user.ID)
		if err != nil {
			ResponseError(w, encoder, http.StatusInternalServerError, "Error generation token")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:    "token",
			Value:   token,
			Expires: time.Now().Add(24 * time.Hour),
		})

		w.WriteHeader(http.StatusOK)
		encoder.Encode(TokenResponse{
			Token: token,
		})

		return
	}

	if !RedirectLoggedIn(w, r, encoder) {
		w.WriteHeader(http.StatusOK)
	}
}
