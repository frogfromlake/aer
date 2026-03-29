package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/frogfromlake/aer/pkg/logger"
	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/handler"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func main() {
	// 1. Setup Context FIRST so backoff respects interrupts
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 2. Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}

	// 3. Initialize Logger
	logger.Init(cfg.Environment, cfg.LogLevel)
	slog.Info("Bootstrapping AĒR BFF API...", "environment", cfg.Environment)

	// 4. Initialize Storage (Passing Context for Backoff)
	chAddr := cfg.ClickHouseHost + ":" + cfg.ClickHousePort
	chStore, err := storage.NewClickHouseStorage(
		ctx,
		chAddr,
		cfg.ClickHouseUser,
		cfg.ClickHousePassword,
		cfg.ClickHouseDB,
	)
	if err != nil {
		slog.Error("Failed to connect to ClickHouse", "error", err)
		os.Exit(1)
	}
	slog.Info("ClickHouse connected successfully")

	// 5. Setup Handlers and Router
	serverLogic := handler.NewServer(chStore)
	strictHandler := handler.NewStrictHandler(serverLogic, nil)

	r := chi.NewRouter()
	handler.HandlerFromMuxWithBaseURL(strictHandler, r, "/api/v1")

	// --- GRACEFUL SHUTDOWN LOGIC ---
	server := &http.Server{
		Addr:    ":" + cfg.BFFPort,
		Handler: r,
	}

	// Start server in a separate goroutine
	go func() {
		slog.Info("AĒR BFF API listening", "port", cfg.BFFPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server crashed", "error", err)
			os.Exit(1)
		}
	}()

	// Block main thread until a signal is received
	<-ctx.Done()
	slog.Info("Shutdown signal received. Shutting down BFF API gracefully...")

	// Allow up to 5 seconds for active requests to finish
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Forced server shutdown", "error", err)
	} else {
		slog.Info("BFF API stopped cleanly.")
	}
}
