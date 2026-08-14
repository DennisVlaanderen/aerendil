package store

import (
	"fmt"
	"sort"
)

// ApplicationCredential is a machine identity for the OAuth2
// client-credentials grant (see auth.Service.AuthenticateClientCredentials).
// ID doubles as the OAuth2 client_id. Scopes is validated against
// auth.CredentialScopes at the API layer, not here.
type ApplicationCredential struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	ClientSecretHash []byte   `json:"clientSecretHash,omitempty"`
	EnvironmentID    string   `json:"environmentId"`
	Scopes           []string `json:"scopes,omitempty"`
	Active           bool     `json:"active"`
	Version          uint64   `json:"version"`
}

func (f *fsm) applyApplicationCredential(index uint64, cmd command) any {
	switch cmd.Op {
	case opSet:
		if _, ok := f.environments[cmd.ApplicationCredential.EnvironmentID]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownEnvironment, cmd.ApplicationCredential.EnvironmentID)
		}
		cmd.ApplicationCredential.Version = index
		f.applicationCredentials[cmd.ApplicationCredential.ID] = *cmd.ApplicationCredential
		return *cmd.ApplicationCredential
	case opDelete:
		delete(f.applicationCredentials, cmd.Key)
		return nil
	default:
		return fmt.Errorf("unknown command op %q", cmd.Op)
	}
}

func (f *fsm) getApplicationCredential(id string) (ApplicationCredential, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	c, ok := f.applicationCredentials[id]
	return c, ok
}

// listApplicationCredentials returns every credential ordered by ID --
// map iteration order is otherwise randomized.
func (f *fsm) listApplicationCredentials() []ApplicationCredential {
	f.mu.RLock()
	defer f.mu.RUnlock()
	creds := make([]ApplicationCredential, 0, len(f.applicationCredentials))
	for _, c := range f.applicationCredentials {
		creds = append(creds, c)
	}
	sort.Slice(creds, func(i, j int) bool { return creds[i].ID < creds[j].ID })
	return creds
}

// hasCredentialsInEnvironmentLocked reports whether any credential is
// scoped to environmentID. Caller must already hold f.mu.
func (f *fsm) hasCredentialsInEnvironmentLocked(environmentID string) bool {
	for _, c := range f.applicationCredentials {
		if c.EnvironmentID == environmentID {
			return true
		}
	}
	return false
}

// hasCredentialsInEnvironment is the read-locking counterpart of
// hasCredentialsInEnvironmentLocked, for use outside of Apply.
func (f *fsm) hasCredentialsInEnvironment(environmentID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.hasCredentialsInEnvironmentLocked(environmentID)
}

// ApplicationCredentialRepository provides application-credential operations
// against the store. Obtain one via Store.ApplicationCredentials().
type ApplicationCredentialRepository struct {
	store *Store
}

// Get returns the current state of an application credential, if it exists.
func (r ApplicationCredentialRepository) Get(id string) (ApplicationCredential, bool) {
	return r.store.fsm.getApplicationCredential(id)
}

// List returns all known application credentials, ordered by ID.
func (r ApplicationCredentialRepository) List() []ApplicationCredential {
	return r.store.fsm.listApplicationCredentials()
}

// Set applies an application-credential create/update through Raft
// consensus.
func (r ApplicationCredentialRepository) Set(cred ApplicationCredential) (ApplicationCredential, error) {
	resp, err := r.store.apply(command{Op: opSet, Entity: entityApplicationCredential, ApplicationCredential: &cred})
	if err != nil {
		return ApplicationCredential{}, err
	}
	switch v := resp.(type) {
	case ApplicationCredential:
		return v, nil
	case error:
		return ApplicationCredential{}, v
	default:
		return ApplicationCredential{}, fmt.Errorf("unexpected apply response type %T", resp)
	}
}

// Delete removes an application credential by ID.
func (r ApplicationCredentialRepository) Delete(id string) error {
	resp, err := r.store.apply(command{Op: opDelete, Entity: entityApplicationCredential, Key: id})
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	return nil
}
