-- SPDX-License-Identifier: BUSL-1.1
-- SPDX-FileCopyrightText: 2026 Gradion

-- A live Dispact connection holds a sealed token only the API can destroy, so
-- going down refuses until every Dispact bot is disconnected. An archived row
-- has no token left and is removed. The Telegram-only CHECK comes back NOT
-- VALID, and the file before this one validates it in its own transaction.
SET LOCAL lock_timeout = '3s';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM channel_connection WHERE provider = 'dispact' AND archived_at IS NULL) THEN
        RAISE EXCEPTION 'disconnect the Gradion Dispact bot before migrating down';
    END IF;
END
$$;
DELETE FROM channel_connection WHERE provider = 'dispact';
ALTER TABLE channel_connection RENAME CONSTRAINT channel_connection_provider_check TO channel_connection_provider_check_v2;
ALTER TABLE channel_connection ADD CONSTRAINT channel_connection_provider_check CHECK (provider = 'telegram') NOT VALID;
