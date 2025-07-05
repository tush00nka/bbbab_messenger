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

func usersHandler(w http.ResponseWriter, r *http.Request) {
	db := GetDB()

	vars := mux.Vars(r)
	username := vars["username"]

	var user User
	db.Where("username = ?", username).First(&user)

	data := UserGet{
		CurrentUsersPage: false,
		Username:         username,
	}

	current_username, err := GetCurrentUser(r)

	if err == nil {
		data.CurrentUsersPage = username == current_username
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}
