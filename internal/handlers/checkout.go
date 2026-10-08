package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/diegoGarciaCo/raffles/internal/database"
	"github.com/diegoGarciaCo/raffles/internal/payments"
	"github.com/diegoGarciaCo/raffles/internal/pricing"
	"github.com/google/uuid"
)

// checkoutRequest is the body for POST /raffles/{id}/checkout.
// For pay_what_you_pull (and any user_picks raffle), send `ticket_numbers`.
// For fixed/bundle/donation with random assignment, send `quantity`.
// `donation_cents` is an optional add-on gift on top of the tickets.
type checkoutRequest struct {
	TicketNumbers   []int32 `json:"ticket_numbers,omitempty"`
	Quantity        int32   `json:"quantity,omitempty"`
	DonationCents   int64   `json:"donation_cents,omitempty"`
	PaymentMethodID string  `json:"payment_method_id,omitempty"`
}

type plannedTicket struct {
	number     int32
	hasNumber  bool
	priceCents int64
}

// planTickets prices a checkout from the raffle + request, returning the tickets
// to create and the subtotal in cents. Shared by HandleCheckout (to size the
// PaymentIntent) and the Stripe webhook (to create tickets after payment), so
// pricing can never drift between the two.
func (cfg *apiCfg) planTickets(raffle database.GetRaffleByIDRow, quantity int32, ticketNumbers []int32) ([]plannedTicket, int64, error) {
	pr := pricing.Raffle{
		Strategy:         pricing.Strategy(raffle.TicketStrategy),
		TicketPriceCents: numericStringToCents(raffle.TicketPrice),
		FreeStart:        int32OrZero(raffle.FreeTicketStartRange),
		FreeEnd:          int32OrZero(raffle.FreeTicketEndRange),
		MaxTickets:       int32OrZero(raffle.MaxTickets),
		TicketsSold:      raffle.TicketsSold,
	}

	var planned []plannedTicket
	var subtotalCents int64

	if len(ticketNumbers) > 0 {
		sub, err := pr.QuotePicked(ticketNumbers)
		if err != nil {
			return nil, 0, err
		}
		subtotalCents = sub
		for _, n := range ticketNumbers {
			planned = append(planned, plannedTicket{number: n, hasNumber: true, priceCents: pr.PriceForNumber(n)})
		}
	} else {
		sub, err := pr.QuoteQuantity(quantity)
		if err != nil {
			return nil, 0, err
		}
		subtotalCents = sub
		per := pr.TicketPriceCents
		if pr.Strategy == pricing.Free {
			per = 0
		}
		for i := int32(0); i < quantity; i++ {
			planned = append(planned, plannedTicket{hasNumber: false, priceCents: per})
		}
	}
	return planned, subtotalCents, nil
}

// insertTickets creates each planned ticket inside the given tx. The DB triggers
// assign numbers (for random), compute price when 0, and bump sold + raised.
func insertTickets(ctx context.Context, qtx *database.Queries, raffleID, userID, orderID uuid.UUID, planned []plannedTicket) ([]database.Ticket, error) {
	out := make([]database.Ticket, 0, len(planned))
	for _, pt := range planned {
		t, err := qtx.CreateTicket(ctx, database.CreateTicketParams{
			RaffleItemID: raffleID,
			UserID:       userID,
			OrderID:      uuid.NullUUID{UUID: orderID, Valid: true},
			TicketNumber: nullInt32(pt.number, pt.hasNumber),
			PricePaid:    centsToNumericString(pt.priceCents),
		})
		if err != nil {
			return nil, fmt.Errorf("create ticket: %w", err)
		}
		out = append(out, t)
	}
	return out, nil
}

