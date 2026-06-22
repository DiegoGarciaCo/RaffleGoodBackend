-- ============================================================================
-- RaffleGood seed data
-- ============================================================================
-- Realistic test data centered on the "Ministry of Giving" example org.
-- Safe to re-run: it wipes the app tables first (NOT the better-auth tables,
-- except seeded test users it created).
--
-- Run with:  psql "$DATABASE_URL" -f seed.sql
--
-- Notes:
--  * Ticket inserts intentionally pass price_paid explicitly so the
--    assign_ticket_number trigger doesn't have to compute it. ticket_number
--    is left to the trigger (sequential) EXCEPT where we need specific numbers.
--  * Counters (tickets_sold, follower_count, total_raised_cents) are maintained
--    by triggers, so we let them populate naturally as we insert.
-- ============================================================================
BEGIN
;

-- ── Clean slate (child → parent order) ──────────────────────────────────────
TRUNCATE TABLE shares,
activity_events,
search_events,
raffle_views,
draw_winners,
draw_results,
prize_tiers,
tickets,
orders,
saved_raffles,
org_follows,
nonprofit_reviews,
notification_preferences,
payment_methods,
shipping_addresses,
nonprofit_social_links,
nonprofit_bank_accounts,
payouts,
nonprofit_users,
nonprofit_verifications,
raffle_items,
categories,
nonprofits RESTART IDENTITY CASCADE;

-- Seeded test users (delete prior seed users by a marker email domain).
DELETE FROM
    users
WHERE
    email LIKE '%@seed.rafflegood.test';

-- ============================================================================
-- USERS  (better-auth table — we insert test users with a marker domain)
-- ============================================================================
INSERT INTO
    users (
        id,
        name,
        email,
        "emailVerified",
        image,
        "createdAt",
        "updatedAt",
        preferred_categories,
        location_region
    )
VALUES
    (
        '11111111-1111-1111-1111-111111111111',
        'James Carter',
        'james@seed.rafflegood.test',
        TRUE,
        NULL,
        NOW() - INTERVAL '400 days',
        NOW(),
        '{}',
        'Nashville, TN'
    ),
    (
        '22222222-2222-2222-2222-222222222222',
        'Sarah Mitchell',
        'sarah@seed.rafflegood.test',
        TRUE,
        NULL,
        NOW() - INTERVAL '380 days',
        NOW(),
        '{}',
        'Nashville, TN'
    ),
    (
        '33333333-3333-3333-3333-333333333333',
        'David Reyes',
        'david@seed.rafflegood.test',
        TRUE,
        NULL,
        NOW() - INTERVAL '300 days',
        NOW(),
        '{}',
        'Memphis, TN'
    ),
    (
        '44444444-4444-4444-4444-444444444444',
        'Jamie Davidson',
        'jamie@seed.rafflegood.test',
        TRUE,
        NULL,
        NOW() - INTERVAL '200 days',
        NOW(),
        ARRAY ['electronics','cash'],
        'Atlanta, GA'
    ),
    (
        '55555555-5555-5555-5555-555555555555',
        'Maria Robinson',
        'maria@seed.rafflegood.test',
        TRUE,
        NULL,
        NOW() - INTERVAL '150 days',
        NOW(),
        ARRAY ['vehicles'],
        'Chicago, IL'
    ),
    (
        '66666666-6666-6666-6666-666666666666',
        'Alex Stone',
        'alex@seed.rafflegood.test',
        TRUE,
        NULL,
        NOW() - INTERVAL '120 days',
        NOW(),
        '{}',
        'Dallas, TX'
    ),
    (
        '77777777-7777-7777-7777-777777777777',
        'Marcus Thompson',
        'marcus@seed.rafflegood.test',
        TRUE,
        NULL,
        NOW() - INTERVAL '90 days',
        NOW(),
        ARRAY ['community','faith'],
        'Nashville, TN'
    ),
    (
        '88888888-8888-8888-8888-888888888888',
        'Grace Lee',
        'grace@seed.rafflegood.test',
        TRUE,
        NULL,
        NOW() - INTERVAL '60 days',
        NOW(),
        '{}',
        'Houston, TX'
    ),
    (
        '99999999-9999-9999-9999-999999999999',
        'Tom Becker',
        'tom@seed.rafflegood.test',
        TRUE,
        NULL,
        NOW() - INTERVAL '30 days',
        NOW(),
        '{}',
        'Denver, CO'
    );

-- ============================================================================
-- NONPROFITS
-- ============================================================================
INSERT INTO
    nonprofits (
        id,
        name,
        slug,
        description,
        image,
        address,
        phone,
        ein,
        "hasToPay",
        website,
        categories
    )
VALUES
    (
        'a0000000-0000-0000-0000-000000000001',
        'Ministry of Giving',
        'ministry-of-giving',
        'Inspired by 2 Corinthians 9:6-15, we exist to resource generous giving that meets real needs and brings glory to God. Every raffle connects cheerful givers with communities in need.',
        NULL,
        'Nashville, TN',
        '(615) 555-0142',
        '47-1234567',
        FALSE,
        'ministryofgiving.org',
        ARRAY ['community','faith','education']
    ),
    (
        'a0000000-0000-0000-0000-000000000002',
        'Hope Harbor Outreach',
        'hope-harbor-outreach',
        'Serving the unhoused and food-insecure across middle Tennessee.',
        NULL,
        'Memphis, TN',
        '(901) 555-0188',
        '62-9876543',
        TRUE,
        'hopeharbor.org',
        ARRAY ['community','health']
    );

