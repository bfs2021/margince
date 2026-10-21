-- SPDX-License-Identifier: BUSL-1.1
-- SPDX-FileCopyrightText: 2026 Gradion

SET LOCAL lock_timeout = '3s';

DROP TABLE deal_channel;
ALTER TABLE channel_connection DROP CONSTRAINT channel_connection_provider_check_v2;

-- The 1792544164 down, which runs first, restored the Telegram-only CHECK NOT VALID;
-- validating it here, in a transaction of its own, leaves the schema a fresh install has.
ALTER TABLE channel_connection VALIDATE CONSTRAINT channel_connection_provider_check;
