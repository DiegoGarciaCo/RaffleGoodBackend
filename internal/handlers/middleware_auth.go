package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/google/uuid"
)

// authenticate validates the request's session token and loads the user with
// their nonprofit membership. Returns:
//   - (user, nil)            authenticated
//   - (nil, nil)             no token present (anonymous)
//   - (nil, errUnauthorized) token present but invalid/expired
//   - (nil, otherErr)        unexpected DB error
func (cfg *apiCfg) authenticate(r *http.Request) (*auth.User, error) {
	token := auth.ExtractToken(r)
	if token == "" {
		return nil, nil
	}

	// Optional signature check (off by default — see VerifySignature docs):
	//   raw := rawTokenValue(r) // the unstripped "<token>.<sig>"
	//   if !auth.VerifySignature(raw, cfg.betterAuthSecret) {
	//       return nil, errUnauthorized
	//   }

	sess, err := cfg.DB.GetSessionByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errUnauthorized
		}
		return nil, err
	}

	if time.Now().After(sess.ExpiresAt) {
		return nil, errUnauthorized
	}

	u := &auth.User{
		ID:    sess.UserID,
		Name:  sess.UserName,
		Email: sess.UserEmail,
	}

	// Load nonprofit membership; absence just means "participant".
	mem, err := cfg.DB.GetUserMembership(r.Context(), sess.UserID)
	switch {
	case err == nil:
		u.NonprofitID = uuid.NullUUID{UUID: mem.NonprofitID, Valid: true}
		u.NonprofitRole = auth.Role(mem.Role)
	case errors.Is(err, sql.ErrNoRows):
		// participant — no org
	default:
		return nil, err
	}

	return u, nil
}

// RequireAuth rejects requests that don't carry a valid session.
func (cfg *apiCfg) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := cfg.authenticate(r)
		if err != nil && !errors.Is(err, errUnauthorized) {
			respondWithError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if u == nil {
			respondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), u)))
	})
}

// OptionalAuth attaches the user when a valid session is present, otherwise
// continues anonymously. Use on routes that serve both (e.g. browsing raffles,
// where a logged-in user also gets "saved" state).
func (cfg *apiCfg) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := cfg.authenticate(r)
		if err != nil && !errors.Is(err, errUnauthorized) {
			respondWithError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if u != nil {
			r = r.WithContext(auth.WithUser(r.Context(), u))
		}
		next.ServeHTTP(w, r)
	})
}

// RequireNonprofit ensures the user manages an org. MUST be chained after
// RequireAuth (it reads the user from context).
func (cfg *apiCfg) RequireNonprofit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := auth.UserFromContext(r.Context())
		if !ok || !u.IsNonprofitMember() {
			respondWithError(w, http.StatusForbidden, "nonprofit account required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─── Route helpers ──────────────────────────────────────────────────────────────
// Small wrappers so route registration reads cleanly:
//
//	mux.Handle("GET  /raffles",        cfg.optional(cfg.HandleListRaffles))
//	mux.Handle("GET  /users/me/tickets", cfg.protected(cfg.HandleListMyTickets))
//	mux.Handle("POST /raffles",        cfg.orgOnly(cfg.HandleCreateRaffle))

// optional: attach user if present, allow anonymous.
func (cfg *apiCfg) optional(h http.HandlerFunc) http.Handler {
	return cfg.OptionalAuth(h)
}

// protected: require a valid session.
func (cfg *apiCfg) protected(h http.HandlerFunc) http.Handler {
	return cfg.RequireAuth(h)
}

// orgOnly: require a valid session AND nonprofit membership.
func (cfg *apiCfg) orgOnly(h http.HandlerFunc) http.Handler {
	return cfg.RequireAuth(cfg.RequireNonprofit(h))
}
