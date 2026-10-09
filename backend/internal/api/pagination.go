package api

import (
	"net/http"
	"strconv"
)

// Pagination per docs/REST_API_Standards.md §8.4; out-of-range ?limit= uses the default.
const (
	defaultPageLimit = 25
	maxPageLimit     = 100
)

// listPage is the list metadata. Start/End are 1-based inclusive row
// positions (0 when empty); cursors are set only while entries remain.
type listPage struct {
	Limit      int    `json:"limit"`
	Total      int    `json:"total"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	PrevCursor string `json:"prevCursor,omitempty"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// parseLimit reads ?limit=, falling back to the default when absent or out of range.
func parseLimit(r *http.Request) int {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > maxPageLimit {
		return defaultPageLimit
	}
	return limit
}

// paginate slices an already-ordered collection by ?limit= and ?cursor=.
// Pagination is opt-in: with neither param it returns all items and a nil
// page, so unpaginated callers keep the full list. The cursor is an offset;
// these collections are small and admin-bounded, so offset paging is enough.
func paginate[T any](r *http.Request, items []T) ([]T, *listPage, error) {
	query := r.URL.Query()
	if !query.Has("limit") && !query.Has("cursor") {
		return items, nil, nil
	}
	limit := parseLimit(r)
	offset := 0
	if cursor := query.Get("cursor"); cursor != "" {
		var err error
		offset, err = strconv.Atoi(cursor)
		if err != nil || offset < 0 || offset > len(items) {
			return nil, nil, badRequest(CodeBadRequestCursorInvalid, MsgBadRequestCursorInvalid)
		}
	}

	end := min(offset+limit, len(items))
	page := &listPage{Limit: limit, Total: len(items)}
	if end > offset {
		page.Start, page.End = offset+1, end
	}
	if offset > 0 {
		page.PrevCursor = strconv.Itoa(max(0, offset-limit))
	}
	if end < len(items) {
		page.NextCursor = strconv.Itoa(end)
	}
	return items[offset:end], page, nil
}

// listBody builds a collection response, adding "page" only when paginated.
func listBody[T any](name string, items []T, page *listPage) map[string]any {
	body := map[string]any{name: items}
	if page != nil {
		body["page"] = page
	}
	return body
}
