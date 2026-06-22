package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/diegoGarciaCo/raffles/internal/pricing"
	"github.com/google/uuid"
)

// checkoutRequest is the body for POST /raffles/{id}/checkout.
// For pay_what_you_pull (and any user_picks raffle), send `ticket_numbers`.
// For fixed/bundle/donation with random assignment, send `quantity`.
// `donation_cents` is an optional add-on gift on top of the tickets.
type checkoutRequest struct {
	TicketNumbers []int32 `json:"ticket_numbers,omitempty"`
	Quantity      int32   `json:"quantity,omitempty"`
	DonationCents int64   `json:"donation_cents,omitempty"`
	// PaymentMethodID would be passed to Stripe in a real charge.
	PaymentMethodID string `json:"payment_method_id,omitempty"`
}

// HandleCheckout purchases tickets for a raffle: prices server-side, charges,
// then creates the order + tickets atomically.
//
//	POST /raffles/{id}/checkout   (auth required)
func (cfg *apiCfg) HandleCheckout(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	raffleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid raffle id")
		return
	}

	var req checkoutRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.DonationCents < 0 {
		respondWithError(w, http.StatusBadRequest, "donation cannot be negative")
		return
	}

	// Load the raffle and confirm it's open for sales.
	raffle, err := cfg.DB.GetRaffleByID(r.Context(), raffleID)
	if err != nil {
		cfg.handleDBError(w, err, "raffle")
		return
	}
	if string(raffle.Status) != "active" {
		respondWithError(w, http.StatusConflict, "this raffle is not currently selling tickets")
		return
	}

	// Build the pricing model from the raffle row.
	pr := pricing.Raffle{
		Strategy:         pricing.Strategy(raffle.TicketStrategy),
		TicketPriceCents: numericStringToCents(raffle.TicketPrice),
		FreeStart:        int32OrZero(raffle.FreeTicketStartRange),
		FreeEnd:          int32OrZero(raffle.FreeTicketEndRange),
		MaxTickets:       int32OrZero(raffle.MaxTickets),
		TicketsSold:      raffle.TicketsSold,
	}

	// Determine the tickets to create + the subtotal.
	type plannedTicket struct {
		number     int32
		hasNumber  bool
		priceCents int64
	}
	var planned []plannedTicket
	var subtotalCents int64

	if len(req.TicketNumbers) > 0 {
		// Explicit numbers (pay_what_you_pull / user_picks).
		subtotalCents, err = pr.QuotePicked(req.TicketNumbers)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		for _, n := range req.TicketNumbers {
			planned = append(planned, plannedTicket{number: n, hasNumber: true, priceCents: pr.PriceForNumber(n)})
		}
	} else {
		// Quantity-based (fixed/bundle/donation, random assignment).
		subtotalCents, err = pr.QuoteQuantity(req.Quantity)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		per := pr.TicketPriceCents
		if pr.Strategy == pricing.Free {
			per = 0
		}
		for i := int32(0); i < req.Quantity; i++ {
			planned = append(planned, plannedTicket{hasNumber: false, priceCents: per})
		}
	}

	totalCents := subtotalCents + req.DonationCents

	// ── Charge ──────────────────────────────────────────────────────────────
	// TODO (payments step): charge `totalCents` via Stripe using req.PaymentMethodID.
	// For now we assume payment succeeded and record processorPaymentID below.
	processorPaymentID := "seed_dev_no_charge"

	// ── Persist atomically ────────────────────────────────────────────────────
	receipt := newReceiptNumber()
	var createdOrder database.Order
	var createdTickets []database.Ticket

	err = cfg.withTx(r.Context(), func(qtx *database.Queries) error {
		order, err := qtx.CreateOrder(r.Context(), database.CreateOrderParams{
			UserID:             user.ID,
			RaffleItemID:       raffleID,
			TicketCount:        int32(len(planned)),
			SubtotalCents:      int32(subtotalCents),
			DonationCents:      int32(req.DonationCents),
			TotalCents:         int32(totalCents),
			Processor:          "stripe",
			ProcessorPaymentID: nullStr(processorPaymentID),
			Status:             "paid",
			ReceiptNumber:      nullStr(receipt),
		})
		if err != nil {
			return fmt.Errorf("create order: %w", err)
		}
		createdOrder = order

		for _, pt := range planned {
			ticket, err := qtx.CreateTicket(r.Context(), database.CreateTicketParams{
				RaffleItemID: raffleID,
				UserID:       user.ID,
				OrderID:      uuid.NullUUID{UUID: order.ID, Valid: true},
				TicketNumber: nullInt32(pt.number, pt.hasNumber),
				PricePaid:    centsToNumericString(pt.priceCents),
			})
			if err != nil {
				// A unique-violation here (taken number) rolls back the whole order.
				return fmt.Errorf("create ticket: %w", err)
			}
			createdTickets = append(createdTickets, ticket)
		}
		return nil
	})
	if err != nil {
		// TODO (payments step): if the charge already succeeded, refund here.
		respondWithError(w, http.StatusConflict, "could not complete purchase: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]any{
		"order":   createdOrder,
		"tickets": createdTickets,
	})
}

// ── small helpers ───────────────────────────────────────────────────────────

// newReceiptNumber builds a human-ish unique receipt id: RG-YYYYMMDD-<short>.
func newReceiptNumber() string {
	return fmt.Sprintf("RG-%s-%s",
		time.Now().UTC().Format("20060102"),
		uuid.NewString()[:8],
	)
}
