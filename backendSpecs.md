# RaffleGood — Backend Implementation Spec

This document maps every screen built in the app to the data and endpoints the
Go backend must provide. It also lists **schema changes** needed to support the
frontend, and **suggested data to collect** for future recommendation engines
and analytics.

Organized as:
1. Schema changes (required)
2. Schema additions (new tables required)
3. Endpoint catalog (per screen)
4. The draw system (most complex — commit/reveal + multi-winner)
5. Suggested data collection for ML / analytics
6. Open questions

---

## 1. SCHEMA CHANGES (required)

These are conflicts between the current schema and what the frontend was built
against. Each needs a migration.

### 1.1 `ticket_strategy` enum mismatch

**Current:** `('fixed_price', 'sequential_pricing')`
**Frontend uses:** `fixed`, `bundle`, `donation`, `free`, `pay_what_you_pull`

`sequential_pricing` is conceptually the same as the frontend's
`pay_what_you_pull` (ticket number = price). Recommend aligning names. Migration:

```sql
ALTER TYPE ticket_strategy ADD VALUE 'bundle';
ALTER TYPE ticket_strategy ADD VALUE 'donation';
ALTER TYPE ticket_strategy ADD VALUE 'free';
-- keep 'fixed_price' and 'sequential_pricing', OR rename for clarity:
--   fixed_price        → matches frontend 'fixed'
--   sequential_pricing → matches frontend 'pay_what_you_pull'
```

Either rename the enum values to match the frontend, or add a translation layer
in the API handler. Renaming is cleaner. Pick one and document it.

### 1.2 `pulling_strategy` enum mismatch

**Current:** `('random', 'user_picks')`
**Frontend uses:** `single`, `ranked`, `multiple_equal`, `countdown`

These describe two different things:
- **Current** describes how a *ticket number* is assigned to a buyer
  (random vs. the buyer picks their number).
- **Frontend** describes how *winners* are pulled at draw time.

These are both legitimate and should be **separate columns**:

```sql
-- rename existing to reflect what it actually controls
ALTER TYPE pulling_strategy RENAME TO ticket_assignment_strategy;
-- random | user_picks  (how buyers get their number)

-- new enum for the draw mechanic
CREATE TYPE draw_strategy AS ENUM ('single', 'ranked', 'multiple_equal', 'countdown');
ALTER TABLE raffle_items ADD COLUMN draw_strategy draw_strategy NOT NULL DEFAULT 'single';
```

### 1.3 `raffle_item_status` missing values

**Current:** `('draft', 'active', 'closed', 'drawn')`
**Frontend uses:** `draft`, `active`, `closed`, `drawing`, `completed`, `cancelled`

```sql
ALTER TYPE raffle_item_status ADD VALUE 'drawing'  AFTER 'closed';   -- draw in progress
ALTER TYPE raffle_item_status ADD VALUE 'cancelled';
-- map frontend 'completed' to existing 'drawn', OR add 'completed'
```

`drawing` is needed so the participant draw-watch screen and the nonprofit
raffles "Drawing" filter tab work. `cancelled` is needed for the cancel flow.

### 1.4 Add `slug` to `raffle_items`

The create-raffle wizard generates a shareable slug. Add it:

```sql
ALTER TABLE raffle_items ADD COLUMN slug VARCHAR(140);
CREATE UNIQUE INDEX idx_items_slug ON raffle_items (slug);
```

### 1.5 `nonprofits` — fields the Org screen expects

The Org screen reads categories, social links, and a verified flag surfaced
directly. Current table has most basics but is missing:

```sql
ALTER TABLE nonprofits ADD COLUMN slug VARCHAR(140);            -- public profile URL
ALTER TABLE nonprofits ADD COLUMN categories TEXT[] NOT NULL DEFAULT '{}';
CREATE UNIQUE INDEX idx_nonprofits_slug ON nonprofits (slug);
```

Social links are better as their own table (see 2.6).

### 1.6 Draw winner model conflict (IMPORTANT)

**Current:** `raffle_items.winner_user_id UUID` — a single winner per raffle.

**Frontend:** supports multiple ordered winners with per-rank prizes
(ranked/multiple_equal/countdown draws). A single column cannot represent this.

**Fix:** drop `winner_user_id` from `raffle_items` and introduce a
`draw_results` + `draw_winners` model (see section 2.1). Keep `drawn_at`.

---

## 2. SCHEMA ADDITIONS (new tables required)

