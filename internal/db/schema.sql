-- Barivara backend schema (Postgres on Neon).
-- Applied idempotently on server startup; no migration framework at this size.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name          text NOT NULL,
    phone         text NOT NULL UNIQUE,
    email         text UNIQUE,
    password_hash text NOT NULL,
    role          text NOT NULL DEFAULT 'renter', -- landlord | renter | pending_landlord | rejected_landlord
    status        text NOT NULL DEFAULT 'none',   -- none | pending | approved | rejected
    plan          text NOT NULL DEFAULT 'free',   -- free | pro
    photo_url     text,
    fcm_token     text,                           -- Android FCM device token, for push notifications
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- Older deployments created `users` before `fcm_token` existed; this
-- re-applies cleanly against a fresh DB too.
ALTER TABLE users ADD COLUMN IF NOT EXISTS fcm_token text;
ALTER TABLE users ADD COLUMN IF NOT EXISTS contacts jsonb NOT NULL DEFAULT '[]'::jsonb;

-- Admin accounts are kept entirely separate from app users (renters/
-- landlords) — no phone required, email+password login of their own.
CREATE TABLE IF NOT EXISTS admins (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name          text NOT NULL,
    email         text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS properties (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        uuid NOT NULL REFERENCES users(id),
    name            text NOT NULL,
    address         text NOT NULL,
    reminder_day    int NOT NULL DEFAULT 5,
    reminder_hour   int NOT NULL DEFAULT 8,
    sms_enabled     boolean NOT NULL DEFAULT false,
    google_maps_url text,
    property_type   text,
    image_url       text,
    image_urls      jsonb NOT NULL DEFAULT '[]',
    video_url       text,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_properties_owner ON properties(owner_id);

-- Older deployments created `properties` before `image_urls` existed (single
-- `image_url` only); this re-applies cleanly against a fresh DB too.
ALTER TABLE properties ADD COLUMN IF NOT EXISTS image_urls jsonb NOT NULL DEFAULT '[]';

CREATE TABLE IF NOT EXISTS notices (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    landlord_id uuid NOT NULL REFERENCES users(id),
    property_id uuid NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    title       text NOT NULL,
    body        text NOT NULL,
    priority    text NOT NULL DEFAULT 'normal', -- normal | urgent
    published_at timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notices_property ON notices(property_id, published_at DESC);

CREATE TABLE IF NOT EXISTS units (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id       uuid NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_number       text NOT NULL,
    base_rent         int NOT NULL,
    is_occupied       boolean NOT NULL DEFAULT false,
    default_utilities jsonb NOT NULL DEFAULT '{}',
    rent_history      jsonb NOT NULL DEFAULT '[]',
    deposit_history   jsonb NOT NULL DEFAULT '[]',
    floor_level       int,
    unit_size         text,
    unit_type         text,
    security_deposit  int,
    status            text NOT NULL DEFAULT 'vacant',
    meters            jsonb NOT NULL DEFAULT '{}',
    image_url         text,
    video_url         text
);
CREATE INDEX IF NOT EXISTS idx_units_property ON units(property_id);

CREATE TABLE IF NOT EXISTS renters (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id      uuid NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_id          uuid NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    name             text NOT NULL,
    phone            text NOT NULL,
    nid              text NOT NULL,
    advance_deposit  int NOT NULL DEFAULT 0,
    move_in_date     date NOT NULL,
    email            text,
    occupation       text,
    alternate_phone  text,
    photo_url        text,
    move_out_date    date,
    is_active        boolean NOT NULL DEFAULT true,
    sms_enabled      boolean NOT NULL DEFAULT false,
    family_members   jsonb NOT NULL DEFAULT '[]',
    documents        jsonb NOT NULL DEFAULT '[]',
    landlord_notes   text,
    join_code        text UNIQUE,
    linked_user_id   uuid REFERENCES users(id)
);
ALTER TABLE renters ADD COLUMN IF NOT EXISTS rent_history jsonb NOT NULL DEFAULT '[]';
ALTER TABLE renters ADD COLUMN IF NOT EXISTS deposit_history jsonb NOT NULL DEFAULT '[]';
ALTER TABLE renters ADD COLUMN IF NOT EXISTS sms_enabled boolean NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_renters_property_unit ON renters(property_id, unit_id);
CREATE INDEX IF NOT EXISTS idx_renters_linked_user ON renters(linked_user_id);

CREATE TABLE IF NOT EXISTS notice_recipients (
    notice_id uuid NOT NULL REFERENCES notices(id) ON DELETE CASCADE,
    renter_id uuid NOT NULL REFERENCES renters(id) ON DELETE CASCADE,
    tenant_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (notice_id, tenant_user_id)
);
CREATE INDEX IF NOT EXISTS idx_notice_recipients_user ON notice_recipients(tenant_user_id, read_at);

CREATE TABLE IF NOT EXISTS conversations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    landlord_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    renter_id   uuid NOT NULL REFERENCES renters(id) ON DELETE CASCADE,
    property_id uuid NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_id     uuid NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (landlord_id, renter_id)
);
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS conversation_type text NOT NULL DEFAULT 'landlord';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS participant_a_user_id uuid REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS participant_b_user_id uuid REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS request_sender_user_id uuid REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'accepted';
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_conversation_pair
    ON conversations(property_id, participant_a_user_id, participant_b_user_id)
    WHERE conversation_type = 'tenant';

CREATE TABLE IF NOT EXISTS conversation_messages (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body            text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    read_at         timestamptz,
    edited_at       timestamptz,
    deleted_at      timestamptz
);
ALTER TABLE conversation_messages ADD COLUMN IF NOT EXISTS edited_at timestamptz;
ALTER TABLE conversation_messages ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
CREATE INDEX IF NOT EXISTS idx_conversation_messages
    ON conversation_messages(conversation_id, created_at DESC);

CREATE TABLE IF NOT EXISTS conversation_message_hidden (
    message_id uuid NOT NULL REFERENCES conversation_messages(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id)
);

CREATE TABLE IF NOT EXISTS invoices (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id    uuid NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_id        uuid NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    renter_id       uuid REFERENCES renters(id),
    renter_name     text,
    invoice_number  text NOT NULL DEFAULT '',
    month_year      text NOT NULL,
    base_rent       int NOT NULL DEFAULT 0,
    utilities       jsonb NOT NULL DEFAULT '{}',
    other_charges   int NOT NULL DEFAULT 0,
    total_amount    int NOT NULL DEFAULT 0,
    status          text NOT NULL DEFAULT 'unpaid', -- unpaid | due | paid
    payments        jsonb NOT NULL DEFAULT '[]',
    created_at      timestamptz NOT NULL DEFAULT now(),
    paid_at         timestamptz
);
CREATE INDEX IF NOT EXISTS idx_invoices_property_unit ON invoices(property_id, unit_id);
CREATE INDEX IF NOT EXISTS idx_invoices_renter ON invoices(renter_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoices_unit_month ON invoices(unit_id, month_year);

CREATE TABLE IF NOT EXISTS landlord_requests (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                uuid NOT NULL REFERENCES users(id),
    user_name              text NOT NULL,
    phone                  text NOT NULL,
    nid_number             text NOT NULL,
    owner_proof_image_url  text,
    status                 text NOT NULL DEFAULT 'pending', -- pending | approved | rejected
    created_at             timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tenant_link_requests (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    renter_id       uuid NOT NULL REFERENCES renters(id) ON DELETE CASCADE,
    property_id     uuid NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_id         uuid NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    tenant_user_id  uuid NOT NULL REFERENCES users(id),
    tenant_name     text NOT NULL,
    tenant_email    text,
    tenant_phone    text NOT NULL,
    status          text NOT NULL DEFAULT 'pending', -- pending | approved | rejected
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tenant_link_renter ON tenant_link_requests(renter_id);
CREATE INDEX IF NOT EXISTS idx_tenant_link_tenant ON tenant_link_requests(tenant_user_id);

CREATE TABLE IF NOT EXISTS rent_posts (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id   uuid NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    unit_id       uuid NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    landlord_id   uuid NOT NULL REFERENCES users(id),
    title         text NOT NULL,
    description   text NOT NULL,
    location      text NOT NULL,
    rent_amount   int NOT NULL,
    image_urls    jsonb NOT NULL DEFAULT '[]',
    is_active     boolean NOT NULL DEFAULT true,
    property_type text,
    beds          int,
    baths         int,
    area_sqft     text,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rent_posts_unit ON rent_posts(unit_id);

CREATE TABLE IF NOT EXISTS sms_logs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    renter_id   uuid,
    invoice_id  uuid,
    type        text NOT NULL, -- reminder | receipt
    phone       text NOT NULL,
    message     text NOT NULL,
    status      text NOT NULL, -- sent | failed
    error       text,
    sent_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sms_packages (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    message_count int NOT NULL CHECK (message_count > 0),
    price       int NOT NULL CHECK (price >= 0),
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sms_wallets (
    landlord_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance     int NOT NULL DEFAULT 0 CHECK (balance >= 0),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sms_purchase_requests (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    landlord_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    package_id  uuid NOT NULL REFERENCES sms_packages(id),
    package_name text NOT NULL,
    message_count int NOT NULL,
    amount      int NOT NULL,
    note        text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'pending', -- pending | approved | rejected
    created_at  timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz
);

CREATE TABLE IF NOT EXISTS counters (
    property_id      uuid PRIMARY KEY REFERENCES properties(id) ON DELETE CASCADE,
    invoice_current  int NOT NULL DEFAULT 0
);
