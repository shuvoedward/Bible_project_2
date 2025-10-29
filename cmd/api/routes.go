package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.RedirectFixedPath = false
	router.RedirectTrailingSlash = false

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthCheckHandler)

	router.HandlerFunc(http.MethodGet, "/v1/bible/:book/:chapter", app.getChapterOrVerses)

	router.HandlerFunc(http.MethodGet, "/v1/autocomplete/bible", app.autoCompleteHandler)
	router.HandlerFunc(http.MethodGet, "/v1/search/bible", app.searchHandler)
	router.HandlerFunc(http.MethodGet, "/v1/search/notes", app.requireActivatedUser(app.searchNoteHandler))

	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
	router.HandlerFunc(http.MethodGet, "/v1/users/activated/:token", app.activateUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/password", app.updateUserPasswordHandler)

	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)
	router.HandlerFunc(http.MethodGet, "/v1/tokens/password-reset", app.createPasswordResetTokenHandler)

	router.HandlerFunc(http.MethodPost, "/v1/highlights", app.requireActivatedUser(app.insertHighlightHandler))
	router.HandlerFunc(http.MethodPatch, "/v1/highlights/:id", app.requireActivatedUser(app.updateHighlightHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/highlights/:id", app.requireActivatedUser(app.deleteHighlightHandler))

	router.HandlerFunc(http.MethodPost, "/v1/grammar/check", app.requireActivatedUser(app.grammarCheckHanlder))

	router.HandlerFunc(http.MethodGet, "/v1/notes", app.requireActivatedUser(app.listNotesHandler))
	router.HandlerFunc(http.MethodGet, "/v1/notes/:id", app.requireActivatedUser(app.getNoteHandler))
	router.HandlerFunc(http.MethodPost, "/v1/notes", app.requireActivatedUser(app.createNoteHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/notes/:id", app.requireActivatedUser(app.deleteNoteHandler))
	router.HandlerFunc(http.MethodPut, "/v1/notes/:id", app.requireActivatedUser(app.updateNoteHandler))
	router.HandlerFunc(http.MethodPost, "/v1/notes/:id/locations", app.requireActivatedUser(app.linkNoteHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/notes/:id/locations/:locationID", app.requireActivatedUser(app.deleteLinkHandler))

	router.HandlerFunc(http.MethodPost, "/v1/notes/:id/images", app.requireActivatedUser(app.imageUploadHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/notes/:id/images/*s3_key", app.requireActivatedUser(app.imageDeleteHandler))

	return app.authenticate(router)
}
