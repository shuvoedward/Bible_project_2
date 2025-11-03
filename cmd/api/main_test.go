package main

import (
	"log/slog"
	"os"
	"shuvoedward/Bible_project/internal/data"
	"testing"
)

type mockPassageModel struct{}

func (m *mockPassageModel) Get(filters *data.LocationFilters) (*data.Passage, error) {
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
func (m *mockPassageModel) AutocompleteWord(word string) ([]string, error) {
	return nil, nil
}
func (m *mockPassageModel) SuggestWords(word string) ([]*data.WordMatch, error) {
	if word == "faith" {
		return []*data.WordMatch{
			{Word: "faith", Frequency: 1},
		}, nil
	}
	return nil, nil
}
func (m *mockPassageModel) SuggestVerses(phrase string) ([]*data.VerseMatch, error) {
	if phrase == "john 3:16" {
		return []*data.VerseMatch{
			{Book: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world..."},
		}, nil
	}
	return nil, nil
}
func (m *mockPassageModel) SearchVersesByWord(params data.SearchQueryParams) ([]*data.VerseMatch, error) {
	if params.Word == "love" {
		return []*data.VerseMatch{
			{Book: "1 John", Chapter: 4, Verse: 8, Text: "Whoever does not love does not know God, because God is love."},
		}, nil
	}
	return nil, nil
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

	testApp = &application{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
		books:  books,
		booksSearchIndex: booksSearchIndex,
		models: data.Models{
			Passages: &mockPassageModel{},
		},
	}
	os.Exit(m.Run())

}
