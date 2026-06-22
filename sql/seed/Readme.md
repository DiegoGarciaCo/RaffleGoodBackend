# Database seeding

`seed.sql` populates a realistic test dataset built around the
**Ministry of Giving** example org so you can develop endpoints against real
data and swap the frontend mock data for live API calls.

## Prerequisites

Run migrations first (the seed assumes all tables/triggers exist):

```bash
goose -dir sql/schema postgres "$DATABASE_URL" up
```

## Run the seed

```bash
psql "$DATABASE_URL" -f seed.sql
```

It runs in a transaction and **truncates the app tables first**, so it's safe
to re-run as often as you like. It does NOT touch the better-auth tables
(`users`, `session`, `accounts`, `verification`) — except it inserts/deletes
test users under the `@seed.rafflegood.test` email domain so re-runs stay clean.

## What you get

- **2 nonprofits** — Ministry of Giving (verified, the main org) and Hope Harbor
  (unverified, for cross-org testing).
- **6 categories** with attribute schemas (electronics, vehicles, clothing, cash,
  home & garden, experiences).
- **7 raffles** covering every status the UI renders:
  - active (fixed price), active/ending-soon, active (pay-what-you-pull),
    draft, completed (single winner), completed (ranked 3 winners), + one for the
    second org.
- **Tickets & orders** including a set owned by **Jamie Davidson**
  (`jamie@seed.rafflegood.test`) so the participant "My Tickets" screen has data.
- **Draw results & winners** for the two completed raffles, with Jamie set as a
  winner — including a deliberately **unclaimed** 2nd-place prize so the
  dashboard "unclaimed prize" alert and the Won-tab claim button both light up.
- **Engagement**: saved raffles, follows, reviews, notification prefs.
- **Profile**: a saved Visa card and shipping address for Jamie.
- **Behavioral events**: ~320 views, searches (incl. zero-result ones for
  demand-sensing), an activity feed for the dashboard, and shares.

## Test accounts

| Role        | Name           | Email                          | Notes |
|-------------|----------------|--------------------------------|-------|
| Org owner   | James Carter   | james@seed.rafflegood.test     | Ministry of Giving owner |
| Org admin   | Sarah Mitchell | sarah@seed.rafflegood.test     | |
| Org member  | David Reyes    | david@seed.rafflegood.test     | |
| Participant | Jamie Davidson | jamie@seed.rafflegood.test     | Has tickets, a win, an unclaimed prize |

> These users have no auth credentials (no `accounts` rows). To log in as them,
> either create credentials through your better-auth signup flow using these
> emails, or point your dev session at the relevant user id.

## Notes on counters

`tickets_sold`, `follower_count`, and `total_raised_cents` are maintained by
triggers. The seed inserts a representative sample of tickets (not thousands),
then **overrides** the counters at the end to the display values used in the
mock data (e.g. 1,240 sold, $98.5K raised, 1,840 followers) so the UI looks
populated without millions of rows. If you'd rather have exact counts, remove
the override `UPDATE` statements near the end of `seed.sql`.
