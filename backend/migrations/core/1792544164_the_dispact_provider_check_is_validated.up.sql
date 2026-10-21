-- SPDX-License-Identifier: BUSL-1.1
-- SPDX-FileCopyrightText: 2026 Gradion

-- Validates the wider provider CHECK 1792544161 recorded, in a transaction of its
-- own, then retires the narrower one under the name it had. Every stored row
-- passes: the new set admits the old.
SET LOCAL lock_timeout = '3s';

ALTER TABLE channel_connection VALIDATE CONSTRAINT channel_connection_provider_check_v2;
ALTER TABLE channel_connection DROP CONSTRAINT channel_connection_provider_check;
ALTER TABLE channel_connection RENAME CONSTRAINT channel_connection_provider_check_v2 TO channel_connection_provider_check;
