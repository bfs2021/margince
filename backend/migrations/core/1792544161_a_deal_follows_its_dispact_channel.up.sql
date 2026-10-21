-- SPDX-License-Identifier: BUSL-1.1
-- SPDX-FileCopyrightText: 2026 Gradion

-- A Gradion Dispact bot is the second channel provider, so the provider CHECK
-- widens to carry it. Recorded NOT VALID here, without a scan; 1792544164
-- validates it and retires the narrower one.
--
-- deal_channel remembers the Dispact channel each deal posts to, and the card
-- message the sync edits in place. One row per deal: a deal has one channel.
SET LOCAL lock_timeout = '3s';

ALTER TABLE channel_connection ADD CONSTRAINT channel_connection_provider_check_v2
    CHECK (provider IN ('telegram', 'dispact')) NOT VALID;

CREATE TABLE deal_channel (
    deal_id uuid PRIMARY KEY REFERENCES deal(id) ON DELETE CASCADE,
    connection_id uuid NOT NULL REFERENCES channel_connection(id) ON DELETE CASCADE,
    channel_id text NOT NULL CHECK (channel_id <> ''),
    channel_slug text NOT NULL CHECK (channel_slug <> ''),
    card_ts text NOT NULL CHECK (card_ts <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX deal_channel_connection_idx ON deal_channel(connection_id);
