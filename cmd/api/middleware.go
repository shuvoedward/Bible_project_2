package main

import (
	"errors"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
	"strings"
)

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")

		authorizationHeader := r.Header.Get("Authorization")

		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		headerParts := strings.Fields(authorizationHeader)
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidAuthTokenResponse(w, r)
			return
		}

		token := headerParts[1]

		v := validator.New()

		v.Check(token != "", "token", "must be provided")
		v.Check(len(token) == 26, "token", "must be 26 bytes long")

		if !v.Valid() {
			app.invalidAuthTokenResponse(w, r)
			return
		}

		user, err := app.models.Users.GetForToken(token, data.ScopeAuthentication)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}

		r = app.contextSetUser(r, user)

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