-- Verification (Ministry is verified, Hope Harbor pending)
INSERT INTO
    nonprofit_verifications (
        nonprofit_id,
        "isVerified",
        "verificationMethod",
        "exemptStatus",
        subsection,
        "nteeCd",
        "rulingDate"
    )
VALUES
    (
        'a0000000-0000-0000-0000-000000000001',
        TRUE,
        'irs_pub78',
        1,
        3,
        'X20',
        '2019-05-01'
    ),
    (
        'a0000000-0000-0000-0000-000000000002',
        FALSE,
        NULL,
        NULL,
        NULL,
        NULL,
        NULL
    );

-- Team for Ministry of Giving
INSERT INTO
    nonprofit_users (nonprofit_id, user_id, role, MODE)
VALUES
    (
        'a0000000-0000-0000-0000-000000000001',
        '11111111-1111-1111-1111-111111111111',
        'owner',
        'active'
    ),
    (
        'a0000000-0000-0000-0000-000000000001',
        '22222222-2222-2222-2222-222222222222',
        'admin',
        'active'
    ),
    (
        'a0000000-0000-0000-0000-000000000001',
        '33333333-3333-3333-3333-333333333333',
        'member',
        'active'
    );

-- Social links
INSERT INTO
    nonprofit_social_links (nonprofit_id, platform, url)
VALUES
    (
        'a0000000-0000-0000-0000-000000000001',
        'instagram',
        'https://instagram.com/ministryofgiving'
    ),
    (
        'a0000000-0000-0000-0000-000000000001',
        'facebook',
        'https://facebook.com/ministryofgiving'
    );

-- Bank account
INSERT INTO
    nonprofit_bank_accounts (
        nonprofit_id,
        processor,
        processor_account_id,
        bank_name,
        last4,
        account_type,
        is_verified
    )
VALUES
    (
        'a0000000-0000-0000-0000-000000000001',
        'stripe_connect',
        'acct_seedtest001',
        'First Community Bank',
        '4821',
        'checking',
        TRUE
    );

-- ============================================================================
-- CATEGORIES  (tree: root → children)
-- ============================================================================
INSERT INTO
    categories (
        id,
        parent_id,
        path,
        depth,
        name,
        slug,
        icon,
        sort_order,
        attribute_schema
    )
VALUES
    (
        'c0000000-0000-0000-0000-000000000001',
        NULL,
        'electronics',
        0,
        'Electronics',
        'electronics',
        'phone-portrait-outline',
        1,
        '{"brand":"string","model":"string","condition":["New","Like new","Refurbished","Used"]}'
    ),
    (
        'c0000000-0000-0000-0000-000000000002',
        NULL,
        'vehicles',
        0,
        'Vehicles',
        'vehicles',
        'car-outline',
        2,
        '{"make":"string","model":"string","year":"number","color":"string","mileage":"number"}'
    ),
    (
        'c0000000-0000-0000-0000-000000000003',
        NULL,
        'clothing',
        0,
        'Clothing',
        'clothing',
        'shirt-outline',
        3,
        '{"gender":["Unisex","Men","Women","Kids"],"size":["XS","S","M","L","XL"],"color":"string"}'
    ),
    (
        'c0000000-0000-0000-0000-000000000004',
        NULL,
        'cash',
        0,
        'Cash',
        'cash',
        'cash-outline',
        4,
        '{"amount":"number"}'
    ),
    (
        'c0000000-0000-0000-0000-000000000005',
        NULL,
        'home-garden',
        0,
        'Home & Garden',
        'home-garden',
        'home-outline',
        5,
        '{"item_type":"string","brand":"string"}'
    ),
    (
        'c0000000-0000-0000-0000-000000000006',
        NULL,
        'experiences',
        0,
        'Experiences',
        'experiences',
        'ticket-outline',
        6,
        '{"experience_type":"string","location":"string"}'
    );

-- ============================================================================
-- RAFFLE ITEMS
-- ============================================================================
-- Ministry of Giving raffles spanning all states for testing every screen.
-- 1) ACTIVE — fixed price, partially sold
INSERT INTO
    raffle_items (
        id,
        category_id,
        title,
        slug,
        description,
        STATUS,
        ticket_strategy,
        ticket_assignment_strategy,
        draw_strategy,
        ticket_price,
        free_tickets,
        max_tickets,
        attributes,
        image_urls,
        draw_at,
        created_by,
        created_at
    )
