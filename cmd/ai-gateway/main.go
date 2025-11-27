package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Parse command-line flags
	var (
		listenAddr = flag.String("listen", ":8080", "Server listen address")
		configFile = flag.String("config", "config/proxy-config.yaml", "Path to YAML configuration file")
		envFile    = flag.String("env-file", ".env", "Path to .env file (optional)")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	// Create logger
	logger := NewLogger(os.Stdout)

	// Load .env file if it exists
	if err := loadEnvFile(*envFile, logger); err != nil {
		log.Fatalf("Error loading environment file: %v", err)
	}

	// Load configuration
	logger.InfoMsg(fmt.Sprintf("Loading configuration from %s", *configFile))
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create server configuration
	serverCfg := &ServerConfig{
		ListenAddr: *listenAddr,
		Verbose:    *verbose,
	}

	// Create proxy server
	logger.InfoMsg("Initializing proxy server...")
	server, err := NewProxyServer(config, serverCfg, logger)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Start()
	}()

	// Setup signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for either server error or shutdown signal
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-stop:
		logger.InfoMsg(fmt.Sprintf("Received signal: %v", sig))

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Server shutdown error: %v", err)
		}

		logger.InfoMsg("Server stopped gracefully")
	}
}

// loadEnvFile loads environment variables from a .env file
func loadEnvFile(path string, logger *Logger) error {
	// Try to load .env file
	if err := godotenv.Load(path); err != nil {
		// Check if file doesn't exist
		if os.IsNotExist(err) {
			logger.InfoMsg(fmt.Sprintf(".env file not found at %s, using system environment variables", path))
			return nil
		}
		// Other errors are fatal
		return fmt.Errorf("failed to load .env file: %w", err)
	}

	logger.InfoMsg(fmt.Sprintf("Loaded environment variables from %s", path))
	return nil
}
