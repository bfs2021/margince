// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package capture_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/margince/margince/backend/internal/modules/capture"
	"github.com/margince/margince/backend/internal/modules/capture/dispact"
	"github.com/margince/margince/backend/internal/shared/apperrors"
)

// fakeDispactBot answers GetMe for whichever bot the test names; the deal-sync
// calls are not reached by connecting.
type fakeDispactBot struct {
	dispact.API
	bot     dispact.Bot
	refuses error
	calls   int
}

func (f *fakeDispactBot) GetMe(context.Context, string) (dispact.Bot, error) {
	f.calls++
	return f.bot, f.refuses
}

func (f *channelFixture) withDispact(bot *fakeDispactBot) {
	f.store = f.store.WithDispact(bot)
	f.handlers = capture.NewChannelHandlers(f.store)
}

func TestADispactBotConnectsBesideTheTelegramBot(t *testing.T) {
	f := newChannelFixture(t, nil)
	f.withDispact(&fakeDispactBot{bot: dispact.Bot{ID: "bot-user-1", Username: "margince", Team: "Gradion"}})
	telegramToken, _ := f.api.withNewBot("margince_bot")
	if _, err := f.store.Connect(f.ctx, capture.ConnectRequest{Provider: capture.ProviderTelegram, BotToken: telegramToken}); err != nil {
		t.Fatalf("connecting Telegram: %v", err)
	}

	conn, err := f.store.Connect(f.ctx, capture.ConnectRequest{Provider: capture.ProviderDispact, BotToken: "gwapp_one"})
	if err != nil {
		t.Fatalf("connecting Dispact: %v", err)
	}
	if conn.Provider != capture.ProviderDispact || conn.ChannelID != "bot-user-1" || conn.ChannelLabel != "margince · Gradion" {
		t.Errorf("connection = %+v", conn)
	}
	if n := len(f.liveConnections(t)); n != 2 {
		t.Errorf("%d live connections, want Telegram and Dispact", n)
	}
	if len(f.api.clearedWebhooks()) != 1 {
		t.Errorf("a webhook was cleared for the Dispact bot too")
	}

	_, err = f.store.Connect(f.ctx, capture.ConnectRequest{Provider: capture.ProviderDispact, BotToken: "gwapp_two"})
	if !errors.Is(err, capture.ErrChannelWorkspaceBotAlreadyBound) {
		t.Errorf("a second Dispact bot: err = %v, want ErrChannelWorkspaceBotAlreadyBound", err)
	}
}

func TestADispactTokenIsCheckedBeforeAnythingIsSealed(t *testing.T) {
	f := newChannelFixture(t, nil)
	bot := &fakeDispactBot{refuses: dispact.ErrTokenRejected}
	f.withDispact(bot)

	if _, err := f.store.Connect(f.ctx, capture.ConnectRequest{Provider: capture.ProviderDispact, BotToken: "123:telegram-shaped"}); !errors.Is(err, dispact.ErrTokenRejected) {
		t.Errorf("a Telegram-shaped token: err = %v, want ErrTokenRejected", err)
	}
	if bot.calls != 0 {
		t.Errorf("a malformed token was spent on a call")
	}
	if _, err := f.store.Connect(f.ctx, capture.ConnectRequest{Provider: capture.ProviderDispact, BotToken: "gwapp_revoked"}); !errors.Is(err, dispact.ErrTokenRejected) {
		t.Errorf("a revoked token: err = %v, want ErrTokenRejected", err)
	}
	if f.vault.putCount() != 0 || len(f.liveConnections(t)) != 0 {
		t.Errorf("a refused token left a sealed secret or a row behind")
	}
}

func TestADispactTokenIsReplacedInPlace(t *testing.T) {
	f := newChannelFixture(t, nil)
	bot := &fakeDispactBot{bot: dispact.Bot{ID: "bot-user-1", Username: "margince"}}
	f.withDispact(bot)
	conn, err := f.store.Connect(f.ctx, capture.ConnectRequest{Provider: capture.ProviderDispact, BotToken: "gwapp_one"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.ReplaceToken(f.ctx, conn.ID, "123:telegram-shaped"); !errors.Is(err, dispact.ErrTokenRejected) {
		t.Errorf("the Dispact row took a Telegram-shaped token: err = %v", err)
	}
	bot.bot.Username = "margince-renamed"
	if err := f.store.ReplaceToken(f.ctx, conn.ID, "gwapp_two"); err != nil {
		t.Fatalf("ReplaceToken: %v", err)
	}
	after, err := f.store.Get(f.ctx, conn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Provider != capture.ProviderDispact || after.ChannelLabel != "margince-renamed" {
		t.Errorf("after the rotation = %+v", after)
	}
}

func TestAnUnconfiguredDispactIsRefusedByName(t *testing.T) {
	f := newChannelFixture(t, nil)
	_, err := f.store.Connect(f.ctx, capture.ConnectRequest{Provider: capture.ProviderDispact, BotToken: "gwapp_one"})
	if !errors.Is(err, capture.ErrChannelProviderNotConfigured) {
		t.Errorf("err = %v, want ErrChannelProviderNotConfigured", err)
	}
	if _, err := f.store.Connect(f.ctx, capture.ConnectRequest{Provider: "carrier-pigeon", BotToken: "x"}); !errors.Is(err, apperrors.ErrConflict) {
		t.Errorf("an unknown provider: err = %v, want ErrConflict", err)
	}

	w, r := f.connectRequest(`{"provider":"dispact","botToken":"gwapp_one"}`)
	f.handlers.ConnectChannel(w, r)
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusServiceUnavailable || problem.Code != "channel_provider_not_configured" {
		t.Errorf("answer = %d %q, want 503 channel_provider_not_configured", w.Code, problem.Code)
	}
}
