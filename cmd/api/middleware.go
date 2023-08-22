package main

import (
	"errors"
	"expvar"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"example.com/gofurther/internal/data"
	"example.com/gofurther/internal/validator"
	"github.com/felixge/httpsnoop"
	"github.com/tomasen/realip"
	"golang.org/x/time/rate"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// If panic, set "Connection: close" header on the response, triggering
				// http.server to close the connection after sending response.
				w.Header().Set("Connection", "close")
				// recover() returns type interface{}, so we use fmt.Errorf() to normalise
				// it into an error and call our serverErrorResponse() helper.
				// In turn, this will log the error using custom loger at ERROR level
				// and send the client a 500.
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// Global Rate Limit
// func (app *application) rateLimit(next http.Handler) http.Handler {
// 	// Any code here will run once upon wrapping http.Handler
// 	// r = 2, b = 4
// 	limiter := rate.NewLimiter(2, 4)

// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Any code here will run for every request.
// 		// Allow() is behind a mutex.
// 		if !limiter.Allow() {
// 			app.rateLimitExceededResponse(w, r)
// 			return
// 		}

// 		next.ServeHTTP(w, r)
// 	})
// }

// Per Client Rate Limit
func (app *application) rateLimit(next http.Handler) http.Handler {
	// Any code here will run once upon wrapping http.Handler

	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// Fire a background goroutine to remove old entries from client map
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Any code here will run for every request.
		if app.config.limiter.enabled {
			// Use realip to solve behind proxy.
			ip := realip.FromRequest(r)

			// ip, _, err := net.SplitHostPort(r.RemoteAddr)
			// if err != nil {
			// 	app.serverErrorResponse(w, r, err)
			// 	return
			// }

			mu.Lock()

			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
				}
			}

			clients[ip].lastSeen = time.Now()

			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}

			// Note this is not `defer` because that would mean the mutex
			// not unlocked untill all hanlers downstream of this middleware have
			// returned
			mu.Unlock()

		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This indicates to any caches that the response may vary based on the value of
		// the Authorization header in the request.
		w.Header().Add("Vary", "Authorization")

		authorizationHeader := r.Header.Get("Authorization")

		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymouseUser)
			next.ServeHTTP(w, r)
			return
		}

		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		token := headerParts[1]

		v := validator.New()

		if data.ValidateTokenPlaintext(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}

		r = app.contextSetUser(r, user)

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAuthenicatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// NOTE input and output different from others as HandlerFunc. This makes
// it possible to wrap /v1/movie** handler functions directly with this
// middleware, saving any conversions.
func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	// Rather than returning this we assign it to the variable fn
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})

	// Wrap fn with the requireAuthenicatedUser() middleware so that it auto-call
	// requireAuthenicatedUser() middleware BEFORE being executed ITSELF.
	// This makes sense as we should not be checking if a user is activated
	// unless we know who they are.
	return app.requireAuthenicatedUser(fn)
}

func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		permissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		if !permissions.Include(code) {
			app.notPermittedResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	}

	return app.requireActivatedUser(fn)
}

func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// NOTE: This is generally critical: whenever response varies per request
		// header "Vary" must be set for that key to warn cache system.
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")

		origin := r.Header.Get("Origin")

		if origin != "" {
			for i := range app.config.cors.trustedOrigins {
				if origin == app.config.cors.trustedOrigins[i] {
					w.Header().Set("Access-Control-Allow-Origin", origin)

					// Check for "preflight"
					if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
						// Set the necessary preflight response headers
						w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
						w.WriteHeader(http.StatusOK)
						return
					}
					break
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) metrics(next http.Handler) http.Handler {
	totalRequestsReceived := expvar.NewInt("total_requests_received")
	totalResponsesSent := expvar.NewInt("total_responses_sent")
	totalProcessingTimeMicroseconds := expvar.NewInt("total_processing_time_microseconds")
	totalResponsesSentByStatus := expvar.NewMap("total_responses_sent_by_status")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		totalRequestsReceived.Add(1)

		metrics := httpsnoop.CaptureMetrics(next, w, r)

		// On the way back up the middleware chain, incr the number of responses
		// sent by 1.
		totalResponsesSent.Add(1)

		totalProcessingTimeMicroseconds.Add(metrics.Duration.Microseconds())

		totalResponsesSentByStatus.Add(strconv.Itoa(metrics.Code), 1)
	})
}
