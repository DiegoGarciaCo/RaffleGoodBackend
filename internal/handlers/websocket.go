package handlers

import (
	"net/http"

	"github.com/coder/websocket"
	"github.com/diegoGarciaCo/raffles/internal/realtime"
	"github.com/google/uuid"
)

// HandleDrawWebSocket upgrades to a WebSocket and streams live draw events
// (status changes + watcher count) for one raffle. The client already holds the
// frozen result and animates locally, so this channel only carries small state
// signals and is safe to drop/reconnect.
//
//	GET /raffles/{id}/draw/ws
func (cfg *apiCfg) HandleDrawWebSocket(w http.ResponseWriter, r *http.Request) {
	if cfg.Hub == nil {
		respondWithError(w, http.StatusServiceUnavailable, "live draws unavailable")
		return
	}

	raffleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid raffle id")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Mobile clients don't send an Origin header. For web clients you should
		// instead set OriginPatterns to your app's domain(s).
		InsecureSkipVerify: true,
	})
	if err != nil {
		return // Accept already wrote the error
	}

	client := cfg.Hub.NewClient()
	cfg.Hub.Join(raffleID, client)
	defer func() {
		cfg.Hub.Leave(raffleID, client)
		_ = conn.CloseNow()
	}()

	ctx := r.Context()

	// Push initial state so a late joiner starts animating immediately.
	if result, err := cfg.DB.GetDrawResultForRaffle(ctx, raffleID); err == nil {
		if result.Status == "in_progress" {
			startedAt := result.DrawStartedAt.Time
			client.Send(realtime.NewStatusChange("in_progress", startedAt, result.DrawDurationMs))
		}
	}
	client.Send(realtime.NewWatcherCount(cfg.Hub.Count(raffleID)))

	// Write pump in the background; read pump blocks until the client disconnects.
	go client.WritePump(ctx, conn)
	client.ReadPump(ctx, conn)
}
