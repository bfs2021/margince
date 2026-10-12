// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package integration

// The visit baseline and the suggestion dismissals on an account page are the
// human's own. An agent carries its human's user id, so the read side asks
// RequireHuman. The agent reads a first visit, every suggestion still raised.

import (
	"context"
	"testing"

	"github.com/margince/margince/backend/internal/compose/company360"
	crmcontracts "github.com/margince/margince/backend/internal/contracts"
	"github.com/margince/margince/backend/internal/modules/activities"
	"github.com/margince/margince/backend/internal/modules/contacts"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
)

func TestAnAgentReadsItsHumansAccountAsAFirstVisitWithNothingDismissed(t *testing.T) {
	e := Setup(t)
	admin := e.Admin()
	contact := e.SeedContact(t, "Dana Buyer", &e.Rep1)
	company := e.SeedCompany(t, "Acme", &e.Rep1)
	contactID, companyID := ContactIDOf(contact), companyIDOf(company)
	if _, err := e.Contacts.CreateRelationship(admin, contacts.CreateRelationshipInput{
		Kind: "employment", ContactID: &contactID, CompanyID: &companyID,
	}); err != nil {
		t.Fatalf("employing the contact: %v", err)
	}
	// An outbound mail nobody answered raises the no-reply suggestion.
	sent := roomFixedNow.AddDate(0, 0, -20)
	if _, _, err := e.Activities.LogActivity(admin, activities.LogActivityInput{
		Kind: "email", Direction: new("outbound"), Subject: new("Renewal terms"), OccurredAt: &sent,
		Links: []activities.ActivityLinkInput{
			{EntityType: "contact", EntityID: contact},
			{EntityType: "company", EntityID: company},
		},
	}); err != nil {
		t.Fatalf("logging the unanswered mail: %v", err)
	}

	svc := companySurfaceService(e)
	human := e.As(e.Rep1, []ids.UUID{e.Team1}, AdminPerms)
	agent := e.AgentFor(t, e.Rep1, []ids.UUID{e.Team1}, AdminPerms)
	if _, err := svc.Acknowledge(human, companyID); err != nil {
		t.Fatalf("the human acknowledging the account: %v", err)
	}
	before := assembleAccount(human, t, svc, companyID)
	if before.Suggestions == nil || len(*before.Suggestions) == 0 {
		t.Fatal("the account raises no suggestion, so a dismissal has nothing to hide")
	}
	dismissed := (*before.Suggestions)[0].Fingerprint
	if err := svc.DismissSuggestion(human, companyID, dismissed, nil); err != nil {
		t.Fatalf("the human dismissing a suggestion: %v", err)
	}

	mine := assembleAccount(human, t, svc, companyID)
	if mine.SinceLastVisit == nil || mine.SinceLastVisit.BaselineAt == nil {
		t.Fatalf("the human's since-last-visit = %+v, want their acknowledged baseline", mine.SinceLastVisit)
	}
	if raisesFingerprint(mine, dismissed) {
		t.Fatal("the human's page still raises the suggestion they dismissed")
	}

	theirs := assembleAccount(agent, t, svc, companyID)
	if theirs.SinceLastVisit == nil {
		t.Fatal("the agent's page carries no since-last-visit section to compare")
	}
	if theirs.SinceLastVisit.BaselineAt != nil {
		t.Errorf("the agent reads its human's visit baseline %v, want a first visit", *theirs.SinceLastVisit.BaselineAt)
	}
	if !raisesFingerprint(theirs, dismissed) {
		t.Error("the agent's page omits the suggestion its human dismissed, which tells it what they set aside")
	}
}

func assembleAccount(ctx context.Context, t *testing.T, svc *company360.Service, companyID ids.CompanyID) crmcontracts.Company360 {
	t.Helper()
	page, err := svc.Assemble(ctx, companyID)
	if err != nil {
		t.Fatalf("assembling the account page: %v", err)
	}
	return page
}

func raisesFingerprint(page crmcontracts.Company360, fingerprint string) bool {
	if page.Suggestions == nil {
		return false
	}
	for _, s := range *page.Suggestions {
		if s.Fingerprint == fingerprint {
			return true
		}
	}
	return false
}
