// @title BBBAB Messenger
// @version 0.1
// @description This is a sample server.

// @host localhost:8080
// @BasePath /
// @query.collection.format multi

package main

import (
	"log"

	_ "tush00nka/bbbab_messenger/docs"
	"tush00nka/bbbab_messenger/internal/app"
	"tush00nka/bbbab_messenger/internal/config"
)

const DSN string = "host=localhost user=bor password=bor dbname=bbbab sslmode=disable"

// var db *gorm.DB

// func GetDB() *gorm.DB {
// 	var err error

// 	if db == nil {
// 		db, err = gorm.Open(postgres.Open(DSN), &gorm.Config{})
// 		if err != nil {
// 			fmt.Fprintf(os.Stderr, "CONNECTION ERROR: %s", err)
// 			os.Exit(-1)
// 		}
// 	}

// 	return db
// }

// func RedirectLoggedIn(w http.ResponseWriter, r *http.Request, encoder *json.Encoder) bool {
// 	c, err := r.Cookie("token")
// 	if err != nil {
// 		if err == http.ErrNoCookie {
// 			// ResponseError(w, encoder, http.StatusUnauthorized, "No Token")
// 			return false
// 		}
// 		// ResponseError(w, encoder, http.StatusBadRequest, "Bad Request")
// 		return false
// 	}

// 	tokenStr := c.Value
// 	claims, err := ValidateToken(tokenStr)
// 	if err != nil {
// 		// ResponseError(w, encoder, http.StatusUnauthorized, "Invalid Token")
// 		return false
// 	}

// 	http.Redirect(w, r, fmt.Sprintf("/user/%d", claims.UserID), http.StatusFound)
// 	return true
// }

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	app.Run(cfg)
}

// func main() {
// 	router := mux.NewRouter()
// 	router.HandleFunc("/user/{userID}", usersHandler)

// 	fs := http.FileServer(http.Dir("static"))
// 	router.Handle("/", fs)

// 	router.HandleFunc("/login", loginHandler).Methods("POST", "GET")
// 	router.HandleFunc("/register", registerHandler).Methods("POST", "GET")
// 	// router.HandleFunc("/chats", chatsHandler).Methods("POST", "GET")
// 	router.HandleFunc("/usersearch", userSearchHandler).Methods("POST")
// 	router.HandleFunc("/sendmessage", messageHandler).Methods("POST")
// 	router.HandleFunc("/listmessages", listMessagesHandler).Methods("POST")
// 	router.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
// 		http.SetCookie(w, &http.Cookie{
// 			Name:    "token",
// 			Expires: time.Now().Add(-7 * 24 * time.Hour),
// 		})
// 		http.Redirect(w, r, "/login", http.StatusFound)
// 	})

// 	// Настройка Swagger
// 	swaggerHandler := httpSwagger.Handler(
// 		httpSwagger.URL("/swagger/doc.json"), // Важно: относительный путь
// 	)

// 	router.PathPrefix("/swagger/").Handler(swaggerHandler)

// 	// Явно обслуживаем doc.json
// 	router.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
// 		http.ServeFile(w, r, "./docs/swagger.json")
// 	})

// 	fmt.Println("Server is listening on 8080...")
// 	log.Fatal(http.ListenAndServe(":8080", router))
// }