### 2.1 Draw results & winners (replaces `winner_user_id`)

The frontend draw system computes ALL winners atomically when the draw fires,
freezes them in order, then reveals gradually. This needs two tables.

```sql
CREATE TABLE draw_results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    raffle_item_id UUID NOT NULL UNIQUE REFERENCES raffle_items(id) ON DELETE CASCADE,
    draw_strategy draw_strategy NOT NULL,
    reveal_order TEXT NOT NULL DEFAULT 'forward',   -- forward | reverse
    winner_count INT NOT NULL,
    -- Provable fairness (commit-reveal)
    commitment_hash TEXT NOT NULL,    -- sha256(seed), published BEFORE sales close
    seed TEXT,                        -- revealed AFTER draw fires (NULL until then)
    -- Timing for the reveal animation
    draw_started_at TIMESTAMPTZ,
    draw_duration_ms INT NOT NULL DEFAULT 0,
    -- Snapshot stats at draw time
    total_tickets INT NOT NULL,
    total_participants INT NOT NULL,
    verification_url TEXT,
    status TEXT NOT NULL DEFAULT 'scheduled',  -- scheduled | in_progress | completed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE draw_winners (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    draw_result_id UUID NOT NULL REFERENCES draw_results(id) ON DELETE CASCADE,
    pull_index INT NOT NULL,          -- 0-based order pulled
    rank INT NOT NULL,                -- 1 = grand prize
    ticket_id UUID NOT NULL REFERENCES tickets(id),
    ticket_number INT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    prize_title TEXT NOT NULL,
    prize_description TEXT,
    prize_value_cents INT,
    revealed_at_ms INT NOT NULL,      -- ms offset from draw_started_at
    prize_claimed BOOLEAN NOT NULL DEFAULT FALSE,
    claimed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (draw_result_id, pull_index)
);
CREATE INDEX idx_draw_winners_user ON draw_winners (user_id);
CREATE INDEX idx_draw_winners_result ON draw_winners (draw_result_id);
```

**Commit-reveal flow (critical for trust):**
1. When a raffle is published, generate a cryptographically random `seed` via
   `crypto/rand` (NOT `math/rand`). Store it server-side, publish only
   `commitment_hash = sha256(seed)`.
2. Each raffle gets a fresh independent seed — never derive one seed from
   another, or revealed past seeds could predict future draws.
3. At draw time, compute winners deterministically with **HMAC-SHA256** keyed
   by the seed (mirrors the client verifier exactly), freeze them, reveal seed.
4. Client re-runs the same HMAC shuffle to verify. The algorithm being public
   is a feature; security comes from the seed being secret until sales close.

### 2.2 Prize tiers (draw configuration, set at create time)

The create wizard and draw-config screen define prizes per rank.

```sql
CREATE TABLE prize_tiers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    raffle_item_id UUID NOT NULL REFERENCES raffle_items(id) ON DELETE CASCADE,
    rank INT NOT NULL,                -- 1 = first place
    title TEXT NOT NULL,
    description TEXT,
    value_cents INT,
    image_url TEXT,
    UNIQUE (raffle_item_id, rank)
);
```

Alternatively keep these inside `raffle_items.attributes->'draw'` as the wizard
currently serializes them. A dedicated table is better for querying "total prize
value awarded" analytics later. Recommend the table.

### 2.3 Saved raffles (participant "Saved" tab)

```sql
CREATE TABLE saved_raffles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    raffle_item_id UUID NOT NULL REFERENCES raffle_items(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, raffle_item_id)
);
CREATE INDEX idx_saved_user ON saved_raffles (user_id);
```

### 2.4 Followed organizations (participant "Saved" → Following tab)

```sql
CREATE TABLE org_follows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, nonprofit_id)
);
CREATE INDEX idx_follows_user ON org_follows (user_id);
CREATE INDEX idx_follows_org ON org_follows (nonprofit_id);
```

### 2.5 Payment methods & shipping addresses (participant profile)

```sql
CREATE TABLE payment_methods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    processor TEXT NOT NULL,           -- 'stripe'
    processor_pm_id TEXT NOT NULL,     -- Stripe PaymentMethod id; never store PAN
    brand TEXT NOT NULL,
    last4 TEXT NOT NULL,
    exp_month INT NOT NULL,
    exp_year INT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_pm_user ON payment_methods (user_id);

CREATE TABLE shipping_addresses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    line1 TEXT NOT NULL,
    line2 TEXT,
    city TEXT NOT NULL,
    state TEXT NOT NULL,
    zip TEXT NOT NULL,
    country TEXT NOT NULL DEFAULT 'US',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_addr_user ON shipping_addresses (user_id);
```