VALUES
    (
        'd0000000-0000-0000-0000-000000000001',
        'c0000000-0000-0000-0000-000000000001',
        'Community Food Pantry Fundraiser',
        'community-food-pantry-fundraiser',
        'Win a PlayStation 5 and help us stock the shelves to feed 200 families this month.',
        'active',
        'fixed',
        'random',
        'single',
        5.00,
        0,
        2000,
        '{"prize":{"brand":"Sony","model":"PlayStation 5","condition":"New"}}',
        '{}',
        NULL,
        'a0000000-0000-0000-0000-000000000001',
        NOW() - INTERVAL '10 days'
    );

-- 2) ACTIVE / ending soon — fixed price, nearly sold out
INSERT INTO
    raffle_items (
        id,
        category_id,
        title,
        slug,
        description,
        STATUS,
        ticket_strategy,
        ticket_assignment_strategy,
        draw_strategy,
        ticket_price,
        free_tickets,
        max_tickets,
        attributes,
        image_urls,
        draw_at,
        created_by,
        created_at
    )
VALUES
    (
        'd0000000-0000-0000-0000-000000000002',
        'c0000000-0000-0000-0000-000000000004',
        'Back-to-School Supply Drive',
        'back-to-school-supply-drive',
        'Win $500 cash. Proceeds buy backpacks and supplies for kids in need.',
        'active',
        'fixed',
        'random',
        'single',
        5.00,
        0,
        500,
        '{"prize":{"amount":500}}',
        '{}',
        NOW() + INTERVAL '6 hours',
        'a0000000-0000-0000-0000-000000000001',
        NOW() - INTERVAL '20 days'
    );

-- 3) ACTIVE — pay-what-you-pull (the PS5 example)
INSERT INTO
    raffle_items (
        id,
        category_id,
        title,
        slug,
        description,
        STATUS,
        ticket_strategy,
        ticket_assignment_strategy,
        draw_strategy,
        ticket_price,
        free_tickets,
        free_ticket_start_range,
        free_ticket_end_range,
        max_tickets,
        attributes,
        image_urls,
        draw_at,
        created_by,
        created_at
    )
VALUES
    (
        'd0000000-0000-0000-0000-000000000003',
        'c0000000-0000-0000-0000-000000000002',
        'Single Mothers Support Fund',
        'single-mothers-support-fund',
        'Win a 2024 Toyota Camry. Pay-what-you-pull: your ticket number is your price.',
        'active',
        'pay_what_you_pull',
        'user_picks',
        'single',
        0.00,
        10,
        1,
        10,
        65,
        '{"prize":{"make":"Toyota","model":"Camry","year":2024,"color":"Silver","mileage":12000}}',
        '{}',
        NULL,
        'a0000000-0000-0000-0000-000000000001',
        NOW() - INTERVAL '5 days'
    );

-- 4) DRAFT — incomplete
INSERT INTO
    raffle_items (
        id,
        category_id,
        title,
        slug,
        description,
        STATUS,
        ticket_strategy,
        ticket_assignment_strategy,
        draw_strategy,
        ticket_price,
        free_tickets,
        max_tickets,
        attributes,
        image_urls,
        created_by,
        created_at
    )
VALUES
    (
        'd0000000-0000-0000-0000-000000000004',
        'c0000000-0000-0000-0000-000000000001',
        'Holiday Giving Campaign 2026',
        'holiday-giving-campaign-2026',
        'Win the latest iPhone. Draft — finishing setup.',
        'draft',
        'fixed',
        'random',
        'ranked',
        10.00,
        0,
        1500,
        '{"prize":{"brand":"Apple","model":"iPhone 16 Pro","condition":"New"}}',
        '{}',
        'a0000000-0000-0000-0000-000000000001',
        NOW() - INTERVAL '2 days'
    );

-- 5) COMPLETED — single winner, claimed
INSERT INTO
    raffle_items (
        id,
        category_id,
        title,
        slug,
        description,
        STATUS,
        ticket_strategy,
        ticket_assignment_strategy,
        draw_strategy,
        ticket_price,
        free_tickets,
        max_tickets,
        attributes,
        image_urls,
        draw_at,
        drawn_at,
        created_by,
        created_at
    )
VALUES
    (
        'd0000000-0000-0000-0000-000000000005',
        'c0000000-0000-0000-0000-000000000004',
        'Easter Food Basket Drive',
        'easter-food-basket-drive',
        'Won: $2,500 cash. Thank you to everyone who gave!',
        'completed',
        'fixed',
        'random',
        'single',
        5.00,
        0,
        500,
        '{"prize":{"amount":2500}}',
        '{}',
        NOW() - INTERVAL '45 days',
        NOW() - INTERVAL '45 days',
        'a0000000-0000-0000-0000-000000000001',
        NOW() - INTERVAL '75 days'
    );

-- 6) COMPLETED — ranked, 3 winners
INSERT INTO
    raffle_items (
        id,
        category_id,
        title,
        slug,
        description,
        STATUS,
        ticket_strategy,
        ticket_assignment_strategy,
        draw_strategy,
        ticket_price,
        free_tickets,
        max_tickets,
        attributes,
        image_urls,
        draw_at,
        drawn_at,
        created_by,
        created_at
    )
