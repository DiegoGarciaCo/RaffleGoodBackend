package handlers

import (
	"net/http"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/google/uuid"
)

// HandleGetNonprofit returns a public nonprofit profile by id or slug, with its
// social links and review summary.
//
//	GET /nonprofits/{idOrSlug}
func (cfg *apiCfg) HandleGetNonprofit(w http.ResponseWriter, r *http.Request) {
	idOrSlug := r.PathValue("idOrSlug")
	ctx := r.Context()

	var org database.Nonprofit
	var err error
	if id, perr := uuid.Parse(idOrSlug); perr == nil {
		org, err = cfg.DB.GetNonprofitByID(ctx, id)
	} else {
		org, err = cfg.DB.GetNonprofitBySlug(ctx, nullStr(idOrSlug))
	}
	if err != nil {
		cfg.handleDBError(w, err, "nonprofit")
		return
	}

	social, _ := cfg.DB.ListSocialLinks(ctx, org.ID)
	reviews, _ := cfg.DB.GetOrgReviewSummary(ctx, org.ID)

	// Latest verification row decides verified status (missing row → false).
	isVerified := false
	if v, verr := cfg.DB.GetNonprofitVerification(ctx, org.ID); verr == nil && v.IsVerified.Valid {
		isVerified = v.IsVerified.Bool
	}

	following := false
	if u, ok := auth.UserFromContext(ctx); ok {
		following, _ = cfg.DB.IsFollowingOrg(ctx, database.IsFollowingOrgParams{
			UserID: u.ID, NonprofitID: org.ID,
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]any{
		"profile":        org,
		"social_links":   social,
		"review_summary": reviews,
		"is_verified":    isVerified,
		"is_following":   following,
	})
}

// HandleFollowOrg follows a nonprofit.
//
//	POST /nonprofits/{id}/follow   (auth)
func (cfg *apiCfg) HandleFollowOrg(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid nonprofit id")
		return
	}
	_, err = cfg.DB.FollowOrg(r.Context(), database.FollowOrgParams{
		UserID: user.ID, NonprofitID: orgID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not follow")
		return
	}
	respondWithJSON(w, http.StatusNoContent, nil)
}

// HandleUnfollowOrg unfollows a nonprofit.
//
//	DELETE /nonprofits/{id}/follow   (auth)
func (cfg *apiCfg) HandleUnfollowOrg(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid nonprofit id")
		return
	}
	if err := cfg.DB.UnfollowOrg(r.Context(), database.UnfollowOrgParams{
		UserID: user.ID, NonprofitID: orgID,
	}); err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not unfollow")
		return
	}
	respondWithJSON(w, http.StatusNoContent, nil)
}

// HandleNonprofitReviews lists reviews for a nonprofit.
//
//	GET /nonprofits/{id}/reviews
func (cfg *apiCfg) HandleNonprofitReviews(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid nonprofit id")
		return
	}
	limit, offset := paginationParams(r, defaultPageLimit)
	reviews, err := cfg.DB.ListReviewsForOrg(r.Context(), database.ListReviewsForOrgParams{
		NonprofitID:  orgID,
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load reviews")
		return
	}
	respondWithJSON(w, http.StatusOK, reviews)
}

// HandleListNonprofitRaffles returns a nonprofit's public raffles for the org
// profile screen. The client splits the result into active/past sections, so
// this only ever exposes 'active' and 'completed' raffles (never drafts or
// cancelled ones).
//
//	GET /nonprofits/{id}/raffles?status=active|past|all   (optional auth)
func (cfg *apiCfg) HandleListNonprofitRaffles(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid nonprofit id")
		return
	}

	status := r.URL.Query().Get("status")
	switch status {
	case "active", "past", "all":
		// valid
	default:
		status = "all"
	}

	limit, offset := paginationParams(r, defaultPageLimit)

	rows, err := cfg.DB.ListNonprofitRaffles(r.Context(), database.ListNonprofitRafflesParams{
		NonprofitID:  orgID,
		StatusFilter: status,
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load raffles")
		return
	}
	if rows == nil {
		rows = []database.ListNonprofitRafflesRow{} // marshal [] not null for empty results
	}

	respondWithJSON(w, http.StatusOK, rows)
}
