package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
)

// HandleGetMyOrg returns the authed org's full settings payload.
//
//	GET /nonprofits/me   (org only)
func (cfg *apiCfg) HandleGetMyOrg(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	id := nonprofitID(user)
	ctx := r.Context()

	org, err := cfg.DB.GetNonprofitByID(ctx, id)
	if err != nil {
		cfg.handleDBError(w, err, "nonprofit")
		return
	}

	isVerified := false
	if v, verr := cfg.DB.GetNonprofitVerification(ctx, id); verr == nil && v.IsVerified.Valid {
		isVerified = v.IsVerified.Bool
	}
	team, _ := cfg.DB.ListNonprofitTeam(ctx, id)
	social, _ := cfg.DB.ListSocialLinks(ctx, id)
	bank, _ := cfg.DB.GetBankAccount(ctx, id)
	stats, _ := cfg.DB.GetNonprofitAllTimeStats(ctx, id)

	respondWithJSON(w, http.StatusOK, map[string]any{
		"profile":      org,
		"is_verified":  isVerified,
		"team":         team,
		"social_links": social,
		"bank_account": bank, // zero value if none connected
		"all_time": map[string]any{
			"total_raised_cents": stats.TotalRaisedCents,
			"raffles_run":        stats.RafflesRun,
			"follower_count":     stats.FollowerCount,
		},
	})
}

type updateOrgProfileRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Image       string   `json:"image"`
	Categories  []string `json:"categories"`
}

// HandleUpdateOrgProfile updates name/bio/logo/categories.
//
//	PATCH /nonprofits/me   (org only)
func (cfg *apiCfg) HandleUpdateOrgProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	var req updateOrgProfileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Categories == nil {
		req.Categories = []string{}
	}
	org, err := cfg.DB.UpdateNonprofitProfile(r.Context(), database.UpdateNonprofitProfileParams{
		ID:          nonprofitID(user),
		Name:        req.Name,
		Description: nullStr(req.Description),
		Image:       nullStr(req.Image),
		Categories:  req.Categories,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not update profile")
		return
	}
	respondWithJSON(w, http.StatusOK, org)
}

type updateOrgContactRequest struct {
	Website string `json:"website"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
}

// HandleUpdateOrgContact updates website/phone/address.
//
//	PATCH /nonprofits/me/contact   (org only)
func (cfg *apiCfg) HandleUpdateOrgContact(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	var req updateOrgContactRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	org, err := cfg.DB.UpdateNonprofitContact(r.Context(), database.UpdateNonprofitContactParams{
		ID:      nonprofitID(user),
		Website: nullStr(req.Website),
		Phone:   nullStr(req.Phone),
		Address: nullStr(req.Address),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not update contact")
		return
	}
	respondWithJSON(w, http.StatusOK, org)
}

// HandleListTeam returns the org's team members.
//
//	GET /nonprofits/me/team   (org only)
func (cfg *apiCfg) HandleListTeam(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	team, err := cfg.DB.ListNonprofitTeam(r.Context(), nonprofitID(user))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load team")
		return
	}
	respondWithJSON(w, http.StatusOK, team)
}

// HandleListPayouts returns the org's payout history.
//
//	GET /nonprofits/me/payouts   (org only)
func (cfg *apiCfg) HandleListPayouts(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	limit, offset := paginationParams(r, defaultPageLimit)
	payouts, err := cfg.DB.ListPayouts(r.Context(), database.ListPayoutsParams{
		NonprofitID:  nonprofitID(user),
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load payouts")
		return
	}
	respondWithJSON(w, http.StatusOK, payouts)
}

// HandleUpdateOrgNotifPrefs upserts the org's notification preferences (JSONB).
//
//	PATCH /nonprofits/me/notification-preferences   (org only)
func (cfg *apiCfg) HandleUpdateOrgNotifPrefs(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	var prefs json.RawMessage
	if !decodeJSON(w, r, &prefs) {
		return
	}
	row, err := cfg.DB.UpsertOrgNotificationPrefs(r.Context(), database.UpsertOrgNotificationPrefsParams{
		NonprofitID: orgNullable(user),
		Prefs:       prefs,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not save preferences")
		return
	}
	respondWithJSON(w, http.StatusOK, row)
}