VALUES
    (
        'd0000000-0000-0000-0000-000000000006',
        'c0000000-0000-0000-0000-000000000005',
        'Winter Coat & Blanket Fund',
        'winter-coat-blanket-fund',
        'Three winners! Grand prize patio set plus runner-up gift cards.',
        'completed',
        'fixed',
        'random',
        'ranked',
        5.00,
        0,
        1200,
        '{"prize":{"item_type":"Patio furniture set","brand":"Weber"}}',
        '{}',
        NOW() - INTERVAL '120 days',
        NOW() - INTERVAL '120 days',
        'a0000000-0000-0000-0000-000000000001',
        NOW() - INTERVAL '150 days'
    );

-- 7) Hope Harbor — one active raffle (second org for cross-org testing)
INSERT INTO
    raffle_items (
        id,
        category_id,
        title,
        slug,
        description,
        STATUS,
        ticket_strategy,
        ticket_assignment_strategy,
        draw_strategy,
        ticket_price,
        free_tickets,
        max_tickets,
        attributes,
        image_urls,
        created_by,
        created_at
    )
VALUES
    (
        'd0000000-0000-0000-0000-000000000007',
        'c0000000-0000-0000-0000-000000000006',
        'Concert Night Fundraiser',
        'concert-night-fundraiser',
        'Win two VIP concert tickets. Supports our winter shelter program.',
        'active',
        'fixed',
        'random',
        'single',
        8.00,
        0,
        800,
        '{"prize":{"experience_type":"VIP concert tickets","location":"Nashville, TN"}}',
        '{}',
        'a0000000-0000-0000-0000-000000000002',
        NOW() - INTERVAL '7 days'
    );

-- ============================================================================
-- PRIZE TIERS  (for ranked/multi raffles)
-- ============================================================================
INSERT INTO
    prize_tiers (
        raffle_item_id,
        rank,
        title,
        description,
        value_cents
    )
VALUES
    (
        'd0000000-0000-0000-0000-000000000006',
        1,
        'Weber patio furniture set',
        'Full 6-piece outdoor set',
        240000
    ),
    (
        'd0000000-0000-0000-0000-000000000006',
        2,
        '$500 gift card',
        NULL,
        50000
    ),
    (
        'd0000000-0000-0000-0000-000000000006',
        3,
        '$100 gift card',
        NULL,
        10000
    ),
    -- single-winner raffles get a single tier too
    (
        'd0000000-0000-0000-0000-000000000001',
        1,
        'PlayStation 5',
        'Brand new, disc edition',
        50000
    ),
    (
        'd0000000-0000-0000-0000-000000000002',
        1,
        '$500 cash',
        NULL,
        50000
    ),
    (
        'd0000000-0000-0000-0000-000000000003',
        1,
        '2024 Toyota Camry',
        'Silver, 12k miles',
        2800000
    ),
    (
        'd0000000-0000-0000-0000-000000000005',
        1,
        '$2,500 cash',
        NULL,
        250000
    ),
    (
        'd0000000-0000-0000-0000-000000000007',
        1,
        'VIP concert tickets (x2)',
        NULL,
        40000
    );

-- ============================================================================
-- ORDERS + TICKETS
-- ============================================================================
-- We insert orders, then tickets that reference them. The trigger assigns
-- sequential ticket_number and bumps tickets_sold + total_raised_cents.
-- Helper approach: insert many tickets per active raffle to simulate sales.
-- Raffle 1 (Food Pantry): ~1240 sold. We insert a representative sample of 60
-- across several buyers (enough to test UI; bump tickets_sold to realistic
-- value afterward to match the "62% sold" look without 1240 rows).
-- A few real orders for the current user (Jamie) so "My Tickets" has data.
INSERT INTO
    orders (
        id,
        user_id,
        raffle_item_id,
        ticket_count,
        subtotal_cents,
        donation_cents,
        total_cents,
        processor,
        STATUS,
        receipt_number,
        created_at
    )
VALUES
    (
        'e0000000-0000-0000-0000-000000000001',
        '44444444-4444-4444-4444-444444444444',
        'd0000000-0000-0000-0000-000000000001',
        5,
        2500,
        0,
        2500,
        'stripe',
        'paid',
        'RG-20260520-0001',
        NOW() - INTERVAL '4 days'
    ),
    (
        'e0000000-0000-0000-0000-000000000002',
        '44444444-4444-4444-4444-444444444444',
        'd0000000-0000-0000-0000-000000000002',
        3,
        1500,
        1000,
        2500,
        'stripe',
        'paid',
        'RG-20260521-0002',
        NOW() - INTERVAL '3 days'
    ),
    (
        'e0000000-0000-0000-0000-000000000003',
        '44444444-4444-4444-4444-444444444444',
        'd0000000-0000-0000-0000-000000000005',
        2,
        1000,
        0,
        1000,
        'stripe',
        'paid',
        'RG-20260301-0003',
        NOW() - INTERVAL '46 days'
    ),
    (
        'e0000000-0000-0000-0000-000000000004',
        '44444444-4444-4444-4444-444444444444',
        'd0000000-0000-0000-0000-000000000006',
        4,
        2000,
        0,
        2000,
        'stripe',
        'paid',
        'RG-20260101-0004',
        NOW() - INTERVAL '121 days'
    );

