package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Chat struct {
	ID           int
	Participants [2]int
}

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
		rows, err := db.Query("SELECT * FROM chats WHERE $1 = user1 OR $1 = user2", val.(int))
		if err != nil {
			http.Error(w, fmt.Sprintf("Query Error: %s", err), http.StatusNotFound)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var chat Chat
			if err := rows.Scan(&chat.ID, &chat.Participants[0], &chat.Participants[1]); err != nil {
				http.Error(w, fmt.Sprintf("Parse Error: %s", err), http.StatusConflict)
				return
			}
			data.Chats = append(data.Chats, chat)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}
