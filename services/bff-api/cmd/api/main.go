package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/frogfromlake/aer/pkg/logger"
	"github.com/frogfromlake/aer/services/bff-api/internal/api"
	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func main() {
	// 1. Load configuration specific to the BFF service
	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}

	// 2. Initialize the shared structured logger
	logger.Init(cfg.Environment, cfg.LogLevel)
	slog.Info("Bootstrapping AĒR BFF API...", "environment", cfg.Environment)

	// 3. Initialize ClickHouse Storage Adapter (Port 9002 for native protocol)
	chStore, err := storage.NewClickHouseStorage(
		"localhost:9002",
		cfg.ClickHouseUser,
		cfg.ClickHousePassword,
		cfg.ClickHouseDB,
	)
	if err != nil {
		slog.Error("Failed to connect to ClickHouse", "error", err)
		os.Exit(1)
	}
	slog.Info("ClickHouse connected successfully")

	// 4. Initialize BFF Server Logic
	server := api.NewServer(chStore)

	// 5. Wrap with the generated strict handler
	strictHandler := api.NewStrictHandler(server, nil)

	// 6. Setup Router and mount the handler
	r := chi.NewRouter()
	api.HandlerFromMuxWithBaseURL(strictHandler, r, "/api/v1")

	// 7. Start the HTTP server
	slog.Info("AĒR BFF API listening", "port", 8080)
	if err := http.ListenAndServe(":8080", r); err != nil {
		slog.Error("Server crashed", "error", err)
		os.Exit(1)
	}
}