-- Jamie's tickets (let trigger assign numbers + price for the simple cases).
-- Food Pantry (raffle 1): 5 tickets @ $5
INSERT INTO
    tickets (
        raffle_item_id,
        user_id,
        order_id,
        price_paid,
        purchased_at
    )
SELECT
    'd0000000-0000-0000-0000-000000000001',
    '44444444-4444-4444-4444-444444444444',
    'e0000000-0000-0000-0000-000000000001',
    5.00,
    NOW() - INTERVAL '4 days'
FROM
    generate_series(1, 5);

-- Back-to-School (raffle 2): 3 tickets @ $5
INSERT INTO
    tickets (
        raffle_item_id,
        user_id,
        order_id,
        price_paid,
        purchased_at
    )
SELECT
    'd0000000-0000-0000-0000-000000000002',
    '44444444-4444-4444-4444-444444444444',
    'e0000000-0000-0000-0000-000000000002',
    5.00,
    NOW() - INTERVAL '3 days'
FROM
    generate_series(1, 3);

-- Easter (raffle 5, completed): 2 tickets — one of these will be a winner
INSERT INTO
    tickets (
        raffle_item_id,
        user_id,
        order_id,
        price_paid,
        purchased_at
    )
SELECT
    'd0000000-0000-0000-0000-000000000005',
    '44444444-4444-4444-4444-444444444444',
    'e0000000-0000-0000-0000-000000000003',
    5.00,
    NOW() - INTERVAL '46 days'
FROM
    generate_series(1, 2);

-- Winter Coat (raffle 6, completed ranked): 4 tickets
INSERT INTO
    tickets (
        raffle_item_id,
        user_id,
        order_id,
        price_paid,
        purchased_at
    )
SELECT
    'd0000000-0000-0000-0000-000000000006',
    '44444444-4444-4444-4444-444444444444',
    'e0000000-0000-0000-0000-000000000004',
    5.00,
    NOW() - INTERVAL '121 days'
FROM
    generate_series(1, 4);

-- Other buyers across active raffles (spreads ownership for realistic draws).
INSERT INTO
    tickets (
        raffle_item_id,
        user_id,
        price_paid,
        purchased_at
    )
SELECT
    'd0000000-0000-0000-0000-000000000001',
    u.uid,
    5.00,
    NOW() - (random() * INTERVAL '9 days')
FROM
    (
        VALUES
            ('55555555-5555-5555-5555-555555555555'::uuid),
            ('66666666-6666-6666-6666-666666666666'::uuid),
            ('77777777-7777-7777-7777-777777777777'::uuid),
            ('88888888-8888-8888-8888-888888888888'::uuid)
    ) AS u(uid),
    generate_series(1, 10);

-- 40 more tickets
-- Easter completed: other buyers so there's a pool to draw a winner from
INSERT INTO
    tickets (
        raffle_item_id,
        user_id,
        price_paid,
        purchased_at
    )
SELECT
    'd0000000-0000-0000-0000-000000000005',
    u.uid,
    5.00,
    NOW() - INTERVAL '46 days'
FROM
    (
        VALUES
            ('55555555-5555-5555-5555-555555555555'::uuid),
            ('66666666-6666-6666-6666-666666666666'::uuid),
            ('77777777-7777-7777-7777-777777777777'::uuid)
    ) AS u(uid),
    generate_series(1, 5);

-- Winter coat completed: other buyers
INSERT INTO
    tickets (
        raffle_item_id,
        user_id,
        price_paid,
        purchased_at
    )
SELECT
    'd0000000-0000-0000-0000-000000000006',
    u.uid,
    5.00,
    NOW() - INTERVAL '121 days'
FROM
    (
        VALUES
            ('55555555-5555-5555-5555-555555555555'::uuid),
            ('66666666-6666-6666-6666-666666666666'::uuid),
            ('88888888-8888-8888-8888-888888888888'::uuid)
    ) AS u(uid),
    generate_series(1, 6);

-- Make the active raffles LOOK realistically sold without inserting thousands
-- of rows. (Counters were bumped by the trigger for the rows above; we override
-- to the display values used throughout the mock data.)
UPDATE
    raffle_items
SET
    tickets_sold = 1240
WHERE
    id = 'd0000000-0000-0000-0000-000000000001';

UPDATE
    raffle_items
SET
    tickets_sold = 380
WHERE
    id = 'd0000000-0000-0000-0000-000000000002';

UPDATE
    raffle_items
SET
    tickets_sold = 55
WHERE
    id = 'd0000000-0000-0000-0000-000000000003';

UPDATE
    raffle_items
SET
    tickets_sold = 500
WHERE
    id = 'd0000000-0000-0000-0000-000000000005';

UPDATE
    raffle_items
SET
    tickets_sold = 1200
WHERE
    id = 'd0000000-0000-0000-0000-000000000006';

UPDATE
    raffle_items
SET
    tickets_sold = 310
WHERE
    id = 'd0000000-0000-0000-0000-000000000007';

