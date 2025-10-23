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

func (app *application) grammarCheckHanlder(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Text string `json:"text"`
	}

	err := app.readJSON(r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

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
		app.serverErrorResponse(w, r, err)
		return
	}

	var result GrammarResponse

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	fmt.Println(result)

	err = app.writeJSON(w, http.StatusOK, envelope{"matches": result}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
