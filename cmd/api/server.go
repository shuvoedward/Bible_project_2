package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve(handlers *Handlers) error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(handlers),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  time.Minute,
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
	}

	shutdownError := make(chan error)

	// Start goroutine to listen for shutdown OS signals
	go func() {
		// Channel to receive OS signals
		quit := make(chan os.Signal, 1)

		// Listen for interrupt signals (Ctrl+C) and termination signals
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Block until signal is received
		s := <-quit

		app.logger.Info("shutting down server", "signal", s.String())

		// Give the server 30 seconds to finish processing existing requests
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
			return
		}

		app.logger.Info("completing background tasks", "addr", srv.Addr)

		// Wait for all background tasks started with app.background() to complete
		// app.wg is already initialized in main.go
		app.wg.Wait()
		shutdownError <- nil
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

	// Start server, this blocks until server is shut down
	err := srv.ListenAndServe()

	// if the error is not expected "server closed" error, return it
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Wait for shutdown to complete (or error)
	err = <-shutdownError
	if err != nil {
		return err
	}

	app.logger.Info("stopped server", "addr", srv.Addr)
	return nil
}
