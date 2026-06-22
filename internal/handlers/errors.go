package handlers

import (
	"database/sql"
	"errors"
	"net/http"
)

// handleDBError maps common database errors to appropriate HTTP responses.
// Use this in handlers after a query call:
//
//	row, err := cfg.DB.GetRaffleByID(ctx, id)
//	if err != nil {
//	    cfg.handleDBError(w, err, "raffle")
//	    return
//	}
func (cfg *apiCfg) handleDBError(w http.ResponseWriter, err error, resource string) {
	if errors.Is(err, sql.ErrNoRows) {
		respondWithError(w, http.StatusNotFound, resource+" not found")
		return
	}
	// Anything else is an unexpected server error.
	respondWithError(w, http.StatusInternalServerError, "internal server error")
}

// Common sentinel errors for handler logic.
var (
	errUnauthorized = errors.New("unauthorized")
	errForbidden    = errors.New("forbidden")
)
