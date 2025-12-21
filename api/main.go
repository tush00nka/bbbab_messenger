// @title BBBAB Messenger
// @version 0.1
// @description This is a sample server.

// @host localhost:8080
// @BasePath /api
// @query.collection.format multi
// @schemes http

package main

import (
	"log"

	_ "tush00nka/bbbab_messenger/docs"
	"tush00nka/bbbab_messenger/internal/app"
	"tush00nka/bbbab_messenger/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	app.Run(cfg)
}
