package httputils

import (
	"encoding/json"
	"log"
	"net/http"
	"tush00nka/bbbab_messenger/api/response"
)

func ResponseError(w http.ResponseWriter, errorCode int, errorMessage string) {
	ResponseJSON(w, errorCode, response.ErrorResponse{
		Message: errorMessage,
	})
}

func ResponseJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
