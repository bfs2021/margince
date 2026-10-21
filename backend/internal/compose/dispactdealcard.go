// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package compose

import (
	"strings"

	crmcontracts "github.com/margince/margince/backend/internal/contracts"
	"github.com/margince/margince/backend/internal/modules/deals"
	"github.com/margince/margince/backend/internal/platform/mailcopy"
	"github.com/margince/margince/backend/internal/shared/kernel/draftfloor"
)

// dealCard renders the card: the deal as it is now, so editing it in place
// always shows the current facts, whichever change caused the edit.
func dealCard(deal dispactDeal, base string) string {
	var b strings.Builder
	b.WriteString("**" + draftfloor.OneLine(deal.Name) + "**\n")
	b.WriteString("Stage: " + deal.Stage + " · " + deal.Status + "\n")
	if deal.AmountMinor != nil && deal.Currency != nil {
		b.WriteString("Amount: " + mailcopy.For(string(mailcopy.English)).Money(*deal.AmountMinor, *deal.Currency) + "\n")
	}
	if deal.CloseDate != nil {
		b.WriteString("Expected close: " + deal.CloseDate.Format("2006-01-02") + "\n")
	}
	if deal.Owner != nil {
		b.WriteString("Owner: " + *deal.Owner + "\n")
	}
	if deal.Archived {
		b.WriteString("Archived in Margince\n")
	}
	if link := dealLink(deal.ID, base); link != "" {
		b.WriteString("Open in Margince: " + link + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// dealEventLine is the line a change posts under the card, or "" for a change
// the card alone shows.
func dealEventLine(eventType string, deal dispactDeal) string {
	switch eventType {
	case deals.EventDealStageChanged:
		return "Stage moved to " + deal.Stage + " (" + deal.Status + ")"
	case string(crmcontracts.DealOwnerChanged):
		if deal.Owner == nil {
			return "The deal no longer has an owner"
		}
		return "Owner is now " + *deal.Owner
	case string(crmcontracts.DealArchived):
		return "The deal was archived in Margince"
	}
	return ""
}
