package mailer

import "fmt"

type ErrorCode string

const (
	ErrCodeTemplateMissing   ErrorCode = "TEMPLATE_MISSING"
	ErrCodeTemplateExecution ErrorCode = "TEMPLATE_EXECUTION"
	ErrCodeNetworkFailure    ErrorCode = "NETWORK_FAILURE"
	ErrCodeAuthFailure       ErrorCode = "AUTH_FAILURE"
	ErrCodeRateLimited       ErrorCode = "RATE_LIMITED"
	ErrCodeInvalidRicipient  ErrorCode = "INVALID_RECIPIENT"
)

type MailerError struct {
	Code       ErrorCode
	Message    string
	Retrieable bool
	Err        error          // original error
	Metadata   map[string]any // extra context
}

func (e *MailerError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *MailerError) Unwarp() error { return e.Err }

func NewTemplateError(template string, err error) *MailerError {
	return &MailerError{
		Code:       ErrCodeTemplateExecution,
		Message:    fmt.Sprintf("failed to execute template %s", template),
		Retrieable: false,
		Err:        err,
		Metadata:   map[string]any{"template": template},
	}
}

func NewNetworkError(op string, err error) *MailerError {
	return &MailerError{
		Code:       ErrCodeNetworkFailure,
		Message:    fmt.Sprintf("network failure during %s", op),
		Retrieable: true,
		Err:        err,
		Metadata:   map[string]any{"operation": op},
	}
}
