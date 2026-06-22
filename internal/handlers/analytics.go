package handlers

import (
	"net/http"
	"time"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
)

// rangeToWindow converts a range key to a [start, end) window and the matching
// previous window (for trend comparison).
func rangeToWindow(rangeKey string, now time.Time) (start, end, prevStart, prevEnd time.Time) {
	end = now
	var d time.Duration
	switch rangeKey {
	case "7d":
		d = 7 * 24 * time.Hour
	case "90d":
		d = 90 * 24 * time.Hour
	case "year":
		d = 365 * 24 * time.Hour
	case "all":
		start = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		return start, end, start, start
	default: // "30d"
		d = 30 * 24 * time.Hour
	}
	start = now.Add(-d)
	prevEnd = start
	prevStart = start.Add(-d)
	return start, end, prevStart, prevEnd
}

func trendPct(current, previous float64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return ((current - previous) / previous) * 100
}

// HandleAnalytics returns the analytics payload for the selected range.
//
//	GET /nonprofits/me/analytics?range=7d|30d|90d|year|all   (org only)
func (cfg *apiCfg) HandleAnalytics(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	org := orgNullable(user)
	ctx := r.Context()

	rangeKey := r.URL.Query().Get("range")
	if rangeKey == "" {
		rangeKey = "30d"
	}
	now := time.Now().UTC()
	start, end, prevStart, prevEnd := rangeToWindow(rangeKey, now)

	// Current + previous KPI windows for trends.
	cur, err := cfg.DB.GetAnalyticsKPIs(ctx, database.GetAnalyticsKPIsParams{
		NonprofitID: org, WindowStart: start, WindowEnd: end,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load analytics")
		return
	}
	prev, _ := cfg.DB.GetAnalyticsKPIs(ctx, database.GetAnalyticsKPIsParams{
		NonprofitID: org, WindowStart: prevStart, WindowEnd: prevEnd,
	})

	newFollowers, _ := cfg.DB.GetNewFollowersInWindow(ctx, database.GetNewFollowersInWindowParams{
		NonprofitID: org.UUID, WindowStart: start, WindowEnd: end,
	})
	prevFollowers, _ := cfg.DB.GetNewFollowersInWindow(ctx, database.GetNewFollowersInWindowParams{
		NonprofitID: org.UUID, WindowStart: prevStart, WindowEnd: prevEnd,
	})

	// Revenue series: daily for short ranges, monthly for long ones.
	var series any
	if rangeKey == "year" || rangeKey == "all" {
		series, _ = cfg.DB.GetRevenueByMonth(ctx, database.GetRevenueByMonthParams{
			NonprofitID: org, WindowStart: start,
		})
	} else {
		series, _ = cfg.DB.GetRevenueByDay(ctx, database.GetRevenueByDayParams{
			NonprofitID: org, WindowStart: start, WindowEnd: end,
		})
	}

	topRaffles, _ := cfg.DB.GetTopRaffles(ctx, database.GetTopRafflesParams{
		NonprofitID: org, WindowStart: start, ResultLimit: 5,
	})

	breakdown, _ := cfg.DB.GetBuyerBreakdown(ctx, database.GetBuyerBreakdownParams{
		NonprofitID: org.UUID, WindowStart: start, WindowEnd: end,
	})

	conversion, _ := cfg.DB.GetConversionStats(ctx, database.GetConversionStatsParams{
		NonprofitID: org, WindowStart: start, WindowEnd: end,
	})

	respondWithJSON(w, http.StatusOK, map[string]any{
		"range": rangeKey,
		"kpis": map[string]any{
			"total_raised":        cur.TotalRaised,
			"raised_trend_pct":    round1(trendPct(numericToFloat(cur.TotalRaised), numericToFloat(prev.TotalRaised))),
			"tickets_sold":        cur.TicketsSold,
			"tickets_trend_pct":   round1(trendPct(float64(cur.TicketsSold), float64(prev.TicketsSold))),
			"unique_buyers":       cur.UniqueBuyers,
			"new_followers":       newFollowers,
			"followers_trend_pct": round1(trendPct(float64(newFollowers), float64(prevFollowers))),
		},
		"revenue_series":  series,
		"top_raffles":     topRaffles,
		"buyer_breakdown": breakdown,
		"conversion":      conversion,
	})
}
