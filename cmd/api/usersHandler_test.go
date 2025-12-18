package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func TestUserHandler_Register(t *testing.T) {
	mockService := &mockUserService{}

	handler := NewUserHandler(testApp, mockService)

	type payload struct {
		Name     string
		Email    string
		Password string
	}

	tests := []struct {
		name           string
		payload        payload
		expectedStatus int
	}{
		{
			name: "valid registration",
			payload: payload{
				Name:     "John Doe",
				Email:    "john@example.com",
				Password: "password123",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "duplicate email",
			payload: payload{
				Name:     "Jane Doe",
				Email:    "duplicate@example.com",
				Password: "password123",
			},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.Register(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}

}

func TestUserHandler_Activate(t *testing.T) {
	mockService := &mockUserService{}

	handler := NewUserHandler(testApp, mockService)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{
			name:           "valid token",
			token:          "valid-token-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid-token",
			token:          "invalid-token",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/users/activated/"+tt.token, nil)

			// test is not calling the httprouter, but the handler uses httprouter.ParamsFromContext()
			// set up the context manually
			params := httprouter.Params{
				httprouter.Param{Key: "token", Value: tt.token},
			}
			ctx := context.WithValue(req.Context(), httprouter.ParamsKey, params)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			// internally creates rr.Body = &bytes.Buffer{}

			handler.Activated(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if rr.Code == http.StatusOK && rr.Body.Len() > 0 {
				var response map[string]any
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}

				user, ok := response["user"].(map[string]any)
				if !ok {
					t.Fatalf("user not found in response")
				}

				if !user["activated"].(bool) {
					t.Errorf("expected user to be activated")
				}
			}
		})
	}
}

func TestUserHandler_UpdatePassword(t *testing.T) {
	test := []struct {
		name           string
		payload        map[string]string
		expectedStatus int
	}{
		{
			name: "valid-token",
			payload: map[string]string{
				"password": "valid-password-123",
				"token":    "valid-token-123",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "invalid-password",
			payload: map[string]string{
				"password": "short",
				"token":    "invalid-token",
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	mockService := &mockUserService{}

	handler := NewUserHandler(testApp, mockService)

	for _, tt := range test {
		body, _ := json.Marshal(tt.payload)
		req := httptest.NewRequest(http.MethodPut, "/v1/users/password", bytes.NewReader(body))

		rr := httptest.NewRecorder()
		handler.UpdatePassword(rr, req)

		if rr.Code != tt.expectedStatus {
			t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
		}

	}
}
