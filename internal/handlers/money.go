package handlers

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ── Money ─────────────────────────────────────────────────────────────────────
// sqlc maps NUMERIC(10,2) to string by default. These convert between the
// decimal string the DB uses and the integer cents we compute with.

// numericStringToCents parses a nullable NUMERIC string ("5.00") into cents
// (500). Null or empty/invalid input yields 0 — correct for free raffles where
// ticket_price is NULL.
func numericStringToCents(n sql.NullString) int64 {
	if !n.Valid {
		return 0
	}
	s := strings.TrimSpace(n.String)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	// Round to nearest cent to avoid float drift.
	return int64(f*100 + 0.5)
}

// centsToNumericString formats cents (500) as a NUMERIC string ("5.00").
func centsToNumericString(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

// ── Nullable field accessors ──────────────────────────────────────────────────

// nullStr wraps a string as a valid sql.NullString.
func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

// int32OrZero returns the value of a sql.NullInt32, or 0 if not valid.
func int32OrZero(n sql.NullInt32) int32 {
	if n.Valid {
		return n.Int32
	}
	return 0
}

// nullTimeZero returns an invalid (NULL) sql.NullTime, for unset timestamps
// like a raffle's draw_at before it's scheduled.
func nullTimeZero() sql.NullTime {
	return sql.NullTime{Valid: false}
}

// numericToFloat parses a NUMERIC string ("123.45") to a float64 (0 on error).
// Aggregate columns cast ::NUMERIC come back as strings via sqlc.
func numericToFloat(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

// round1 rounds to one decimal place (correct for negatives too).
func round1(f float64) float64 {
	return math.Round(f*10) / 10
}
