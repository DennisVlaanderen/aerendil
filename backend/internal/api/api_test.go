package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func TestIsProductionEnvironment(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"production", true},
		{"Production", true},
		{"PRODUCTION", true},
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

func TestJWTSecretFromEnvironmentUsesConfiguredSecret(t *testing.T) {
	t.Setenv("AERENDIL_JWT_SECRET", "a-real-secret")
	t.Setenv("AERENDIL_ENV", "")

	if got := jwtSecretFromEnvironment(); got != "a-real-secret" {
		t.Fatalf("expected configured secret, got %q", got)
	}
}

func TestJWTSecretFromEnvironmentFallsBackInDevelopment(t *testing.T) {
	t.Setenv("AERENDIL_JWT_SECRET", "")
	t.Setenv("AERENDIL_ENV", "")

	if got := jwtSecretFromEnvironment(); got != devJWTSecret {
		t.Fatalf("expected dev fallback secret, got %q", got)
	}
}

func TestHandleErrorsHidesInternalDetailsForUnrecognizedErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := handleErrors(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("boom: /some/internal/path")
	})
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "boom") || strings.Contains(rec.Body.String(), "/some/internal/path") {
		t.Fatalf("expected internal error details to not appear in response, got %s", rec.Body.String())
	}
}

func TestHandleErrorsMapsUsernameTakenTo409(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := handleErrors(func(w http.ResponseWriter, r *http.Request) error {
		return store.ErrUsernameTaken
	})
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "username is already taken") {
		t.Fatalf("expected a clear username-taken message, got %s", rec.Body.String())
	}
}

func TestHandleErrorsWritesAPIErrorStatusVerbatim(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := handleErrors(func(w http.ResponseWriter, r *http.Request) error {
		return conflict("a custom conflict message")
	})
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "a custom conflict message") {
		t.Fatalf("expected the apiError's own message, got %s", rec.Body.String())
	}
}

func TestHandleErrorsWritesNothingOnNilError(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := handleErrors(func(w http.ResponseWriter, r *http.Request) error {
		return respond(w, http.StatusTeapot, map[string]string{"ok": "true"})
	})
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected the handler's own response to pass through untouched, got %d", rec.Code)
	}
}

func TestMeHandlerReturnsResolvedPrincipal(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"permissions"`) || !strings.Contains(rec.Body.String(), `"isAdmin"`) {
		t.Fatalf("expected resolved principal shape in response, got %s", rec.Body.String())
	}
}

// TestMeHandlerResolvesEnvironmentNamesWithoutEnvironmentsReadPermission
// guards against a regression where the UI's environment selector/flag
// scoping silently broke for any user without environments:read -- that
// permission gates the admin configuration surface (POST/PUT/DELETE
// /api/environments, and GET for the full list), not a user's ability to
// learn the *names* of environments their own groups already grant them.
func TestMeHandlerResolvesEnvironmentNamesWithoutEnvironmentsReadPermission(t *testing.T) {
	mux := newTestMux(t)
	envID := seedEnvironmentForTest(t, "Staging")
	token := tokenForWithEnvironments(t, []string{envID}, auth.PermFlagsRead, auth.PermFlagsWrite)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Environments []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"environments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Environments) != 1 || payload.Environments[0].ID != envID || payload.Environments[0].Name != "Staging" {
		t.Fatalf("expected the granted environment resolved with its name, got %+v", payload.Environments)
	}
}

func TestMeHandlerRequiresAuthentication(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d", rec.Code)
	}
}
