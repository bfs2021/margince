// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package compose

// A seat's rules about an erased address keep working under its marker.
// A second erasure of the same address still commits.

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/margince/margince/backend/internal/compose/integration"
	"github.com/margince/margince/backend/internal/modules/capture"
	"github.com/margince/margince/backend/internal/modules/privacy"
	"github.com/margince/margince/backend/internal/platform/database"
	"github.com/margince/margince/backend/internal/platform/database/storekit"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

const (
	ruledSubject   = "ruled.subject@example.test"
	ruledBystander = "ruled.bystander@example.test"
)

func TestAnErasedSendersKeepOutStillHidesTheirLaterMail(t *testing.T) {
	e := integration.Setup(t)
	eraseAddressHolder(t, e, func() {
		seedSenderOverride(t, e, e.Rep1, ruledSubject, capture.OverrideKeepOut)
		seedSenderOverride(t, e, e.Rep1, ruledBystander, capture.OverrideBusiness)
	})

	if got := e.WsScalar(t, `SELECT string_agg(address || '=' || decision, ',' ORDER BY address)
		FROM capture_sender_override WHERE user_id = $1`, e.Rep1); got !=
		storekit.ErasedAddressMarker(ruledSubject)+"=keep_out,"+ruledBystander+"=business" {
		t.Errorf("the seat's sender decisions read %q after the erasure", got)
	}
	if got := overrideFor(t, e, ruledSubject); got != capture.OverrideKeepOut {
		t.Errorf("the decision about the erased sender reads %q, want keep_out", got)
	}

	later := seedCapturedMail(t, e, ruledSubject, "after the erasure")
	if err := sweepingEngine(e).HideNoiseStragglersWorkspace(
		principal.WithWorkspaceID(context.Background(), e.WS)); err != nil {
		t.Fatalf("straggler sweep: %v", err)
	}
	if n := countIn(t, e, `SELECT count(*) FROM activity WHERE id = $1 AND archived_at IS NOT NULL`, later); n != 1 {
		t.Error("the erased sender's later mail stayed visible: the seat's keep_out stopped applying")
	}
}

func TestASecondErasureOfAnAddressKeepsOneRulePerSeat(t *testing.T) {
	e := integration.Setup(t)
	seat := principal.WithActor(e.Admin(), principal.Principal{
		Type: principal.PrincipalHuman, ID: "human:" + e.Rep1.String(), UserID: e.Rep1, SeatType: principal.SeatFull,
		Permissions: principal.Permissions{
			Objects:  map[string]principal.ObjectGrant{"contact": {Read: true, Create: true, Delete: true}},
			RowScope: principal.RowScopeAll,
		},
	})
	addRules := func(decision string) {
		if _, err := capture.NewCounterpartyHoldStore(e.DB()).Add(seat, capture.HoldKindAddress, ruledSubject); err != nil {
			t.Fatalf("holding the subject: %v", err)
		}
		if _, err := capture.NewExclusionStore(e.DB()).Add(seat, capture.ExclusionScopeUser, capture.ExclusionKindAddress, ruledSubject); err != nil {
			t.Fatalf("excluding the subject: %v", err)
		}
		if _, err := capture.NewSenderOverrideStore(e.DB()).Set(seat, ruledSubject, decision); err != nil {
			t.Fatalf("deciding about the subject: %v", err)
		}
	}
	eraseAddressHolder(t, e, func() { addRules(capture.OverrideKeepOut) })
	// The seat writes the address again, and the subject comes back as a
	// record the workspace may hold and later erases a second time.
	eraseAddressHolder(t, e, func() { addRules(capture.OverrideBusiness) })

	marker := storekit.ErasedAddressMarker(ruledSubject)
	for table, query := range map[string]string{
		"capture_counterparty_hold": `SELECT string_agg(value, ',') FROM capture_counterparty_hold WHERE user_id = $1`,
		"capture_exclusion":         `SELECT string_agg(value, ',') FROM capture_exclusion WHERE user_id = $1`,
		"capture_sender_override":   `SELECT string_agg(address, ',') FROM capture_sender_override WHERE user_id = $1`,
	} {
		if got := e.WsScalar(t, query, e.Rep1); got != marker {
			t.Errorf("%s holds %q after the second erasure, want the one marker row", table, got)
		}
	}
	if got := overrideFor(t, e, ruledSubject); got != capture.OverrideBusiness {
		t.Errorf("the kept decision is %q, want the seat's newer business decision", got)
	}
}

// eraseAddressHolder seeds a contact known by ruledSubject, lets rules be
// written against the address, and erases the contact.
func eraseAddressHolder(t *testing.T, e *integration.Env, writeRules func()) {
	t.Helper()
	subject := e.SeedContact(t, "Ruled Subject", &e.AdminUser)
	e.WsExec(t, `INSERT INTO contact_email (contact_id, email, source, captured_by)
		VALUES ($1, $2, 'manual', 'human:x')`, subject, ruledSubject)
	writeRules()
	if err := privacy.NewEraser(e.DB()).EraseContact(e.Admin(), subject, "subject request"); err != nil {
		t.Fatalf("EraseContact: %v", err)
	}
}

func overrideFor(t *testing.T, e *integration.Env, address string) string {
	t.Helper()
	var decision string
	err := database.WithWorkspaceTx(e.Admin(), e.Pool, func(tx pgx.Tx) error {
		var err error
		decision, err = capture.OverrideForTx(context.Background(), tx, e.Rep1, address)
		return err
	})
	if err != nil {
		t.Fatalf("reading the seat's decision: %v", err)
	}
	return decision
}
