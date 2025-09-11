package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func (app *application) readJSON(r *http.Request, dst any) error {
	err := json.NewDecoder(r.Body).Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		// syntax error
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		// type error
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		// empty body
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}
	}

	return nil
}
