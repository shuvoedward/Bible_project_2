package service

import "shuvoedward/Bible_project/internal/validator"

// BibleValidator validates Bible references (books, chapters, verses, offsets)
// Shared across Notes and Highlights
type BibleValidator struct {
	books map[string]struct{} // Valid Bible books
}

func NewBibleValidator(books map[string]struct{}) *BibleValidator {
	return &BibleValidator{books: books}
}

// ValidateBibleLocation validates a Bible verse location
// This is reusable for Notes, Highlights, or any feature referencing Bible verses
func (bv *BibleValidator) ValidateBibleLocation(
	v *validator.Validator,
	book string,
	chapter, startVerse, endVerse int,
	startOffset, endOffset int,
) {
	// Validate book
	v.Check(book != "", "book", "must be provided")
	if book != "" {
		_, exists := bv.books[book]
		v.Check(exists, "book", "must be a valid Bible book")
	}

	// Validate chapter
	v.Check(chapter > 0 && chapter <= 150, "chapter", "must be between 1 and 150")

	// Validate verses
	v.Check(startVerse > 0 && startVerse <= 176, "start_verse", "must be between 1 and 176")
	v.Check(endVerse > 0 && endVerse <= 176, "end_verse", "must be between 1 and 176")
	v.Check(startVerse <= endVerse, "verse", "start verse must be less than or equal to end verse")

	// Validate offsets
	v.Check(startOffset >= 0, "start_offset", "cannot be negative")
	v.Check(endOffset >= 0, "end_offset", "cannot be negative")

	// For single verse, offsets must be in order
	if startVerse == endVerse && endOffset != 0 {
		v.Check(startOffset < endOffset, "offset", "start offset must be less than end offset")
	}
}
