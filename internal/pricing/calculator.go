// Package pricing computes authoritative ticket prices server-side. The client
// never decides what it pays — this is the source of truth for the charge
// amount, and it mirrors the DB's assign_ticket_number trigger so the amount
// charged matches the price_paid the trigger will store.
package pricing

import "fmt"

// Strategy mirrors the ticket_strategy enum.
type Strategy string

const (
	Fixed          Strategy = "fixed"
	Bundle         Strategy = "bundle"
	Donation       Strategy = "donation"
	Free           Strategy = "free"
	PayWhatYouPull Strategy = "pay_what_you_pull"
)

// Raffle carries the pricing-relevant fields of a raffle.
type Raffle struct {
	Strategy         Strategy
	TicketPriceCents int64 // for fixed/bundle/donation
	FreeStart        int32 // free number range start (0 = none)
	FreeEnd          int32 // free number range end
	MaxTickets       int32 // 0 = unlimited
	TicketsSold      int32
}

// isFree reports whether a given ticket number falls in the free range.
func (r Raffle) isFree(number int32) bool {
	return r.FreeStart > 0 && number >= r.FreeStart && number <= r.FreeEnd
}

// PriceForNumber returns the cents price of a single ticket by its number.
// Used for pay_what_you_pull (and any number-dependent pricing).
func (r Raffle) PriceForNumber(number int32) int64 {
	if r.Strategy == Free || r.isFree(number) {
		return 0
	}
	if r.Strategy == PayWhatYouPull {
		// The number IS the price, in whole dollars → cents.
		return int64(number) * 100
	}
	return r.TicketPriceCents
}

// QuotePicked prices a pay-what-you-pull (or user_picks) purchase of specific
// ticket numbers. Validates the numbers are in range and returns the subtotal.
func (r Raffle) QuotePicked(numbers []int32) (subtotalCents int64, err error) {
	if len(numbers) == 0 {
		return 0, fmt.Errorf("no ticket numbers provided")
	}
	seen := make(map[int32]bool, len(numbers))
	for _, n := range numbers {
		if n < 1 {
			return 0, fmt.Errorf("ticket number %d is invalid", n)
		}
		if r.MaxTickets > 0 && n > r.MaxTickets {
			return 0, fmt.Errorf("ticket number %d exceeds max %d", n, r.MaxTickets)
		}
		if seen[n] {
			return 0, fmt.Errorf("ticket number %d listed twice", n)
		}
		seen[n] = true
		subtotalCents += r.PriceForNumber(n)
	}
	return subtotalCents, nil
}

// QuoteQuantity prices a quantity-based purchase (fixed/bundle/donation), where
// the buyer doesn't choose numbers. Returns the subtotal for `qty` tickets.
//
// Note: free-range numbers aren't knowable in advance under random assignment,
// so quantity pricing uses the flat ticket price. If a raffle has a free range,
// prefer user_picks so QuotePicked can zero those numbers exactly.
func (r Raffle) QuoteQuantity(qty int32) (subtotalCents int64, err error) {
	if qty < 1 {
		return 0, fmt.Errorf("quantity must be at least 1")
	}
	if r.MaxTickets > 0 && r.TicketsSold+qty > r.MaxTickets {
		return 0, fmt.Errorf("only %d tickets remaining", r.MaxTickets-r.TicketsSold)
	}
	switch r.Strategy {
	case Free:
		return 0, nil
	case PayWhatYouPull:
		return 0, fmt.Errorf("pay_what_you_pull requires choosing ticket numbers")
	default:
		return int64(qty) * r.TicketPriceCents, nil
	}
}
