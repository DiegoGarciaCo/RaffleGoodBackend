package draw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Notifier lets the worker push live updates without importing the realtime
// package (avoids an import cycle). *realtime.Hub satisfies this.
type Notifier interface {
	BroadcastDrawStarted(raffleID uuid.UUID, startedAt time.Time, durationMs int32)
	BroadcastDrawCompleted(raffleID uuid.UUID)
}

// Worker fires raffle draws when their draw_at time arrives.
type Worker struct {
	db       *database.Queries
	rawDB    *sql.DB
	notifier Notifier
	interval time.Duration
}

// NewWorker builds the draw worker. interval is the poll cadence (e.g. 30s).
func NewWorker(db *database.Queries, rawDB *sql.DB, notifier Notifier, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Worker{db: db, rawDB: rawDB, notifier: notifier, interval: interval}
}

// Run polls until ctx is cancelled. Start it as a goroutine from main.
func (wk *Worker) Run(ctx context.Context) {
	logrus.Infof("draw worker started (poll every %s)", wk.interval)
	ticker := time.NewTicker(wk.interval)
	defer ticker.Stop()

	// Run once promptly, then on each tick.
	wk.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			logrus.Info("draw worker stopped")
			return
		case <-ticker.C:
			wk.tick(ctx)
		}
	}
}

func (wk *Worker) tick(ctx context.Context) {
	ready, err := wk.db.ListRafflesReadyToDraw(ctx)
	if err != nil {
		logrus.WithError(err).Error("draw worker: list ready raffles failed")
		return
	}
	for _, raffle := range ready {
		if err := wk.fireDraw(ctx, raffle); err != nil {
			logrus.WithError(err).WithField("raffle_id", raffle.ID).
				Error("draw worker: fire draw failed")
		}
	}
}

