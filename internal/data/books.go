package data

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Builds a map for fast book search
// Example:
// "1"    -> [1 John, 1 Corinthians... books starting with 1]
// "jo"   -> [1 John, John, 2 John, 3 John, Jonah] // all books starts with "jo" including numbered books.
// "joh"  -> [1 John, John, 2 John, 3 John, Jonah]
// "john" -> [1 John, John, 2 John, 3 John]
// "mat"  -> [Matthew]
func BuildBookSearchIndex(allBooks []string) map[string][]string {
	bookMap := map[string][]string{}

	for _, book := range allBooks {
		if strings.HasPrefix(book, "1") {
			bookMap["1"] = append(bookMap["1"], book)

			word := strings.TrimSpace(strings.TrimPrefix(book, "1"))
			word = strings.ToLower(word[:3])
			bookMap[word] = append(bookMap[word], book)
			bookMap[word[:2]] = append(bookMap[word[:2]], book)

		} else if strings.HasPrefix(book, "2") {
			bookMap["2"] = append(bookMap["2"], book)

			word := strings.TrimSpace(strings.TrimPrefix(book, "2"))
			word = strings.ToLower(word[:3])
			bookMap[word] = append(bookMap[word], book)
			bookMap[word[:2]] = append(bookMap[word[:2]], book)

		} else if strings.HasPrefix(book, "3") {
			bookMap["3"] = append(bookMap["3"], book)

			word := strings.TrimSpace(strings.TrimPrefix(book, "3"))
			word = strings.ToLower(word[:3])
			bookMap[word] = append(bookMap[word], book)

			bookMap[word[:2]] = append(bookMap[word[:2]], book)
		} else {
			word := strings.ToLower(book[:3])
			bookMap[word] = append(bookMap[word], book)
			bookMap[word[:2]] = append(bookMap[word[:2]], book)
			bookMap[strings.ToLower(book[:1]+book[1:])] = append(bookMap[book], book)
		}
	}

	return bookMap
}

type PassageModel interface {
	Get(filters *LocationFilters) (*Passage, error)
	SuggestWords(word string) ([]*WordMatch, error)
	SuggestVerses(phrase string) ([]*VerseMatch, error)
	SearchVersesByWord(params SearchQueryParams) ([]*VerseMatch, error)
}

