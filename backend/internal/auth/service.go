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

// ServiceTokenTTL is deliberately much shorter than defaultTokenTTL:
// OAuth2 client-credentials access tokens are conventionally short-lived and
// re-requested often, unlike a human's 24h browser session.
const ServiceTokenTTL = 1 * time.Hour

// serviceTokenClaim is the "typ" JWT claim value that marks a token as
// issued by GenerateServiceToken (an application credential) rather than
// GenerateToken (a human login) -- absent entirely on every human token, so
// this is fully backward compatible with tokens already issued before this
// claim existed.
const serviceTokenClaim = "service"

// Sentinel errors so callers (the api package's login/token-auth handlers)
// can errors.Is against a specific failure instead of matching on message
// text -- mirrors how store's sentinels (store.ErrUsernameTaken etc.) are
// consumed via storeErrorToAPIError.
var (
	ErrInvalidCredentials            = errors.New("invalid username or password")
	ErrUserNotFound                  = errors.New("user not found or inactive")
	ErrInvalidToken                  = errors.New("invalid token")
	ErrInvalidClaims                 = errors.New("invalid token claims")
	ErrMissingSubject                = errors.New("missing subject claim")
	ErrInvalidClientCredentials      = errors.New("invalid client id or client secret")
	ErrApplicationCredentialInactive = errors.New("application credential not found or inactive")
)

// dummyPasswordHash is compared against on every failed username lookup in
// Authenticate, so an unknown/inactive username still costs exactly one
// bcrypt compare -- without this, response latency alone would reveal
// whether a username exists even though the returned error text is
// identical either way.
var dummyPasswordHash = mustBcryptHash("aerendil-timing-safe-dummy-password")

// dummyClientSecretHash is AuthenticateClientCredentials' equivalent of
// dummyPasswordHash -- an unknown/inactive client_id still costs exactly one
// bcrypt compare, so response latency can't reveal whether a client_id
// exists.
var dummyClientSecretHash = mustBcryptHash("aerendil-timing-safe-dummy-client-secret")

func mustBcryptHash(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic("auth: failed to precompute dummy bcrypt hash: " + err.Error())
	}
	return hash
}

// User is the request-scoped resolved principal returned by Authenticate
// and ParseToken. It's deliberately a different type from store.User,
// which additionally carries PasswordHash/GroupIDs/Active and never leaves
// the store/auth layers.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// AdminConfig describes the admin account seeded into the store at
// bootstrap (see SeedAdminGroupAndUser) -- it is no longer held directly by
// Service, since the admin is just a regular persisted User once seeded.
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

func NewService(secret string, s *store.Store) *Service {
	return &Service{
		secretKey: []byte(secret),
		tokenTTL:  defaultTokenTTL,
		store:     s,
	}
}

func (s *Service) Authenticate(username, password string) (*User, error) {
	// Usernames are stored lowercase (see api.usersPostHandler/usersPutHandler
	// and SeedAdminGroupAndUser) precisely so lookups here don't need to
	// special-case casing -- normalizing on both the write and read side is
	// what makes "Admin" and "admin" collide instead of silently coexisting
	// as distinct accounts.
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
// (client_id/client_secret) against the persisted application-credential
// store. Mirrors Authenticate's shape exactly, including the timing-safe
// dummy-hash comparison for an unknown or inactive client_id.
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
// credential, redeemed via the OAuth2 client-credentials grant
// (api.oauthTokenHandler). The "typ" claim distinguishes it from a human
// GenerateToken token so AuthenticateToken can resolve either kind into the
// same principal shape.
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

// ParseToken validates the token's signature/expiry and resolves the
// carried subject against the live user store -- username is never trusted
// from the token itself, so a renamed or deactivated user is reflected
// immediately rather than only after the token expires.
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

// parseTokenClaims validates the token's signature/expiry and returns its
// raw claims, without interpreting them -- split out so both
// parseTokenSubject (human-only callers like ParseToken) and
// AuthenticateToken (which also needs the "typ" claim to distinguish a
// service token from a human one) share one signature-verification pass
// instead of each parsing independently.
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

// parseTokenSubject validates the token's signature/expiry and returns its
// subject (user ID) claim, without touching the store -- split out of
// ParseToken so AuthenticateToken can validate the token and fetch the user
// exactly once, instead of ParseToken and Resolve each fetching it
// independently.
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
// permission set, environment-access set, and admin flag in a single store
// fetch -- the combined form api.authenticateRequest uses on every request,
// replacing what used to be a ParseToken call followed by a separate
// Resolve call that each independently fetched the same user record.
//
// A token whose "typ" claim is "service" (see GenerateServiceToken) is
// resolved against the application-credential store instead of Users, but
// returns the exact same (*User, PermissionSet, EnvironmentSet, bool)
// shape a human token does, plus isService -- this is the entire mechanism
// by which requirePermission/withAudit/principalFromContext in the api
// package work unchanged for both principal kinds; they only ever see a
// *User{ID, Username}, never anything that reveals which kind of token
// produced it, except through the explicit isService flag (used by
// withAudit to record AuditEntry.ActorType).
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

// authenticateServicePrincipal resolves an already-validated service token's
// subject (an application-credential ID) into the same principal shape
// AuthenticateToken returns for a human user. An application credential is
// never an admin and its permission set is exactly its own Scopes (already
// restricted to auth.CredentialScopes at creation/update time) rather than
// anything resolved through group membership.
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
