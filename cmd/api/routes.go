package main

import (
	"expvar"
	"net/http"

	_ "example.com/gofurther/docs"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
	httpSwagger "github.com/swaggo/http-swagger"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

	router.HandlerFunc(http.MethodGet, "/v1/movies", app.requirePermission("movies:read", app.listMoviesHandler))
	router.HandlerFunc(http.MethodPost, "/v1/movies", app.requirePermission("movies:write", app.createMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.requirePermission("movies:read", app.showMovieHandler))
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.requirePermission("movies:write", app.updateMovieHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.requirePermission("movies:write", app.deleteMovieHandler))

	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
	// PUT over POST for idempotency
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler)

	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)

	router.Handler(http.MethodGet, "/debug/vars", expvar.Handler())

	router.HandlerFunc(http.MethodGet, "/docs/*any", httpSwagger.WrapHandler)

	// NOTE CORS is wrapping rate-limit (early in the chain) because otherwise
	// client would be blocked due to same-origin policy rather than 429 Too Many Request
	// (because rateLimit returns first without CORS set)
	// return app.metrics(app.recoverPanic(app.enableCORS(app.rateLimit(app.authenticate(router)))))
	chain := alice.New(app.metrics, app.recoverPanic, app.enableCORS, app.rateLimit, app.authenticate)

	return chain.Then(router)
}
