package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type UserGet struct {
	CurrentUsersPage bool
	Username         string
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	db := GetDB()
	session, err := Store.Get(r, "test")
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var user User
	db.First(&user, id)

	data := UserGet{
		CurrentUsersPage: false,
		Username:         user.Username,
	}

	if val, ok := session.Values["currentUser"]; ok {
		data.CurrentUsersPage = val.(int) == id
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}