### 2.6 Nonprofit social links, bank account, payouts (Org screen)

```sql
CREATE TABLE nonprofit_social_links (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,            -- instagram | twitter | facebook | linkedin | youtube
    url TEXT NOT NULL
);

CREATE TABLE nonprofit_bank_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    processor TEXT NOT NULL,           -- 'stripe_connect'
    processor_account_id TEXT NOT NULL,
    bank_name TEXT,
    last4 TEXT,
    account_type TEXT,                 -- checking | savings
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE payouts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    raffle_item_id UUID REFERENCES raffle_items(id),
    amount_cents INT NOT NULL,
    status TEXT NOT NULL,              -- pending | paid | failed
    processor_payout_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ
);
CREATE INDEX idx_payouts_org ON payouts (nonprofit_id, created_at DESC);
```

### 2.7 Notification preferences (participant + org)

```sql
CREATE TABLE notification_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- exactly one of these is set
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    nonprofit_id UUID REFERENCES nonprofits(id) ON DELETE CASCADE,
    prefs JSONB NOT NULL DEFAULT '{}',  -- {"draw_results":true,...}
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 2.8 Orders (group ticket purchases into one transaction)

`tickets` records individual tickets but there's no concept of a purchase
"order" (buying 5 tickets in one checkout). Needed for receipts and the
participant tickets screen grouping.

```sql
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    raffle_item_id UUID NOT NULL REFERENCES raffle_items(id),
    ticket_count INT NOT NULL,
    subtotal_cents INT NOT NULL,
    donation_cents INT NOT NULL DEFAULT 0,   -- optional add-on donation
    total_cents INT NOT NULL,
    processor TEXT NOT NULL,
    processor_payment_id TEXT,
    status TEXT NOT NULL,              -- pending | paid | refunded | failed
    receipt_number TEXT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_orders_user ON orders (user_id, created_at DESC);
-- link tickets to their order
ALTER TABLE tickets ADD COLUMN order_id UUID REFERENCES orders(id);
```

---

## 3. ENDPOINT CATALOG (per screen)

All authenticated. Role derived from session → `nonprofit_users` membership.

### Participant — Home
```
GET /raffles/featured           → featured raffle for the hero
GET /raffles?sort=ending_soon&limit=N
GET /raffles/feed?page=N        → infinite scroll feed
GET /stats/community            → total raised across platform, etc.
```

### Participant — Explore
```
GET /raffles/search?q=&category=&sort=&page=    → uses idx_items_fts + path
GET /nonprofits/search?q=&category=&page=
GET /categories                 → from v_category_item_counts
```

### Participant — Raffle detail
```
GET /raffles/:idOrSlug          → from v_active_raffles + prize_tiers + nonprofit
POST /raffles/:id/save          → saved_raffles insert
DELETE /raffles/:id/save
```

### Participant — Nonprofit profile
```
GET /nonprofits/:idOrSlug       → profile + social links + stats
GET /nonprofits/:id/raffles?status=active|past
POST /nonprofits/:id/follow     → org_follows insert
DELETE /nonprofits/:id/follow
GET /nonprofits/:id/reviews     → (needs a reviews table — see 5)
```

### Participant — My Tickets
```
GET /users/me/tickets?status=active|won|past   → join tickets + orders + draw_winners
GET /users/me/tickets/stats                     → counts for the tab badges
POST /tickets/:winnerId/claim                   → mark draw_winners.prize_claimed
```

### Participant — Saved
```
GET /users/me/saved-raffles
GET /users/me/following
DELETE /users/me/saved-raffles/:id    (swipe to remove)
```

### Participant — Profile
```
GET /users/me/profile           → user + payment_methods + addresses + prefs + stats
PATCH /users/me                 → name, phone
GET/POST/DELETE /users/me/payment-methods
GET/POST/PATCH/DELETE /users/me/shipping-addresses
PATCH /users/me/notification-preferences
```

### Participant — Draw watch & verify
```
GET /raffles/:id/draw           → draw_results + draw_winners (the frozen result)
GET /raffles/:id/draw/live      → { watchers_now, is_live }  (websocket ideal)
GET /raffles/:id/draw/verify    → { seed, commitment_hash, algorithm, tickets }
```
The verify screen recomputes client-side; this endpoint just supplies raw data.

### Nonprofit — Dashboard
```
GET /nonprofits/me/dashboard    → {
    this_month: { raised, raised_trend_pct, tickets_today, tickets_last_hour },
    alerts: [...],
    active_raffles: [...],
    all_time: { total_raised, raffles_run, follower_count, total_tickets_sold },
    recent_activity: [...]
}
```
Most of this is aggregation over `tickets`, `orders`, `org_follows`,
`raffle_items`. `recent_activity` needs an activity log (see 5.3).

### Nonprofit — Manage Raffles
```
GET /nonprofits/me/raffles?status=&search=&page=
POST /raffles                   → create (the wizard payload)
POST /raffles/:id/publish       → draft → active
POST /raffles/:id/duplicate
DELETE /raffles/:id             → draft only
GET /raffles/:id/export-csv     → ticket data export
```

### Nonprofit — Create raffle (the wizard)
```
POST /raffles
Body = CreateRaffle payload. Server:
  - validates category exists (trigger syncs category_path)
  - generates slug (ensure unique)
  - generates seed + commitment_hash, stores seed, returns nothing secret
  - inserts prize_tiers rows from attributes.draw.prize_tiers
  - draw_at stays NULL (set later when sold out)
  - status = 'draft' or 'active'
