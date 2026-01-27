package service

import (
	"errors"
	"fmt"
	"log/slog"
	"shuvoedward/Bible_project/internal/cache"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/scheduler"
	"shuvoedward/Bible_project/internal/validator"
	"strconv"
	"strings"
	"time"
)

type TokenService struct {
	tokenModel data.TokenModel
	userModel  data.UserModel
	redis      *cache.RedisClient
	scheduler  *scheduler.Scheduler
	logger     *slog.Logger
}

func NewTokenService(
	tokenModel data.TokenModel,
	userModel data.UserModel,
	redis *cache.RedisClient,
	scheduler *scheduler.Scheduler,
	logger *slog.Logger) *TokenService {
	return &TokenService{
		tokenModel: tokenModel,
		userModel:  userModel,
		redis:      redis,
		logger:     logger,
	}
}

// CreatePasswordResetToken validates email and creates password reset token for the user by their email
// Returns the user email, plaintext token, validation and error
func (s *TokenService) CreatePasswordResetToken(email string) (*validator.Validator, error) {
	v := validator.New()

	validateEmail(v, email)
	if !v.Valid() {
		return v, nil
	}

	user, err := s.userModel.GetByEmail(email)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrEmailNotFound
		}
		return nil, err
	}

	if !user.Activated {
		return nil, ErrUserNotActivated
	}

	token, err := s.tokenModel.New(user.ID, 45*time.Minute, data.ScopePasswordReset)
	if err != nil {
		return nil, err
	}

	task := scheduler.Task{
		Type: scheduler.SendPasswordResetEmail,
		Data: scheduler.TaskPasswordResetEmail{
			Email:            user.Email,
			PasswordResetURL: fmt.Sprintf("http://localhost:4000/v1/users/password/%s", token.Plaintext),
		},
		MaxRetries: 1,
		CreatedAt:  time.Now(),
	}

	s.scheduler.Submit(task)

	return nil, nil
}

// CreateAuthToken validates user email and password and creates authentication token
// Returns token plain text and validation and error
func (s *TokenService) CreateAuthToken(email, password string) (string, *validator.Validator, error) {
	v := validator.New()
	validateEmail(v, email)
	validatePassword(v, password)
	if !v.Valid() {
		return "", v, nil
	}

	user, err := s.userModel.GetByEmail(email)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return "", nil, ErrEmailNotFound
		}
		return "", nil, err
	}

	match, err := user.Password.Matches(password)
	if err != nil {
		return "", nil, err
	}

	if !match {
		return "", nil, ErrPasswordNotMatch
	}

	token, err := s.tokenModel.New(user.ID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		return "", nil, err
	}

	fmt.Println(token.Plaintext)
	fmt.Println(user.ID)
	err = s.redis.SetToken(token.Plaintext, user.ID, user.Activated)
	if err != nil {
		s.logger.Error(err.Error())
	}

	return token.Plaintext, nil, nil
}

// CreateActivationToken validates email and creates activation token. Also validates if user exists
// Returns token plaintext, user's email, validatoin and error
func (s *TokenService) CreateActivationToken(email string) (*validator.Validator, error) {
	v := validator.New()
	validateEmail(v, email)
	if !v.Valid() {
		return nil, ErrEmailNotFound
	}

	user, err := s.userModel.GetByEmail(email)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrEmailNotFound
		}
		return nil, err
	}

	if user.Activated {
		// v.AddError("email", "user has already been activated")
		return nil, ErrUserActivated
	}

	token, err := s.tokenModel.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		return nil, err
	}

	task := scheduler.Task{
		Type:       scheduler.SendTokenActivatoinEmail,
		MaxRetries: 3,
		Data: scheduler.TaskTokenActivationData{
			Email:         user.Email,
			ActivationURL: fmt.Sprintf("http://localhost:4000/v1/users/activated/%s", token.Plaintext),
		},
		CreatedAt: time.Now(),
	}

	s.scheduler.Submit(task)

	return nil, nil
}

func (s *TokenService) GetUserForToken(tokenPlainText string) (*data.User, error) {
	v := validator.New()
	v.Check(len(tokenPlainText) == 26, "token", "must be 26 bytes long")
	if !v.Valid() {
		return nil, ErrInvalidToken
	}

	userDataStr, err := s.redis.GetForToken(tokenPlainText)
	if err == nil {
		// cache hit
		// userId, activated
		// id:userID,activated:t
		tempUserData := strings.Split(userDataStr, ",")
		idStr, _ := strings.CutPrefix(tempUserData[0], "id:")
		activatedStr, _ := strings.CutPrefix(tempUserData[1], "activated:")

		id, _ := strconv.ParseInt(idStr, 10, 64)
		activated, _ := strconv.ParseBool(activatedStr)

		return &data.User{
			ID:        id,
			Activated: activated,
		}, nil
	}

	user, err := s.userModel.GetForToken(tokenPlainText, data.ScopeAuthentication)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	err = s.redis.SetToken(tokenPlainText, user.ID, user.Activated)
	if err != nil {
		s.logger.Error("failed to cache token", "error", err)
	}

	return user, nil
}
