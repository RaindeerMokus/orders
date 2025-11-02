package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/v2/internal/api"
	"example.com/v2/internal/api/server"
	"example.com/v2/internal/event"
	"example.com/v2/internal/middlewares"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

//go:generate go tool oapi-codegen -o ../../internal/api/gen.go -config ../../oapi-cfg.yaml ../../api/openapi-spec/openapi.yaml

func main() {
	// Load config from environment variables or use defaults
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@localhost:5432/ordersdb?sslmode=disable"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Setup logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal().Msgf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal().Msgf("Database ping failed: %v", err)
	}
	log.Info().Msgf("Connected to PostgreSQL")

	// Create event queue and worker
	eventQueue := make(chan event.OrderCreated, 100)
	eventWorker := event.NewEventWorker(eventQueue, log.Logger)

	// Context for graceful shutdown and event worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background event processing worker
	eventWorker.StartEventWorker(ctx)

	// Create server instance
	srv := server.NewServer(db, eventQueue, log.Logger)

	// Setup Gin and routes using generated router binder
	router := gin.Default()
	router.Use(middlewares.LoggerMiddleware(log.Logger))

	// Register routes with handler
	api.RegisterHandlers(router, srv)

	// HTTP server
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Channel to listen for OS signals for shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Starting server on port %s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Msgf("Could not listen on %s: %v\n", port, err)
		}
	}()

	// Wait for SIGINT/SIGTERM
	<-stop
	log.Info().Msg("Shutdown signal received")

	// Shutdown server with timeout
	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelTimeout()

	if err := httpServer.Shutdown(ctxTimeout); err != nil {
		log.Fatal().Msgf("Server forced to shutdown: %v", err)
	}

	// Cancel event worker context to stop background processing
	cancel()

	log.Info().Msgf("Server exiting gracefully")
}