```

### Nonprofit — Analytics
```
GET /nonprofits/me/analytics?range=7d|30d|90d|year|all → {
    kpis, revenue_series, top_raffles, buyer_breakdown, conversion, insights
}
GET /nonprofits/me/analytics/export?range=  → PDF/CSV report
```
`buyer_breakdown` (repeat vs first-time vs from-followers) needs join logic
across `orders`, `org_follows`, and historical purchase counts. `conversion`
needs view tracking (see 5.1). `insights` is a rules engine over the data.

### Nonprofit — Org
```
GET /nonprofits/me              → profile + bank + team + prefs + all_time
PATCH /nonprofits/me            → name, bio, categories, logo
PATCH /nonprofits/me/contact    → website, phone, address, social_links
GET/POST /nonprofits/me/bank-account     → Stripe Connect onboarding
GET /nonprofits/me/payouts
GET /nonprofits/me/team                  → from nonprofit_users
POST /nonprofits/me/team/invite
PATCH /nonprofits/me/notification-preferences
```

---

## 4. THE DRAW SYSTEM (most complex)

A background job (or scheduled worker) is the heart of this. Flow:

1. **Sold-out trigger:** when `tickets_sold` reaches `max_tickets`, OR the org
   manually schedules, set `draw_at`. (The frontend defers `draw_at` until
   sold out by design.)
2. **At `draw_at`:** a worker fires the draw:
   - set status `drawing`, create `draw_results` row, set `draw_started_at`
   - load the secret seed, compute the winning order with HMAC-SHA256 keyed by
     `seed` over `(raffle_id, sorted ticket numbers)`
   - take first `winner_count`, assign ranks per `reveal_order`
   - insert `draw_winners` rows with computed `revealed_at_ms` pacing
   - reveal the `seed` on `draw_results`
   - set status `completed` / `drawn` after `draw_duration_ms`
3. **Live sync:** websocket channel per raffle pushes status changes + watcher
   counts. Polling every 2-3s is an acceptable v1 fallback.
4. **Privacy:** winner names are masked (`J••• D•••`) for everyone except the
   winner viewing their own result. Compute `is_current_user` server-side from
   the authed session.

See the existing `DRAW_BACKEND_CONTRACT.md` for the full Go struct shapes and
the reveal-timing model.

---

## 5. SUGGESTED DATA TO COLLECT (for ML / analytics / future features)

Your current schema captures transactions well but little **behavioral** data.
For recommendation engines, conversion analytics, and data-science projects,
consider collecting:

### 5.1 View / impression events (highest value)
Conversion analytics on the dashboard need views, which you don't track yet.

```sql
CREATE TABLE raffle_views (
    id BIGSERIAL PRIMARY KEY,
    raffle_item_id UUID NOT NULL REFERENCES raffle_items(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),     -- NULL for anonymous
    session_id TEXT,                        -- correlate anonymous → signup
    source TEXT,                            -- home | explore | search | share | org_page
    referrer TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_views_raffle ON raffle_views (raffle_item_id, created_at);
CREATE INDEX idx_views_user ON raffle_views (user_id, created_at);
```
Enables: view→purchase conversion, "trending" ranking, abandoned-interest
retargeting, and per-source attribution.

### 5.2 Search queries
```sql
CREATE TABLE search_events (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    query TEXT NOT NULL,
    result_count INT NOT NULL,
    clicked_raffle_id UUID REFERENCES raffle_items(id),  -- did they click a result?
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
Enables: demand sensing (what prizes people want that you don't have),
search-quality tuning, autocomplete.

### 5.3 Activity / event log (powers dashboard feed + ML features)
A single append-only event stream is gold for data science.

```sql
CREATE TABLE activity_events (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id UUID REFERENCES users(id),
    nonprofit_id UUID REFERENCES nonprofits(id),
    raffle_item_id UUID REFERENCES raffle_items(id),
    event_type TEXT NOT NULL,    -- ticket_purchase | follow | unfollow | view |
                                 -- raffle_created | draw_complete | prize_claimed | share
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_activity_org ON activity_events (nonprofit_id, created_at DESC);
CREATE INDEX idx_activity_type ON activity_events (event_type, created_at DESC);
```
Powers the dashboard's recent-activity feed directly, AND becomes the feature
source for churn prediction, LTV modeling, and collaborative filtering.

### 5.4 Recommendation-engine signals
With views + purchases + follows + categories you can build:
- **Content-based:** recommend raffles in categories a user buys/views/saves in
  (you already have `category_path` and JSONB `attributes` to match on).
- **Collaborative filtering:** "users who bought tickets to X also bought Y"
  from the `tickets`/`orders` co-occurrence matrix.
- **Org affinity:** followers + repeat-buyer rate per org → "orgs you may like."

To make these strong, also capture:
```sql
ALTER TABLE users ADD COLUMN preferred_categories TEXT[] DEFAULT '{}';  -- onboarding
ALTER TABLE users ADD COLUMN location_region TEXT;   -- for local-cause recs (coarse, privacy-safe)
```

### 5.5 Share / referral tracking
The app has share buttons everywhere. Track them to measure virality.
```sql
CREATE TABLE shares (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    raffle_item_id UUID REFERENCES raffle_items(id),
    nonprofit_id UUID REFERENCES nonprofits(id),
    channel TEXT,            -- copy_link | sms | email | social
    share_token TEXT UNIQUE, -- attribute resulting signups/purchases back
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
Enables: viral coefficient, top-referrer rewards, attribution of purchases to
the share that drove them.

### 5.6 Reviews (the nonprofit profile shows reviews)
The participant nonprofit-profile screen renders reviews but there's no table.
```sql
CREATE TABLE nonprofit_reviews (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nonprofit_id UUID NOT NULL REFERENCES nonprofits(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    body TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (nonprofit_id, user_id)
);
```

### 5.7 Denormalized counters (performance)
Computing follower counts and totals on every dashboard load is expensive.
Add maintained counters (via triggers, like `tickets_sold` already uses):
```sql
ALTER TABLE nonprofits ADD COLUMN follower_count INT NOT NULL DEFAULT 0;
ALTER TABLE nonprofits ADD COLUMN total_raised_cents BIGINT NOT NULL DEFAULT 0;
ALTER TABLE nonprofits ADD COLUMN raffles_run INT NOT NULL DEFAULT 0;
```

---

## 6. OPEN QUESTIONS / DECISIONS

1. **Money type:** schema uses `NUMERIC(10,2)`; new tables above use
   `*_cents INT`. Pick one convention platform-wide. Cents-as-INT avoids float
   issues and is recommended; if staying with NUMERIC, change the new tables.
2. **`completed` vs `drawn`:** the frontend uses `completed`; schema uses
   `drawn`. Map one to the other consistently.
3. **Donation add-on:** the tax/receipt feature (deferred) needs `orders`
   to track `donation_cents` separately — already included above.
4. **Sequential pricing & free ranges:** `assign_ticket_number()` currently
   auto-increments. For `pay_what_you_pull` the ticket NUMBER determines price,
   and free ranges (e.g. 1–10 free) change `price_paid`. The purchase handler
   must compute `price_paid` from the ticket number + strategy + free range, not
   assume a flat `ticket_price`.
5. **Anonymous viewing:** decide whether non-logged-in users can browse (affects
   whether `raffle_views.user_id` is nullable — assumed yes above).
