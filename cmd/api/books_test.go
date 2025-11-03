package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"shuvoedward/Bible_project/internal/data"
	"testing"
)

func TestGetChapterOrVerses(t *testing.T) {
	testRouter := testApp.routes()

	rr := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/v1/bible/Genesis/1", nil)
	if err != nil {
		t.Fatal(err)
	}

	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := data.Passage{
		Book:    "Genesis",
		Chapter: 1,
		Verses: []data.VerseDetail{
			{Number: 1, Text: "this is a mock verse"},
		},
	}

	var actual struct {
		Passage data.Passage `json:"passage"`
	}
	err = json.Unmarshal(rr.Body.Bytes(), &actual)
	if err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}

	if actual.Passage.Book != expected.Book || actual.Passage.Chapter != expected.Chapter ||
		len(actual.Passage.Verses) != 1 {
		t.Errorf("handler returned unexpected body: got %+v want %+v", actual, expected)
	}

}

func TestAutoCompleteHandler_Book(t *testing.T) {
	testRouter := testApp.routes()

	rr := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/v1/autocomplete?q=joh", nil)
	if err != nil {
		t.Fatal(err)
	}

	testRouter.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v wants %v", status, http.StatusOK)
	}

}

func TestAutoCompleteHandler_Word(t *testing.T) {
	testRouter := testApp.routes()
	rr := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/v1/autocomplete?q=faith", nil)
	if err != nil {
		t.Fatal(err)
	}
	testRouter.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestAutoCompleteHandler_Verse(t *testing.T) {
	testRouter := testApp.routes()
	rr := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/v1/autocomplete?q=john 3:16", nil)
	if err != nil {
		t.Fatal(err)
	}
	testRouter.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestSearchHandler(t *testing.T) {
	testRouter := testApp.routes()
	rr := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/v1/search?q=love", nil)
	if err != nil {
		t.Fatal(err)
	}
	testRouter.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}
