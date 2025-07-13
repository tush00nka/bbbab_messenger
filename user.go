package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type UserGet struct {
	CurrentUsersPage bool
	Username         string
}

// @Summary Get user
// @Description Get user
// @ID get-user
// @Produce  json
// @Success 200 {object} UserGet
// @Failure 404 {object} ErrorGet
// @Param username path string true "Username"
// @Router /user/{username} [get]
func usersHandler(w http.ResponseWriter, r *http.Request) {
	db := GetDB()
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)

	vars := mux.Vars(r)
	username := vars["username"]

	var user User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		ResponseError(w, encoder, http.StatusNotFound, "No such user")
		return
	}

	data := UserGet{
		CurrentUsersPage: false,
		Username:         username,
	}

	current_username, err := GetCurrentUser(r)

	if err == nil {
		data.CurrentUsersPage = username == current_username
	}

	encoder.Encode(data)
}

type SearchRequest struct {
	Prompt string
}

// @Summary Search for user
// @Description Search for user
// @ID user-search
// @Produce  json
// @Success 200 {object} []User
// @Failure 400 {object} ErrorGet
// @Param Prompt body SearchRequest true "Search Request"
// @Router /usersearch [post]
func userSearchHandler(w http.ResponseWriter, r *http.Request) {
	db := GetDB()
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	var search SearchRequest
	if err := decoder.Decode(&search); err != nil {
		ResponseError(w, encoder, http.StatusBadRequest, "Bad Request")
		return
	}

	var users []User
	db.Where("LOWER(username) LIKE ?", strings.ToLower(fmt.Sprint("%"+search.Prompt+"%"))).Find(&users)
	encoder.Encode(users)
}
