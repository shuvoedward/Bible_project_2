package main

import (
	"context"
	"log/slog"
	"shuvoedward/Bible_project/internal/data"
	"time"
)

type DeleteExpiredTokenInterface interface {
	DeleteExpiredTokens(ctx context.Context) (int64, error)
}
type backgroundTasks struct {
	deleteToken DeleteExpiredTokenInterface
	logger      *slog.Logger
}

func newBackgroundTasks(
	tokenModel data.TokenModel,
	logger *slog.Logger,
) *backgroundTasks {
	return &backgroundTasks{
		deleteToken: tokenModel,
		logger:      logger,
	}
}

func (bt *backgroundTasks) start() {
	defer func() {
		if pv := recover(); pv != nil {
			bt.logger.Error("background task panic", "panic", pv)
		}
	}()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		bt.cleanupExpiredTokens()
	}
}

func (bt *backgroundTasks) cleanupExpiredTokens() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := bt.deleteToken.DeleteExpiredTokens(ctx)

	if err != nil {
		bt.logger.Error("failed to cleanup tokens", "error", err)
	}
}
