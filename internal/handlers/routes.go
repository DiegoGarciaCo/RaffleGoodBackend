package handlers

import "net/http"

// RegisterRoutes wires every route onto the mux. main.go calls this so the
// whole API surface lives in one place.
//
// Uses Go 1.22+ enhanced ServeMux patterns: "METHOD /path/{param}".
// Read a path param in a handler with r.PathValue("param").
//
// As you implement each handler method, uncomment its route. Everything is
// grouped by domain to mirror the query files and the screens they serve.
func (cfg *apiCfg) RegisterRoutes(mux *http.ServeMux) {
	// ── Health ──────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /healthz", cfg.HandleHealthz)

	// ── Categories ──────────────────────────────────────────────────────────
	mux.Handle("GET /categories", cfg.optional(cfg.HandleListCategories))
	mux.Handle("GET /categories/{slug}", cfg.optional(cfg.HandleGetCategory))

	// ── Raffles (participant browse) ──────────────────────────────────────────
	// Literal paths (featured/ending-soon/search) take precedence over the
	// {idOrSlug} wildcard automatically under Go 1.22+ ServeMux.
	mux.Handle("GET /raffles", cfg.optional(cfg.HandleListRaffles)) // ?category= to scope
	mux.Handle("GET /raffles/featured", cfg.optional(cfg.HandleFeaturedRaffles))
	mux.Handle("GET /raffles/ending-soon", cfg.optional(cfg.HandleEndingSoonRaffles))
	mux.Handle("GET /raffles/search", cfg.optional(cfg.HandleSearchRaffles))
	mux.Handle("GET /raffles/{idOrSlug}", cfg.optional(cfg.HandleGetRaffle))

	// ── Raffles (nonprofit manage) ────────────────────────────────────────────
	mux.Handle("POST /raffles", cfg.orgOnly(cfg.HandleCreateRaffle))
	mux.Handle("POST /raffles/{id}/publish", cfg.orgOnly(cfg.HandlePublishRaffle))
	mux.Handle("DELETE /raffles/{id}", cfg.orgOnly(cfg.HandleDeleteRaffle))
	mux.Handle("GET /nonprofits/me/raffles", cfg.orgOnly(cfg.HandleListMyRaffles))

	// ── Tickets & orders ──────────────────────────────────────────────────────
	mux.Handle("POST /raffles/{id}/checkout", cfg.protected(cfg.HandleCheckout))
	mux.Handle("GET /users/me/tickets", cfg.protected(cfg.HandleListMyTickets))
	mux.Handle("GET /users/me/tickets/stats", cfg.protected(cfg.HandleMyTicketStats))
	mux.Handle("GET /users/me/wins", cfg.protected(cfg.HandleListMyWins))
	mux.Handle("POST /tickets/winners/{winnerId}/claim", cfg.protected(cfg.HandleClaimPrize))
	mux.Handle("GET /orders/{receiptNumber}/receipt", cfg.protected(cfg.HandleGetReceipt))

	// ── Draws ─────────────────────────────────────────────────────────────────
	mux.Handle("GET /raffles/{id}/draw", cfg.optional(cfg.HandleGetDraw))
	mux.Handle("GET /raffles/{id}/draw/verify", cfg.optional(cfg.HandleVerifyData))
	// WebSocket: register raw (no JSON middleware wrapping the hijacked conn).
	mux.HandleFunc("GET /raffles/{id}/draw/ws", cfg.HandleDrawWebSocket)

	// ── Nonprofits (public) ───────────────────────────────────────────────────
	mux.Handle("GET /nonprofits/{idOrSlug}", cfg.optional(cfg.HandleGetNonprofit))
	mux.Handle("GET /nonprofits/{id}/reviews", cfg.optional(cfg.HandleNonprofitReviews))
	mux.Handle("POST /nonprofits/{id}/follow", cfg.protected(cfg.HandleFollowOrg))
	mux.Handle("DELETE /nonprofits/{id}/follow", cfg.protected(cfg.HandleUnfollowOrg))

	// ── Nonprofit org (settings, owned data) ──────────────────────────────────
	mux.Handle("GET /nonprofits/me", cfg.orgOnly(cfg.HandleGetMyOrg))
	mux.Handle("PATCH /nonprofits/me", cfg.orgOnly(cfg.HandleUpdateOrgProfile))
	mux.Handle("PATCH /nonprofits/me/contact", cfg.orgOnly(cfg.HandleUpdateOrgContact))
	mux.Handle("GET /nonprofits/me/team", cfg.orgOnly(cfg.HandleListTeam))
	mux.Handle("GET /nonprofits/me/payouts", cfg.orgOnly(cfg.HandleListPayouts))
	mux.Handle("PATCH /nonprofits/me/notification-preferences", cfg.orgOnly(cfg.HandleUpdateOrgNotifPrefs))

	// ── Dashboard & analytics ─────────────────────────────────────────────────
	mux.Handle("GET /nonprofits/me/dashboard", cfg.orgOnly(cfg.HandleDashboard))
	mux.Handle("GET /nonprofits/me/analytics", cfg.orgOnly(cfg.HandleAnalytics))

	// ── Participant profile ───────────────────────────────────────────────────
	mux.Handle("GET /users/me/profile", cfg.protected(cfg.HandleGetProfile))
	mux.Handle("PATCH /users/me/notification-preferences", cfg.protected(cfg.HandleUpdateUserNotifPrefs))

	// ── Engagement ────────────────────────────────────────────────────────────
	mux.Handle("GET /users/me/saved-raffles", cfg.protected(cfg.HandleListSaved))
	mux.Handle("POST /raffles/{id}/save", cfg.protected(cfg.HandleSaveRaffle))
	mux.Handle("DELETE /raffles/{id}/save", cfg.protected(cfg.HandleUnsaveRaffle))
	mux.Handle("GET /users/me/following", cfg.protected(cfg.HandleListFollowing))

	// ── Events (behavioral logging) ───────────────────────────────────────────
	mux.Handle("POST /events/view", cfg.optional(cfg.HandleRecordView))
	mux.Handle("POST /events/search", cfg.optional(cfg.HandleRecordSearch))
	mux.Handle("POST /events/share", cfg.optional(cfg.HandleRecordShare))

	// ── Uploads ───────────────────────────────────────────────────────────────
	// mux.HandleFunc("POST /uploads/presign", cfg.HandlePresignUpload) // S3 presigned URL for raffle/org images

	// ── Webhooks (signature-auth, bypass session middleware) ──────────────────
	mux.HandleFunc("POST /webhooks/stripe", cfg.HandleStripeWebhook)
}
