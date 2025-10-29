package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

type envelope map[string]any

func (app *application) getLocationFilters(r *http.Request) (*data.LocationFilters, error) {
	params := httprouter.ParamsFromContext(r.Context())
	book := params.ByName("book")

	chapter, err := strconv.Atoi(params.ByName("chapter"))
	if err != nil {
		return nil, errors.New("invalid chapter parameter")
	}

	var svs, evs int

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

func (app *application) backgournd(fn func()) {
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

func (app *application) parseNoteQuery(r *http.Request) (*data.NoteQueryParams, error) {
	// get note_type, page, page_size
	var validationErrors []string

	query := r.URL.Query()

	noteType := query.Get("type")

	// check noteType
	if noteType != data.NoteTypeGeneral && noteType != data.NoteTypeBible {
		validationErrors = append(validationErrors, "note type must be 'GENERAL' or 'BIBLE'")
	}

	sort := query.Get("sort")
	if sort == "" {
		validationErrors = append(validationErrors, "sort can not be empty")
	}

	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page < 1 {
		validationErrors = append(validationErrors, "page must be at least 1")
	}

	pageSize, err := strconv.Atoi(query.Get("page_size"))
	if err != nil || pageSize < 1 {
		validationErrors = append(validationErrors, "page_size must be at least 1")
	}

	if len(validationErrors) > 0 {
		return nil, errors.New(strings.Join(validationErrors, "; "))
	}

	filters := data.Filters{
		Page:     page,
		PageSize: pageSize,
		Sort:     sort,
	}

	return &data.NoteQueryParams{
		Filters:  filters,
		NoteType: noteType,
	}, nil
}

