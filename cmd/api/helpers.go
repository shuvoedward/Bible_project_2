package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

type envelope map[string]any

func (app *application) getPassageFilters(r *http.Request) (*data.PassageFilters, error) {
	params := httprouter.ParamsFromContext(r.Context())
	book := params.ByName("book")

	if _, exists := app.books[book]; !exists {
		return nil, data.ErrRecordNotFound
	}

	chapter, err := strconv.Atoi(params.ByName("chapter"))
	if err != nil {
		return nil, errors.New("invalid chapter parameter")
	}
	if chapter < 1 || chapter > 150 {
		return nil, data.ErrRecordNotFound
	}

	var singleVs, svs, evs int

	query := r.URL.Query()

	switch {
	case query.Has("vs"):
		singleVs, err = strconv.Atoi(query.Get("vs"))
		if err != nil {
			return nil, errors.New("invalid verse parameter")
		}
		if singleVs < 1 || singleVs > 176 {
			return nil, data.ErrRecordNotFound
		}
		fmt.Print(singleVs)

	case query.Has("svs") && query.Has("evs"):
		svs, err = strconv.Atoi(query.Get("svs"))
		if err != nil {
			return nil, errors.New("invalid start verse parameter")
		}
		if svs < 1 || svs > 176 {
			return nil, data.ErrRecordNotFound
		}

		evs, err = strconv.Atoi(query.Get("evs"))
		if err != nil {
			return nil, errors.New("invalid end verse parameter")
		}
		if evs < 1 || evs > 176 {
			return nil, data.ErrRecordNotFound
		}
	}

	return &data.PassageFilters{
		Book:       book,
		Chapter:    chapter,
		Verse:      singleVs,
		StartVerse: svs,
		EndVerse:   evs,
	}, nil
}

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {

	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err = w.Write(js); err != nil {
		return err
	}

	return nil
}
