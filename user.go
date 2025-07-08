package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

type UserGet struct {
	CurrentUsersPage bool
	Username         string
}

// @Summary Get user
// @Description Get user
// @ID get-user
// @Produce  json
// @Success 200 {object} UserGet
// @Failure 404 {object} ErrorGet
// @Param username path string true "Username"
// @Router /user/{username} [get]
func usersHandler(w http.ResponseWriter, r *http.Request) {
	db := GetDB()
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)

	vars := mux.Vars(r)
	username := vars["username"]

	var user User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		ResponseError(w, encoder, http.StatusNotFound, "No such user")
		return
	}

	data := UserGet{
		CurrentUsersPage: false,
		Username:         username,
	}

	current_username, err := GetCurrentUser(r)

	if err == nil {
		data.CurrentUsersPage = username == current_username
	}

	encoder.Encode(data)
}
