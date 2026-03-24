package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenHandler_CreateAuthToken(t *testing.T) {
	mockService := &mockTokenService{}

	handler := NewTokenHandler(testApp, mockService)

	type payload struct {
		Email    string
		Password string
	}

	tests := []struct {
		name           string
		payload        payload
		expectedStatus int
	}{
		{"valid test", payload{"test@email.com", "strong password"}, http.StatusCreated},
		{"invalid email", payload{"invalid-email", "invalid password"}, http.StatusUnauthorized},
		{"invalid password", payload{"valid-email", "invalid-password"}, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/tokens/authentication", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.CreateAuthenticationToken(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}
