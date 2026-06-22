package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/google/uuid"
)

// HandleGetProfile returns the user's profile bundle.
//
//	GET /users/me/profile   (auth)
func (cfg *apiCfg) HandleGetProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	ctx := r.Context()

	methods, _ := cfg.DB.ListPaymentMethods(ctx, user.ID)
	addresses, _ := cfg.DB.ListShippingAddresses(ctx, user.ID)
	stats, _ := cfg.DB.GetUserTicketStats(ctx, user.ID)
	prefs, _ := cfg.DB.GetUserNotificationPrefs(ctx, uuid.NullUUID{UUID: user.ID, Valid: true})

	respondWithJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		},
		"payment_methods":          methods,
		"shipping_addresses":       addresses,
		"notification_preferences": prefs,
		"stats":                    stats,
	})
}

// HandleListSaved returns the user's saved raffles.
//
//	GET /users/me/saved-raffles   (auth)
func (cfg *apiCfg) HandleListSaved(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	saved, err := cfg.DB.ListSavedRaffles(r.Context(), user.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load saved raffles")
		return
	}
	respondWithJSON(w, http.StatusOK, saved)
}

// HandleSaveRaffle saves a raffle for later.
//
//	POST /raffles/{id}/save   (auth)
func (cfg *apiCfg) HandleSaveRaffle(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	raffleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid raffle id")
		return
	}
	_, err = cfg.DB.SaveRaffle(r.Context(), database.SaveRaffleParams{
		UserID: user.ID, RaffleItemID: raffleID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not save raffle")
		return
	}
	respondWithJSON(w, http.StatusNoContent, nil)
}

// HandleUnsaveRaffle removes a saved raffle.
//
//	DELETE /raffles/{id}/save   (auth)
func (cfg *apiCfg) HandleUnsaveRaffle(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	raffleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid raffle id")
		return
	}
	if err := cfg.DB.UnsaveRaffle(r.Context(), database.UnsaveRaffleParams{
		UserID: user.ID, RaffleItemID: raffleID,
	}); err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not unsave raffle")
		return
	}
	respondWithJSON(w, http.StatusNoContent, nil)
}

// HandleListFollowing returns the orgs the user follows.
//
//	GET /users/me/following   (auth)
func (cfg *apiCfg) HandleListFollowing(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	orgs, err := cfg.DB.ListFollowedOrgs(r.Context(), user.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load following")
		return
	}
	respondWithJSON(w, http.StatusOK, orgs)
}

// HandleUpdateUserNotifPrefs upserts the user's notification preferences.
//
//	PATCH /users/me/notification-preferences   (auth)
func (cfg *apiCfg) HandleUpdateUserNotifPrefs(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	var prefs json.RawMessage
	if !decodeJSON(w, r, &prefs) {
		return
	}
	row, err := cfg.DB.UpsertUserNotificationPrefs(r.Context(), database.UpsertUserNotificationPrefsParams{
		UserID: uuid.NullUUID{UUID: user.ID, Valid: true},
		Prefs:  prefs,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not save preferences")
		return
	}
	respondWithJSON(w, http.StatusOK, row)
}
