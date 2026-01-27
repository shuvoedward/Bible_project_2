package service

import (
	"errors"
	"fmt"
	"log/slog"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/scheduler"
	"shuvoedward/Bible_project/internal/validator"
	"time"
)

type UserService struct {
	userModel  data.UserModel
	tokenModel data.TokenModel
	scheduler  *scheduler.Scheduler
	logger     *slog.Logger
}

func NewUserService(userModel data.UserModel, tokenModel data.TokenModel, scheduler *scheduler.Scheduler, logger *slog.Logger) *UserService {
	return &UserService{
		userModel:  userModel,
		tokenModel: tokenModel,
		logger:     logger,
	}
}

// RegisterUser creates a new user account and token
// Returns inserted user, plaintext token, validation and error
func (s *UserService) RegisterUser(name, email, password string) (*data.User, *validator.Validator, error) {
	user := data.User{
		Name:      name,
		Email:     email,
		Activated: false,
	}

	v := validateUserRegistration(user, password)
	if !v.Valid() {
		return nil, v, nil
	}

	err := user.Password.Set(password)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	err = s.userModel.Insert(&user)
	if err != nil {
		s.logger.Error("failed to register user", "email", email, "error", err)
		if errors.Is(err, data.ErrDuplicateEmail) {
			return nil, nil, ErrDuplicateEmail
		}
		return nil, nil, err
	}

	token, err := s.tokenModel.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		s.logger.Error("failed to create activation token",
			"user_id", user.ID,
			"error", err)
		return nil, nil, err
	}

	task := scheduler.Task{
		Type: scheduler.SendActivationEmail,
		Data: scheduler.TaskEmailData{
			UserName:      user.Name,
			Email:         user.Email,
			ActivationURL: fmt.Sprintf("http://localhost:4000/v1/users/activated/%s", token.Plaintext),
		},
		MaxRetries: 3,
		CreatedAt:  time.Now(),
	}

	s.scheduler.Submit(task)

	return &user, nil, nil
}

// ActivateUser activates a user based on token
// Returns user data and error
func (s *UserService) ActivateUser(token string) (*data.User, error) {
	// validate token
	// Get user associated with the token
	// Also validates if user exists
	user, err := s.userModel.GetForToken(token, data.ScopeActivation)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	user.Activated = true

	err = s.userModel.Update(user)
	if err != nil {
		return nil, err
	}

	err = s.tokenModel.DeleteAllForUser(data.ScopeActivation, user.ID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// UpdatePassword validates and updates user password only. Deletes the password reset token for the user
// Returns validation error and error
func (s *UserService) UpdatePassword(tokenPlaintext, password string) (*validator.Validator, error) {
	v := validator.New()
	validatePassword(v, password)
	validateToken(v, tokenPlaintext)
	if !v.Valid() {
		return v, nil
	}

	user, err := s.userModel.GetForToken(data.ScopePasswordReset, tokenPlaintext)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	err = user.Password.Set(password)
	if err != nil {
		return nil, err
	}

	err = s.userModel.Update(user)
	if err != nil {
		return nil, err
	}

	err = s.tokenModel.DeleteAllForUser(data.ScopePasswordReset, user.ID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func validateUserRegistration(user data.User, password string) *validator.Validator {
	v := validator.New()

	v.Check(user.Name != "", "name", "must be provided")
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")

	validateEmail(v, user.Email)

	validatePassword(v, password)

	return v
}

func validateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email")
}

func validatePassword(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

func validateToken(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
}
