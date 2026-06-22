package handlers

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ──────────────────────────────────────────────────────────────────────────────
// Context keys
// ──────────────────────────────────────────────────────────────────────────────

type contextKey string

const (
	requestIDKey contextKey = "requestID"
	userIDKey    contextKey = "userID" // set by the auth middleware (step 2)
)

// RequestIDFromContext returns the per-request ID set by LoggerMiddleware.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// ──────────────────────────────────────────────────────────────────────────────
// Middleware type + chaining
// ──────────────────────────────────────────────────────────────────────────────

// Middleware is a standard net/http middleware: takes a handler, returns one.
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares to a handler. The first listed is the OUTERMOST
// (runs first on the way in, last on the way out):
//
//	Chain(h, Recovery, CORS, Logger) == Recovery(CORS(Logger(h)))
func Chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// ──────────────────────────────────────────────────────────────────────────────
// responseWriter — wraps http.ResponseWriter to capture status + bytes, and to
// forward optional interfaces (Hijacker for WebSockets, Flusher for streaming).
// ──────────────────────────────────────────────────────────────────────────────

type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	bytes       int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.statusCode = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		// Default to 200 if a handler writes without calling WriteHeader.
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

// Hijack lets the WebSocket upgrade take over the connection.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// Flush supports streaming responses (e.g. SSE) if you ever need it.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Recovery — catch panics so one bad request can't crash the server.
// Put this OUTERMOST so it also catches panics in other middleware.
// ──────────────────────────────────────────────────────────────────────────────

func (cfg *apiCfg) RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap the writer once here, at the outermost layer, so every inner
		// middleware and handler shares the same status/bytes tracking.
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		defer func() {
			if rec := recover(); rec != nil {
				logrus.WithFields(logrus.Fields{
					"panic":      rec,
					"path":       r.URL.Path,
					"method":     r.Method,
					"request_id": RequestIDFromContext(r.Context()),
				}).Error("recovered from panic")

				// Only try to write an error if nothing's been written yet.
				// (A hijacked WebSocket conn, for instance, can't take a JSON body.)
				if rw.wroteHeader {
					return
				}
				respondWithError(rw, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(rw, r)
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// CORS — permissive in dev, locked to the app origin in prod.
// ──────────────────────────────────────────────────────────────────────────────

func (cfg *apiCfg) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := "*"
		if !cfg.dev {
			// TODO: set to your production app/web origin(s)
			origin = "https://app.rafflegood.com"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Logger — request ID, timing, status, client IP. (Your original, upgraded.)
// ──────────────────────────────────────────────────────────────────────────────

func (cfg *apiCfg) LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Reuse an existing wrapper if an outer middleware already wrapped w,
		// so status/bytes are captured once and shared.
		rw, ok := w.(*responseWriter)
		if !ok {
			rw = &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		}

		// Request ID: honor an incoming one (from a gateway) or generate it.
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		rw.Header().Set("X-Request-Id", requestID)
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey, requestID))

		next.ServeHTTP(rw, r)

		clientIP := r.Header.Get("X-Forwarded-For")
		if clientIP == "" {
			clientIP = r.Header.Get("X-Real-IP")
		}
		if clientIP == "" {
			clientIP = r.RemoteAddr
		}

		fields := logrus.Fields{
			"request_id": requestID,
			"method":     r.Method,
			"path":       r.URL.Path,
			"status":     rw.statusCode,
			"bytes":      rw.bytes,
			"duration":   time.Since(start).Milliseconds(),
			"client_ip":  clientIP,
		}

		switch {
		case rw.statusCode >= 500:
			logrus.WithFields(fields).Error("request failed")
		case rw.statusCode >= 400:
			logrus.WithFields(fields).Warn("request error")
		default:
			logrus.WithFields(fields).Info("request processed")
		}
	})
}
