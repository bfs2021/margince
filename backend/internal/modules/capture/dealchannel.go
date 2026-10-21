// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package capture

// Where each deal posts in Gradion Dispact. deal_channel records the channel the
// bot made for a deal and the message that is its card. So the next change edits
// that card instead of making a second one. It is sync bookkeeping, like a poll
// offset, and not audited: it records nothing a colleague decided.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/margince/margince/backend/internal/shared/kernel/ids"
)

// DealChannel is where one deal's updates go.
type DealChannel struct {
	DealID       ids.UUID
	ConnectionID ids.UUID
	ChannelID    string
	ChannelSlug  string
	// CardTS is the card message's ts; "" until the card is posted, which is
	// before the row is saved.
	CardTS string
}

// DispactBinding is the live Dispact bot and the token it spends.
type DispactBinding struct {
	ConnectionID ids.UUID
	Token        string
}

// LiveDispactBinding resolves the workspace's live Dispact bot and unseals its
// token. It answers ErrNoConnection when no Dispact bot is connected.
func (r *Registry) LiveDispactBinding(ctx context.Context) (DispactBinding, error) {
	binding, err := r.liveChannelBinding(ctx, ProviderDispact)
	if err != nil {
		return DispactBinding{}, err
	}
	token, err := r.resolveCredential(ctx, &binding.credentialRef, nil)
	if err != nil {
		return DispactBinding{}, err
	}
	return DispactBinding{ConnectionID: binding.id, Token: string(token)}, nil
}

// DealChannelFor reads the channel deal posts to through connection. A row made
// through an earlier connection does not count: that bot's channel is not one
// the current bot can post in.
func (r *Registry) DealChannelFor(ctx context.Context, deal, connection ids.UUID) (DealChannel, bool, error) {
	out := DealChannel{DealID: deal}
	err := r.db.Tx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT connection_id, channel_id, channel_slug, card_ts FROM deal_channel
			 WHERE deal_id = $1 AND connection_id = $2`, deal, connection).
			Scan(&out.ConnectionID, &out.ChannelID, &out.ChannelSlug, &out.CardTS)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DealChannel{}, false, nil
	}
	if err != nil {
		return DealChannel{}, false, fmt.Errorf("capture: reading deal %s's channel: %w", deal, err)
	}
	return out, true, nil
}

// SaveDealChannel records where deal posts, replacing what an earlier
// connection recorded.
func (r *Registry) SaveDealChannel(ctx context.Context, ch DealChannel) error {
	err := r.db.Tx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO deal_channel (deal_id, connection_id, channel_id, channel_slug, card_ts)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (deal_id) DO UPDATE
			   SET connection_id = EXCLUDED.connection_id, channel_id = EXCLUDED.channel_id,
			       channel_slug = EXCLUDED.channel_slug, card_ts = EXCLUDED.card_ts, updated_at = now()`,
			ch.DealID, ch.ConnectionID, ch.ChannelID, ch.ChannelSlug, ch.CardTS)
		return err
	})
	if err != nil {
		return fmt.Errorf("capture: recording deal %s's channel: %w", ch.DealID, err)
	}
	return nil
}
