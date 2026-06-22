package realtime

import (
	"context"
	"time"

	"github.com/coder/websocket"
)

// writeWait bounds how long a single write may take.
const writeWait = 10 * time.Second

// Send queues a message to this client (used to push initial state on connect).
func (c *Client) Send(msg Message) {
	data, err := marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

// WritePump delivers queued messages to the socket until send is closed
// (by Hub.Leave) or the context is cancelled.
func (c *Client) WritePump(ctx context.Context, conn *websocket.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-c.send:
			if !ok {
				// Hub closed the channel → graceful close.
				_ = conn.Close(websocket.StatusNormalClosure, "")
				return
			}
			wctx, cancel := context.WithTimeout(ctx, writeWait)
			err := conn.Write(wctx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// ReadPump drains inbound frames. Watchers don't send anything meaningful, but
// we must read to process control frames (ping/close) and detect disconnects.
// It blocks until the connection closes; the caller then leaves the hub.
func (c *Client) ReadPump(ctx context.Context, conn *websocket.Conn) {
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}
