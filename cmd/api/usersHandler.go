package main

import (
	"errors"
	"fmt"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/service"
	"shuvoedward/Bible_project/internal/validator"

	"github.com/julienschmidt/httprouter"
)

type UserHandlerInterface interface {
	ActivateUser(token string) (*data.User, error)
	RegisterUser(name string, email string, password string) (*data.User, string, *validator.Validator, error)
	UpdatePassword(tokenPlaintext string, password string) (*validator.Validator, error)
}
type UserHandler struct {
	app     *application
	service UserHandlerInterface
}

func NewUserHandler(app *application, service UserHandlerInterface) *UserHandler {
	return &UserHandler{
		app:     app,
		service: service,
	}
}

func (h *UserHandler) RegisterRoutes(router *httprouter.Router) {
	router.HandlerFunc(http.MethodPost, "/v1/users", h.app.authRateLimit(h.Register))

	router.HandlerFunc(http.MethodGet, "/v1/users/activated/:token", h.app.authRateLimit(h.Activated))

	router.HandlerFunc(http.MethodPut, "/v1/users/password", h.app.authRateLimit(h.UpdatePassword))
}

func (h *UserHandler) handleUserError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrDuplicateEmail):
		h.app.editConflictResponse(w, r, err)
	case errors.Is(err, service.ErrTokenNotFound):
		h.app.notFoundResponse(w, r)
	default:
		h.app.serverErrorResponse(w, r, err)
	}
}

// @Summary Register a new user
// @Description Register a new user account with name, email, and password. Sends activation email.
// @Tags users
// @Accept json
// @Produce json
// @Param input body object{name=string,email=string,password=string} true "User registration details"
// @Success 201 {object} object{user=data.User} "User created successfully"
// @Failure 400 {object} object{error=string} "Invalid request payload"
// @Failure 422 {object} object{error=map[string]string} "Validation failed"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /v1/users [post]
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := h.app.readJSON(r, &input)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	user, tokenPlaintext, v, err := h.service.RegisterUser(input.Name, input.Email, input.Password)

	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}
	if err != nil {
		h.handleUserError(w, r, err)
		return
	}

	h.app.background(func() {
		data := map[string]any{
			"username":      user.Name,
			"activationURL": fmt.Sprintf("http://localhost:4000/v1/users/activated/%s", tokenPlaintext),
		}

		err = h.app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			h.app.logger.Error(err.Error())
		}
	})

	err = h.app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}

// @Summary Activate user account
// @Description Activate a user account using the activation token from email
// @Tags users
// @Accept json
// @Produce json
// @Param token path string true "Activation token"
// @Success 200 {object} object{user=data.User} "User activated successfully"
// @Failure 422 {object} object{error=map[string]string} "Invalid or expired token"
// @Failure 409 {object} object{error=string} "Edit conflict"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /v1/users/activated/{token} [get]
func (h *UserHandler) Activated(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	token := params.ByName("token")

	user, err := h.service.ActivateUser(token)
	if err != nil {
		h.handleUserError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}

// @Summary Update user password
// @Description Reset user password using a password reset token
// @Tags users
// @Accept json
// @Produce json
// @Param input body object{password=string,token=string} true "New password and reset token"
// @Success 200 {object} object{message=string} "Password reset successfully"
// @Failure 400 {object} object{error=string} "Invalid request payload"
// @Failure 422 {object} object{error=map[string]string} "Invalid or expired token"
// @Failure 409 {object} object{error=string} "Edit conflict"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /v1/users/password [put]
func (h *UserHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password       string `json:"password"`
		TokenPlaintext string `json:"token"`
	}

	err := h.app.readJSON(r, &input)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	v, err := h.service.UpdatePassword(input.TokenPlaintext, input.Password)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}
	if err != nil {
		h.handleUserError(w, r, err)
		return
	}

	env := envelope{"message": "your password was successfully reset"}

	err = h.app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}
