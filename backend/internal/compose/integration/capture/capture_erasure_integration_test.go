// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package capture

// Art. 17 reaching the capture records that keep a sender's address after the
// message is gone: the addresses a thread verdict read, and a seat's hold.

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/margince/margince/backend/internal/compose/integration"
	capturemod "github.com/margince/margince/backend/internal/modules/capture"
	"github.com/margince/margince/backend/internal/modules/privacy"
	"github.com/margince/margince/backend/internal/platform/database"
	"github.com/margince/margince/backend/internal/platform/database/storekit"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

const (
	erasedSender    = "anwalt@bigfirm.example"
	bystanderSender = "billing@bigfirm.example"
)

func TestErasureTakesTheSubjectsAddressOutOfCaptureAndTheHoldStillHolds(t *testing.T) {
	env := newCaptureEnv(t)
	e, sync := env.e, env.sync
	allowSharedPosture(t, e)
	setPosture(t, env, e.Rep1, capturemod.PostureShared)
	subject := seedMailContact(t, e, "Erased Lawyer", erasedSender)
	holdCounterparty(t, e, e.Rep1, capturemod.HoldKindAddress, erasedSender)
	holdCounterparty(t, e, e.Rep1, capturemod.HoldKindAddress, bystanderSender)
	// Mixed case on the subject's address, because the stored list is not
	// folded by every writer and the erasure must match it anyway.
	seedSeenVerdict(t, e, "erasure-thread-both", "Anwalt@BigFirm.example", bystanderSender)
	seedSeenVerdict(t, e, "erasure-thread-other", bystanderSender)

	if err := privacy.NewEraser(e.DB()).EraseContact(eraserContext(e), subject, "subject request"); err != nil {
		t.Fatalf("EraseContact: %v", err)
	}

	if got := seenAddresses(t, e, "erasure-thread-both"); !slices.Equal(got, []string{bystanderSender}) {
		t.Errorf("the verdict lists %v after the erasure, want only the bystander %q", got, bystanderSender)
	}
	if got := seenAddresses(t, e, "erasure-thread-other"); !slices.Equal(got, []string{bystanderSender}) {
		t.Errorf("a verdict that never saw the subject changed to %v", got)
	}
	holds := holdValues(t, e, e.Rep1)
	if slices.Contains(holds, erasedSender) {
		t.Errorf("the seat's hold still names the erased address: %v", holds)
	}
	if !slices.Contains(holds, storekit.ErasedAddressMarker(erasedSender)) || !slices.Contains(holds, bystanderSender) {
		t.Errorf("the holds are %v, want the subject's marker and the bystander's address", holds)
	}

	// The seat's decision outlives the address: the subject's next message to
	// this seat is still held.
	sync(t, customerMailFrom(erasedSender, "after-erasure@bigfirm.example"))
	if got, reason := audienceOf(t, e, activityIDOf(t, e, "after-erasure@bigfirm.example")); got != "participants" || reason != "counterparty" {
		t.Errorf("mail from the erased sender was born %q / %q, want participants / counterparty", got, reason)
	}
}

// seedMailContact plants a contact the cascade knows by one address.
func seedMailContact(t *testing.T, e *integration.SearchEnv, name, address string) ids.UUID {
	t.Helper()
	contact := e.SeedID(t, `INSERT INTO contact (id, full_name, source, captured_by)
		VALUES ($1, $2, 'manual', 'human:x')`, name)
	if _, err := e.Owner.Exec(context.Background(),
		`INSERT INTO contact_email (contact_id, email, source, captured_by) VALUES ($1, $2, 'manual', 'human:x')`,
		contact, address); err != nil {
		t.Fatalf("seeding the address of %s: %v", name, err)
	}
	return contact
}

// seedSeenVerdict records a cleared verdict on Rep1's thread that read the
// given senders.
func seedSeenVerdict(t *testing.T, e *integration.SearchEnv, thread string, seen ...string) {
	t.Helper()
	if _, err := e.Owner.Exec(context.Background(), `
		INSERT INTO capture_thread_verdict (thread_key, user_id, status, kind, seen_addresses, resolved_at)
		VALUES ($1, $2, $3, 'business', $4, now())`,
		thread, e.Rep1, capturemod.VerdictCleared, seen); err != nil {
		t.Fatalf("seeding the verdict on %s: %v", thread, err)
	}
}

