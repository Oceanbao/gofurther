package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", app.config.port),
		Handler: app.routes(),
		// // Create a new log.Logger instance and in pass custom logger as target
		// // "" and 0 indicate that the log.Looger instance should not use a prefix
		// // or any flags.
		// // Now any log `http.Server` writes will be passed to our `Logger.Write()`
		// // that in turn will output a log entry in JSON format at ERROR level.
		// // `zerolog` lib for more powerful.
		// ErrorLog:     log.New(logger, "", 0),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	shutdownError := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)
		// Listen for SIGINT and SIGTERM and relay them to quit.
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		// Block until a signal above received.
		s := <-quit

		app.logger.PrintInfo("shuting down server", map[string]string{
			"signal": s.String(),
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// With background(), now only send on the shutdownError chan if it
		// returns an error.
		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
		}

		app.logger.PrintInfo("completing background tasks", map[string]string{
			"addr": srv.Addr,
		})

		app.wg.Wait()
		shutdownError <- nil
	}()

	app.logger.PrintInfo("starting server", map[string]string{
		"addr": srv.Addr,
		"env":  app.config.env,
	})

	// Shutdown() will cause ListenAdnServer() to immediately return a http.ErrServerClosed
	// that here we check if it is NOT that then return this other error.
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Otherwise, wait to receive a return value from Shutdown() and handle it.
	err = <-shutdownError
	if err != nil {
		return err
	}

	app.logger.PrintInfo("stopped server", map[string]string{
		"addr": srv.Addr,
	})

	return nil
}
