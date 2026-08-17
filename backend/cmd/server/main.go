// Command server is Aerendil's entry point: it opens the Raft-backed flag
// store, then starts the FQDP TCP listener and the HTTP API against it.
package main

import (
	"context"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	flagStore, err := store.Open(storeConfigFromEnvironment())
	if err != nil {
		log.Fatalf("failed to open flag store: %v", err)
	}
	defer flagStore.Close()

	go func() {
		if err := fqdp.StartTCPServer(ctx, ":9000"); err != nil {
			log.Printf("fqdp server stopped: %v", err)
		}
	}()

	mux := http.NewServeMux()
	api.RegisterRoutes(mux, flagStore)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Println("http api listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	if err := auth.SeedAdminGroupAndUser(flagStore, adminConfigFromEnvironment()); err != nil {
		log.Fatalf("failed to seed admin account: %v", err)
	}

	<-ctx.Done()
	log.Println("shutdown requested")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
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
