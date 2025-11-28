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

	router.Handler(http.MethodGet, "/swagger/*any", httpSwagger.WrapHandler)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthCheckHandler)

	router.HandlerFunc(http.MethodGet, "/v1/bible/:book/:chapter", app.generalRateLimit(app.getChapterOrVerses))

	router.HandlerFunc(http.MethodGet, "/v1/autocomplete/bible", app.generalRateLimit(app.autoCompleteHandler))
	router.HandlerFunc(http.MethodGet, "/v1/search/bible", app.generalRateLimit(app.searchHandler))

	router.HandlerFunc(http.MethodPost, "/v1/users", app.authRateLimit(app.registerUserHandler))
	router.HandlerFunc(http.MethodGet, "/v1/users/activated/:token", app.authRateLimit(app.activateUserHandler))
	router.HandlerFunc(http.MethodPut, "/v1/users/password", app.authRateLimit(app.updateUserPasswordHandler))

	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.authRateLimit(app.createAuthenticationTokenHandler))
	router.HandlerFunc(http.MethodGet, "/v1/tokens/password-reset", app.authRateLimit(app.createPasswordResetTokenHandler))
	router.HandlerFunc(http.MethodGet, "/v1/tokens/activation", app.createActivationTokenHandler)

	router.HandlerFunc(http.MethodPost, "/v1/highlights", app.requireActivatedUser(app.generalRateLimit(app.insertHighlightHandler)))
	router.HandlerFunc(http.MethodPatch, "/v1/highlights/:id", app.requireActivatedUser(app.generalRateLimit(app.updateHighlightHandler)))
	router.HandlerFunc(http.MethodDelete, "/v1/highlights/:id", app.requireActivatedUser(app.generalRateLimit(app.deleteHighlightHandler)))

	router.HandlerFunc(http.MethodPost, "/v1/grammar/check", app.requireActivatedUser(app.grammarCheckHandler))

	router.HandlerFunc(http.MethodPost, "/v1/notes/:id/images", app.requireActivatedUser(app.generalRateLimit(app.imageUploadHandler)))
	router.HandlerFunc(http.MethodDelete, "/v1/notes/:id/images/*s3_key", app.requireActivatedUser(app.generalRateLimit(app.imageDeleteHandler)))

	router.Handler(http.MethodGet, "/debug/vars", expvar.Handler())

	return app.metrics(app.recoverPanic(app.enableCORS(app.authenticate(router))))
}
