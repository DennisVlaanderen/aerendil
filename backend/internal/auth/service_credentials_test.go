package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"

	"aerendil/backend/internal/store"
)

// seedCredential creates an ApplicationCredential with a bcrypt-hashed
// secret and returns both the stored credential and the plaintext secret
// (which, like a password, is never persisted -- only the caller's local
// copy from seeding it here can be used to authenticate).
func seedCredential(t *testing.T, s *store.Store, envID string, scopes []string, active bool) (store.ApplicationCredential, string) {
	t.Helper()
	secret := "test-client-secret"
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash client secret: %v", err)
	}
	cred, err := s.ApplicationCredentials().Set(store.ApplicationCredential{
		ID:               store.NewID(),
		Name:             "billing-service",
		ClientSecretHash: hash,
		EnvironmentID:    envID,
		Scopes:           scopes,
		Active:           active,
	})
	if err != nil {
		t.Fatalf("seed application credential: %v", err)
	}
	return cred, secret
}

func TestAuthenticateClientCredentialsSucceeds(t *testing.T) {
	service, s := newTestService(t, DefaultAdminConfig())
	env, err := s.Environments().Set(store.Environment{ID: store.NewID(), Name: "Production"})
	if err != nil {
		t.Fatalf("seed environment: %v", err)
	}
	cred, secret := seedCredential(t, s, env.ID, []string{PermFlagsRead}, true)

	got, err := service.AuthenticateClientCredentials(cred.ID, secret)
	if err != nil {
		t.Fatalf("expected client-credentials auth to succeed: %v", err)
	}
	if got.ID != cred.ID {
		t.Fatalf("expected credential id %q, got %q", cred.ID, got.ID)
	}
}

func TestAuthenticateClientCredentialsFailsForWrongSecret(t *testing.T) {
	service, s := newTestService(t, DefaultAdminConfig())
	env, err := s.Environments().Set(store.Environment{ID: store.NewID(), Name: "Production"})
	if err != nil {
		t.Fatalf("seed environment: %v", err)
	}
	cred, _ := seedCredential(t, s, env.ID, []string{PermFlagsRead}, true)

	if _, err := service.AuthenticateClientCredentials(cred.ID, "wrong-secret"); err == nil {
		t.Fatal("expected wrong client secret to be rejected")
	}
}

func TestAuthenticateClientCredentialsFailsForUnknownClientID(t *testing.T) {
	service, _ := newTestService(t, DefaultAdminConfig())

	if _, err := service.AuthenticateClientCredentials("unknown-client-id", "whatever"); err == nil {
		t.Fatal("expected unknown client_id to be rejected")
	}
}

func TestAuthenticateClientCredentialsFailsForInactiveCredential(t *testing.T) {
	service, s := newTestService(t, DefaultAdminConfig())
	env, err := s.Environments().Set(store.Environment{ID: store.NewID(), Name: "Production"})
	if err != nil {
		t.Fatalf("seed environment: %v", err)
	}
	cred, secret := seedCredential(t, s, env.ID, []string{PermFlagsRead}, false)

	if _, err := service.AuthenticateClientCredentials(cred.ID, secret); err == nil {
		t.Fatal("expected inactive credential to be rejected")
	}
}

// TestGenerateServiceTokenRoundTripsThroughAuthenticateToken is the core
// "reuse the existing auth flow" guarantee: a service token resolves
// through the exact same AuthenticateToken entrypoint human tokens do,
// producing a principal whose permissions/environment access come from the
// credential's own Scopes/EnvironmentID rather than group membership, and
// which is never an admin.
func TestGenerateServiceTokenRoundTripsThroughAuthenticateToken(t *testing.T) {
	service, s := newTestService(t, DefaultAdminConfig())
	env, err := s.Environments().Set(store.Environment{ID: store.NewID(), Name: "Production"})
	if err != nil {
		t.Fatalf("seed environment: %v", err)
	}
	cred, _ := seedCredential(t, s, env.ID, []string{PermFlagsRead, PermFlagsWrite}, true)

	token, err := service.GenerateServiceToken(cred)
	if err != nil {
		t.Fatalf("expected service token generation to succeed: %v", err)
	}

	user, perms, envs, isAdmin, isService, err := service.AuthenticateToken(token)
	if err != nil {
		t.Fatalf("expected AuthenticateToken to succeed for a service token: %v", err)
	}
	if user.ID != cred.ID {
		t.Fatalf("expected principal id %q, got %q", cred.ID, user.ID)
	}
	if isAdmin {
		t.Fatal("expected a service token to never resolve as admin")
	}
	if !isService {
		t.Fatal("expected a service token to resolve with isService=true")
	}
	if !perms.Has(PermFlagsRead) || !perms.Has(PermFlagsWrite) {
		t.Fatalf("expected resolved perms to match the credential's scopes, got %+v", perms)
	}
	if !envs.Has(env.ID) {
		t.Fatalf("expected resolved envs to include %q, got %+v", env.ID, envs)
	}
}

// TestAuthenticateTokenStillResolvesHumanTokens guards the backward-
// compatibility claim on the "typ" claim: a token with no "typ" at all
// (every token issued before this feature existed, and every token
// GenerateToken still issues today) must keep resolving as a human user.
func TestAuthenticateTokenStillResolvesHumanTokens(t *testing.T) {
	service, _ := newTestService(t, DefaultAdminConfig())

	user, err := service.Authenticate("admin", "admin123")
	if err != nil {
		t.Fatalf("expected admin login to succeed: %v", err)
	}
	token, err := service.GenerateToken(user)
	if err != nil {
		t.Fatalf("expected token generation to succeed: %v", err)
	}

	principal, _, _, isAdmin, isService, err := service.AuthenticateToken(token)
	if err != nil {
		t.Fatalf("expected AuthenticateToken to succeed for a human token: %v", err)
	}
	if principal.Username != "admin" || !isAdmin {
		t.Fatalf("expected the seeded admin to resolve as admin, got %+v isAdmin=%v", principal, isAdmin)
	}
	if isService {
		t.Fatal("expected a human token to resolve with isService=false")
	}
}

// TestAuthenticateTokenRejectsServiceTokenAfterCredentialDeactivation
// mirrors TestParseTokenRejectsAlreadyIssuedTokenAfterDeactivation's human
// equivalent: revocation (AC-7.4) must take effect on the very next
// request, without the server needing to track or blacklist the token
// itself -- the same live re-check design human accounts already use.
func TestAuthenticateTokenRejectsServiceTokenAfterCredentialDeactivation(t *testing.T) {
	service, s := newTestService(t, DefaultAdminConfig())
	env, err := s.Environments().Set(store.Environment{ID: store.NewID(), Name: "Production"})
	if err != nil {
		t.Fatalf("seed environment: %v", err)
	}
	cred, _ := seedCredential(t, s, env.ID, []string{PermFlagsRead}, true)

	token, err := service.GenerateServiceToken(cred)
	if err != nil {
		t.Fatalf("expected service token generation to succeed: %v", err)
	}

	cred.Active = false
	if _, err := s.ApplicationCredentials().Set(cred); err != nil {
		t.Fatalf("deactivate credential: %v", err)
	}

	if _, _, _, _, _, err := service.AuthenticateToken(token); err == nil {
		t.Fatal("expected an already-issued service token to stop working immediately after the credential is deactivated")
	}
}
