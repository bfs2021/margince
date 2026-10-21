// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package compose

import (
	"strings"
	"testing"
	"time"

	crmcontracts "github.com/margince/margince/backend/internal/contracts"
	"github.com/margince/margince/backend/internal/modules/capture/dispact"
	"github.com/margince/margince/backend/internal/modules/deals"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
)

func TestTheCardShowsTheDealAsItIs(t *testing.T) {
	amount, currency, owner := int64(1250050), "EUR", "Ada Lovelace"
	closes := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	deal := dispactDeal{
		ID: ids.NewV7(), Name: "Fleet renewal", Status: "open", Stage: "Proposal",
		AmountMinor: &amount, Currency: &currency, CloseDate: &closes, Owner: &owner,
	}

	card := dealCard(deal, "https://crm.example.test/")
	want := "**Fleet renewal**\nStage: Proposal · open\nAmount: 12,500.50 EUR\nExpected close: 2026-12-01\n" +
		"Owner: Ada Lovelace\nOpen in Margince: https://crm.example.test/#/deals/" + deal.ID.String()
	if card != want {
		t.Errorf("card =\n%s\nwant\n%s", card, want)
	}
}

func TestTheCardLeavesOutWhatTheDealLacks(t *testing.T) {
	card := dealCard(dispactDeal{ID: ids.NewV7(), Name: "Bare", Status: "lost", Stage: "Closed", Archived: true}, "")
	if card != "**Bare**\nStage: Closed · lost\nArchived in Margince" {
		t.Errorf("card = %q", card)
	}
}

func TestOnlyStageOwnerAndArchivePostALine(t *testing.T) {
	owner := "Grace Hopper"
	deal := dispactDeal{Stage: "Won", Status: "won", Owner: &owner}
	cases := map[string]string{
		deals.EventDealStageChanged:           "Stage moved to Won (won)",
		string(crmcontracts.DealOwnerChanged): "Owner is now Grace Hopper",
		string(crmcontracts.DealArchived):     "The deal was archived in Margince",
		string(crmcontracts.DealUpdated):      "",
		string(crmcontracts.DealCreated):      "",
	}
	for eventType, want := range cases {
		if got := dealEventLine(eventType, deal); got != want {
			t.Errorf("%s: line = %q, want %q", eventType, got, want)
		}
	}
	if got := dealEventLine(string(crmcontracts.DealOwnerChanged), dispactDeal{}); got != "The deal no longer has an owner" {
		t.Errorf("unowned line = %q", got)
	}
}

func TestTheChannelNameFitsDispactsLimit(t *testing.T) {
	deal := dispactDeal{ID: ids.NewV7(), Name: strings.Repeat("ü", 200)}
	name := dealChannelName(deal)
	if n := len([]rune(name)); n > 80 {
		t.Errorf("name is %d runes, over Dispact's 80", n)
	}
	if !strings.HasSuffix(name, idTail(deal.ID)) {
		t.Errorf("name %q does not end in the deal's id tail", name)
	}
	twin := dispactDeal{ID: ids.NewV7(), Name: deal.Name}
	if dealChannelName(twin) == name {
		t.Errorf("two deals of one name made in the same instant share the channel name %q", name)
	}
	if n := len([]rune(name + " " + idTail(ids.NewV7()))); n > 80 {
		t.Errorf("a name with the collision suffix is %d runes, over Dispact's 80", n)
	}
	if n := len([]rune(dealChannelTopic(deal))); n > 250 {
		t.Errorf("topic is %d runes, over Dispact's 250", n)
	}
}

func TestOnlyAChannelThatTakesNoPostsIsGone(t *testing.T) {
	for code, gone := range map[string]bool{
		dispact.CodeChannelNotFound: true, dispact.CodeNotInChannel: true,
		dispact.CodeIsArchived: true, dispact.CodeMessageNotFound: false,
	} {
		if got := channelGone(&dispact.APIError{Code: code}); got != gone {
			t.Errorf("%s: gone = %v, want %v", code, got, gone)
		}
	}
	if channelGone(nil) {
		t.Error("no error read as a gone channel")
	}
}
