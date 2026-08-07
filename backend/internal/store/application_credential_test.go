package store

import (
	"errors"
	"testing"
)

func TestStoreSetAndGetApplicationCredential(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	applied, err := s.ApplicationCredentials().Set(ApplicationCredential{
		ID:               NewID(),
		Name:             "billing-service",
		ClientSecretHash: []byte("hash"),
		EnvironmentID:    envID,
		Scopes:           []string{"flags:read"},
		Active:           true,
	})
	if err != nil {
		t.Fatalf("expected Set to succeed: %v", err)
	}
	if applied.Version == 0 {
		t.Fatal("expected applied credential to have a non-zero version")
	}

	got, ok := s.ApplicationCredentials().Get(applied.ID)
	if !ok {
		t.Fatal("expected credential to be present after Set")
	}
	if got.Name != "billing-service" || got.EnvironmentID != envID || !got.Active {
		t.Fatalf("unexpected credential state: %+v", got)
	}
	if len(got.Scopes) != 1 || got.Scopes[0] != "flags:read" {
		t.Fatalf("unexpected scopes: %+v", got.Scopes)
	}
}

func TestStoreListApplicationCredentialsReturnsStableOrder(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	var ids []string
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		applied, err := s.ApplicationCredentials().Set(ApplicationCredential{
			ID: NewID(), Name: name, EnvironmentID: envID, Active: true,
		})
		if err != nil {
			t.Fatalf("seed credential %q: %v", name, err)
		}
		ids = append(ids, applied.ID)
	}

	list := s.ApplicationCredentials().List()
	if len(list) != 3 {
		t.Fatalf("expected 3 credentials, got %d", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].ID >= list[i].ID {
			t.Fatalf("expected credentials ordered by ID ascending, got %+v", list)
		}
	}
}

func TestStoreDeleteApplicationCredential(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")

	applied, err := s.ApplicationCredentials().Set(ApplicationCredential{
		ID: NewID(), Name: "billing-service", EnvironmentID: envID, Active: true,
	})
	if err != nil {
		t.Fatalf("seed credential: %v", err)
	}

	if err := s.ApplicationCredentials().Delete(applied.ID); err != nil {
		t.Fatalf("expected Delete to succeed: %v", err)
	}
	if _, ok := s.ApplicationCredentials().Get(applied.ID); ok {
		t.Fatal("expected credential to be gone after Delete")
	}
}

func TestStoreDeleteEnvironmentRejectsWhenCredentialsStillReferenceIt(t *testing.T) {
	s := newTestStore(t)
	envID := seedEnvironment(t, s, "Production")
	// A second environment exists so this test isolates
	// ErrEnvironmentHasCredentials from ErrLastEnvironment.
	seedEnvironment(t, s, "Staging")

	if _, err := s.ApplicationCredentials().Set(ApplicationCredential{
		ID: NewID(), Name: "billing-service", EnvironmentID: envID, Active: true,
	}); err != nil {
		t.Fatalf("expected Set to succeed: %v", err)
	}

	if err := s.Environments().Delete(envID); !errors.Is(err, ErrEnvironmentHasCredentials) {
		t.Fatalf("expected ErrEnvironmentHasCredentials, got %v", err)
	}
	if _, ok := s.Environments().Get(envID); !ok {
		t.Fatal("expected the environment to still exist after rejected delete")
	}
}
