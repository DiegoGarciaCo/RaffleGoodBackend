package handlers

import (
	"net/http"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/google/uuid"
)

// HandleListMyTickets returns the signed-in user's tickets grouped by raffle.
// The frontend splits these into Active / Won / Past using each raffle's status.
//
//	GET /users/me/tickets   (auth required)
func (cfg *apiCfg) HandleListMyTickets(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	groups, err := cfg.DB.ListUserTicketsGrouped(r.Context(), user.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load tickets")
		return
	}
	respondWithJSON(w, http.StatusOK, groups)
}

// HandleMyTicketStats returns the four header stats for the My Tickets screen.
//
//	GET /users/me/tickets/stats   (auth required)
func (cfg *apiCfg) HandleMyTicketStats(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	stats, err := cfg.DB.GetUserTicketStats(r.Context(), user.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load ticket stats")
		return
	}

	wins, err := cfg.DB.CountUserWins(r.Context(), user.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load win count")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]any{
		"total_tickets_purchased": stats.TotalTicketsPurchased,
		"total_raffles_entered":   stats.TotalRafflesEntered,
		"total_spent":             stats.TotalSpent,
		"total_wins":              wins,
	})
}

// HandleListMyWins returns the user's winning entries (the Won tab detail).
//
//	GET /users/me/wins   (auth required)
func (cfg *apiCfg) HandleListMyWins(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	wins, err := cfg.DB.ListUserWins(r.Context(), user.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load wins")
		return
	}
	respondWithJSON(w, http.StatusOK, wins)
}

// HandleClaimPrize marks a winning entry as claimed. The query is scoped to the
// authenticated user, so a user can only claim their own prize.
//
//	POST /tickets/winners/{winnerId}/claim   (auth required)
func (cfg *apiCfg) HandleClaimPrize(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	winnerID, err := uuid.Parse(r.PathValue("winnerId"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid winner id")
		return
	}

	claimed, err := cfg.DB.ClaimPrize(r.Context(), database.ClaimPrizeParams{
		WinnerID: winnerID,
		UserID:   user.ID,
	})
	if err != nil {
		// No row updated → either not found or not this user's prize.
		cfg.handleDBError(w, err, "prize")
		return
	}
	respondWithJSON(w, http.StatusOK, claimed)
}

// HandleGetReceipt returns an order by its receipt number for the receipt view.
//
//	GET /orders/{receiptNumber}/receipt   (auth required)
func (cfg *apiCfg) HandleGetReceipt(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	receiptNumber := r.PathValue("receiptNumber")

	order, err := cfg.DB.GetOrderByReceiptNumber(r.Context(), nullStr(receiptNumber))
	if err != nil {
		cfg.handleDBError(w, err, "receipt")
		return
	}
	// Don't leak another user's receipt.
	if order.UserID != user.ID {
		respondWithError(w, http.StatusForbidden, "not your receipt")
		return
	}
	respondWithJSON(w, http.StatusOK, order)
}
