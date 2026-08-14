// Package auth issues and verifies JWTs, hashes passwords, and resolves a
// principal's permissions live against the store on every request. See
// Service.
package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"aerendil/backend/internal/store"
)

const defaultTokenTTL = 24 * time.Hour

// ServiceTokenTTL is shorter than defaultTokenTTL: OAuth2 client-credentials
// tokens are conventionally short-lived and re-requested often.
const ServiceTokenTTL = 1 * time.Hour

// serviceTokenClaim marks a token as issued by GenerateServiceToken rather
// than GenerateToken; absent on human tokens, so old tokens stay valid.
const serviceTokenClaim = "service"

// Sentinel errors so callers can errors.Is against a specific failure
// instead of matching on message text.
var (
	ErrInvalidCredentials            = errors.New("invalid username or password")
	ErrUserNotFound                  = errors.New("user not found or inactive")
	ErrInvalidToken                  = errors.New("invalid token")
	ErrInvalidClaims                 = errors.New("invalid token claims")
	ErrMissingSubject                = errors.New("missing subject claim")
	ErrInvalidClientCredentials      = errors.New("invalid client id or client secret")
	ErrApplicationCredentialInactive = errors.New("application credential not found or inactive")
)

// dummyPasswordHash keeps a failed lookup in Authenticate costing exactly
// one bcrypt compare, so latency can't reveal whether a username exists.
var dummyPasswordHash = mustBcryptHash("aerendil-timing-safe-dummy-password")

// dummyClientSecretHash is dummyPasswordHash's equivalent for
// AuthenticateClientCredentials.
var dummyClientSecretHash = mustBcryptHash("aerendil-timing-safe-dummy-client-secret")

func mustBcryptHash(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic("auth: failed to precompute dummy bcrypt hash: " + err.Error())
	}
	return hash
}

// User is the resolved principal returned by Authenticate and ParseToken --
// distinct from store.User, which never leaves the store/auth layers.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// AdminConfig describes the admin account seeded into the store at
// bootstrap; see SeedAdminGroupAndUser.
type AdminConfig struct {
	Username string
	Password string
}

// DefaultAdminConfig returns the insecure admin/admin123 pair used when no
// admin credentials are configured, so local deploys work out of the box.
func DefaultAdminConfig() AdminConfig {
	return AdminConfig{Username: "admin", Password: "admin123"}
}

// Service issues/parses JWTs and authenticates against the persisted user
// store. Permission resolution (Resolve, in permissions.go) also reads
// from the same store, fresh per call.
type Service struct {
	secretKey []byte
	tokenTTL  time.Duration
	store     *store.Store
}

// NewService returns a Service that signs tokens with secret and resolves
// users/permissions against s.
func NewService(secret string, s *store.Store) *Service {
	return &Service{
		secretKey: []byte(secret),
		tokenTTL:  defaultTokenTTL,
		store:     s,
	}
}

// Authenticate verifies username/password against the user store, returning
// ErrInvalidCredentials for both an unknown user and a wrong password (see
// dummyPasswordHash for why both cost one bcrypt compare).
func (s *Service) Authenticate(username, password string) (*User, error) {
	// Usernames are stored lowercase (see api.usersPostHandler/usersPutHandler
	// and SeedAdminGroupAndUser), so "Admin" and "admin" collide instead of
	// coexisting as distinct accounts.
	u, ok := s.store.Users().GetByUsername(strings.ToLower(strings.TrimSpace(username)))
	valid := ok && u.Active

	hash := dummyPasswordHash
	if valid {
		hash = u.PasswordHash
	}
	compareErr := bcrypt.CompareHashAndPassword(hash, []byte(password))

	if !valid || compareErr != nil {
		return nil, ErrInvalidCredentials
	}
	return &User{ID: u.ID, Username: u.Username}, nil
}

