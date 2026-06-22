// Package draw computes provably-fair raffle results and runs the background
// worker that fires draws when their scheduled time arrives.
package draw

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// PROVABLE FAIRNESS — commit-reveal
//
// 1. At publish: GenerateSeed() makes a fresh 256-bit random seed (hex). We
//    publish Commitment(seed) = sha256(seed) BEFORE sales close, and keep the
//    seed secret until the draw fires.
// 2. At draw: Permute(seed, tickets) deterministically orders all tickets using
//    HMAC-SHA256 keyed by the seed. The first N are the winners. Then we reveal
//    the seed.
// 3. Anyone can verify: sha256(revealedSeed) must equal the published
//    commitment, and re-running Permute reproduces the exact winners.
//
// The client verifier MUST mirror this exactly:
//   - HMAC key   = the seed STRING's bytes (the hex string itself, not decoded)
//   - HMAC msg   = the ticket number as base-10 ASCII
//   - sort       = ascending by the raw HMAC bytes, tie-broken by ticket number
// ─────────────────────────────────────────────────────────────────────────────

// GenerateSeed returns a fresh cryptographically-random seed as a 64-char hex
// string. Each raffle gets its own independent seed.
func GenerateSeed() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Commitment returns sha256(seed) as hex — the value published before sales
// close so the result can later be proven untampered.
func Commitment(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

// ticketKey is the deterministic sort key for a ticket:
// HMAC-SHA256(key=seed, msg=ticketNumber-as-decimal-ASCII).
func ticketKey(seed string, ticketNumber int32) []byte {
	mac := hmac.New(sha256.New, []byte(seed))
	mac.Write([]byte(strconv.FormatInt(int64(ticketNumber), 10)))
	return mac.Sum(nil)
}

// Entry is one ticket in the draw pool.
type Entry struct {
	TicketID     uuid.UUID
	TicketNumber int32
	UserID       uuid.UUID
}

// Permute returns the entries in their deterministic draw order. The ordering
// is a uniform random permutation (HMAC is a PRF) fully determined by the seed,
// so it's reproducible by anyone who knows the revealed seed.
func Permute(seed string, entries []Entry) []Entry {
	type keyed struct {
		entry Entry
		key   []byte
	}
	ks := make([]keyed, len(entries))
	for i, e := range entries {
		ks[i] = keyed{entry: e, key: ticketKey(seed, e.TicketNumber)}
	}

	sort.Slice(ks, func(i, j int) bool {
		if c := bytes.Compare(ks[i].key, ks[j].key); c != 0 {
			return c < 0
		}
		// Astronomically unlikely tie → stable tie-break by ticket number.
		return ks[i].entry.TicketNumber < ks[j].entry.TicketNumber
	})

	out := make([]Entry, len(ks))
	for i := range ks {
		out[i] = ks[i].entry
	}
	return out
}
