// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package compose

// The /channel-connections wiring (telegram-oa design v2 §4): the workspace-level
// Telegram bot binding. Ingress PULLS, so this surface needs nothing about where
// this installation can be reached — only the vault that seals the bot token,
// which WithKeyvault hands to the transport composed here.
//
// WithChannelSurface must therefore be applied BEFORE WithKeyvault, the same
// ordering contract WithOverlayBackfillLimit states, and cmd/api holds it.

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/margince/margince/backend/internal/modules/capture"
	"github.com/margince/margince/backend/internal/modules/capture/dispact"
	"github.com/margince/margince/backend/internal/modules/capture/telegram"
)

// WithChannelSurface composes the channel-connection transport. Connecting a bot
// needs a token and a vault, and a poll dials out. So no address of this
// installation's own is configured here.
//
// The vault arrives later, through WithKeyvault — until then the read surface
// lists what is bound and every mutating path refuses by name
// (`channel_credentials_not_configured`) rather than sealing a token nothing
// could unseal.
//
// dispactBase is where Gradion Dispact lives; empty leaves Dispact unconnectable.
func WithChannelSurface(dispactBase string) Option {
	return func(s *Server, pool *pgxpool.Pool) {
		store := capture.NewChannelStore(InstallationDB(pool), nil, telegram.NewAPI(nil, ""), s.log)
		if dispactBase != "" {
			store = store.WithDispact(dispact.NewAPI(nil, dispactBase))
		}
		s.channelHandlers = capture.NewChannelHandlers(store)
	}
}
