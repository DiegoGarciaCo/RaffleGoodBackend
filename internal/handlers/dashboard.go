package handlers

import (
	"net/http"
	"time"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/google/uuid"
)

// HandleDashboard returns the nonprofit dashboard payload: this-month money,
// today's tickets, all-time stats, active raffles, and the recent activity feed.
//
//	GET /nonprofits/me/dashboard   (org only)
func (cfg *apiCfg) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	orgID := orgNullable(user)
	ctx := r.Context()

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	hourAgo := now.Add(-time.Hour)

	allTime, err := cfg.DB.GetNonprofitAllTimeStats(ctx, orgID.UUID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load stats")
		return
	}

	month, err := cfg.DB.GetNonprofitMonthStats(ctx, database.GetNonprofitMonthStatsParams{
		NonprofitID: orgID,
		WindowStart: monthStart,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load month stats")
		return
	}

	ticketsToday, _ := cfg.DB.GetTicketsSoldSince(ctx, database.GetTicketsSoldSinceParams{
		NonprofitID: orgID,
		Since:       dayStart,
	})
	ticketsLastHour, _ := cfg.DB.GetTicketsSoldSince(ctx, database.GetTicketsSoldSinceParams{
		NonprofitID: orgID,
		Since:       hourAgo,
	})

	active, err := cfg.DB.ListActiveRafflesForNonprofit(ctx, database.ListActiveRafflesForNonprofitParams{
		NonprofitID: orgID,
		ResultLimit: 10,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load active raffles")
		return
	}

	activity, err := cfg.DB.ListActivityForNonprofit(ctx, database.ListActivityForNonprofitParams{
		NonprofitID:  orgID,
		ResultLimit:  15,
		ResultOffset: 0,
	})
	if err != nil {
		activity = nil // feed is non-critical
	}

	unclaimed, _ := cfg.DB.ListUnclaimedPrizesForNonprofit(ctx, orgID)

	respondWithJSON(w, http.StatusOK, map[string]any{
		"this_month": map[string]any{
			"raised":            month.Raised,
			"tickets_sold":      month.TicketsSold,
			"tickets_today":     ticketsToday,
			"tickets_last_hour": ticketsLastHour,
		},
		"all_time": map[string]any{
			"total_raised_cents": allTime.TotalRaisedCents,
			"raffles_run":        allTime.RafflesRun,
			"follower_count":     allTime.FollowerCount,
			"total_tickets_sold": allTime.TotalTicketsSold,
		},
		"active_raffles":   active,
		"recent_activity":  activity,
		"unclaimed_prizes": unclaimed,
	})
}

// orgNullable wraps the user's nonprofit id as a uuid.NullUUID for queries whose
// param compares against the nullable created_by column.
func orgNullable(u *auth.User) uuid.NullUUID {
	return uuid.NullUUID{UUID: nonprofitID(u), Valid: true}
}
