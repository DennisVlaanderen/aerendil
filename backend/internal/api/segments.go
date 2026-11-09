package api

import (
	"net/http"
	"strings"

	"aerendil/backend/internal/auth"
	"aerendil/backend/internal/store"
)

func registerSegmentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/segments", requirePermission(auth.PermSegmentsRead, handleErrors(segmentsGetHandler)))
	mux.HandleFunc("GET /api/segments/{key}", requirePermission(auth.PermSegmentsRead, handleErrors(segmentsGetByKeyHandler)))
	// Create-only (ErrSegmentExists -> 409), so there's never a Before state.
	mux.HandleFunc("POST /api/segments", requirePermission(auth.PermSegmentsWrite, withAudit(auditConfig{
		Action:     "segment.create",
		TargetType: "segment",
	}, handleErrors(segmentsPostHandler))))
	mux.HandleFunc("PUT /api/segments/{key}", requirePermission(auth.PermSegmentsUpdate, withAudit(auditConfig{
		Action:     "segment.update",
		TargetType: "segment",
		Before:     segmentBefore,
	}, handleErrors(segmentsPutHandler))))
	mux.HandleFunc("DELETE /api/segments/{key}", requirePermission(auth.PermSegmentsDelete, withAudit(auditConfig{
		Action:     "segment.delete",
		TargetType: "segment",
		Before:     segmentBefore,
	}, handleErrors(segmentsDeleteHandler))))
}

func segmentBefore(r *http.Request, _ []byte) (any, bool) {
	s, ok := dataStore.Segments().Get(strings.TrimSpace(r.URL.Query().Get("environmentId")), r.PathValue("key"))
	return s, ok
}

// segmentEnvironment checks that environmentID is set and that the caller
// may access it.
func segmentEnvironment(r *http.Request, environmentID string) error {
	if environmentID == "" {
		return badRequest(CodeBadRequestSegmentEnvironmentIDRequired, MsgBadRequestSegmentEnvironmentIDRequired)
	}
	principal, found := principalFromContext(r)
	if !found || !principal.hasEnvironmentAccess(environmentID) {
		return forbidden(CodeAuthForbidden, MsgAuthForbidden)
	}
	return nil
}

func segmentsGetHandler(w http.ResponseWriter, r *http.Request) error {
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if err := segmentEnvironment(r, environmentID); err != nil {
		return err
	}
	segments, page, err := paginate(r, dataStore.Segments().List(environmentID))
	if err != nil {
		return err
	}
	return ok(w, listBody("segments", segments, page))
}

func segmentsGetByKeyHandler(w http.ResponseWriter, r *http.Request) error {
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if err := segmentEnvironment(r, environmentID); err != nil {
		return err
	}
	segment, exists := dataStore.Segments().Get(environmentID, r.PathValue("key"))
	if !exists {
		return notFound(CodeNotFoundSegment, MsgNotFoundSegment)
	}
	return ok(w, segment)
}

func segmentsPostHandler(w http.ResponseWriter, r *http.Request) error {
	// The §6.2 segment, environmentId included. Fields the store owns
	// (revision, schemaVersion, evaluatorLevel) are ignored.
	var segment store.Segment
	if err := decodeJSON(w, r, &segment); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}
	segment.EnvironmentID = strings.TrimSpace(segment.EnvironmentID)
	if err := segmentEnvironment(r, segment.EnvironmentID); err != nil {
		return err
	}
	if err := validateSegment(&segment); err != nil {
		return err
	}

	applied, err := dataStore.Segments().Create(segment)
	if err != nil {
		return err
	}
	return created(w, applied)
}

func segmentsPutHandler(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if err := segmentEnvironment(r, environmentID); err != nil {
		return err
	}
	if _, exists := dataStore.Segments().Get(environmentID, key); !exists {
		return notFound(CodeNotFoundSegment, MsgNotFoundSegment)
	}

	// The full segment replaces the stored one. The path and query name it.
	var segment store.Segment
	if err := decodeJSON(w, r, &segment); err != nil {
		return badRequest(CodeBadRequestBody, MsgBadRequestBody)
	}
	segment.Key, segment.EnvironmentID = key, environmentID
	if err := validateSegment(&segment); err != nil {
		return err
	}

	applied, err := dataStore.Segments().Set(segment)
	if err != nil {
		return err
	}
	return ok(w, applied)
}

func segmentsDeleteHandler(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	environmentID := strings.TrimSpace(r.URL.Query().Get("environmentId"))
	if err := segmentEnvironment(r, environmentID); err != nil {
		return err
	}
	if _, exists := dataStore.Segments().Get(environmentID, key); !exists {
		return notFound(CodeNotFoundSegment, MsgNotFoundSegment)
	}
	if err := dataStore.Segments().Delete(environmentID, key); err != nil {
		return err
	}
	return ok(w, map[string]string{"status": "deleted"})
}