-- ============================================================================
-- DRAW RESULTS + WINNERS  (for the two completed raffles)
-- ============================================================================
-- Raffle 5: Easter — single winner. Make Jamie's ticket the winner so the
-- "Won" tab + claim flow has data.
INSERT INTO
    draw_results (
        id,
        raffle_item_id,
        draw_strategy,
        reveal_order,
        winner_count,
        commitment_hash,
        seed,
        draw_started_at,
        draw_duration_ms,
        total_tickets,
        total_participants,
        verification_url,
        STATUS
    )
VALUES
    (
        'f0000000-0000-0000-0000-000000000005',
        'd0000000-0000-0000-0000-000000000005',
        'single',
        'forward',
        1,
        'e7b4c9f2a1d83b5c6e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d',
        'a3f9c2e8b1d47f6e29ac843b0f7e12d5',
        NOW() - INTERVAL '45 days',
        6000,
        500,
        180,
        'https://rafflegood.com/verify/easter-food-basket-drive',
        'completed'
    );

-- Winner: pick Jamie's first Easter ticket
INSERT INTO
    draw_winners (
        draw_result_id,
        pull_index,
        rank,
        ticket_id,
        ticket_number,
        user_id,
        prize_title,
        prize_value_cents,
        revealed_at_ms,
        prize_claimed,
        claimed_at
    )
SELECT
    'f0000000-0000-0000-0000-000000000005',
    0,
    1,
    t.id,
    t.ticket_number,
    '44444444-4444-4444-4444-444444444444',
    '$2,500 cash',
    250000,
    4000,
    TRUE,
    NOW() - INTERVAL '44 days'
FROM
    tickets t
WHERE
    t.raffle_item_id = 'd0000000-0000-0000-0000-000000000005'
    AND t.user_id = '44444444-4444-4444-4444-444444444444'
ORDER BY
    t.ticket_number
LIMIT
    1;

-- Raffle 6: Winter Coat — ranked, 3 winners. Make Jamie 2nd place + UNCLAIMED
-- so the dashboard "unclaimed prize" alert and the Won-tab claim CTA show.
INSERT INTO
    draw_results (
        id,
        raffle_item_id,
        draw_strategy,
        reveal_order,
        winner_count,
        commitment_hash,
        seed,
        draw_started_at,
        draw_duration_ms,
        total_tickets,
        total_participants,
        verification_url,
        STATUS
    )
VALUES
    (
        'f0000000-0000-0000-0000-000000000006',
        'd0000000-0000-0000-0000-000000000006',
        'ranked',
        'forward',
        3,
        'b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2',
        '9f8e7d6c5b4a39281706f5e4d3c2b1a0',
        NOW() - INTERVAL '120 days',
        16000,
        1200,
        410,
        'https://rafflegood.com/verify/winter-coat-blanket-fund',
        'completed'
    );

-- 1st place: another user
INSERT INTO
    draw_winners (
        draw_result_id,
        pull_index,
        rank,
        ticket_id,
        ticket_number,
        user_id,
        prize_title,
        prize_value_cents,
        revealed_at_ms,
        prize_claimed,
        claimed_at
    )
SELECT
    'f0000000-0000-0000-0000-000000000006',
    0,
    1,
    t.id,
    t.ticket_number,
    '55555555-5555-5555-5555-555555555555',
    'Weber patio furniture set',
    240000,
    4000,
    TRUE,
    NOW() - INTERVAL '119 days'
FROM
    tickets t
WHERE
    t.raffle_item_id = 'd0000000-0000-0000-0000-000000000006'
    AND t.user_id = '55555555-5555-5555-5555-555555555555'
ORDER BY
    t.ticket_number
LIMIT
    1;

-- 2nd place: Jamie — UNCLAIMED
INSERT INTO
    draw_winners (
        draw_result_id,
        pull_index,
        rank,
        ticket_id,
        ticket_number,
        user_id,
        prize_title,
        prize_value_cents,
        revealed_at_ms,
        prize_claimed
    )
SELECT
    'f0000000-0000-0000-0000-000000000006',
    1,
    2,
    t.id,
    t.ticket_number,
    '44444444-4444-4444-4444-444444444444',
    '$500 gift card',
    50000,
    9000,
    FALSE
FROM
    tickets t
WHERE
    t.raffle_item_id = 'd0000000-0000-0000-0000-000000000006'
    AND t.user_id = '44444444-4444-4444-4444-444444444444'
ORDER BY
    t.ticket_number
LIMIT
    1;

-- 3rd place: another user
INSERT INTO
    draw_winners (
        draw_result_id,
        pull_index,
        rank,
        ticket_id,
        ticket_number,
        user_id,
        prize_title,
        prize_value_cents,
        revealed_at_ms,
        prize_claimed,
        claimed_at
    )
SELECT
    'f0000000-0000-0000-0000-000000000006',
    2,
    3,
    t.id,
    t.ticket_number,
    '66666666-6666-6666-6666-666666666666',
    '$100 gift card',
    10000,
    14000,
    TRUE,
    NOW() - INTERVAL '118 days'
FROM
    tickets t
WHERE
    t.raffle_item_id = 'd0000000-0000-0000-0000-000000000006'
    AND t.user_id = '66666666-6666-6666-6666-666666666666'