// HandleCheckout starts a purchase. It prices server-side, then:
//
//   - total == 0 (free raffle): creates the paid order + tickets immediately.
//
//   - total  > 0: creates a PENDING order + a Stripe PaymentIntent and returns
//     its client_secret. Tickets are created by the webhook once payment lands.
//
//     POST /raffles/{id}/checkout   (auth required)
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

	raffle, err := cfg.DB.GetRaffleByID(r.Context(), raffleID)
	if err != nil {
		cfg.handleDBError(w, err, "raffle")
		return
	}
	if string(raffle.Status) != "active" {
		respondWithError(w, http.StatusConflict, "this raffle is not currently selling tickets")
		return
	}

	planned, subtotalCents, err := cfg.planTickets(raffle, req.Quantity, req.TicketNumbers)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(planned) == 0 {
		respondWithError(w, http.StatusBadRequest, "no tickets selected")
		return
	}

	totalCents := subtotalCents + req.DonationCents

	// ── Free / zero-total: no payment needed, fulfill immediately. ──
	if totalCents <= 0 {
		receipt := newReceiptNumber()
		var order database.Order
		var tickets []database.Ticket
		err := cfg.withTx(r.Context(), func(qtx *database.Queries) error {
			o, err := qtx.CreateOrder(r.Context(), database.CreateOrderParams{
				UserID:             user.ID,
				RaffleItemID:       raffleID,
				TicketCount:        int32(len(planned)),
				SubtotalCents:      int32(subtotalCents),
				DonationCents:      int32(req.DonationCents),
				TotalCents:         int32(totalCents),
				Processor:          "none",
				ProcessorPaymentID: nullStr("free_no_charge"),
				Status:             "paid",
				ReceiptNumber:      nullStr(receipt),
			})
			if err != nil {
				return fmt.Errorf("create order: %w", err)
			}
			order = o
			ts, err := insertTickets(r.Context(), qtx, raffleID, user.ID, o.ID, planned)
			if err != nil {
				return err
			}
			tickets = ts
			return nil
		})
		if err != nil {
			respondWithError(w, http.StatusConflict, "could not complete purchase: "+err.Error())
			return
		}
		respondWithJSON(w, http.StatusCreated, map[string]any{
			"free":    true,
			"order":   order,
			"tickets": tickets,
		})
		return
	}

	// ── Paid: pending order + Stripe PaymentIntent. Tickets on webhook. ──
	receipt := newReceiptNumber()
	order, err := cfg.DB.CreateOrder(r.Context(), database.CreateOrderParams{
		UserID:             user.ID,
		RaffleItemID:       raffleID,
		TicketCount:        int32(len(planned)),
		SubtotalCents:      int32(subtotalCents),
		DonationCents:      int32(req.DonationCents),
		TotalCents:         int32(totalCents),
		Processor:          "stripe",
		ProcessorPaymentID: sql.NullString{}, // filled by the webhook on success
		Status:             "pending",
		ReceiptNumber:      nullStr(receipt),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not create order")
		return
	}

	meta := map[string]string{
		"order_id":       order.ID.String(),
		"raffle_item_id": raffleID.String(),
		"user_id":        user.ID.String(),
		"quantity":       strconv.Itoa(len(planned)),
	}
	if len(req.TicketNumbers) > 0 {
		meta["ticket_numbers"] = joinInt32(req.TicketNumbers)
	}

	pi, err := cfg.Payments.CreatePaymentIntent(payments.PaymentIntentInput{
		AmountCents: totalCents,
		Currency:    "usd",
		Metadata:    meta,
	})
	if err != nil {
		respondWithError(w, http.StatusBadGateway, "could not start payment")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]any{
		"client_secret":   pi.ClientSecret,
		"order_id":        order.ID,
		"publishable_key": cfg.StripePublishableKey,
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

// newReceiptNumber builds a human-ish unique receipt id: RG-YYYYMMDD-<short>.
func newReceiptNumber() string {
	return fmt.Sprintf(
		"RG-%s-%s",
		time.Now().UTC().Format("20060102"),
		uuid.NewString()[:8],
	)
}

func joinInt32(ns []int32) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = strconv.Itoa(int(n))
	}
	return strings.Join(parts, ",")
}

func parseInt32CSV(s string) []int32 {
	if s == "" {
		return nil
	}
	var out []int32
	for _, p := range strings.Split(s, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			out = append(out, int32(n))
		}
	}
	return out
}
