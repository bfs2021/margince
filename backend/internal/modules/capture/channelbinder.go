// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package capture

import (
	"context"
	"fmt"

	"github.com/margince/margince/backend/internal/modules/capture/dispact"
	"github.com/margince/margince/backend/internal/modules/capture/telegram"
)

// channelIdentity is the bot a token belongs to. ID is the provider's stable
// handle for it, and Label is display only.
type channelIdentity struct {
	ID    string
	Label string
}

// channelBinder is the provider half of connecting a bot. validate checks the
// shape before any network call; bind proves the token and names the bot.
type channelBinder interface {
	validate(token string) error
	bind(ctx context.Context, token string) (channelIdentity, error)
}

type telegramBinder struct{ api telegram.API }

func (b telegramBinder) validate(token string) error { return telegram.ValidateToken(token) }

// bind runs getMe, then clears any webhook the bot still carries, because
// Telegram refuses getUpdates for a bot with one registered. Pending updates are
// kept: they are the customer's messages, which the first poll collects.
//
// Clearing the webhook takes the bot from whatever registered it. The two
// ingress modes cannot share one bot, and the operator pasting the token asserts
// they may take it.
func (b telegramBinder) bind(ctx context.Context, token string) (channelIdentity, error) {
	bot, err := b.api.GetMe(ctx, token)
	if err != nil {
		return channelIdentity{}, err
	}
	if err := b.api.DeleteWebhook(ctx, token); err != nil {
		return channelIdentity{}, fmt.Errorf("capture: clearing the bot's webhook so its updates can be polled: %w", err)
	}
	return channelIdentity{ID: fmt.Sprintf("%d", bot.ID), Label: bot.Username}, nil
}

type dispactBinder struct{ api dispact.API }

func (b dispactBinder) validate(token string) error { return dispact.ValidateToken(token) }

// bind keys the connection on the bot user, which survives a token rotation and
// keeps the channels the bot already created reachable by their keys.
func (b dispactBinder) bind(ctx context.Context, token string) (channelIdentity, error) {
	bot, err := b.api.GetMe(ctx, token)
	if err != nil {
		return channelIdentity{}, err
	}
	label := bot.Username
	if bot.Team != "" {
		label = bot.Username + " · " + bot.Team
	}
	return channelIdentity{ID: bot.ID, Label: label}, nil
}
