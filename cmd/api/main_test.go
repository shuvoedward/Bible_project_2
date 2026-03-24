package main

import (
	"context"
	"log/slog"
	"os"
	"shuvoedward/Bible_project/internal/cache"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/ratelimit"
	"shuvoedward/Bible_project/internal/service"
	"shuvoedward/Bible_project/internal/validator"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

type mockUserService struct{}

func (s *mockUserService) ActivateUser(ctx context.Context, token string) (*data.User, error) {
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

func (s *mockUserService) RegisterUser(ctx context.Context, name string, email string, password string) (*data.User, *validator.Validator, error) {
	if email == "duplicate@example.com" {
		return nil, nil, service.ErrDuplicateEmail
	}

	user := &data.User{
		ID:        1,
		Name:      name,
		Email:     email,
		Activated: false,
		CreatedAt: time.Now(),
	}

	return user, nil, nil
}

func (s *mockUserService) UpdatePassword(ctx context.Context, tokenPlaintext, password string) (*validator.Validator, error) {
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

func (s *mockTokenService) CreateActivationToken(ctx context.Context, email string) (*validator.Validator, error) {
	if email == "email-not-found" {
		return nil, service.ErrEmailNotFound
	}

	return nil, nil
}

func (s *mockTokenService) CreateAuthToken(ctx context.Context, email, password string) (string, *validator.Validator, error) {
	if email == "invalid-email" {
		return "", nil, service.ErrEmailNotFound
	}

	if password == "invalid-password" {
		return "", nil, service.ErrPasswordNotMatch
	}

	return "valid-token", nil, nil
}

func (s *mockTokenService) CreatePasswordResetToken(ctx context.Context, email string) (*validator.Validator, error) {
	if email == "invalid-email" {
		return nil, service.ErrEmailNotFound
	}

	return nil, nil
}

func (s *mockTokenService) GetUserForToken(ctx context.Context, tokenPlainText string) (*data.User, error) {
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

	// mockMailer, _ := mailer.NewMailer(
	// 	"smtp.example.com",
	// 	25,
	// 	"test",
	// 	"test",
	// 	"test@example.com",
	// )
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mr.Close()

	redisClient, err := cache.NewRedisClient(cache.RedisConfig{
		Host: mr.Host(),
		Port: mr.Port(),
	}, time.Minute)
	if err != nil {
		panic(err)
	}

	mockRateLimiter := ratelimit.NewLimiters(false, 100, 100, 100, redisClient, time.Minute)

	cfg := config{
		port: 4000,
		env:  "testing",
	}

	testApp = &application{
		config:      cfg,
		logger:      slog.New(slog.NewTextHandler(os.Stdout, nil)),
		rateLimiter: mockRateLimiter,
		wg:          &sync.WaitGroup{},
	}

	os.Exit(m.Run())
}
