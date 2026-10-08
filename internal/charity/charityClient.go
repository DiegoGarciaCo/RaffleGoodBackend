package charity

// CharityAPI client. Looks up an IRS tax-exempt organization by EIN against the
// IRS Business Master File. We use it to verify a nonprofit and to decide the
// platform fee: foundation code 10 ("Church 170(b)(1)(A)(i)") is fee-exempt.
//
// Docs: https://charityapi.org — GET /api/organizations/{ein}, auth via the
// "apikey" request header.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.charityapi.org"

// FoundationChurch is the IRS foundation code for a church.
const FoundationChurch = 10

type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Organization is the subset of the IRS BMF record we use. The numeric-ish
// fields are decoded leniently (flexStr) because the API may send them as JSON
// strings or bare numbers depending on the record.
type Organization struct {
	EIN        string  `json:"ein"`
	Name       string  `json:"name"`
	NteeCd     string  `json:"ntee_cd"`
	Ruling     flexStr `json:"ruling"`     // YYYYMM
	Subsection flexStr `json:"subsection"` // 3 = 501(c)(3)
	Foundation flexStr `json:"foundation"` // 10 = church
	Status     flexStr `json:"status"`     // IRS exempt status code
	City       string  `json:"city"`
	State      string  `json:"state"`
}

// Lookup fetches a single organization by EIN. Returns (org, found, error).
// A 404 means the EIN is not in the IRS exempt list → found=false, no error.
func (c *Client) Lookup(ctx context.Context, ein string) (*Organization, bool, error) {
	ein = NormalizeEIN(ein)
	if len(ein) != 9 {
		return nil, false, fmt.Errorf("invalid EIN: expected 9 digits, got %q", ein)
	}

	url := fmt.Sprintf("%s/api/organizations/%s", c.baseURL, ein)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("charityapi request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("charityapi status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// The API wraps the record under "data"; fall back to a top-level object
	// in case a plan returns it unwrapped.
	var env struct {
		Data *Organization `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Data != nil && env.Data.Name != "" {
		return env.Data, true, nil
	}
	var org Organization
	if err := json.Unmarshal(body, &org); err != nil {
		return nil, false, fmt.Errorf("decode charityapi response: %w", err)
	}
	if org.Name == "" {
		return nil, false, nil
	}
	return &org, true, nil
}

// ── Typed accessors (so callers never touch the internal flexStr type) ─────────

func (o *Organization) IsChurch() bool {
	n, ok := o.Foundation.int32()
	return ok && n == FoundationChurch
}

func (o *Organization) FoundationCode() (int32, bool) { return o.Foundation.int32() }
func (o *Organization) SubsectionCode() (int32, bool) { return o.Subsection.int32() }
func (o *Organization) StatusCode() (int32, bool)     { return o.Status.int32() }

// RulingTime parses the IRS "YYYYMM" ruling into the first of that month.
func (o *Organization) RulingTime() (time.Time, bool) {
	s := strings.TrimSpace(o.Ruling.String())
	if len(s) < 6 {
		return time.Time{}, false
	}
	year, err1 := strconv.Atoi(s[:4])
	month, err2 := strconv.Atoi(s[4:6])
	if err1 != nil || err2 != nil || month < 1 || month > 12 {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC), true
}

// NormalizeEIN strips everything but digits (so "12-3456789" → "123456789").
func NormalizeEIN(ein string) string {
	var b strings.Builder
	for _, r := range ein {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ── flexStr: decodes a JSON value that may be a quoted string or a number ──────

type flexStr string

func (f *flexStr) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = ""
		return nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*f = flexStr(strings.TrimSpace(str))
		return nil
	}
	*f = flexStr(s)
	return nil
}

func (f flexStr) String() string { return string(f) }

func (f flexStr) int32() (int32, bool) {
	s := strings.TrimSpace(string(f))
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return int32(n), true
}
