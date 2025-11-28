package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"zliu.org/goutil/rest"
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

	// Load .env file if it exists
	if err := loadEnvFile(*envFile); err != nil {
		rest.Log().Fatal().Err(err).Msg("Error loading environment file")
	}

	// Load configuration
	rest.Log().Info().Msgf("Loading configuration from %s", *configFile)
	config, err := LoadConfig(*configFile)
	if err != nil {
		rest.Log().Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Create server configuration
	serverCfg := &ServerConfig{
		ListenAddr: *listenAddr,
		Verbose:    *verbose,
	}

	// Create proxy server
	rest.Log().Info().Msg("Initializing proxy server...")
	server, err := NewProxyServer(config, serverCfg)
	if err != nil {
		rest.Log().Fatal().Err(err).Msg("Failed to create proxy server")
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
			rest.Log().Fatal().Err(err).Msg("Server error")
		}
	case sig := <-stop:
		rest.Log().Info().Msgf("Received signal: %v", sig)

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		if err := server.Shutdown(ctx); err != nil {
			rest.Log().Fatal().Err(err).Msg("Server shutdown error")
		}

		rest.Log().Info().Msg("Server stopped gracefully")
	}
}

// loadEnvFile loads environment variables from a .env file
func loadEnvFile(path string) error {
	// Try to load .env file
	if err := godotenv.Load(path); err != nil {
		// Check if file doesn't exist
		if os.IsNotExist(err) {
			rest.Log().Info().Msgf(".env file not found at %s, using system environment variables", path)
			return nil
		}
		// Other errors are fatal
		return fmt.Errorf("failed to load .env file: %w", err)
	}

	rest.Log().Info().Msgf("Loaded environment variables from %s", path)
	return nil
}
