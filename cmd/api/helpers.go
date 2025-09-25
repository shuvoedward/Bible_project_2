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

func (app *application) getLocationFilters(r *http.Request) (*data.LocationFilters, error) {
	params := httprouter.ParamsFromContext(r.Context())
	book := params.ByName("book")

	chapter, err := strconv.Atoi(params.ByName("chapter"))
	if err != nil {
		return nil, errors.New("invalid chapter parameter")
	}

	var svs, evs int

	query := r.URL.Query()

	switch {
	case query.Has("svs") && query.Has("evs"):
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

func (app *application) readIDParam(r *http.Request) (*int64, error) {
	param := httprouter.ParamsFromContext(r.Context())

	id, err := strconv.ParseInt(param.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return nil, err
	}
	return &id, nil
}
