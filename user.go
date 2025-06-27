package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

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
