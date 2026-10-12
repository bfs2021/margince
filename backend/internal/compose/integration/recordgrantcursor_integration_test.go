// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package integration

// The record-grant page, its HasMore and its cursor are drawn from visible
// grants only.

import (
	"context"
	"testing"

	"github.com/margince/margince/backend/internal/modules/identity"
	"github.com/margince/margince/backend/internal/platform/database/storekit"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// A seat walking the grant list one row at a time never has a page, a HasMore
// or a cursor stand on a grant it cannot see.
//
// The two newest grants are hidden from the seat by both halves of visibility:
// a deal share for a role with no deal.read, and a share of a colleague's
// capture-private company. They sort first, so a page cut before the filter
// would name one of them in its cursor.
func TestTheGrantListCursorNeverNamesAGrantTheSeatCannotSee(t *testing.T) {
	e := Setup(t)
	admin := e.As(e.AdminUser, nil, AdminPerms)
	shares := identity.NewServiceFor(e.DB())
	share := func(recordType string, record ids.UUID) ids.UUID {
		t.Helper()
		grant, err := shares.CreateRecordGrant(admin, identity.CreateGrantInput{
			RecordType: recordType, RecordID: record, SubjectType: "user", SubjectID: e.Rep3, Access: "read",
		})
		if err != nil {
			t.Fatalf("sharing the %s: %v", recordType, err)
		}
		return grant.ID
	}

	visible := share("company", e.SeedCompany(t, "Shared Holding", &e.Rep1))
	private := e.SeedCompany(t, "Captured Holding", &e.Rep1)
	privateGrant := share("company", private)
	pipeline, open, _ := DealFixture(t, e)
	dealGrant := share("deal", e.SeedDeal(t, "Quiet expansion", pipeline, open, &e.Rep1))
	e.WsExec(t, `UPDATE company SET visibility = 'owner' WHERE id = $1`, private)

	noDeals := principal.Permissions{
		RoleKeys: []string{"custom"},
		Objects:  map[string]principal.ObjectGrant{"company": {Read: true}},
		RowScope: principal.RowScopeAll,
	}
	seen := walkGrants(e.As(e.Rep2, nil, noDeals), t, shares)
	if len(seen) != 1 || seen[0] != visible {
		t.Fatalf("the seat walked %v, want only the visible grant %s; the deal grant %s and the private "+
			"company grant %s are not theirs to page through", seen, visible, dealGrant, privateGrant)
	}

	// The owner of the private company, holding deal.read, walks all three.
	all := walkGrants(e.As(e.Rep1, nil, AdminPerms), t, shares)
	want := []ids.UUID{dealGrant, privateGrant, visible}
	if len(all) != len(want) {
		t.Fatalf("the owner walked %v, want %v", all, want)
	}
	for i := range want {
		if all[i] != want[i] {
			t.Fatalf("the owner walked %v, want %v", all, want)
		}
	}
}

// walkGrants pages the list one grant at a time. It fails when a cursor names a
// grant its page did not serve, which is the only way a cursor can point at a
// hidden row.
func walkGrants(ctx context.Context, t *testing.T, shares *identity.Service) []ids.UUID {
	t.Helper()
	limit := 1
	var seen []ids.UUID
	var cursor *string
	for range 5 {
		grants, page, err := shares.ListRecordGrants(ctx, identity.ListGrantsInput{Limit: &limit, Cursor: cursor})
		if err != nil {
			t.Fatalf("listing after %d grant(s): %v", len(seen), err)
		}
		if len(grants) != 1 {
			t.Fatalf("page %d served %d grant(s), want 1 for limit=1", len(seen)+1, len(grants))
		}
		seen = append(seen, grants[0].ID)
		if !page.HasMore {
			if page.NextCursor != "" {
				t.Fatal("the last page still hands back a cursor")
			}
			return seen
		}
		position, err := storekit.DecodeCursor(page.NextCursor)
		if err != nil {
			t.Fatalf("decoding the cursor: %v", err)
		}
		if position.ID != grants[0].ID {
			t.Fatalf("the cursor names grant %s, but the page served %s", position.ID, grants[0].ID)
		}
		cursor = &page.NextCursor
	}
	t.Fatalf("the walk did not end after %d pages", len(seen))
	return nil
}
