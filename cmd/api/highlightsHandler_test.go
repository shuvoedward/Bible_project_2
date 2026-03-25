package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
	"testing"
)

type mockHighlightService struct{}

func (s *mockHighlightService) DeleteHighlight(
	ctx context.Context,
	highlightID int64,
	userID int64,
) (*validator.Validator, error) {
	return nil, nil
}

func (s *mockHighlightService) InsertHighlight(
	ctx context.Context,
	highlight *data.Highlight,
	userID int64,
) (*validator.Validator, error) {
	return nil, nil
}

func (s *mockHighlightService) UpdateHighlight(
	ctx context.Context,
	highlightID int64,
	userID int64,
	color string,
) (*validator.Validator, error) {
	return nil, nil
}

func TestHighlightHandler_Insert(t *testing.T) {
	handler := NewHighlightHandler(testApp, &mockHighlightService{})

	tests := []struct {
		name           string
		body           map[string]any
		user           *data.User
		expectedStatus int
	}{
		{
			name: "valid insert",
			body: map[string]any{
				"book":    "Genesis",
				"chapter": 1,
				"verse":   1,
				"color":   "#FFFF00",
			},
			user:           &data.User{ID: 1, Activated: true},
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			r := httptest.NewRequest(http.MethodPost, "/v1/highlights", bytes.NewReader(body))
			r.Header.Set("Content-TYpe", "application/json")

			r = testApp.contextSetUser(r, tt.user)

			w := httptest.NewRecorder()

			handler.Insert(w, r)

			if w.Code != tt.expectedStatus {
				t.Errorf("got %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}

}
