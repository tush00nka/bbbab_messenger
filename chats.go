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

	session, err := Store.Get(r, "test")
	if err != nil {
		http.Error(w, "Session Error", http.StatusInternalServerError)
		return
	}

	var data ChatsGet

	if val, ok := session.Values["currentUser"]; ok {
		var chats []Chat
		db.Where("? IN users", val.(uint)).Find(&chats)
		data.Chats = append(data.Chats, chats...)
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}
