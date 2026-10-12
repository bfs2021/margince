// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package compose

// The contact anonymize reaches what the Art. 17 cascade reaches in capture and
// on the AI-activity rail, through the same two statements.

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/margince/margince/backend/internal/compose/integration"
	"github.com/margince/margince/backend/internal/modules/aiactivity"
	"github.com/margince/margince/backend/internal/modules/capture"
	"github.com/margince/margince/backend/internal/modules/privacy"
	"github.com/margince/margince/backend/internal/platform/database/storekit"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

func TestAnonymisingAContactForgetsItsAddressInCaptureAndItsNameOnTheRail(t *testing.T) {
	e := integration.Setup(t)
	const address, other = "anon.subject@example.test", "someone@else.test"
	subject := e.SeedContact(t, "Anon Subject", &e.AdminUser)
	bystander := e.SeedContact(t, "Unrelated Contact", &e.AdminUser)
	e.WsExec(t, `INSERT INTO contact_email (contact_id, email, source, captured_by)
		VALUES ($1, $2, 'manual', 'human:x')`, subject, address)
	seat := e.As(e.Rep1, nil, principal.Permissions{RowScope: principal.RowScopeAll})
	holds := capture.NewCounterpartyHoldStore(e.DB())
	for _, value := range []string{address, other} {
		if _, err := holds.Add(seat, capture.HoldKindAddress, value); err != nil {
			t.Fatalf("holding %s: %v", value, err)
		}
	}
	e.WsExec(t, `INSERT INTO capture_thread_verdict (thread_key, user_id, status, kind, seen_addresses, resolved_at)
		VALUES ('anon-thread', $1, 'cleared', 'business', $2, now())`, e.Rep1, []string{address, other})
	seedSenderOverride(t, e, e.Rep1, address, capture.OverrideKeepOut)
	e.WsExec(t, `INSERT INTO capture_exclusion (scope, user_id, kind, value, created_by)
		VALUES ('user', $1, 'address', $2, 'human:x')`, e.Rep1, address)
	aboutSubject := railLineAbout(t, e, subject, "Anon Subject")
	aboutBystander := railLineAbout(t, e, bystander, "Unrelated Contact")

	service := NewRetentionServiceFor(e.DB(), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := service.AnonymiseContacts(e.Admin(), []ids.UUID{subject}, privacy.PurgeOwnerRule, anonymiseAsChosen); err != nil {
		t.Fatalf("AnonymiseContacts: %v", err)
	}

	if got := e.WsScalar(t, `SELECT array_to_string(seen_addresses, ',') FROM capture_thread_verdict
		WHERE thread_key = 'anon-thread'`); got != other {
		t.Errorf("the verdict lists %q after the anonymize, want only %q", got, other)
	}
	marker := storekit.ErasedAddressMarker(address)
	for table, query := range map[string]string{
		"capture_sender_override": `SELECT string_agg(address, ',') FROM capture_sender_override WHERE user_id = $1`,
		"capture_exclusion":       `SELECT string_agg(value, ',') FROM capture_exclusion WHERE user_id = $1`,
	} {
		if got := e.WsScalar(t, query, e.Rep1); got != marker {
			t.Errorf("%s holds %q after the anonymize, want the marker", table, got)
		}
	}
	held := e.WsScalar(t, `SELECT string_agg(value, ',' ORDER BY value) FROM capture_counterparty_hold WHERE user_id = $1`, e.Rep1)
	if want := storekit.ErasedAddressMarker(address) + "," + other; held != want {
		t.Errorf("the seat's holds are %q, want %q", held, want)
	}
	if got := e.WsScalar(t, `SELECT coalesce(subject_label, '') FROM ai_task_run WHERE occurrence_key = $1`, aboutSubject); got != "" {
		t.Errorf("the rail still names the anonymized contact %q", got)
	}
	if got := e.WsScalar(t, `SELECT coalesce(subject_label, '') FROM ai_task_run WHERE occurrence_key = $1`, aboutBystander); got != "Unrelated Contact" {
		t.Errorf("the bystander's rail line reads %q, want it untouched", got)
	}
}

// railLineAbout projects one settled occurrence about a contact through the
// projection's own writer, and answers its occurrence key.
func railLineAbout(t *testing.T, e *integration.Env, contact ids.UUID, label string) string {
	t.Helper()
	var now time.Time
	if err := integration.OwnerConn(t).QueryRow(t.Context(), `SELECT now()`).Scan(&now); err != nil {
		t.Fatalf("reading the database clock: %v", err)
	}
	started, finished := now, now.Add(time.Second)
	key := "anon-rail:" + ids.NewV7().String()
	applied, err := aiactivity.NewStore(e.DB()).ApplyStateChange(e.Admin(), aiactivity.Change{
		Source: "relationship_brief", OccurrenceKey: key, Kind: "relationship_brief", Attempt: 1,
		ActorScope: aiactivity.ScopePersonal, ActorUserID: e.Rep1, State: "done",
		QueuedAt: now.Add(-time.Second), StartedAt: &started, FinishedAt: &finished,
		SubjectType: "contact", SubjectID: contact, SubjectLabel: label, EventID: ids.NewV7(),
	})
	if err != nil || !applied {
		t.Fatalf("projecting the rail line about %s: applied=%v err=%v", label, applied, err)
	}
	return key
}
