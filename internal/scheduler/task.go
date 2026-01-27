package scheduler

import "time"

const (
	SendActivationEmail      = "send-activation-email"
	SendPasswordResetEmail   = "send-password-reset-email"
	SendTokenActivatoinEmail = "send-token-activation-email"
)

type Task struct {
	ID         string
	Type       string
	Data       any
	Retries    int
	MaxRetries int
	ExecuteAt  time.Time
	CreatedAt  time.Time
}

type TaskEmailData struct {
	UserName      string
	Email         string
	ActivationURL string
}

type TaskPasswordResetEmail struct {
	Email            string
	PasswordResetURL string
}

type TaskTokenActivationData struct {
	Email         string
	ActivationURL string
}