ORDER BY
    t.ticket_number
LIMIT
    1;

-- Pre-publish commitment hashes for ACTIVE raffles (seed not yet revealed)
INSERT INTO
    draw_results (
        raffle_item_id,
        draw_strategy,
        reveal_order,
        winner_count,
        commitment_hash,
        total_tickets,
        total_participants,
        STATUS
    )
VALUES
    (
        'd0000000-0000-0000-0000-000000000001',
        'single',
        'forward',
        1,
        'aaaa1111bbbb2222cccc3333dddd4444eeee5555ffff6666aaaa7777bbbb8888',
        2000,
        0,
        'scheduled'
    ),
    (
        'd0000000-0000-0000-0000-000000000002',
        'single',
        'forward',
        1,
        'cccc1111dddd2222eeee3333ffff4444aaaa5555bbbb6666cccc7777dddd8888',
        500,
        0,
        'scheduled'
    ),
    (
        'd0000000-0000-0000-0000-000000000003',
        'single',
        'forward',
        1,
        'eeee1111ffff2222aaaa3333bbbb4444cccc5555dddd6666eeee7777ffff8888',
        65,
        0,
        'scheduled'
    );

-- ============================================================================
-- ENGAGEMENT  (saved, follows, reviews)
-- ============================================================================
-- Jamie saves a couple raffles
INSERT INTO
    saved_raffles (user_id, raffle_item_id)
VALUES
    (
        '44444444-4444-4444-4444-444444444444',
        'd0000000-0000-0000-0000-000000000003'
    ),
    (
        '44444444-4444-4444-4444-444444444444',
        'd0000000-0000-0000-0000-000000000007'
    );

