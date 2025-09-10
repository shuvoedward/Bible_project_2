package main

import (
	"log/slog"
	"os"
	"shuvoedward/Bible_project/internal/data"
	"testing"
)

type mockPassageModel struct{}

func (m *mockPassageModel) Get(filters *data.PassageFilters) (*data.Passage, error) {
	if filters.Book == "Genesis" && filters.Chapter == 1 {
		return &data.Passage{
			Book:    "Genesis",
			Chapter: 1,
			Verses: []data.VerseDetail{
				{Number: 1, Text: "this is a mock verse"},
			},
		}, nil
	}
	return nil, data.ErrRecordNotFound
}

var testApp *application

func TestMain(m *testing.M) {
	books := make(map[string]struct{}, 1)
	books["Genesis"] = struct{}{}

	testApp = &application{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
		books:  books,
		models: data.Models{
			Passages: &mockPassageModel{},
		},
	}
	os.Exit(m.Run())

}
