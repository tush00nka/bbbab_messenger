package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"tush00nka/bbbab_messenger/internal/model"
	"tush00nka/bbbab_messenger/internal/pkg/auth"
	"tush00nka/bbbab_messenger/internal/pkg/httputils"
	"tush00nka/bbbab_messenger/internal/service"

	"github.com/gorilla/mux"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (c *UserHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/login", c.loginUser).Methods("POST", "OPTIONS")
	router.HandleFunc("/register", c.registerUser).Methods("POST", "OPTIONS")
	router.HandleFunc("/user/{id}", c.getUser).Methods("GET", "OPTIONS")
	router.HandleFunc("/search/{prompt}", c.searchUser).Methods("GET", "OPTIONS")
	// router.HandleFunc("/users/{id}", c.updateUser).Methods("PUT")
	// router.HandleFunc("/users/{id}", c.deleteUser).Methods("DELETE")
	// router.HandleFunc("/users", c.listUsers).Methods("GET")
}

type TokenResponse struct {
	Token string `json:"token"`
}

// @Summary Register
// @Description Register an account
// @ID register
// @Accept json
// @Produce json
// @Success 201 {object} TokenResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Param registerData body RegisterRequest true "Register data"
// @Router /register [post]
func (c *UserHandler) registerUser(w http.ResponseWriter, r *http.Request) {
	var request RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "Invalid request format")
		return
	}
	r.Body.Close()

	if request.Username == "" || request.Password == "" {
		httputils.ResponseError(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	if request.Password != request.ConfirmPassword {
		httputils.ResponseError(w, http.StatusBadRequest, "Passwords do not match")
		return
	}

	exists, err := c.userService.UsernameExists(request.Username)
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "Failed to check username availability")
		return
	}
	if exists {
		httputils.ResponseError(w, http.StatusConflict, fmt.Sprintf("User with username %s exists", request.Username))
		return
	}

	hash, err := auth.HashPassword(request.Password)
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "Failed to generate password hash")
		return
	}
	user := &model.User{Username: request.Username, Password: hash}
	if err = c.userService.CreateUser(user); err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	httputils.ResponseJSON(w, http.StatusCreated, TokenResponse{
		Token: token,
	})
}

type RegisterRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

// @Summary Login
// @Description Loing into account
// @ID login
// @Accept json
// @Produce json
// @Success 201 {object} TokenResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Param loginData body LoginRequest true "Login data"
// @Router /login [post]
func (h *UserHandler) loginUser(w http.ResponseWriter, r *http.Request) {
	var request LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httputils.ResponseError(w, http.StatusBadRequest, "Invalid request format")
		return
	}
	r.Body.Close()

	if request.Username == "" || request.Password == "" {
		httputils.ResponseError(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	// exists, err := h.userService.UsernameExists(request.Username)
	// if err != nil {
	// 	httputils.ResponseError(w, http.StatusInternalServerError, "Failed to check user existance")
	// 	return
	// }
	// if !exists {
	// 	httputils.ResponseError(w, http.StatusConflict, fmt.Sprintf("User %s does not exist", request.Username))
	// 	return
	// }

	user, err := h.userService.GetUserByUsername(request.Username)
	if err != nil {
		httputils.ResponseError(w, http.StatusConflict, fmt.Sprintf("User %s does not exist", request.Username))
		return
	}

	if !auth.CheckPasswordHash(request.Password, user.Password) {
		httputils.ResponseError(w, http.StatusConflict, "Wrong password")
		return
	}

	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	httputils.ResponseJSON(w, http.StatusCreated, TokenResponse{
		Token: token,
	})
}

type LoginRequest struct {
	Username string
	Password string
}

// @Summary Get user
// @Description Get user by id
// @ID get-user
// @Produce  json
// @Success 200 {object} model.User
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Param id path int true "User ID"
// @Router /user/{id} [get]
func (h *UserHandler) getUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["id"])
	if err != nil {
		httputils.ResponseError(w, http.StatusInternalServerError, "Failed to parse user ID")
		return
	}

	user, err := h.userService.GetUserByID(uint(userID))
	if err != nil {
		httputils.ResponseError(w, http.StatusNotFound, "No such user")
		return
	}

	// currentUserID, err := GetCurrentUser(r)
	//
	// if err == nil {
	// data.CurrentUsersPage = user.ID == currentUserID
	// }

	user.SanitizePassword()
	httputils.ResponseJSON(w, http.StatusOK, user)
}

// @Summary Search users
// @Description Search users by username
// @ID search-user
// @Produce  json
// @Success 200 {object} []model.User
// @Failure 404 {object} response.ErrorResponse
// @Param prompt path string true "Search Prompt"
// @Router /search/{prompt} [get]
func (h *UserHandler) searchUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	prompt := vars["prompt"]

	users, err := h.userService.SearchUsers(prompt)
	if err != nil {
		httputils.ResponseError(w, http.StatusNotFound, "failed to search for users")
		return
	}

	for _, user := range users {
		user.SanitizePassword()
	}

	httputils.ResponseJSON(w, http.StatusOK, users)
}
