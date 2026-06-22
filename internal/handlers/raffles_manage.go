package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/diegoGarciaCo/raffles/internal/draw"
	"github.com/google/uuid"
)

// createRaffleRequest mirrors the wizard's buildPayload output.
type createRaffleRequest struct {
	CategoryID           string           `json:"category_id"`
	Title                string           `json:"title"`
	Description          string           `json:"description"`
	Status               string           `json:"status"` // "draft" or "active"
	TicketStrategy       string           `json:"ticket_strategy"`
	TicketAssignment     string           `json:"ticket_assignment_strategy"`
	DrawStrategy         string           `json:"draw_strategy"`
	TicketPrice          string           `json:"ticket_price"` // "0.00" etc
	FreeTickets          int32            `json:"free_tickets"`
	FreeTicketStartRange int32            `json:"free_ticket_start_range"`
	FreeTicketEndRange   int32            `json:"free_ticket_end_range"`
	MaxTickets           int32            `json:"max_tickets"`
	Attributes           json.RawMessage  `json:"attributes"`
	ImageUrls            []string         `json:"image_urls"`
	PrizeTiers           []prizeTierInput `json:"prize_tiers"`
	WinnerCount          int32            `json:"winner_count"`
}

type prizeTierInput struct {
	Rank        int32  `json:"rank"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ValueCents  int32  `json:"value_cents"`
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// HandleCreateRaffle creates a raffle (draft or active). On publish (active) it
// generates the fairness seed, stores sha256(seed) as the commitment, and
// writes the prize tiers — all atomically.
//
//	POST /raffles   (org only)
func (cfg *apiCfg) HandleCreateRaffle(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	var req createRaffleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		respondWithError(w, http.StatusBadRequest, "title is required")
		return
	}
	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid category_id")
		return
	}
	status := req.Status
	if status != "active" {
		status = "draft"
	}

	// Unique-ish slug: base + short suffix.
	slug := slugify(req.Title)
	if slug == "" {
		slug = "raffle"
	}
	slug = fmt.Sprintf("%s-%s", slug, uuid.NewString()[:6])

	attrs := req.Attributes
	if len(attrs) == 0 {
		attrs = json.RawMessage(`{}`)
	}

	var created database.RaffleItem

	err = cfg.withTx(r.Context(), func(qtx *database.Queries) error {
		raffle, err := qtx.CreateRaffle(r.Context(), database.CreateRaffleParams{
			CategoryID:               categoryID,
			Title:                    req.Title,
			Slug:                     nullStr(slug),
			Description:              nullStr(req.Description),
			Status:                   database.RaffleItemStatus(status),
			TicketStrategy:           database.TicketStrategy(req.TicketStrategy),
			TicketAssignmentStrategy: database.TicketAssignmentStrategy(req.TicketAssignment),
			DrawStrategy:             database.DrawStrategy(req.DrawStrategy),
			TicketPrice:              nullStr(req.TicketPrice),
			FreeTickets:              req.FreeTickets,
			FreeTicketStartRange:     nullInt32(req.FreeTicketStartRange, req.FreeTicketStartRange > 0),
			FreeTicketEndRange:       nullInt32(req.FreeTicketEndRange, req.FreeTicketEndRange > 0),
			MaxTickets:               nullInt32(req.MaxTickets, req.MaxTickets > 0),
			Attributes:               attrs,
			ImageUrls:                req.ImageUrls,
			DrawAt:                   nullTimeZero(), // set later when sold out
			CreatedBy:                uuid.NullUUID{UUID: nonprofitID(user), Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create raffle: %w", err)
		}
		created = raffle

		// Prize tiers.
		for _, t := range req.PrizeTiers {
			if strings.TrimSpace(t.Title) == "" {
				continue
			}
			_, err := qtx.CreatePrizeTier(r.Context(), database.CreatePrizeTierParams{
				RaffleItemID: raffle.ID,
				Rank:         t.Rank,
				Title:        t.Title,
				Description:  nullStr(t.Description),
				ValueCents:   nullInt32(t.ValueCents, t.ValueCents > 0),
				ImageUrl:     nullStr(""),
			})
			if err != nil {
				return fmt.Errorf("create prize tier: %w", err)
			}
		}

		// On publish, commit the fairness seed BEFORE sales open.
		if status == "active" {
			if err := cfg.prepareDrawResult(r.Context(), qtx, raffle, req.WinnerCount); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not create raffle: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, created)
}

// HandlePublishRaffle flips a draft to active and commits the fairness seed.
//
//	POST /raffles/{id}/publish   (org only)
func (cfg *apiCfg) HandlePublishRaffle(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	raffleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid raffle id")
		return
	}

	var published database.RaffleItem
	err = cfg.withTx(r.Context(), func(qtx *database.Queries) error {
		raffle, err := qtx.PublishRaffle(r.Context(), database.PublishRaffleParams{
			RaffleID:    raffleID,
			NonprofitID: uuid.NullUUID{UUID: nonprofitID(user), Valid: true},
		})
		if err != nil {
			return err
		}
		published = raffle
		// winner_count comes from the existing prize tiers / draw config; default 1.
		return cfg.prepareDrawResult(r.Context(), qtx, raffle, 0)
	})
	if err != nil {
		cfg.handleDBError(w, err, "raffle")
		return
	}
	respondWithJSON(w, http.StatusOK, published)
}

// prepareDrawResult generates the seed + commitment and stores the scheduled
// draw_results row. This is the real commit-reveal: the commitment is published
// now (before sales), the secret seed is stored, revealed only after the draw.
func (cfg *apiCfg) prepareDrawResult(ctx context.Context, qtx *database.Queries, raffle database.RaffleItem, winnerCount int32) error {
	seed, err := draw.GenerateSeed()
	if err != nil {
		return fmt.Errorf("generate seed: %w", err)
	}
	if winnerCount < 1 {
		winnerCount = 1
	}
	_, err = qtx.CreateDrawResultWithSeed(ctx, database.CreateDrawResultWithSeedParams{
		RaffleItemID:      raffle.ID,
		DrawStrategy:      raffle.DrawStrategy,
		RevealOrder:       draw.RevealOrderFor(string(raffle.DrawStrategy)),
		WinnerCount:       winnerCount,
		CommitmentHash:    draw.Commitment(seed),
		Seed:              nullStr(seed),
		TotalTickets:      0,
		TotalParticipants: 0,
	})
	if err != nil {
		return fmt.Errorf("prepare draw result: %w", err)
	}
	return nil
}

// HandleListMyRaffles lists the org's raffles, optionally filtered by status.
//
//	GET /nonprofits/me/raffles?status=&limit=&offset=
func (cfg *apiCfg) HandleListMyRaffles(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	limit, offset := paginationParams(r, defaultPageLimit)
	orgID := nonprofitID(user)

	if status := r.URL.Query().Get("status"); status != "" && status != "all" {
		rows, err := cfg.DB.ListRafflesForNonprofitByStatus(r.Context(), database.ListRafflesForNonprofitByStatusParams{
			NonprofitID:  uuid.NullUUID{UUID: orgID, Valid: true},
			Status:       database.RaffleItemStatus(status),
			ResultLimit:  limit,
			ResultOffset: offset,
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "could not load raffles")
			return
		}
		respondWithJSON(w, http.StatusOK, rows)
		return
	}

	rows, err := cfg.DB.ListRafflesForNonprofit(r.Context(), database.ListRafflesForNonprofitParams{
		NonprofitID:  uuid.NullUUID{UUID: orgID, Valid: true},
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not load raffles")
		return
	}
	respondWithJSON(w, http.StatusOK, rows)
}

// HandleDeleteRaffle deletes a draft raffle the org owns.
//
//	DELETE /raffles/{id}   (org only)
func (cfg *apiCfg) HandleDeleteRaffle(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	raffleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid raffle id")
		return
	}
	err = cfg.DB.DeleteRaffle(r.Context(), database.DeleteRaffleParams{
		RaffleID:    raffleID,
		NonprofitID: uuid.NullUUID{UUID: nonprofitID(user), Valid: true},
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not delete raffle")
		return
	}
	respondWithJSON(w, http.StatusNoContent, nil)
}

// nonprofitID returns the user's nonprofit UUID (zero value if none).
func nonprofitID(u *auth.User) uuid.UUID {
	if u != nil && u.NonprofitID.Valid {
		return u.NonprofitID.UUID
	}
	return uuid.Nil
}
