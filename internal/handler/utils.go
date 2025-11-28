package handler

import (
	"net/http"
	"tush00nka/bbbab_messenger/internal/pkg/httputils"
)

type PongResponse struct {
	message string
}

// Ping
// @Summary Пингануть свервер
// @Description Пинганиуть сервер
// @Tags system
// @Produce json
// @Success 200 {object} PongResponse
// @Failure 404
// @Router /ping [get]
func Ping(w http.ResponseWriter, r *http.Request) {
	httputils.ResponseJSON(w, 200, PongResponse{message: "Pong"})
}
