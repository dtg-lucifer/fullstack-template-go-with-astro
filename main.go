package main

import (
	"context"
	"flag"
	"os"

	"github.com/joho/godotenv"
	"github.com/your-username/go-mux-backend-template/server"
	"github.com/your-username/go-mux-backend-template/server/config"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	logger := pkg.NewLogger()
	defer logger.Close()

	// Load .env — non-fatal if missing (production uses real env vars)
	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, relying on environment variables")
	}

	if logger == nil {
		logger.Error("Failed to initialise logger, aborting")
		os.Exit(1)
	}

	cfg, err := config.NewConfig(*configPath)
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// WebFS() is provided by either fs_prod.go (!dev) or fs_dev.go (dev tag).
	// In production it serves from the embedded binary; in dev it reads from disk.
	srv := server.New(cfg, logger, WebFS())

	if err := srv.Setup(context.Background()); err != nil {
		logger.Error("Failed to set up server", "error", err)
		os.Exit(1)
	}

	// Start blocks until SIGINT/SIGTERM, then calls srv.Shutdown() internally
	srv.Start()
}
