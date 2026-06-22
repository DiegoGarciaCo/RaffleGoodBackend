package handlers

import (
	"net/http"

	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/google/uuid"
)

// HandleListRaffles is the main browse feed. Supports ?category=<path-prefix>
// to scope to a category subtree; otherwise returns all active raffles.
//
//	GET /raffles?limit=&offset=&category=
func (cfg *apiCfg) HandleListRaffles(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginationParams(r, defaultPageLimit)

	if cat := r.URL.Query().Get("category"); cat != "" {
		rows, err := cfg.DB.ListRafflesByCategory(r.Context(), database.ListRafflesByCategoryParams{
			CategoryPathPrefix: cat,
			ResultLimit:        limit,
			ResultOffset:       offset,
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "could not load raffles")
			return
		}
		respondWithJSON(w, http.StatusOK, rows)
		return
	}

	rows, err := cfg.DB.ListActiveRaffles(r.Context(), database.ListActiveRafflesParams{
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load raffles")
		return
	}
	respondWithJSON(w, http.StatusOK, rows)
}

// HandleFeaturedRaffles returns the most-popular active raffles for the home hero.
//
//	GET /raffles/featured?limit=
func (cfg *apiCfg) HandleFeaturedRaffles(w http.ResponseWriter, r *http.Request) {
	limit, _ := paginationParams(r, 5)
	rows, err := cfg.DB.ListFeaturedRaffles(r.Context(), limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load featured raffles")
		return
	}
	respondWithJSON(w, http.StatusOK, rows)
}

// HandleEndingSoonRaffles returns active raffles with the nearest draw dates.
//
//	GET /raffles/ending-soon?limit=
func (cfg *apiCfg) HandleEndingSoonRaffles(w http.ResponseWriter, r *http.Request) {
	limit, _ := paginationParams(r, 10)
	rows, err := cfg.DB.ListEndingSoonRaffles(r.Context(), limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load raffles")
		return
	}
	respondWithJSON(w, http.StatusOK, rows)
}

// HandleSearchRaffles runs a full-text search over title + description.
//
//	GET /raffles/search?q=&limit=&offset=
func (cfg *apiCfg) HandleSearchRaffles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		respondWithError(w, http.StatusBadRequest, "missing search query 'q'")
		return
	}
	limit, offset := paginationParams(r, defaultPageLimit)

	rows, err := cfg.DB.SearchRaffles(r.Context(), database.SearchRafflesParams{
		Query:        q,
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "search failed")
		return
	}

	// TODO: record a search_event (fire-and-forget) once events handler exists.
	respondWithJSON(w, http.StatusOK, rows)
}

// raffleDetailResponse bundles the raffle with its prize tiers. The raffle is
// typed as `any` because lookup-by-id and lookup-by-slug produce distinct
// sqlc row types with identical fields.
type raffleDetailResponse struct {
	Raffle     any                  `json:"raffle"`
	PrizeTiers []database.PrizeTier `json:"prize_tiers"`
}

// HandleGetRaffle returns one raffle (by UUID or slug) plus its prize tiers.
//
//	GET /raffles/{idOrSlug}
func (cfg *apiCfg) HandleGetRaffle(w http.ResponseWriter, r *http.Request) {
	idOrSlug := r.PathValue("idOrSlug")

	var raffle any
	var raffleID uuid.UUID

	if id, err := uuid.Parse(idOrSlug); err == nil {
		row, err := cfg.DB.GetRaffleByID(r.Context(), id)
		if err != nil {
			cfg.handleDBError(w, err, "raffle")
			return
		}
		raffle, raffleID = row, row.ID
	} else {
		row, err := cfg.DB.GetRaffleBySlug(r.Context(), idOrSlug)
		if err != nil {
			cfg.handleDBError(w, err, "raffle")
			return
		}
		raffle, raffleID = row, row.ID
	}

	tiers, err := cfg.DB.ListPrizeTiers(r.Context(), raffleID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load prizes")
		return
	}
	if tiers == nil {
		tiers = []database.PrizeTier{}
	}

	// TODO: record a raffle_view (fire-and-forget), using the optional user.
	respondWithJSON(w, http.StatusOK, raffleDetailResponse{Raffle: raffle, PrizeTiers: tiers})
}
