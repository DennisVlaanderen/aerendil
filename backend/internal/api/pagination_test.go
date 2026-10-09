package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"aerendil/backend/internal/store"
)

func TestPaginate(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	call := func(query string) ([]int, *listPage, error) {
		return paginate(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x"+query, nil), items)
	}

	got, page, err := call("")
	if err != nil || page != nil || !reflect.DeepEqual(got, items) {
		t.Fatalf("no params: want all items and nil page, got %v %+v %v", got, page, err)
	}

	cases := []struct {
		query string
		want  []int
		page  listPage
	}{
		{"?limit=2", []int{1, 2}, listPage{Limit: 2, Total: 5, Start: 1, End: 2, NextCursor: "2"}},
		{"?limit=2&cursor=2", []int{3, 4}, listPage{Limit: 2, Total: 5, Start: 3, End: 4, PrevCursor: "0", NextCursor: "4"}},
		{"?limit=2&cursor=4", []int{5}, listPage{Limit: 2, Total: 5, Start: 5, End: 5, PrevCursor: "2"}},
		{"?limit=2&cursor=5", []int{}, listPage{Limit: 2, Total: 5, PrevCursor: "3"}},
		{"?cursor=0", items, listPage{Limit: defaultPageLimit, Total: 5, Start: 1, End: 5}},
		{"?limit=abc", items, listPage{Limit: defaultPageLimit, Total: 5, Start: 1, End: 5}},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			got, page, err := call(c.query)
			if err != nil || page == nil || !reflect.DeepEqual(got, c.want) || *page != c.page {
				t.Fatalf("got %v %+v %v, want %v %+v", got, page, err, c.want, c.page)
			}
		})
	}

	for _, cursor := range []string{"abc", "-1", "6", "1.5"} {
		t.Run("bad cursor "+cursor, func(t *testing.T) {
			if _, _, err := call("?cursor=" + cursor); err == nil {
				t.Fatalf("expected an error for cursor %q", cursor)
			}
		})
	}
}

// TestListEndpointsPaginate checks each collection keeps its full,
// page-less response without params, and that walking ?limit=2 pages
// yields the same items in the same order.
func TestListEndpointsPaginate(t *testing.T) {
	mux := newTestMux(t)
	token := adminToken(t)
	envID := seedEnvironmentForTest(t, "paginate-env")
	for i := range 3 {
		name := "paginate-" + string(rune('a'+i))
		tokenFor(t) // seeds a user
		seedGroupForTest(t, name, nil)
		seedEnvironmentForTest(t, name)
		if _, err := dataStore.Flags().Set(store.Flag{Key: name, EnvironmentID: envID}); err != nil {
			t.Fatalf("seed flag: %v", err)
		}
		createApplicationCredentialForTest(t, mux, token, name, envID, nil)
	}

	endpoints := []struct{ path, field string }{
		{"/api/users?", "users"},
		{"/api/groups?", "groups"},
		{"/api/environments?", "environments"},
		{"/api/flags?environmentId=" + envID + "&", "flags"},
		{"/api/application-credentials?environmentId=" + envID + "&", "applicationCredentials"},
	}
	for _, e := range endpoints {
		t.Run(e.field, func(t *testing.T) {
			get := func(query string) (items []json.RawMessage, page *listPage) {
				t.Helper()
				req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, e.path+query, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("GET %s%s: %d %s", e.path, query, rec.Code, rec.Body.String())
				}
				var body map[string]json.RawMessage
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if err := json.Unmarshal(body[e.field], &items); err != nil {
					t.Fatalf("decode %s: %v", e.field, err)
				}
				if raw, ok := body["page"]; ok {
					page = &listPage{}
					if err := json.Unmarshal(raw, page); err != nil {
						t.Fatalf("decode page: %v", err)
					}
				}
				return items, page
			}

			all, page := get("")
			if page != nil || len(all) < 3 {
				t.Fatalf("no params: want >=3 items and no page, got %d items, page %+v", len(all), page)
			}

			var walked []json.RawMessage
			cursor := ""
			for {
				items, page := get("limit=2&cursor=" + cursor)
				if page == nil || page.Total != len(all) || len(items) > 2 {
					t.Fatalf("bad page: %d items, %+v", len(items), page)
				}
				walked = append(walked, items...)
				if page.NextCursor == "" {
					break
				}
				cursor = page.NextCursor
			}
			if !reflect.DeepEqual(walked, all) {
				t.Fatalf("paged items differ from the full list:\n%s\nvs\n%s", walked, all)
			}
		})
	}
}

func TestListEndpointRejectsMalformedCursor(t *testing.T) {
	mux := newTestMux(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/users?cursor=abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorBody(t, rec, CodeBadRequestCursorInvalid)
}
