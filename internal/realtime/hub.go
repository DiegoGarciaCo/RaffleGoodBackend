package realtime

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Client is one watcher's connection. The hub never touches the socket
// directly; it pushes JSON bytes onto send, and the client's write pump
// (in client.go) delivers them.
type Client struct {
	send chan []byte
}

// Hub tracks watchers per raffle and broadcasts messages to them. All state is
// in-memory and per-instance: watcher counts reflect this server only.
type Hub struct {
	mu    sync.RWMutex
	rooms map[uuid.UUID]map[*Client]struct{}
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{rooms: make(map[uuid.UUID]map[*Client]struct{})}
}

// NewClient makes a client with a buffered send channel.
func (h *Hub) NewClient() *Client {
	return &Client{send: make(chan []byte, 16)}
}

// Join adds a client to a raffle's room and broadcasts the new watcher count.
func (h *Hub) Join(raffleID uuid.UUID, c *Client) {
	h.mu.Lock()
	room := h.rooms[raffleID]
	if room == nil {
		room = make(map[*Client]struct{})
		h.rooms[raffleID] = room
	}
	room[c] = struct{}{}
	count := len(room)
	h.mu.Unlock()

	h.Broadcast(raffleID, NewWatcherCount(count))
}

// Leave removes a client and broadcasts the updated count. Safe to call twice.
func (h *Hub) Leave(raffleID uuid.UUID, c *Client) {
	h.mu.Lock()
	room := h.rooms[raffleID]
	if room != nil {
		if _, ok := room[c]; ok {
			delete(room, c)
			close(c.send)
		}
		if len(room) == 0 {
			delete(h.rooms, raffleID)
		}
	}
	count := 0
	if room != nil {
		count = len(room)
	}
	h.mu.Unlock()

	if count > 0 {
		h.Broadcast(raffleID, NewWatcherCount(count))
	}
}

// Count returns the current watcher count for a raffle (this instance).
func (h *Hub) Count(raffleID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[raffleID])
}

// marshal is a small helper shared by Client.Send.
func marshal(msg Message) ([]byte, error) {
	return json.Marshal(msg)
}

// Broadcast sends a message to every watcher of a raffle. Slow clients whose
// buffers are full are skipped (dropped frame) rather than blocking the hub —
// the client animates from the frozen result anyway, so a missed frame is safe.
func (h *Hub) Broadcast(raffleID uuid.UUID, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	room := h.rooms[raffleID]
	clients := make([]*Client, 0, len(room))
	for c := range room {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		select {
		case c.send <- data:
		default:
			// buffer full → drop this frame for this client
		}
	}
}

// ─── draw.Notifier adapter ────────────────────────────────────────────────────
// These let the draw worker notify watchers without importing realtime types
// (it depends only on a small interface it defines).

// BroadcastDrawStarted notifies watchers a draw is now in progress.
func (h *Hub) BroadcastDrawStarted(raffleID uuid.UUID, startedAt time.Time, durationMs int32) {
	h.Broadcast(raffleID, NewStatusChange("in_progress", startedAt, durationMs))
}

// BroadcastDrawCompleted notifies watchers a draw has finished.
func (h *Hub) BroadcastDrawCompleted(raffleID uuid.UUID) {
	h.Broadcast(raffleID, NewStatusChange("completed", time.Time{}, 0))
}
