# Draw system — backend contract

This document describes what your Go backend must produce so the frontend
reveal engine and org config screens work. Read this before building the
draw endpoints.

## The core idea

A draw result is **computed once, frozen, then revealed gradually**.

- The server picks ALL winners atomically the instant the draw fires.
- It stores the full ordered result + reveal timings.
- Clients (live or replay) animate that frozen result. They never compute winners.
- This means a dropped connection, a late joiner, and a next-day replay all
  see the identical sequence.

## Lifecycle / status

```
scheduled  → in_progress → completed
(selling)    (revealing)    (final)
```

The draw fires automatically at `draw_at`. No manual trigger.

## Go structs (suggested)

```go
type DrawType string
const (
    DrawTypeSingle        DrawType = "single"
    DrawTypeRanked        DrawType = "ranked"
    DrawTypeMultipleEqual DrawType = "multiple_equal"
    DrawTypeCountdown     DrawType = "countdown"
)

type PrizeTier struct {
    Rank        int
    Title       string
    Description sql.NullString
    ValueCents  sql.NullInt64
    ImageURL    sql.NullString
}

type DrawWinner struct {
    PullIndex     int            // 0-based pull order
    Rank          int            // 1 = grand prize
    TicketNumber  int
    UserID        uuid.UUID
    MaskedName    string         // "J••• D•••"
    Prize         PrizeTier
    RevealedAtMs  int            // ms offset from draw start
}

type DrawResult struct {
    RaffleID          uuid.UUID
    Status            string         // scheduled | in_progress | completed
    DrawType          DrawType
    RevealOrder       string         // forward | reverse
    WinnerCount       int
    DrawStartedAt     sql.NullTime
    DrawDurationMs    int
    Winners           []DrawWinner
    TotalTickets      int
    TotalParticipants int
    Seed              sql.NullString // revealed AFTER draw
    VerificationURL   sql.NullString
}
```

## Provable fairness (commit-reveal)

1. When the raffle is **published**, generate a secret random `seed`.
   Store it. Publish `commitment_hash = sha256(seed)` on the raffle.
2. Participants buy tickets. Each ticket gets a fixed sequential number.
3. At `draw_at`, the server:
   - loads the seed
   - computes winners deterministically:
     `winning_order = shuffle(ticket_numbers, rng_seeded_by(seed + raffle_id))`
   - takes the first `winner_count` as winners
   - assigns ranks (forward: pull 0 = rank 1; reverse: last pull = rank 1)
   - freezes the `DrawResult`
   - reveals `seed` in the result
4. Anyone can verify: `sha256(revealed_seed) == commitment_hash`, then
   re-run the same shuffle and confirm they get the same winners.

The seed must be committed BEFORE ticket sales close so neither the org nor
the platform can pick a seed that favors a chosen ticket.

## Reveal timing — how live + replay both work

Each `DrawWinner.RevealedAtMs` is the offset (in ms) from `DrawStartedAt`
when that winner should appear on screen. The server sets these to pace the
drum animation, e.g. for 3 winners with a 4s drum spin between each:

```
winner[0].revealed_at_ms = 4000
winner[1].revealed_at_ms = 9000
winner[2].revealed_at_ms = 14000
draw_duration_ms         = 16000
```

Client logic:
- `elapsed = now - draw_started_at`
- **Live, mid-draw** (`status = in_progress`): show winners where
  `revealed_at_ms <= elapsed`, schedule the rest with setTimeout against
  the remaining offset. If the user joined late, they instantly catch up to
  the right point, then watch the rest live.
- **Completed**: replay from `elapsed = 0`, animating the whole sequence
  fresh. (Or offer a "skip to results" button.)

## Endpoints needed

```
GET  /raffles/:id/draw            → DrawResult (poll or websocket for status changes)
GET  /raffles/:id/draw/live       → DrawLiveState (watcher count; websocket ideal)
POST /raffles/:id/draw/config     → save DrawConfig (org only, before publish)
GET  /raffles/:id/draw/verify     → verification data (seed, formula, recompute)
```

A websocket on the draw channel is ideal for live sync (push status →
in_progress, push watcher counts). Polling every 2-3s is an acceptable
fallback for v1.

## Privacy

`masked_name` should partially obscure non-winning-viewers' identities,
e.g. show first + last initial only. The viewing user's own entry can be
shown in full (the client gets `is_current_user` computed server-side from
the authed user).