-- Follows (drive Ministry's follower_count via trigger)
INSERT INTO
    org_follows (user_id, nonprofit_id)
VALUES
    (
        '44444444-4444-4444-4444-444444444444',
        'a0000000-0000-0000-0000-000000000001'
    ),
    (
        '55555555-5555-5555-5555-555555555555',
        'a0000000-0000-0000-0000-000000000001'
    ),
    (
        '66666666-6666-6666-6666-666666666666',
        'a0000000-0000-0000-0000-000000000001'
    ),
    (
        '77777777-7777-7777-7777-777777777777',
        'a0000000-0000-0000-0000-000000000001'
    ),
    (
        '88888888-8888-8888-8888-888888888888',
        'a0000000-0000-0000-0000-000000000001'
    ),
    (
        '99999999-9999-9999-9999-999999999999',
        'a0000000-0000-0000-0000-000000000002'
    );

-- Reviews for Ministry
INSERT INTO
    nonprofit_reviews (nonprofit_id, user_id, rating, body)
VALUES
    (
        'a0000000-0000-0000-0000-000000000001',
        '55555555-5555-5555-5555-555555555555',
        5,
        'Transparent and the draw was clearly fair. Felt great to give.'
    ),
    (
        'a0000000-0000-0000-0000-000000000001',
        '66666666-6666-6666-6666-666666666666',
        5,
        'Won a prize and the claim process was smooth.'
    ),
    (
        'a0000000-0000-0000-0000-000000000001',
        '77777777-7777-7777-7777-777777777777',
        4,
        'Great cause. Would love more frequent raffles.'
    );

-- Notification preferences
INSERT INTO
    notification_preferences (user_id, prefs)
VALUES
    (
        '44444444-4444-4444-4444-444444444444',
        '{"draw_results":true,"draw_reminders":true,"new_raffles_from_following":false,"promotions":false}'
    );

INSERT INTO
    notification_preferences (nonprofit_id, prefs)
VALUES
    (
        'a0000000-0000-0000-0000-000000000001',
        '{"ticket_purchases":true,"draw_reminders":true,"new_followers":false,"milestone_alerts":true}'
    );

-- ============================================================================
-- PROFILE  (payment methods, addresses for Jamie)
-- ============================================================================
INSERT INTO
    payment_methods (
        user_id,
        processor,
        processor_pm_id,
        brand,
        last4,
        exp_month,
        exp_year,
        is_default
    )
VALUES
    (
        '44444444-4444-4444-4444-444444444444',
        'stripe',
        'pm_seedtest001',
        'visa',
        '4242',
        12,
        2027,
        TRUE
    );

INSERT INTO
    shipping_addresses (
        user_id,
        line1,
        line2,
        city,
        state,
        zip,
        country,
        is_default
    )
VALUES
    (
        '44444444-4444-4444-4444-444444444444',
        '123 Main St',
        'Apt 4B',
        'Atlanta',
        'GA',
        '30301',
        'US',
        TRUE
    );

-- ============================================================================
-- BEHAVIORAL EVENTS  (views, searches, activity feed, shares)
-- ============================================================================
-- Views across active raffles (for conversion analytics)
INSERT INTO
    raffle_views (raffle_item_id, user_id, source, created_at)
SELECT
    'd0000000-0000-0000-0000-000000000001',
    (
        ARRAY ['44444444-4444-4444-4444-444444444444','55555555-5555-5555-5555-555555555555','66666666-6666-6666-6666-666666666666',NULL]::uuid []
    ) [1 + floor(random()*4)],
    (ARRAY ['home','explore','search','share']) [1 + floor(random()*4)],
    NOW() - (random() * INTERVAL '10 days')
FROM
    generate_series(1, 200);

INSERT INTO
    raffle_views (raffle_item_id, user_id, source, created_at)
SELECT
    'd0000000-0000-0000-0000-000000000002',
    NULL,
    (ARRAY ['home','explore','search']) [1 + floor(random()*3)],
    NOW() - (random() * INTERVAL '20 days')
FROM
    generate_series(1, 120);

-- Searches (some with zero results for demand-sensing)
INSERT INTO
    search_events (user_id, query, result_count, created_at)
VALUES
    (
        '44444444-4444-4444-4444-444444444444',
        'playstation',
        1,
        NOW() - INTERVAL '4 days'
    ),
    (
        '55555555-5555-5555-5555-555555555555',
        'car raffle',
        2,
        NOW() - INTERVAL '3 days'
    ),
    (
        '66666666-6666-6666-6666-666666666666',
        'macbook',
        0,
        NOW() - INTERVAL '2 days'
    ),
    (
        '77777777-7777-7777-7777-777777777777',
        'tesla',
        0,
        NOW() - INTERVAL '1 day'
    ),
    (
        NULL,
        'iphone',
        1,
        NOW() - INTERVAL '5 hours'
    );

-- Activity feed for Ministry dashboard
INSERT INTO
    activity_events (
        actor_user_id,
        nonprofit_id,
        raffle_item_id,
        event_type,
        metadata,
        created_at
    )
VALUES
    (
        '44444444-4444-4444-4444-444444444444',
        'a0000000-0000-0000-0000-000000000001',
        'd0000000-0000-0000-0000-000000000001',
        'ticket_purchase',
        '{"count":5}',
        NOW() - INTERVAL '2 minutes'
    ),
    (
        '77777777-7777-7777-7777-777777777777',
        'a0000000-0000-0000-0000-000000000001',
        NULL,
        'follow',
        '{}',
        NOW() - INTERVAL '11 minutes'
    ),
    (
        '55555555-5555-5555-5555-555555555555',
        'a0000000-0000-0000-0000-000000000001',
        'd0000000-0000-0000-0000-000000000002',
        'ticket_purchase',
        '{"count":2}',
        NOW() - INTERVAL '28 minutes'
    ),
    (
        NULL,
        'a0000000-0000-0000-0000-000000000001',
        'd0000000-0000-0000-0000-000000000002',
        'milestone',
        '{"pct":75}',
        NOW() - INTERVAL '55 minutes'
    ),
    (
        '66666666-6666-6666-6666-666666666666',
        'a0000000-0000-0000-0000-000000000001',
        'd0000000-0000-0000-0000-000000000003',
        'ticket_purchase',
        '{"count":5}',
        NOW() - INTERVAL '90 minutes'
    );

-- Shares
INSERT INTO
    shares (
        user_id,
        raffle_item_id,
        nonprofit_id,
        channel,
        share_token
    )
VALUES
    (
        '44444444-4444-4444-4444-444444444444',
        'd0000000-0000-0000-0000-000000000001',
        'a0000000-0000-0000-0000-000000000001',
        'copy_link',
        'shr_seed0001'
    ),
    (
        '55555555-5555-5555-5555-555555555555',
        'd0000000-0000-0000-0000-000000000003',
        'a0000000-0000-0000-0000-000000000001',
        'social',
        'shr_seed0002'
    );

-- ============================================================================
-- Recompute denormalized counters to match the overridden tickets_sold values
-- (the triggers counted only the sample rows; set org totals to display values).
-- ============================================================================
UPDATE
    nonprofits
SET
    total_raised_cents = 9850000,  -- $98,500 all-time
    raffles_run = 22,
    follower_count = 1840 -- real follow rows = 5; bump to display value
WHERE
    id = 'a0000000-0000-0000-0000-000000000001';

COMMIT;

-- ============================================================================
-- Quick verification
-- ============================================================================
-- "table_name" is quoted because TABLE is a reserved SQL keyword.
-- No psql backslash meta-commands here, so this runs anywhere (incl. Neon).
SELECT
    'nonprofits' AS table_name,
    COUNT(*) AS ROWS
FROM
    nonprofits
UNION
ALL
SELECT
    'categories',
    COUNT(*)
FROM
    categories
UNION
ALL
SELECT
    'raffle_items',
    COUNT(*)
FROM
    raffle_items
UNION
ALL
SELECT
    'tickets',
    COUNT(*)
FROM
    tickets
UNION
ALL
SELECT
    'orders',
    COUNT(*)
FROM
    orders
UNION
ALL
SELECT
    'draw_results',
    COUNT(*)
FROM
    draw_results
UNION
ALL
SELECT
    'draw_winners',
    COUNT(*)
FROM
    draw_winners
UNION
ALL
SELECT
    'org_follows',
    COUNT(*)
FROM
    org_follows
UNION
ALL
SELECT
    'raffle_views',
    COUNT(*)
FROM
    raffle_views
ORDER BY
    table_name;
