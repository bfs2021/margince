// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package integration

// A record named through an activity link or a signal subject is named only to
// a seat that may read its type. A deal's row scope admits every
// seat, so the row predicate alone would hand a seat without deal.read the
// deal's id through activity.read or signal.read.

import (
	"context"
	"errors"
	"testing"

	"github.com/margince/margince/backend/internal/modules/signals"
	"github.com/margince/margince/backend/internal/platform/database/storekit"
	"github.com/margince/margince/backend/internal/shared/apperrors"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// subjectReaderPerms is a seat reading every row of what it holds, with or
// without deal.read. contact and company stay capture-private, so the scope
// clauses still render for it.
func subjectReaderPerms(withDeal bool) principal.Permissions {
	objects := map[string]principal.ObjectGrant{
		"contact":               {Read: true},
		"company":               {Read: true},
		"activity":              {Read: true},
		"signal":                {Read: true},
		"installation_settings": {Read: true},
	}
	if withDeal {
		objects["deal"] = principal.ObjectGrant{Read: true}
	}
	return principal.Permissions{Objects: objects, RowScope: principal.RowScopeAll}
}

func TestADealSignalIsWithheldFromASeatWithoutTheDealGrant(t *testing.T) {
	e := Setup(t)
	owner := OwnerConn(t)
	pipeline, open, _ := DealFixture(t, e)
	deal := e.SeedDeal(t, "Renewal 2027", pipeline, open, &e.Rep1)
	signal := seedSignalRow(t, owner, `INSERT INTO signal
		(id, kind, source_channel, entity_type, entity_id, resolution_state, severity, summary, detected_at, source, captured_by)
		VALUES ($1, 'risk', 'derived', 'deal', '`+deal.String()+`', 'resolved', 'warn',
		        'Champion left the buyer', now(), 'manual', 'human:x')`)
	signalID := ids.From[ids.SignalKind](signal)
	store := signals.NewStore(e.DB(), nil)

	withDeal := e.As(e.Rep1, []ids.UUID{e.Team1}, subjectReaderPerms(true))
	if _, err := store.GetSignal(withDeal, signalID, storekit.LiveOnly); err != nil {
		t.Fatalf("GetSignal with deal.read → %v, want the deal signal served", err)
	}
	if listed := listedSignalIDs(withDeal, t, store); !listed[signal] {
		t.Fatal("a seat with deal.read does not list the deal signal")
	}

	noDeal := e.As(e.Rep1, []ids.UUID{e.Team1}, subjectReaderPerms(false))
	if _, err := store.GetSignal(noDeal, signalID, storekit.LiveOnly); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("GetSignal without deal.read → %v, want ErrNotFound", err)
	}
	if listed := listedSignalIDs(noDeal, t, store); listed[signal] {
		t.Error("a seat without deal.read lists a signal whose summary names the deal")
	}
}

func listedSignalIDs(ctx context.Context, t *testing.T, store *signals.Store) map[ids.UUID]bool {
	t.Helper()
	listed, _, err := store.ListSignals(ctx, signals.ListSignalsInput{})
	if err != nil {
		t.Fatalf("listing signals: %v", err)
	}
	out := map[ids.UUID]bool{}
	for _, sig := range listed {
		out[ids.UUID(sig.Id)] = true
	}
	return out
}

func TestAnActivityNamesItsDealOnlyToASeatWithTheDealGrant(t *testing.T) {
	e := Setup(t)
	owner := OwnerConn(t)
	pipeline, open, _ := DealFixture(t, e)
	deal := e.SeedDeal(t, "Renewal 2027", pipeline, open, &e.Rep1)
	contact := e.SeedContact(t, "Dana Buyer", &e.Rep1)

	shared := SeedIDRow(t, owner, `INSERT INTO activity (id, kind, subject, occurred_at, source, captured_by)
		VALUES ($1, 'meeting', 'Pricing call', now(), 'manual', 'human:x')`)
	LinkActivity(t, owner, shared, "contact", contact)
	LinkActivity(t, owner, shared, "deal", deal)
	dealOnly := SeedIDRow(t, owner, `INSERT INTO activity (id, kind, subject, occurred_at, source, captured_by)
		VALUES ($1, 'note', 'Discount approved', now(), 'manual', 'human:x')`)
	LinkActivity(t, owner, dealOnly, "deal", deal)

	withDeal := e.As(e.Rep1, []ids.UUID{e.Team1}, subjectReaderPerms(true))
	if got := activityLinkTargets(withDeal, t, e, shared); !got[deal] || !got[contact] {
		t.Fatalf("links for a seat with deal.read = %v, want both the contact and the deal", got)
	}
	if _, err := e.Activities.GetActivity(withDeal, ids.From[ids.ActivityKind](dealOnly), storekit.LiveOnly); err != nil {
		t.Fatalf("GetActivity of a deal-only note with deal.read → %v, want it readable", err)
	}

	noDeal := e.As(e.Rep1, []ids.UUID{e.Team1}, subjectReaderPerms(false))
	got := activityLinkTargets(noDeal, t, e, shared)
	if got[deal] {
		t.Error("links[] names the deal to a seat without deal.read")
	}
	if !got[contact] {
		t.Errorf("links for a seat without deal.read = %v, want the contact still reported", got)
	}
	// Discovery asks row scope alone: the deal-only note stays readable, as a
	// meeting filed under a project does, and only the deal's id is withheld.
	if got := activityLinkTargets(noDeal, t, e, dealOnly); got[deal] {
		t.Error("the deal-only note's links name the deal to a seat without deal.read")
	}
}

func activityLinkTargets(ctx context.Context, t *testing.T, e *Env, activity ids.UUID) map[ids.UUID]bool {
	t.Helper()
	read, err := e.Activities.GetActivity(ctx, ids.From[ids.ActivityKind](activity), storekit.LiveOnly)
	if err != nil {
		t.Fatalf("GetActivity: %v", err)
	}
	out := map[ids.UUID]bool{}
	if read.Links != nil {
		for _, link := range *read.Links {
			out[ids.UUID(link.EntityId)] = true
		}
	}
	return out
}
