package draw

import "github.com/google/uuid"

// Reveal pacing (milliseconds). The result is frozen; these timestamps tell the
// client when to reveal each winner relative to draw_started_at.
const (
	revealLeadMs = 2000 // drum spins before the first reveal
	revealGapMs  = 2500 // between successive reveals
	revealTailMs = 2000 // celebration after the last reveal
)

// PrizeTier is the prize for a given rank.
type PrizeTier struct {
	Rank        int32
	Title       string
	Description string
	ValueCents  int32
	HasValue    bool
}

// Config describes how winners are drawn.
type Config struct {
	Strategy    string // single | ranked | multiple_equal | countdown
	WinnerCount int32
	RevealOrder string // forward | reverse
}

// Winner is a computed, frozen draw result row.
type Winner struct {
	PullIndex        int32
	Rank             int32
	TicketID         uuid.UUID
	TicketNumber     int32
	UserID           uuid.UUID
	PrizeTitle       string
	PrizeDescription string
	PrizeValueCents  int32
	PrizeHasValue    bool
	RevealedAtMs     int32
}

// RevealOrderFor returns the reveal direction for a strategy. Countdown reveals
// in reverse (grand prize last); everything else reveals grand prize first.
func RevealOrderFor(strategy string) string {
	if strategy == "countdown" {
		return "reverse"
	}
	return "forward"
}

// needsPrizePerRank reports whether each rank has a distinct prize.
func needsPrizePerRank(strategy string) bool {
	return strategy == "ranked" || strategy == "countdown"
}

// ComputeWinners produces the frozen, ordered winners and the total reveal
// duration. The ordering comes entirely from the seed via Permute, so the
// result is deterministic and verifiable.
//
// tiers maps rank -> prize. For single / multiple_equal, every winner shares
// the rank-1 prize. For ranked / countdown, each rank gets its own prize.
func ComputeWinners(seed string, entries []Entry, cfg Config, tiers map[int32]PrizeTier) (winners []Winner, durationMs int32) {
	perm := Permute(seed, entries)

	n := int(cfg.WinnerCount)
	if n > len(perm) {
		n = len(perm)
	}
	if n < 0 {
		n = 0
	}

	reverse := cfg.RevealOrder == "reverse"
	perRank := needsPrizePerRank(cfg.Strategy)

	winners = make([]Winner, 0, n)
	for i := 0; i < n; i++ {
		e := perm[i]

		// Rank: forward → pull i gets rank i+1; reverse → pull i gets rank n-i
		// (so the last revealed is rank 1, the grand prize).
		var rank int32
		if reverse {
			rank = int32(n - i)
		} else {
			rank = int32(i + 1)
		}

		tier := tiers[rank]
		if !perRank {
			tier = tiers[1]
		}

		winners = append(winners, Winner{
			PullIndex:        int32(i),
			Rank:             rank,
			TicketID:         e.TicketID,
			TicketNumber:     e.TicketNumber,
			UserID:           e.UserID,
			PrizeTitle:       tier.Title,
			PrizeDescription: tier.Description,
			PrizeValueCents:  tier.ValueCents,
			PrizeHasValue:    tier.HasValue,
			RevealedAtMs:     int32(revealLeadMs + i*revealGapMs),
		})
	}

	if n == 0 {
		return winners, revealLeadMs + revealTailMs
	}
	durationMs = int32(revealLeadMs + (n-1)*revealGapMs + revealTailMs)
	return winners, durationMs
}