type VerseDetail struct {
	ID     int64  `json:"verse_id"`
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// for whole chapter and few verses and for single verse
type Passage struct {
	Book    string        `json:"book"`
	Chapter int           `json:"chapter"`
	Verses  []VerseDetail `json:"verses"`
}
type WordMatch struct {
	Word      string
	Lexeme    string
	Frequency int
}
type VerseMatch struct {
	ID         int64
	Book       string
	Chapter    int
	Verse      int
	Text       string
	Rank       float64
	ExactMatch string
	Snippet    string
}
type SearchQueryParams struct {
	Filters
	Word string
}
type passageModel struct {
	DB *sql.DB
}

func NewPassageModel(db *sql.DB) *passageModel {
	return &passageModel{DB: db}
}

func (p passageModel) Get(filters *LocationFilters) (*Passage, error) {
	switch {
	case filters.StartVerse != 0 && filters.EndVerse != 0:
		return p.getVerseRange(filters)
	default:
		return p.getChapter(filters)
	}
}

func (p passageModel) getVerseRange(filters *LocationFilters) (*Passage, error) {
	query := `
			SELECT  
				v.verse, v.text, v.id
			FROM 
				verses as v
			JOIN 
				books AS b ON b.id = v.book_id
			WHERE
				b.name = $1 
				AND v.chapter = $2 
				AND v.verse BETWEEN $3 AND $4`

	return p.queryVerses(query, filters.Book, filters.Chapter, filters.StartVerse, filters.EndVerse)
}

func (p passageModel) getChapter(filters *LocationFilters) (*Passage, error) {
	query := `
			SELECT 
				v.verse, v.text, v.id
			FROM 
				verses as v
			JOIN 
				books As b ON v.book_id = b.id
			WHERE 
				b.name = $1 
				AND v.chapter = $2`

	return p.queryVerses(query, filters.Book, filters.Chapter)
}

func (p passageModel) queryVerses(query string, args ...any) (*Passage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := p.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	passage := &Passage{
		Book:    args[0].(string),
		Chapter: args[1].(int),
		Verses:  []VerseDetail{},
	}

	for rows.Next() {
		var verseDetail VerseDetail
		err := rows.Scan(&verseDetail.Number, &verseDetail.Text, &verseDetail.ID)
		if err != nil {
			return nil, err
		}
		passage.Verses = append(passage.Verses, verseDetail)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if len(passage.Verses) == 0 {
		return nil, ErrRecordNotFound
	}

	return passage, nil
}

func (p passageModel) SuggestWords(word string) ([]*WordMatch, error) {
	// find word
	query := `
		SELECT DISTINCT ON (lexeme) word, lexeme, frequency
        FROM bible_words
        WHERE word ILIKE $1 || '%'  -- Match on actual word, not stem
        ORDER BY lexeme, frequency DESC
        LIMIT 10
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := p.DB.QueryContext(ctx, query, word)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	words := []*WordMatch{}

	for rows.Next() {
		var word WordMatch

		err := rows.Scan(&word.Word, &word.Lexeme, &word.Frequency)
		if err != nil {
			return nil, err
		}

		words = append(words, &word)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return words, nil
}

func (p passageModel) SuggestVerses(phrase string) ([]*VerseMatch, error) {
	query := `
		SELECT 
			v.id, b.name, v.chapter, v.verse, v.text, 
 		   	ts_rank(search_vector, phraseto_tsquery('simple', lower($1))) AS rank,
			CASE WHEN text LIKE '%' || $1 || '%' THEN 1 ELSE 0 END AS exact_match, 
			ts_headline('simple', text, phraseto_tsquery('simple', $1)) AS snippet
		FROM 
			verses v
		JOIN 
			books b ON v.book_id = b.id
		WHERE 
			search_vector @@ phraseto_tsquery('simple', $1)
		ORDER BY 
			exact_match DESC, 
			rank DESC
		LIMIT 10;
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := p.DB.QueryContext(ctx, query, phrase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*VerseMatch

	for rows.Next() {
		var verse VerseMatch

		err := rows.Scan(
			&verse.ID,
			&verse.Book,
			&verse.Chapter,
			&verse.Verse,
			&verse.Text,
			&verse.Rank,
			&verse.ExactMatch,
			&verse.Snippet,
		)

		if err != nil {
			return nil, err
		}

		list = append(list, &verse)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return list, nil
}

func (p passageModel) SearchVersesByWord(params SearchQueryParams) ([]*VerseMatch, error) {
	query := `
		SELECT 
			v.id, b.name, v.chapter, v.verse, v.text, 
 		   	ts_rank(search_vector, websearch_to_tsquery('simple', lower($1))) AS rank,
			ts_headline('simple', text, websearch_to_tsquery('simple', $1)) AS snippet
		FROM 
			verses v
		JOIN 
			books b ON v.book_id = b.id
		WHERE 
			search_vector @@ websearch_to_tsquery('simple', $1)
		ORDER BY 
			rank DESC
		LIMIT $2
		OFFSET $3
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := p.DB.QueryContext(ctx, query, params.Word, params.limit(), params.offset())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*VerseMatch

	for rows.Next() {
		var verse VerseMatch

		err := rows.Scan(
			&verse.ID,
			&verse.Book,
			&verse.Chapter,
			&verse.Verse,
			&verse.Text,
			&verse.Rank,
			&verse.Snippet,
		)

		if err != nil {
			return nil, err
		}

		list = append(list, &verse)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return list, nil
}
