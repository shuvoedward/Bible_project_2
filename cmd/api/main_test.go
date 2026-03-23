package main

import (
	"log/slog"
	"os"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/mailer"
	"shuvoedward/Bible_project/internal/ratelimit"
	"shuvoedward/Bible_project/internal/service"
	"shuvoedward/Bible_project/internal/validator"
	"sync"
	"testing"
	"time"
)

type mockUserService struct{}

func (s *mockUserService) ActivateUser(token string) (*data.User, error) {
	if token == "invalid-token" {
		return nil, service.ErrTokenNotFound
	}

	return &data.User{
		ID:        1,
		Name:      "Test User",
		Email:     "test@example.com",
		Activated: true,
	}, nil
}

func (s *mockUserService) RegisterUser(name, email, password string) (*data.User, string, *validator.Validator, error) {
	if email == "duplicate@example.com" {
		return nil, "", nil, service.ErrDuplicateEmail
	}

	user := &data.User{
		ID:        1,
		Name:      name,
		Email:     email,
		Activated: false,
		CreatedAt: time.Now(),
	}

	return user, "mock-token-123", nil, nil
}

func (s *mockUserService) UpdatePassword(tokenPlaintext, password string) (*validator.Validator, error) {
	if tokenPlaintext == "invalid-token" {
		return nil, service.ErrTokenNotFound
	}

	if password == "short" {
		v := validator.New()
		v.AddError("password", "must be at least 8 bytes long")
		return v, nil
	}

	return nil, nil
}

type mockTokenService struct{}

func (s *mockTokenService) CreateActivationToken(email string) (string, string, *validator.Validator, error) {
	if email == "email-not-found" {
		return "", "", nil, service.ErrEmailNotFound
	}

	return "token-plaintext", "test@email.com", nil, nil
}

func (s *mockTokenService) CreateAuthToken(email, password string) (string, *validator.Validator, error) {
	if email == "invalid-email" {
		return "", nil, service.ErrEmailNotFound
	}

	if password == "invalid-password" {
		return "", nil, service.ErrPasswordNotMatch
	}

	return "valid-token", nil, nil
}

func (s *mockTokenService) CreatePasswordResetToken(email string) (string, string, *validator.Validator, error) {
	if email == "invalid-email" {
		return "", "", nil, service.ErrEmailNotFound
	}

	return "test@email.com", "valid-token", nil, nil
}

func (s *mockTokenService) GetUserForToken(tokenPlainText string) (*data.User, error) {
	if tokenPlainText == "invalid-token" {
		return nil, service.ErrInvalidToken
	}

	return &data.User{
		ID:        1,
		Activated: true,
	}, nil
}

var testApp *application

func TestMain(m *testing.M) {
	books := make(map[string]struct{}, 2)
	books["Genesis"] = struct{}{}
	books["John"] = struct{}{}

	booksSearchIndex := make(map[string][]string)
	booksSearchIndex["j"] = []string{"John"}
	booksSearchIndex["jo"] = []string{"John"}
	booksSearchIndex["joh"] = []string{"John"}
	booksSearchIndex["john"] = []string{"John"}

	mockMailer, _ := mailer.NewMailer(
		"smtp.example.com",
		25,
		"test",
		"test",
		"test@example.com",
	)

	mockRateLimiter := ratelimit.NewRateLimiters(100, 100, 100, time.Minute)

	cfg := config{
		port: 4000,
		env:  "testing",
	}

	testApp = &application{
		config:      cfg,
		logger:      slog.New(slog.NewTextHandler(os.Stdout, nil)),
		mailer:      mockMailer,
		rateLimiter: mockRateLimiter,
		wg:          &sync.WaitGroup{},
	}

	os.Exit(m.Run())
}
