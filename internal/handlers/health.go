package handlers

import (
	"net/http"
)

// HandleHealthz is a liveness/readiness probe. Pings the DB so load balancers
// and deploy platforms can tell if the service is actually healthy.
func (cfg *apiCfg) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := cfg.RawDB.PingContext(r.Context()); err != nil {
		respondWithError(w, http.StatusServiceUnavailable, "database unreachable")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
