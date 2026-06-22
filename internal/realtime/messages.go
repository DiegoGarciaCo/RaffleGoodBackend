// Package realtime holds the in-memory WebSocket hub used for live draws.
// Watcher counts are per-instance (no Redis); fine for a single API process.
package realtime

import "time"

// MessageType identifies the kind of payload pushed to draw watchers.
type MessageType string

const (
	// MsgStatusChange tells watchers the draw started or completed. The client
	// already holds the frozen result and animates locally from DrawStartedAt.
	MsgStatusChange MessageType = "status_change"
	// MsgWatcherCount updates the live viewer count.
	MsgWatcherCount MessageType = "watcher_count"
)

// Message is the envelope sent over the socket.
type Message struct {
	Type    MessageType `json:"type"`
	Payload any         `json:"payload"`
}

// StatusChangePayload accompanies MsgStatusChange.
type StatusChangePayload struct {
	Status         string `json:"status"` // in_progress | completed
	DrawStartedAt  string `json:"draw_started_at,omitempty"`
	DrawDurationMs int32  `json:"draw_duration_ms,omitempty"`
}

// WatcherCountPayload accompanies MsgWatcherCount.
type WatcherCountPayload struct {
	Count int `json:"count"`
}

// NewStatusChange builds a status_change message.
func NewStatusChange(status string, startedAt time.Time, durationMs int32) Message {
	p := StatusChangePayload{Status: status, DrawDurationMs: durationMs}
	if !startedAt.IsZero() {
		p.DrawStartedAt = startedAt.UTC().Format(time.RFC3339)
	}
	return Message{Type: MsgStatusChange, Payload: p}
}

// NewWatcherCount builds a watcher_count message.
func NewWatcherCount(count int) Message {
	return Message{Type: MsgWatcherCount, Payload: WatcherCountPayload{Count: count}}
}
