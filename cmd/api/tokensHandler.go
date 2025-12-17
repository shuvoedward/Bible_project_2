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

type TokenServiceInterface interface {
	CreateActivationToken(email string) (string, string, *validator.Validator, error)
	CreateAuthToken(email string, password string) (string, *validator.Validator, error)
	CreatePasswordResetToken(email string) (string, string, *validator.Validator, error)
	GetUserForToken(tokenPlainText string) (*data.User, error)
}

type TokenHandler struct {
	app     *application
	service TokenServiceInterface
}

func NewTokenService(app *application, service TokenServiceInterface) *TokenHandler {
	return &TokenHandler{
		app:     app,
		service: service,
	}
}

func (h *TokenHandler) RegisterRoutes(router *httprouter.Router) {
	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", h.app.authRateLimit(h.CreateAuthenticationToken))

	router.HandlerFunc(http.MethodGet, "/v1/tokens/password-reset", h.app.authRateLimit(h.CreatePasswordResetToken))

	router.HandlerFunc(http.MethodGet, "/v1/tokens/activation", h.CreateActivationToken)
}

func (h *TokenHandler) handleTokenError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrEmailNotFound):
		h.app.invalidCredentialResponse(w, r)
	case errors.Is(err, service.ErrPasswordNotMatch):
		h.app.invalidCredentialResponse(w, r)
	case errors.Is(err, service.ErrUserActivated):
		h.app.invalidCredentialResponse(w, r)
	default:
		h.app.serverErrorResponse(w, r, err)
	}
}

// createAuthenticationTokenHandler authenticates a user and returns a token
// @Summary User login
// @Description Authenticate user with email and password, returns a token valid for 24 hours
// @Tags authentication
// @Accept json
// @Produce json
// @Param credentials body object{email=string,password=string} true "Login credentials" example({"email": "user@example.com", "password": "password123"})
// @Success 201 {object} object{authentication_token=data.Token} "Successfully authenticated"
// @Failure 400 {object} object{error=string} "Invalid request body"
// @Failure 401 {object} object{error=string} "Invalid credentials"
// @Failure 422 {object} object{error=map[string]string} "Validation failed"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /v1/tokens/authentication [post]
func (h *TokenHandler) CreateAuthenticationToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := h.app.readJSON(r, &input)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	tokenPlaintext, v, err := h.service.CreateAuthToken(input.Email, input.Password)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}
	if err != nil {
		h.handleTokenError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusCreated, envelope{"auth_token": tokenPlaintext}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}

// createPasswordResetTokenHandler generates a password reset token and sends reset email
// @Summary Request password reset
// @Description Generate a password reset token and send reset instructions via email. Token valid for 45 minutes.
// @Tags authentication
// @Accept json
// @Produce json
// @Param email body object{email=string} true "User email" example({"email": "user@example.com"})
// @Success 202 {object} object{message=string} "Password reset email will be sent"
// @Failure 400 {object} object{error=string} "Invalid request body"
// @Failure 422 {object} object{error=map[string]string} "Validation failed (email not found or account not activated)"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /v1/tokens/password-reset [get]
func (h *TokenHandler) CreatePasswordResetToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}

	err := h.app.readJSON(r, &input)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	email, tokenPlaintext, v, err := h.service.CreatePasswordResetToken(input.Email)

	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}
	if err != nil {
		h.handleTokenError(w, r, err)
		return
	}

	// TODO: change url based on production variable

	h.app.background(func() {
		data := map[string]any{
			"passwordResetURL": fmt.Sprintf("http://localhost:4000/v1/users/password/%s", tokenPlaintext),
		}

		err := h.app.mailer.Send(email, "token_password_reset.tmpl", data)
		if err != nil {
			h.app.logger.Error(err.Error())
		}
	})

	env := envelope{"message": "an email will be sent to you containing password reset instructions"}

	err = h.app.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}

// @Summary Request account activation
// @Description Generate a token to activate user's account and send activation instructions via email. Token valid for 3 days.
// @Tags authentication
// @Accept json
// @Produce json
// @Param email body object{email=string} true "User email" example({"email": "user@example.com"})
// @Success 202 {object} object{message=string} "Activation token will be sent"
// @Failure 400 {object} object{error=string} "Invalid request body"
// @Failure 422 {object} object{error=map[string]string} "Validation failed (email not found or account already activated)"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /v1/tokens/activation [get]
func (h *TokenHandler) CreateActivationToken(w http.ResponseWriter, r *http.Request) {
	// currently uses get route instead of post for lack of frontend
	var input struct {
		Email string `json:"email"`
	}

	err := h.app.readJSON(r, &input)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	tokenPlaintext, email, v, err := h.service.CreateActivationToken(input.Email)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}

	h.app.background(func() {
		data := map[string]any{
			"activationURL": fmt.Sprintf("http://localhost:4000/v1/users/activated/%s", tokenPlaintext),
		}

		err := h.app.mailer.Send(email, "token_activation.tmpl", data)
		if err != nil {
			h.app.logger.Error(err.Error())
		}
	})

	env := envelope{"message": "an email will be sent to you containing activation instructions"}

	err = h.app.writeJSON(w, http.StatusAccepted, env, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}
