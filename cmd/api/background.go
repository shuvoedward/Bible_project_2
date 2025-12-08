package main

import (
	"log/slog"
	"shuvoedward/Bible_project/internal/data"
	"time"
)

type backgroundTasks struct {
	tokenModel data.TokenModel
	logger     *slog.Logger
}

func newBackgroundTasks(
	tokenModel data.TokenModel,
	logger *slog.Logger,
) *backgroundTasks {
	return &backgroundTasks{
		tokenModel: tokenModel,
		logger:     logger,
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
	_, err := bt.tokenModel.DeleteExpiredToken()
	if err != nil {
		bt.logger.Error("failed to cleanup tokens", "error", err)
	}
}
