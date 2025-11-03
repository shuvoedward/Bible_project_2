package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type GrammarMatch struct {
	Message      string              `json:"message"`
	Offset       int                 `json:"offset"`
	Length       int                 `json:"length"`
	Replacements []map[string]string `json:"replacements"`
}

type GrammarResponse struct {
	Matches []GrammarMatch `json:"matches"`
}

// grammarCheckHandler checks text for grammar and spelling errors
// @Summary Check grammar and spelling
// @Description Check text for grammar, spelling, and style issues using LanguageTool API
// @Tags grammar
// @Accept json
// @Produce json
// @Param input body object{text=string} true "Text to check" example({"text": "This are a sample text with mistake."})
// @Success 200 {object} object{matches=GrammarResponse} "Grammar check results with suggestions"
// @Failure 400 {object} object{error=string} "Invalid request body"
// @Failure 500 {object} object{error=string} "Internal server error or LanguageTool API error"
// @Security ApiKeyAuth
// @Router /v1/grammar/check [post]
func (app *application) grammarCheckHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Text string `json:"text"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Send text to LanguageTool API for grammar checking
	// Language is hardcoded to en-US
	resp, err := http.PostForm(app.config.LanguageToolURL, url.Values{
		"text":     {input.Text},
		"language": {"en-US"},
	})

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("languageTool API returned status: %d", resp.StatusCode)
		app.serverErrorResponse(w, r, err)
		return
	}

	var result GrammarResponse

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"matches": result}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
