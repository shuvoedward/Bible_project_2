package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.RedirectFixedPath = false
	router.RedirectTrailingSlash = false

	router.HandlerFunc(http.MethodGet, "/v1/bible/:book/:chapter", app.getChapterOrVerses)

	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
	return router
}
