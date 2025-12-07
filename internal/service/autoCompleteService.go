package service

import (
	"shuvoedward/Bible_project/internal/data"
	"strings"
)

type AutocompleteService struct {
	passageModel     data.PassageModel
	booksSearchIndex map[string][]string // Index for fast book look ups
}

func NewAutocompleteService(
	passageModel data.PassageModel,
	booksSearchIndex map[string][]string,
) *AutocompleteService {
	return &AutocompleteService{
		passageModel:     passageModel,
		booksSearchIndex: booksSearchIndex,
	}
}

// AutocompleteResult represents autocomplete search results
type AutocompleteResult struct {
	Type   string             // "book", "word", or "verse"
	Books  []string           // For book searches
	Words  []*data.WordMatch  // For word searches
	Verses []*data.VerseMatch // For verse searches
}

func (s *AutocompleteService) Autocomplete(query string) (*AutocompleteResult, error) {
	if query == "" {
		return nil, ErrEmptyQuery
	}

	// Identify search type
	searchType := s.identifySearchType(query, s.booksSearchIndex)
	if searchType == nil {
		// Query too short or invalid
		return &AutocompleteResult{Type: "none"}, nil
	}

	result := &AutocompleteResult{Type: searchType.Type}
	switch searchType.Type {
	case "book":
		result.Books = searchType.Suggestions

	case "word":
		words, err := s.passageModel.SuggestWords(searchType.Query)
		if err != nil {
			return nil, err
		}
		result.Words = words

	case "verse":
		verses, err := s.passageModel.SuggestVerses(searchType.Query)
		if err != nil {
			return nil, err
		}
		result.Verses = verses
	}

	return result, nil
}

// searchTypeResult represents the result of an auto-complete search operation.
// Type indicates the search category: "book", "word", or "verse"
// Suggestion contains matched book names for book searches.
// Query contains the normalized search term for word and verse searches.
// Internal search type result
type searchTypeResult struct {
	Suggestions []string
	Query       string
	Type        string
}

// identifySearchType determines the search type and returns appropriate autocomplete suggestions.
// It handles three search types:
//
//  1. Book search: Matches book names or prefixes
//     Examples: "mat" → ["Matthew"], "1 cor" → ["1 Corinthians", "2 Corinthians"]
//
//  2. Word search: Single words with 3+ characters (avoids common words like "in", "at")
//     Examples: "faith" → word search, "love" → word search
//
//  3. Verse search: Multi-word queries (book + chapter/verse reference)
//     Examples: "john 3:16" → verse search, "matthew 5" → verse search
//
// The index map is structured with:
//   - Full/partial book names as keys → matching books as values
//   - Number prefixes ("1", "2", "3") → all numbered books as values
//
// Returns nil if the query doesn't match any search pattern.
func (s *AutocompleteService) identifySearchType(query string, index map[string][]string) *searchTypeResult {
	normalized := strings.ToLower(strings.TrimSpace(query))

	// 1. Direct book match: check if the normalized query exists in index
	// Handles: "matthew", "mat", "ma", "jo", etc.
	if books, exists := index[normalized]; exists {
		return &searchTypeResult{Suggestions: books, Type: "book"}
	}

	// 2. Numbered books: special handling for "1 cor", "2 tim", etc.
	if books := s.findNumberedBook(normalized); books != nil {
		return &searchTypeResult{Suggestions: books, Type: "book"}
	}

	// 3. Word search: single word, minimum 3 characters
	// Avoids DB calls for common short words (in, at, so) that appear in nearly every verse
	if !strings.Contains(normalized, " ") && len(normalized) >= 3 {
		return &searchTypeResult{
			Query: normalized,
			Type:  "word",
		}
	}

	// 4. Verse search: multi-word query indicates book + reference
	// Examples: "john 3", "matthew 5:10", "romans 8:28"
	if strings.Contains(normalized, " ") {
		return &searchTypeResult{
			Query: normalized,
			Type:  "verse",
		}
	}

	// No match: query too short or doesn't fit any pattern
	return nil
}

// findNumberedBook handles auto-complete for numbered books like "1 Corinthians", "2 Timothy", "3 John"
// It attempts three matching strategies:
// 1. Direct match for 3 char: "1cor" : looks up "cor" in the index
// 2. Direct match for 2 char: "1co" : looks up "co" in the index
// 3. finally filterBooksByPrefix() match: "1c": looks up "c" for books starting with 1.
func (s *AutocompleteService) findNumberedBook(query string) []string {
	if !isNumberedBookPrefix(query[0]) {
		return nil
	}

	// Remove spaces and extract the book name portion after the number
	// Example: "1 c" -> "1c" -> "c"
	partialName := strings.ReplaceAll(query, " ", "")[1:]

	// Try exact match in index first
	// (e.g., "cor" → ["1 Corinthians", "2 Corinthians"],
	// "co" -> ["1 Corinthians", "2 Corinthians", "Colossians"])
	if books, exist := s.booksSearchIndex[partialName]; exist {
		return books
	}

	// fall back to filter by "1c"
	return s.filterBooksByPrefix(query)
}

// fiterBooksByPrefix filters numbered books by their partial name.
// Exmaple: "1c" -> filters book starting with "1" for those containing "C"
// Returns: ["1 Corinthians", "1 Chronicles"]
func (s *AutocompleteService) filterBooksByPrefix(query string) []string {

	bookNo := string(query[0])
	bookList := s.booksSearchIndex[bookNo]

	var suggestionList []string

	// Capitalize first letter for matching against book names
	normalizeBookName := strings.ToUpper(string(query[1]))

	for _, book := range bookList {
		// use byte instead of contains?
		if strings.Contains(book, normalizeBookName) {
			suggestionList = append(suggestionList, book)
		}
	}

	return suggestionList
}

// isNumberedBookPrefix used to identify numbered books like "1 Corinthians", "2 Timothy", "3 John"
func isNumberedBookPrefix(b byte) bool {
	return b == '1' || b == '2' || b == '3'
}