func seenAddresses(t *testing.T, e *integration.SearchEnv, thread string) []string {
	t.Helper()
	var seen []string
	if err := e.Owner.QueryRow(context.Background(),
		`SELECT seen_addresses FROM capture_thread_verdict WHERE thread_key = $1`, thread).Scan(&seen); err != nil {
		t.Fatalf("reading the verdict on %s: %v", thread, err)
	}
	return seen
}

func holdValues(t *testing.T, e *integration.SearchEnv, user ids.UUID) []string {
	t.Helper()
	var values []string
	err := database.WithWorkspaceTx(e.Admin(), e.Pool, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT coalesce(array_agg(value ORDER BY value), '{}') FROM capture_counterparty_hold WHERE user_id = $1`,
			user).Scan(&values)
	})
	if err != nil {
		t.Fatalf("reading the holds: %v", err)
	}
	return values
}

// eraserContext is a seat holding contact.delete, the one grant EraseContact
// asks for.
func eraserContext(e *integration.SearchEnv) context.Context {
	ctx := principal.WithWorkspaceID(context.Background(), e.WS)
	ctx = principal.WithCorrelationID(ctx, ids.NewV7())
	return principal.WithActor(ctx, principal.Principal{
		Type: principal.PrincipalHuman, ID: "human:" + e.Rep1.String(), UserID: e.Rep1,
		SeatType: principal.SeatFull,
		Permissions: principal.Permissions{
			Objects:  map[string]principal.ObjectGrant{"contact": {Read: true, Update: true, Delete: true}},
			RowScope: principal.RowScopeAll,
		},
	})
}

func TestAnErasedAddressStaysExcludedFromTheSeatsMailbox(t *testing.T) {
	env := newCaptureEnv(t)
	e, sync := env.e, env.sync
	subject := seedMailContact(t, e, "Excluded Lawyer", erasedSender)
	store := capturemod.NewExclusionStore(e.DB())
	owner := humanWithScopes(e, e.Rep1, []principal.Scope{principal.ScopeRead, principal.ScopeWrite})
	for _, address := range []string{erasedSender, bystanderSender} {
		if _, err := store.Add(owner, capturemod.ExclusionScopeUser, capturemod.ExclusionKindAddress, address); err != nil {
			t.Fatalf("excluding %s: %v", address, err)
		}
	}

	if err := privacy.NewEraser(e.DB()).EraseContact(eraserContext(e), subject, "subject request"); err != nil {
		t.Fatalf("EraseContact: %v", err)
	}

	var values string
	if err := e.Owner.QueryRow(context.Background(), `SELECT string_agg(value, ',' ORDER BY value)
		FROM capture_exclusion WHERE user_id = $1`, e.Rep1).Scan(&values); err != nil {
		t.Fatalf("reading the exclusions: %v", err)
	}
	if want := bystanderSender + "," + storekit.ErasedAddressMarker(erasedSender); values != want {
		t.Errorf("the seat's exclusions read %q, want %q", values, want)
	}
	sync(t, email(erasedSender, "", captureOwner, "excluded-after@bigfirm.example", ""))
	if n := countRows(t, e, `SELECT count(*) FROM activity WHERE source_id = 'excluded-after@bigfirm.example'`); n != 0 {
		t.Errorf("%d activity rows for the erased sender's later mail, want 0: the exclusion stopped applying", n)
	}
}

// The marker spelled in SQL must equal the one Go writes, or a rule matched on
// the SQL side stops matching after an erasure.
func TestTheSQLMarkerIsTheGoMarker(t *testing.T) {
	e := integration.SetupSearch(t)
	var sqlMarker string
	if err := e.Owner.QueryRow(context.Background(),
		`SELECT `+storekit.ErasedAddressMarkerSQL("$1::text"), erasedSender).Scan(&sqlMarker); err != nil {
		t.Fatalf("computing the marker in SQL: %v", err)
	}
	if want := storekit.ErasedAddressMarker(erasedSender); sqlMarker != want {
		t.Errorf("SQL marker %q, Go marker %q", sqlMarker, want)
	}
}
