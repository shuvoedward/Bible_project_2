package main

import (
	"expvar"
	"net/http"

	"github.com/julienschmidt/httprouter"
	httpSwagger "github.com/swaggo/http-swagger"
)

func (app *application) routes(handlers *Handlers) http.Handler {
	router := httprouter.New()

	router.RedirectFixedPath = false
	router.RedirectTrailingSlash = false

	handlers.Note.RegisterRoutes(router)
	handlers.User.RegisterRoutes(router)
	handlers.Token.RegisterRoutes(router)
	handlers.Book.RegisterRoutes(router)

	router.Handler(http.MethodGet, "/swagger/*any", httpSwagger.WrapHandler)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthCheckHandler)

	router.HandlerFunc(http.MethodPost, "/v1/grammar/check", app.requireActivatedUser(app.grammarCheckHandler))

	router.HandlerFunc(http.MethodPost, "/v1/notes/:id/images", app.requireActivatedUser(app.generalRateLimit(app.imageUploadHandler)))
	router.HandlerFunc(http.MethodDelete, "/v1/notes/:id/images/*s3_key", app.requireActivatedUser(app.generalRateLimit(app.imageDeleteHandler)))

	router.Handler(http.MethodGet, "/debug/vars", expvar.Handler())

	return app.metrics(app.recoverPanic(app.enableCORS(app.authenticate(router))))
}
