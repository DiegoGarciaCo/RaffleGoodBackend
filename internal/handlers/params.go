package handlers

import (
	"net/http"
	"strconv"
)

const defaultPageLimit int32 = 20

// paginationParams reads ?limit= and ?offset= with sane bounds and defaults.
// limit is clamped to 1..100; offset to >= 0.
func paginationParams(r *http.Request, defLimit int32) (limit, offset int32) {
	limit, offset = defLimit, 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	return limit, offset
}
