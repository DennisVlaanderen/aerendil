package api

import (
	"context"
	"net/http"

	"aerendil/backend/internal/auth"
)

// resolvedPrincipal is the authenticated caller of the current request,
// along with their live-resolved permission set -- computed once per
// request by authenticateRequest and threaded through to handlers via the
// request context so it never needs to be re-parsed/re-resolved.
type resolvedPrincipal struct {
	User    *auth.User
	Perms   auth.PermissionSet
	Envs    auth.EnvironmentSet
	IsAdmin bool
}

// hasEnvironmentAccess reports whether p may read/write the given
// environment's flags: Admin's usual unconditional bypass, or explicit
// membership in the environment set resolved from p's groups.
func (p resolvedPrincipal) hasEnvironmentAccess(environmentID string) bool {
	return p.IsAdmin || p.Envs.Has(environmentID)
}

// authenticateRequest parses the bearer token and resolves the caller's
// permissions. On any failure it writes the appropriate 401 response
// itself and returns ok=false; callers should return immediately in that
// case. This is the single place requirePermission and meHandler both go
// through, so their error responses can never drift apart.
func authenticateRequest(w http.ResponseWriter, r *http.Request) (resolvedPrincipal, bool) {
	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		writeError(w, http.StatusUnauthorized, CodeAuthMissingHeader, "missing authorization header")
		return resolvedPrincipal{}, false
	}

	principal, perms, envs, isAdmin, err := authService.AuthenticateToken(authorization)
	if err != nil {
		writeError(w, http.StatusUnauthorized, CodeAuthInvalidToken, "invalid token")
		return resolvedPrincipal{}, false
	}

	return resolvedPrincipal{User: principal, Perms: perms, Envs: envs, IsAdmin: isAdmin}, true
}

type principalContextKey struct{}

// withPrincipal returns a copy of r carrying p, retrievable by handlers via
// principalFromContext.
func withPrincipal(r *http.Request, p resolvedPrincipal) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), principalContextKey{}, p))
}

// principalFromContext retrieves the resolvedPrincipal set by
// requirePermission for the current request.
func principalFromContext(r *http.Request) (resolvedPrincipal, bool) {
	p, ok := r.Context().Value(principalContextKey{}).(resolvedPrincipal)
	return p, ok
}

// requirePermission wraps next so it only runs for requests carrying a
// valid bearer token whose resolved principal either belongs to the Admin
// group (unconditional bypass) or holds perm via at least one of their
// groups. This is the single chokepoint every permission-gated route in
// the API goes through -- gating a new endpoint elsewhere is "reference an
// existing/new permission constant here", nothing else needs to change.
func requirePermission(perm string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authenticateRequest(w, r)
		if !ok {
			return
		}

		if !principal.IsAdmin && !principal.Perms.Has(perm) {
			writeError(w, http.StatusForbidden, CodeAuthForbidden, "forbidden")
			return
		}

		next(w, withPrincipal(r, principal))
	}
}
