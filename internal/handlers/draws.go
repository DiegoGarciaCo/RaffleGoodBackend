package handlers

import (
	"net/http"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/google/uuid"
)

// HandleGetDraw returns the frozen draw result and its winners for a raffle.
// The seed is only included once the draw is completed (commit-reveal).
//
//	GET /raffles/{id}/draw
func (cfg *apiCfg) HandleGetDraw(w http.ResponseWriter, r *http.Request) {
	raffleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid raffle id")
		return
	}

	result, err := cfg.DB.GetDrawResultForRaffle(r.Context(), raffleID)
	if err != nil {
		cfg.handleDBError(w, err, "draw")
		return
	}

	winners, err := cfg.DB.ListDrawWinners(r.Context(), result.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load winners")
		return
	}

	// Mark which winner (if any) is the requesting user, for the "You won!" UI.
	currentUserID := uuid.Nil
	if u, ok := auth.UserFromContext(r.Context()); ok {
		currentUserID = u.ID
	}

	// Don't leak the seed until the draw is completed.
	seed := ""
	if result.Status == "completed" && result.Seed.Valid {
		seed = result.Seed.String
	}

	respondWithJSON(w, http.StatusOK, map[string]any{
		"raffle_item_id":   result.RaffleItemID,
		"status":           result.Status,
		"draw_strategy":    result.DrawStrategy,
		"reveal_order":     result.RevealOrder,
		"winner_count":     result.WinnerCount,
		"draw_started_at":  result.DrawStartedAt,
		"draw_duration_ms": result.DrawDurationMs,
		"commitment_hash":  result.CommitmentHash,
		"seed":             seed, // empty until completed
		"watchers_now":     cfg.watcherCount(raffleID),
		"current_user_id":  currentUserID,
		"winners":          winners,
	})
}

// HandleVerifyData returns everything a client needs to independently verify a
// completed draw: the commitment, the revealed seed, the config, and the full
// ordered ticket pool. The client re-runs the HMAC permutation and confirms the
// winners and the sha256(seed) == commitment.
//
//	GET /raffles/{id}/draw/verify
func (cfg *apiCfg) HandleVerifyData(w http.ResponseWriter, r *http.Request) {
	raffleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid raffle id")
		return
	}

	v, err := cfg.DB.GetVerificationData(r.Context(), raffleID)
	if err != nil {
		cfg.handleDBError(w, err, "draw")
		return
	}
	if !v.Seed.Valid {
		respondWithError(w, http.StatusConflict, "draw not yet completed; seed not revealed")
		return
	}

	tickets, err := cfg.DB.ListTicketNumbersForRaffle(r.Context(), raffleID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load tickets")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]any{
		"commitment_hash": v.CommitmentHash,
		"seed":            v.Seed.String,
		"draw_strategy":   v.DrawStrategy,
		"reveal_order":    v.RevealOrder,
		"winner_count":    v.WinnerCount,
		"algorithm": map[string]string{
			"commitment": "sha256(seed_hex_string)",
			"hmac_key":   "seed hex string bytes",
			"hmac_msg":   "ticket_number as base-10 ASCII",
			"order":      "ascending by raw HMAC-SHA256 bytes, tie-break ticket_number asc; first winner_count are winners",
		},
		"tickets": tickets,
	})
}

// watcherCount safely reads the live count even if the hub isn't wired.
func (cfg *apiCfg) watcherCount(raffleID uuid.UUID) int {
	if cfg.Hub == nil {
		return 0
	}
	return cfg.Hub.Count(raffleID)
}