// fireDraw computes and freezes a raffle's result, then notifies watchers and
// schedules completion after the reveal duration.
func (wk *Worker) fireDraw(ctx context.Context, raffle database.RaffleItem) error {
	// Load (or, as a dev fallback, create) the draw_results row that holds the
	// commitment + secret seed.
	dr, err := wk.db.GetDrawResultForRaffle(ctx, raffle.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("get draw result: %w", err)
		}
		dr, err = wk.devCreateDrawResult(ctx, raffle)
		if err != nil {
			return fmt.Errorf("dev-create draw result: %w", err)
		}
	}

	// Resolve the seed. In production the seed is stored at publish. If it's
	// missing (e.g. seed-data rows), generate one now — DEV ONLY, since this
	// breaks the published commitment.
	seed := dr.Seed.String
	if !dr.Seed.Valid || seed == "" {
		seed, err = GenerateSeed()
		if err != nil {
			return fmt.Errorf("generate seed: %w", err)
		}
		logrus.WithField("raffle_id", raffle.ID).
			Warn("draw worker: no stored seed; generated one (dev only — commitment will not verify)")
	}

	// Build the draw pool.
	rows, err := wk.db.ListTicketNumbersForRaffle(ctx, raffle.ID)
	if err != nil {
		return fmt.Errorf("list tickets: %w", err)
	}
	if len(rows) == 0 {
		// Nothing sold — just close it out.
		if err := wk.db.MarkRaffleDrawn(ctx, raffle.ID); err != nil {
			return fmt.Errorf("close empty raffle: %w", err)
		}
		return nil
	}
	entries := make([]Entry, len(rows))
	for i, r := range rows {
		entries[i] = Entry{TicketID: r.TicketID, TicketNumber: r.TicketNumber, UserID: r.UserID}
	}

	// Build the prize map.
	tierRows, err := wk.db.ListPrizeTiers(ctx, raffle.ID)
	if err != nil {
		return fmt.Errorf("list prize tiers: %w", err)
	}
	tiers := make(map[int32]PrizeTier, len(tierRows))
	for _, t := range tierRows {
		tiers[t.Rank] = PrizeTier{
			Rank:        t.Rank,
			Title:       t.Title,
			Description: t.Description.String,
			ValueCents:  t.ValueCents.Int32,
			HasValue:    t.ValueCents.Valid,
		}
	}

	cfg := Config{
		Strategy:    string(dr.DrawStrategy),
		WinnerCount: dr.WinnerCount,
		RevealOrder: dr.RevealOrder,
	}
	winners, durationMs := ComputeWinners(seed, entries, cfg, tiers)
	startedAt := time.Now().UTC()

	// Persist atomically: flip status, reveal seed + timing, insert winners.
	err = wk.withTx(ctx, func(qtx *database.Queries) error {
		if err := qtx.MarkRaffleDrawing(ctx, raffle.ID); err != nil {
			return fmt.Errorf("mark drawing: %w", err)
		}
		drawResult, err := qtx.StartDrawReveal(ctx, database.StartDrawRevealParams{
			RaffleItemID:   raffle.ID,
			DrawDurationMs: durationMs,
			Seed:           sql.NullString{String: seed, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("start reveal: %w", err)
		}
		for _, win := range winners {
			_, err := qtx.CreateDrawWinner(ctx, database.CreateDrawWinnerParams{
				DrawResultID:     drawResult.ID,
				PullIndex:        win.PullIndex,
				Rank:             win.Rank,
				TicketID:         win.TicketID,
				TicketNumber:     win.TicketNumber,
				UserID:           win.UserID,
				PrizeTitle:       win.PrizeTitle,
				PrizeDescription: sql.NullString{String: win.PrizeDescription, Valid: win.PrizeDescription != ""},
				PrizeValueCents:  sql.NullInt32{Int32: win.PrizeValueCents, Valid: win.PrizeHasValue},
				RevealedAtMs:     win.RevealedAtMs,
			})
			if err != nil {
				return fmt.Errorf("create winner: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	logrus.WithField("raffle_id", raffle.ID).WithField("winners", len(winners)).
		Info("draw fired")

	// Notify live watchers, then schedule completion after the reveal plays out.
	if wk.notifier != nil {
		wk.notifier.BroadcastDrawStarted(raffle.ID, startedAt, durationMs)
	}
	wk.scheduleCompletion(raffle.ID, time.Duration(durationMs)*time.Millisecond)
	return nil
}

// scheduleCompletion marks the draw completed once the reveal has played out.
func (wk *Worker) scheduleCompletion(raffleID uuid.UUID, after time.Duration) {
	time.AfterFunc(after, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := wk.db.CompleteDrawResult(ctx, raffleID); err != nil {
			logrus.WithError(err).WithField("raffle_id", raffleID).Error("complete draw result failed")
		}
		if err := wk.db.MarkRaffleDrawn(ctx, raffleID); err != nil {
			logrus.WithError(err).WithField("raffle_id", raffleID).Error("mark raffle drawn failed")
		}
		if wk.notifier != nil {
			wk.notifier.BroadcastDrawCompleted(raffleID)
		}
	})
}

// devCreateDrawResult creates a draw_results row on the fly for raffles that
// reached draw time without one. DEV-only convenience.
func (wk *Worker) devCreateDrawResult(ctx context.Context, raffle database.RaffleItem) (database.DrawResult, error) {
	seed, err := GenerateSeed()
	if err != nil {
		return database.DrawResult{}, err
	}
	participants, err := wk.db.CountParticipantsForRaffle(ctx, raffle.ID)
	if err != nil {
		participants = 0
	}
	strategy := string(raffle.DrawStrategy)
	winnerCount := int32(1)
	return wk.db.CreateDrawResultWithSeed(ctx, database.CreateDrawResultWithSeedParams{
		RaffleItemID:      raffle.ID,
		DrawStrategy:      raffle.DrawStrategy,
		RevealOrder:       RevealOrderFor(strategy),
		WinnerCount:       winnerCount,
		CommitmentHash:    Commitment(seed),
		Seed:              sql.NullString{String: seed, Valid: true},
		TotalTickets:      raffle.TicketsSold,
		TotalParticipants: participants,
	})
}

// withTx runs fn inside a transaction.
func (wk *Worker) withTx(ctx context.Context, fn func(qtx *database.Queries) error) error {
	tx, err := wk.rawDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(wk.db.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}