// GenerateToken issues an HS256 JWT for user with a sub/exp/iat claim set
// and the Service's configured tokenTTL.
func (s *Service) GenerateToken(user *User) (string, error) {
	claims := jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(s.tokenTTL).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

// AuthenticateClientCredentials validates an OAuth2 client-credentials grant
// against the application-credential store, mirroring Authenticate
// (including the timing-safe dummy-hash comparison).
func (s *Service) AuthenticateClientCredentials(clientID, clientSecret string) (store.ApplicationCredential, error) {
	cred, ok := s.store.ApplicationCredentials().Get(strings.TrimSpace(clientID))
	valid := ok && cred.Active

	hash := dummyClientSecretHash
	if valid {
		hash = cred.ClientSecretHash
	}
	compareErr := bcrypt.CompareHashAndPassword(hash, []byte(clientSecret))

	if !valid || compareErr != nil {
		return store.ApplicationCredential{}, ErrInvalidClientCredentials
	}
	return cred, nil
}

// GenerateServiceToken issues a short-lived access token for an application
// credential (redeemed via api.oauthTokenHandler). The "typ" claim lets
// AuthenticateToken tell it apart from a human GenerateToken token.
func (s *Service) GenerateServiceToken(cred store.ApplicationCredential) (string, error) {
	claims := jwt.MapClaims{
		"sub": cred.ID,
		"typ": serviceTokenClaim,
		"exp": time.Now().Add(ServiceTokenTTL).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

// ParseToken validates the token and resolves its subject against the live
// user store, so a renamed or deactivated user takes effect immediately
// rather than only after the token expires.
func (s *Service) ParseToken(tokenString string) (*User, error) {
	userID, err := s.parseTokenSubject(tokenString)
	if err != nil {
		return nil, err
	}

	u, ok := s.store.Users().Get(userID)
	if !ok || !u.Active {
		return nil, ErrUserNotFound
	}

	return &User{ID: u.ID, Username: u.Username}, nil
}

// parseTokenClaims validates the token and returns its raw claims, shared by
// parseTokenSubject and AuthenticateToken so each doesn't re-verify.
func (s *Service) parseTokenClaims(tokenString string) (jwt.MapClaims, error) {
	trimmed := strings.TrimSpace(tokenString)
	if strings.HasPrefix(strings.ToLower(trimmed), "bearer ") {
		trimmed = strings.TrimSpace(trimmed[7:])
	}

	token, err := jwt.Parse(trimmed, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}
	return claims, nil
}

// parseTokenSubject returns the token's subject (user ID) claim without
// touching the store, so callers fetch the user exactly once.
func (s *Service) parseTokenSubject(tokenString string) (string, error) {
	claims, err := s.parseTokenClaims(tokenString)
	if err != nil {
		return "", err
	}

	userID, _ := claims["sub"].(string)
	if userID == "" {
		return "", ErrMissingSubject
	}
	return userID, nil
}

// AuthenticateToken validates a bearer token and resolves its principal,
// permissions, environment access, and admin flag in one store fetch -- used
// by api.authenticateRequest on every request.
//
// A "typ": "service" token (see GenerateServiceToken) resolves against the
// application-credential store instead of Users, but returns the same shape
// plus isService, so callers work unchanged for both principal kinds.
func (s *Service) AuthenticateToken(tokenString string) (user *User, perms PermissionSet, envs EnvironmentSet, isAdmin bool, isService bool, err error) {
	claims, err := s.parseTokenClaims(tokenString)
	if err != nil {
		return nil, nil, nil, false, false, err
	}

	subject, _ := claims["sub"].(string)
	if subject == "" {
		return nil, nil, nil, false, false, ErrMissingSubject
	}

	if typ, _ := claims["typ"].(string); typ == serviceTokenClaim {
		user, perms, envs, isAdmin, err = s.authenticateServicePrincipal(subject)
		return user, perms, envs, isAdmin, true, err
	}

	u, ok := s.store.Users().Get(subject)
	if !ok || !u.Active {
		return nil, nil, nil, false, false, ErrUserNotFound
	}

	perms, envs, isAdmin = s.resolvePermissionsForUser(u)
	return &User{ID: u.ID, Username: u.Username}, perms, envs, isAdmin, false, nil
}

// authenticateServicePrincipal resolves a service token's subject (an
// application-credential ID) into the same principal shape a human user
// gets. A credential is never an admin; its permissions are exactly its own
// Scopes, not anything resolved through group membership.
func (s *Service) authenticateServicePrincipal(credentialID string) (*User, PermissionSet, EnvironmentSet, bool, error) {
	cred, ok := s.store.ApplicationCredentials().Get(credentialID)
	if !ok || !cred.Active {
		return nil, nil, nil, false, ErrApplicationCredentialInactive
	}

	perms := PermissionSet{}
	for _, scope := range cred.Scopes {
		perms[scope] = struct{}{}
	}
	envs := EnvironmentSet{}
	if cred.EnvironmentID != "" {
		envs[cred.EnvironmentID] = struct{}{}
	}
	return &User{ID: cred.ID, Username: cred.Name}, perms, envs, false, nil
}
