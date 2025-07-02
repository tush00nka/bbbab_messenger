package main

type User struct {
	ID       uint
	Username string
	Password string
}

type Chat struct {
	ID    uint
	Users []User `gorm:"many_to_many:chat_users;"`
}

type Message struct {
	ID          uint
	Chat        Chat
	Sender      User
	MessageText string
}
