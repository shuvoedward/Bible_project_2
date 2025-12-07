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
	handlers.Image.RegisterRoutes(router)

	router.Handler(http.MethodGet, "/swagger/*any", httpSwagger.WrapHandler)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthCheckHandler)

	router.HandlerFunc(http.MethodPost, "/v1/grammar/check", app.requireActivatedUser(app.grammarCheckHandler))

	router.Handler(http.MethodGet, "/debug/vars", expvar.Handler())

	return app.metrics(app.recoverPanic(app.enableCORS(app.authenticate(router))))
}
