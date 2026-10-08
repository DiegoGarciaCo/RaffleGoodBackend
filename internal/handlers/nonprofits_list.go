package handlers

import (
	"database/sql"
	"net/http"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/google/uuid"
)

// nonprofitListItem matches the frontend's NonprofitListItem shape exactly.
type nonprofitListItem struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   sql.NullString `json:"description"`
	Image         sql.NullString `json:"image"`
	Ein           sql.NullString `json:"ein"`
	Categories    []string       `json:"categories"`
	IsVerified    bool           `json:"is_verified"`
	IsFollowing   bool           `json:"is_following"`
	ActiveRaffles int32          `json:"active_raffles"`
	TotalRaised   int64          `json:"total_raised"` // dollars
	FollowerCount int32          `json:"follower_count"`
}

// HandleListNonprofits lists/searches nonprofits for the Explore orgs tab.
//
//	GET /nonprofits?q=&limit=&offset=&sort=   (optional auth → is_following)
//	sort: most_followed (default) | most_raised | most_active | alphabetical
func (cfg *apiCfg) HandleListNonprofits(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginationParams(r, defaultPageLimit)

	search := sql.NullString{}
	if q := r.URL.Query().Get("q"); q != "" {
		search = sql.NullString{String: q, Valid: true}
	}

	viewer := uuid.NullUUID{}
	if u, ok := auth.UserFromContext(r.Context()); ok {
		viewer = uuid.NullUUID{UUID: u.ID, Valid: true}
	}

	rows, err := cfg.DB.ListNonprofits(r.Context(), database.ListNonprofitsParams{
		ViewerID:     viewer,
		Search:       search,
		Sort:         r.URL.Query().Get("sort"),
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load organizations")
		return
	}

	out := make([]nonprofitListItem, 0, len(rows))
	for _, n := range rows {
		cats := n.Categories
		if cats == nil {
			cats = []string{}
		}
		out = append(out, nonprofitListItem{
			ID:            n.ID.String(),
			Name:          n.Name,
			Description:   n.Description,
			Image:         n.Image,
			Ein:           n.Ein,
			Categories:    cats,
			IsVerified:    n.IsVerified,
			IsFollowing:   n.IsFollowing,
			ActiveRaffles: n.ActiveRaffles,
			TotalRaised:   int64(n.TotalRaisedCents) / 100,
			FollowerCount: n.FollowerCount,
		})
	}

	respondWithJSON(w, http.StatusOK, out)
}
