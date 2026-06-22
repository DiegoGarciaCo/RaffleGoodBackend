package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
)

// respondWithJSON writes v as a JSON response with the given status code.
func respondWithJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logrus.WithError(err).Error("failed to encode JSON response")
	}
}

// errorBody is the shape every error response takes, so the client can rely on it.
type errorBody struct {
	Error string `json:"error"`
}

// respondWithError writes a JSON error. 5xx errors are logged; 4xx are not
// (they're client mistakes, not server problems).
func respondWithError(w http.ResponseWriter, status int, msg string) {
	if status >= 500 {
		logrus.WithField("status", status).Error(msg)
	}
	respondWithJSON(w, status, errorBody{Error: msg})
}

// decodeJSON reads and validates a JSON request body into dst.
// Returns false (and writes a 400) if the body is malformed.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}
