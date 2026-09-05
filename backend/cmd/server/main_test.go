package main

import (
	"net/http"
	"testing"
	"time"
)

func TestIsProductionEnvironment(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"production", true},
		{"Production", true},
		{"", false},
		{"development", false},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("AERENDIL_ENV", tc.value)
			if got := isProductionEnvironment(); got != tc.want {
				t.Fatalf("isProductionEnvironment() with AERENDIL_ENV=%q = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestAdminConfigFromEnvironmentUsesConfiguredCredentials(t *testing.T) {
	t.Setenv("AERENDIL_ENV", "")
	t.Setenv("AERENDIL_ADMIN_USERNAME", "root")
	t.Setenv("AERENDIL_ADMIN_PASSWORD", "hunter22")

	cfg := adminConfigFromEnvironment()
	if cfg.Username != "root" || cfg.Password != "hunter22" {
		t.Fatalf("expected configured admin credentials, got %+v", cfg)
	}
}

func TestAdminConfigFromEnvironmentFallsBackInDevelopment(t *testing.T) {
	t.Setenv("AERENDIL_ENV", "")
	t.Setenv("AERENDIL_ADMIN_USERNAME", "")
	t.Setenv("AERENDIL_ADMIN_PASSWORD", "")

	cfg := adminConfigFromEnvironment()
	if cfg.Username != "admin" || cfg.Password != "admin123" {
		t.Fatalf("expected default insecure admin credentials, got %+v", cfg)
	}
}

func TestNewAPIServerTimeouts(t *testing.T) {
	srv := newAPIServer(http.NewServeMux())
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want 5s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 10*time.Second {
		t.Fatalf("ReadTimeout = %v, want 10s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 10*time.Second {
		t.Fatalf("WriteTimeout = %v, want 10s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 120*time.Second {
		t.Fatalf("IdleTimeout = %v, want 120s", srv.IdleTimeout)
	}
}
