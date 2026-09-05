// Command server is Aerendil's entry point: it opens the Raft-backed flag
// store, then starts the FQDP TCP listener and the HTTP API against it.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"aerendil/backend/internal/api"
	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/fqdp"
	"aerendil/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	flagStore, err := store.Open(storeConfigFromEnvironment())
	if err != nil {
		return fmt.Errorf("failed to open flag store: %w", err)
	}
	defer flagStore.Close()

	go func() {
		if err := fqdp.StartTCPServer(ctx, ":9000"); err != nil {
			log.Printf("fqdp server stopped: %v", err)
		}
	}()

	mux := http.NewServeMux()
	api.RegisterRoutes(mux, flagStore)

	srv := newAPIServer(mux)

	serverErr := make(chan error, 1)
	if err := auth.SeedAdminGroupAndUser(flagStore, adminConfigFromEnvironment()); err != nil {
		return fmt.Errorf("failed to seed admin account: %w", err)
	}

	go func() {
		log.Println("http api listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server error: %w", err)
	case <-ctx.Done():
		log.Println("shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}

	return nil
}


// newAPIServer builds the HTTP API server with explicit timeouts so slow or
// incomplete clients (e.g. Slowloris) cannot hold connections open forever.
// Values match common Go net/http guidance and can be tuned with traffic.
func newAPIServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func storeConfigFromEnvironment() store.Config {
	nodeID := strings.TrimSpace(os.Getenv("AERENDIL_RAFT_NODE_ID"))
	if nodeID == "" {
		nodeID = "node1"
	}

	bindAddr := strings.TrimSpace(os.Getenv("AERENDIL_RAFT_BIND_ADDR"))
	if bindAddr == "" {
		// A wildcard address isn't advertisable to raft peers on some hosts
		// (observed on Windows outside Docker); loopback is a safe default.
		bindAddr = "127.0.0.1:9100"
	}

	dataDir := strings.TrimSpace(os.Getenv("AERENDIL_RAFT_DATA_DIR"))
	if dataDir == "" {
		dataDir = "./data"
	}

	bootstrap := true
	if raw := strings.TrimSpace(os.Getenv("AERENDIL_RAFT_BOOTSTRAP")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			log.Fatalf("invalid AERENDIL_RAFT_BOOTSTRAP value %q: %v", raw, err)
		}
		bootstrap = parsed
	}

	return store.Config{
		NodeID:    nodeID,
		BindAddr:  bindAddr,
		DataDir:   dataDir,
		Bootstrap: bootstrap,
	}
}

func adminConfigFromEnvironment() auth.AdminConfig {
	defaults := auth.DefaultAdminConfig()

	username := strings.TrimSpace(os.Getenv("AERENDIL_ADMIN_USERNAME"))
	if username == "" {
		username = defaults.Username
	}

	password := os.Getenv("AERENDIL_ADMIN_PASSWORD")
	if strings.TrimSpace(password) == "" {
		if isProductionEnvironment() {
			log.Fatal("AERENDIL_ADMIN_PASSWORD must be set when AERENDIL_ENV=production")
		}
		log.Println("AERENDIL_ADMIN_PASSWORD not set; using insecure development default")
		password = defaults.Password
	}

	return auth.AdminConfig{Username: username, Password: password}
}

// isProductionEnvironment reports whether AERENDIL_ENV=production, which
// turns insecure-default fallbacks (JWT secret, admin password) into hard
// startup failures instead of warnings.
func isProductionEnvironment() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("AERENDIL_ENV")), "production")
}
