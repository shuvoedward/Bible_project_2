package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/service"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

type envelope map[string]any

// getLocationFilters extracts and parses Bible location parameters from the request.
// It retrieves the book name and chapter from URL path parameters, and optionally
// extracts start verse (svs) and end verse (evs) from query parameters.
// Returns a LocationFilters struct or an error if parsing fails.
func (app *application) getLocationFilters(r *http.Request) (*data.LocationFilters, error) {
	params := httprouter.ParamsFromContext(r.Context())
	book := params.ByName("book")

	chapter, err := strconv.Atoi(params.ByName("chapter"))
	if err != nil {
		return nil, errors.New("invalid chapter parameter")
	}

	svs, evs := -1, -1

	query := r.URL.Query()

	if query.Has("svs") && query.Has("evs") {
		svs, err = strconv.Atoi(query.Get("svs"))
		if err != nil {
			return nil, errors.New("invalid start verse parameter")
		}

		evs, err = strconv.Atoi(query.Get("evs"))
		if err != nil {
			return nil, errors.New("invalid end verse parameter")
		}
	}

	return &data.LocationFilters{
		Book:       book,
		Chapter:    chapter,
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
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
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

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single json value")
	}

	return nil
}

func (app *application) background(fn func()) {
	app.wg.Go(func() {
		defer func() {
			pv := recover()
			if pv != nil {
				app.logger.Error(fmt.Sprintf("%v", pv))
			}
		}()

		fn()
	})
}

func (app *application) readIDParam(r *http.Request, idName string) (int64, error) {
	param := httprouter.ParamsFromContext(r.Context())

	id, err := strconv.ParseInt(param.ByName(idName), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}
	return id, nil
}

func (app *application) parseNoteQuery(r *http.Request) (service.ListNotesInput, error) {
	query := r.URL.Query()

	noteType := query.Get("type")
	if noteType == "" {
		return service.ListNotesInput{}, errors.New("note_type is required")
	}

	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page < 1 {
		page = 1 // default
	}

	pageSize, err := strconv.Atoi(query.Get("page_size"))
	if err != nil || pageSize < 1 {
		pageSize = 10 // default
	}

	sort := query.Get("sort")
	if sort == "" {
		sort = "-created_at" // default
	}

	return service.ListNotesInput{
		NoteType: noteType,
		Page:     page,
		PageSize: pageSize,
		Sort:     sort,
	}, nil
}

func (app *application) readPaginationParams(r *http.Request) (data.Filters, error) {
	query := r.URL.Query()

	page := 1 // default
	if query.Has("page") {
		var err error
		page, err = strconv.Atoi(query.Get("page"))
		if err != nil || page < 1 {
			return data.Filters{}, errors.New("page must be a positive integer")
		}
	}

	pageSize := 10 // default
	if query.Has("page_size") {
		var err error
		pageSize, err = strconv.Atoi(query.Get("page_size"))
		if err != nil || pageSize < 1 || pageSize > 100 {
			return data.Filters{}, errors.New("page_size must be between 1 and 100")
		}
	}

	return data.Filters{Page: page, PageSize: pageSize}, nil
}
