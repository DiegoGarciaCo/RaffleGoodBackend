package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v79"
)

// HandleStripeWebhook is the fulfillment authority: Stripe calls it when a
// payment settles, and it creates the tickets. Signature-verified; no session
// middleware (registered raw in RegisterRoutes).
//
//	POST /webhooks/stripe
func (cfg *apiCfg) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB cap
	if err != nil {
		logrus.WithError(err).Warn("[stripe webhook] could not read body")
		respondWithError(w, http.StatusBadRequest, "could not read body")
		return
	}

	event, err := cfg.Payments.ConstructEvent(payload, r.Header.Get("Stripe-Signature"))
	if err != nil {
		// Log the REAL reason (signature mismatch, API-version mismatch,
		// empty body, clock skew, …) so this is never a guessing game again.
		logrus.WithError(err).WithField("sig_present", r.Header.Get("Stripe-Signature") != "").
			Warn("[stripe webhook] ConstructEvent failed")
		respondWithError(w, http.StatusBadRequest, "invalid signature")
		return
	}

	switch event.Type {
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			logrus.WithError(err).Warn("[stripe webhook] bad payment_intent payload")
			respondWithError(w, http.StatusBadRequest, "bad payload")
			return
		}
		if err := cfg.fulfillStripeOrder(r.Context(), pi); err != nil {
			// 500 → Stripe retries. Fulfillment is atomic + idempotent, so a
			// retry either completes cleanly or no-ops on an already-paid order.
			logrus.WithError(err).WithField("payment_intent", pi.ID).
				Error("[stripe webhook] fulfillment failed")
			respondWithError(w, http.StatusInternalServerError, "fulfillment failed")
			return
		}
		logrus.WithField("payment_intent", pi.ID).Info("[stripe webhook] order fulfilled")
	default:
		// Ignore other event types for now (acknowledge so Stripe stops resending).
	}

	w.WriteHeader(http.StatusOK)
}

// fulfillStripeOrder marks the pending order paid and creates its tickets, in a
// single transaction. Idempotent: a second delivery for an already-paid order
// is a no-op.
func (cfg *apiCfg) fulfillStripeOrder(ctx context.Context, pi stripe.PaymentIntent) error {
	idStr := pi.Metadata["order_id"]
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		return fmt.Errorf("missing/invalid order_id metadata: %q", idStr)
	}

	order, err := cfg.DB.GetOrderByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("load order: %w", err)
	}
	if order.Status == "paid" {
		return nil // already fulfilled
	}

	raffle, err := cfg.DB.GetRaffleByID(ctx, order.RaffleItemID)
	if err != nil {
		return fmt.Errorf("load raffle: %w", err)
	}

	// Re-derive the exact same ticket plan that sized the charge.
	nums := parseInt32CSV(pi.Metadata["ticket_numbers"])
	planned, _, err := cfg.planTickets(raffle, order.TicketCount, nums)
	if err != nil {
		return fmt.Errorf("plan tickets: %w", err)
	}

	return cfg.withTx(ctx, func(qtx *database.Queries) error {
		if _, err := qtx.MarkOrderPaid(ctx, database.MarkOrderPaidParams{
			ID:                 order.ID,
			ProcessorPaymentID: nullStr(pi.ID),
		}); err != nil {
			return fmt.Errorf("mark paid: %w", err)
		}
		if _, err := insertTickets(ctx, qtx, order.RaffleItemID, order.UserID, order.ID, planned); err != nil {
			return err
		}
		return nil
	})
}
