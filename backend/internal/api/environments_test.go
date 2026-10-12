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

func TestEnvironmentsGetByIDRequiresPermission(t *testing.T) {
	mux := newTestMux(t)
	id := seedEnvironmentForTest(t, "GetByIDNoPerm")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/environments/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without environments:read, got %d", rec.Code)
	}
}

func TestEnvironmentsGetByIDReturnsNotFoundForUnknownID(t *testing.T) {
	mux := newTestMux(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/environments/does-not-exist", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, auth.PermEnvironmentsRead))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown environment, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnvironmentsGetByIDReturnsEnvironment(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermEnvironmentsRead)
	id := seedEnvironmentForTest(t, "GetByIDFound")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/environments/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if raw["id"] != id {
		t.Fatalf("expected id %q in response, got %+v", id, raw)
	}
	if raw["name"] != "GetByIDFound" {
		t.Fatalf("expected name %q in response, got %+v", "GetByIDFound", raw)
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

func TestEnvironmentsPostValidatesAndNormalizesColor(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermEnvironmentsCreate)

	cases := []struct {
		name       string
		color      string
		wantStatus int
		wantColor  string
	}{
		{"empty color", "", http.StatusCreated, ""},
		{"palette key", "4", http.StatusCreated, "4"},
		{"hex lowercase", "#1a2b3c", http.StatusCreated, "#1a2b3c"},
		{"hex uppercase", "#1A2B3C", http.StatusCreated, "#1a2b3c"},
		{"short hex lowercase", "#abc", http.StatusCreated, "#abc"},
		{"short hex uppercase", "#ABC", http.StatusCreated, "#abc"},
		{"invalid color", "invalid", http.StatusBadRequest, ""},
		{"three digit number", "100", http.StatusBadRequest, ""},
		{"negative number", "-1", http.StatusBadRequest, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			createBody, _ := json.Marshal(map[string]any{"name": "Production", "color": testCase.color})
			createReq := httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(createBody))
			createReq.Header.Set("Authorization", "Bearer "+token)
			createRec := httptest.NewRecorder()
			mux.ServeHTTP(createRec, createReq)
			if createRec.Code != testCase.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", testCase.wantStatus, createRec.Code, createRec.Body.String())
			}
			if testCase.wantStatus != http.StatusCreated {
				return
			}

			var created map[string]any
			if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
				t.Fatalf("decode create response: %v", err)
			}
			if created["color"] != testCase.wantColor {
				t.Fatalf("expected color %q, got %v", testCase.wantColor, created["color"])
			}
		})
	}
}

func TestEnvironmentsPutUpdatesColor(t *testing.T) {
	mux := newTestMux(t)
	token := tokenFor(t, auth.PermEnvironmentsCreate, auth.PermEnvironmentsUpdate)

	createBody, _ := json.Marshal(map[string]any{"name": "Production", "color": "4"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id := created["id"].(string)

	updateBody, _ := json.Marshal(map[string]any{"name": "Production", "color": "#1a2b3c"})
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
	if updated["color"] != "#1a2b3c" {
		t.Fatalf("expected color to change from palette key to #1a2b3c, got %v", updated["color"])
	}
}

func TestNormalizeEnvironmentColorOnEmptyInput(t *testing.T) {
	_, valid := normalizeEnvironmentColor("")
	if !valid {
		t.Fatalf("expected valid color for empty input")
	}
}

func TestNormalizeEnvironmentColorOnValidHex(t *testing.T) {
	color, valid := normalizeEnvironmentColor("#1a2b3c")
	if !valid || color != "#1a2b3c" {
		t.Fatalf("expected valid color for #1a2b3c, got %q", color)
	}
}

func TestNormalizeEnvironmentColorOnValidShortHex(t *testing.T) {
	color, valid := normalizeEnvironmentColor("#abc")
	if !valid || color != "#abc" {
		t.Fatalf("expected valid color for #abc, got %q", color)
	}
}

func TestNormalizeEnvironmentColorOnValidNumber(t *testing.T) {
	color, valid := normalizeEnvironmentColor("42")
	if !valid || color != "42" {
		t.Fatalf("expected valid color for 42, got %q", color)
	}
}

func TestNormalizeEnvironmentColorOnValidNumberDoubleDigit(t *testing.T) {
	color, valid := normalizeEnvironmentColor("99")
	if !valid || color != "99" {
		t.Fatalf("expected valid color for 99, got %q", color)
	}
}

func TestNormalizeEnvironmentColorOnInvalidInput(t *testing.T) {
	_, valid := normalizeEnvironmentColor("invalid")
	if valid {
		t.Fatalf("expected invalid color for 'invalid'")
	}
}

func TestNormalizeEnvironmentColorOnInvalidHex(t *testing.T) {
	_, valid := normalizeEnvironmentColor("#12345")
	if valid {
		t.Fatalf("expected invalid color for '#12345'")
	}
}

func TestNormalizeEnvironmentColorOnInvalidShortHex(t *testing.T) {
	_, valid := normalizeEnvironmentColor("#1GF")
	if valid {
		t.Fatalf("expected invalid color for '#1GF'")
	}
}

func TestNormalizeEnvironmentColorOnInvalidShortHexWrongFormat(t *testing.T) {
	_, valid := normalizeEnvironmentColor("#]]]")
	if valid {
		t.Fatalf("expected invalid color for '#]]]'")
	}
}

func TestNormalizeEnvironmentColorOnInvalidNumber(t *testing.T) {
	_, valid := normalizeEnvironmentColor("100")
	if valid {
		t.Fatalf("expected invalid color for '100'")
	}
}

func TestNormalizeEnvironmentColorOnInvalidNegativeNumber(t *testing.T) {
	_, valid := normalizeEnvironmentColor("-1")
	if valid {
		t.Fatalf("expected invalid color for '-1'")
	}
}
