package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aerendil/backend/internal/auth"
)

func TestEnvironmentsFullCRUD(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermEnvironmentsCreate, auth.PermEnvironmentsRead, auth.PermEnvironmentsUpdate, auth.PermEnvironmentsDelete)

	createBody, _ := json.Marshal(map[string]any{"name": "Production"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create to succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id := created["id"].(string)
	if created["order"].(float64) != 0 {
		t.Fatalf("expected first environment's order to be 0, got %+v", created)
	}

	updateBody, _ := json.Marshal(map[string]any{"name": "Production (renamed)"})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/environments/"+id, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update to succeed, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/environments", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if !bytes.Contains(listRec.Body.Bytes(), []byte("Production (renamed)")) {
		t.Fatalf("expected renamed environment in list, got %s", listRec.Body.String())
	}

	// A second environment must exist before delete can succeed -- deleting
	// the only environment is rejected (ErrLastEnvironment), tested
	// separately below.
	secondBody, _ := json.Marshal(map[string]any{"name": "Staging"})
	secondReq := httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(secondBody))
	secondReq.Header.Set("Authorization", "Bearer "+token)
	secondRec := httptest.NewRecorder()
	mux.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusCreated {
		t.Fatalf("expected second create to succeed, got %d: %s", secondRec.Code, secondRec.Body.String())
	}
	var second map[string]any
	if err := json.Unmarshal(secondRec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second create response: %v", err)
	}
	if second["order"].(float64) != 1 {
		t.Fatalf("expected second environment's order to be 1, got %+v", second)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/environments/"+id, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete to succeed, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestEnvironmentsPutIgnoresClientSuppliedOrder(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermEnvironmentsCreate, auth.PermEnvironmentsUpdate)

	createBody, _ := json.Marshal(map[string]any{"name": "Production"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id := created["id"].(string)

	updateBody, _ := json.Marshal(map[string]any{"name": "Production", "order": 99})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/environments/"+id, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update to succeed, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	var updated map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated["order"].(float64) != 0 {
		t.Fatalf("expected a client-supplied order to be ignored (stay 0), got %+v", updated)
	}
}

func TestEnvironmentsPutAndDeleteReturnNotFoundForUnknownID(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermEnvironmentsUpdate, auth.PermEnvironmentsDelete)

	updateBody, _ := json.Marshal(map[string]any{"name": "Production"})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/environments/does-not-exist", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for updating an unknown environment, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/environments/does-not-exist", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleting an unknown environment, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestEnvironmentsDeleteRejectsLastEnvironment(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermEnvironmentsCreate, auth.PermEnvironmentsDelete)

	createBody, _ := json.Marshal(map[string]any{"name": "Production"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id := created["id"].(string)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/environments/"+id, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 deleting the last remaining environment, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}
