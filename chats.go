package main

import (
	"encoding/json"
	"net/http"
)

type ChatsGet struct {
	Chats []Chat
}

func chatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		return
	}

	var data ChatsGet

	currentUser, err := GetCurrentUser(r)
	var user User
	db.Where("username = ?", currentUser).First(&user)

	if err == nil {
		var chats []Chat
		db.Where("? IN users", user.ID).Find(&chats)
		data.Chats = append(data.Chats, chats...)
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}
