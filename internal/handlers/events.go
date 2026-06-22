package handlers

import (
	"database/sql"
	"net/http"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/google/uuid"
)

// optionalUserID returns the authed user's id as a nullable, or invalid if anon.
func optionalUserID(r *http.Request) uuid.NullUUID {
	if u, ok := auth.UserFromContext(r.Context()); ok {
		return uuid.NullUUID{UUID: u.ID, Valid: true}
	}
	return uuid.NullUUID{}
}

type recordViewRequest struct {
	RaffleID string `json:"raffle_id"`
	Source   string `json:"source"`
	Referrer string `json:"referrer"`
}

// HandleRecordView logs a raffle view (fire-and-forget; anon allowed).
//
//	POST /events/view   (optional auth)
func (cfg *apiCfg) HandleRecordView(w http.ResponseWriter, r *http.Request) {
	var req recordViewRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	raffleID, err := uuid.Parse(req.RaffleID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid raffle_id")
		return
	}
	_ = cfg.DB.RecordRaffleView(r.Context(), database.RecordRaffleViewParams{
		RaffleItemID: raffleID,
		UserID:       optionalUserID(r),
		SessionID:    sql.NullString{},
		Source:       nullStr(req.Source),
		Referrer:     nullStr(req.Referrer),
	})
	respondWithJSON(w, http.StatusNoContent, nil)
}

type recordSearchRequest struct {
	Query           string `json:"query"`
	ResultCount     int32  `json:"result_count"`
	ClickedRaffleID string `json:"clicked_raffle_id"`
}

// HandleRecordSearch logs a search event for demand sensing.
//
//	POST /events/search   (optional auth)
func (cfg *apiCfg) HandleRecordSearch(w http.ResponseWriter, r *http.Request) {
	var req recordSearchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	clicked := uuid.NullUUID{}
	if id, err := uuid.Parse(req.ClickedRaffleID); err == nil {
		clicked = uuid.NullUUID{UUID: id, Valid: true}
	}
	_ = cfg.DB.RecordSearch(r.Context(), database.RecordSearchParams{
		UserID:          optionalUserID(r),
		Query:           req.Query,
		ResultCount:     req.ResultCount,
		ClickedRaffleID: clicked,
	})
	respondWithJSON(w, http.StatusNoContent, nil)
}

type recordShareRequest struct {
	RaffleID   string `json:"raffle_id"`
	Channel    string `json:"channel"`
	ShareToken string `json:"share_token"`
}

// HandleRecordShare logs a share and returns the row (with its token).
//
//	POST /events/share   (optional auth)
func (cfg *apiCfg) HandleRecordShare(w http.ResponseWriter, r *http.Request) {
	var req recordShareRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	raffle := uuid.NullUUID{}
	if id, err := uuid.Parse(req.RaffleID); err == nil {
		raffle = uuid.NullUUID{UUID: id, Valid: true}
	}
	share, err := cfg.DB.RecordShare(r.Context(), database.RecordShareParams{
		UserID:       optionalUserID(r),
		RaffleItemID: raffle,
		NonprofitID:  uuid.NullUUID{},
		Channel:      nullStr(req.Channel),
		ShareToken:   nullStr(req.ShareToken),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not record share")
		return
	}
	respondWithJSON(w, http.StatusCreated, share)
}
