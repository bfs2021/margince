// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package integration

// Art. 17 reaching the AI-activity rail. The projection keeps a settled line
// for weeks, and the line names the record it was about.

import (
	"testing"
	"time"

	"github.com/margince/margince/backend/internal/modules/aiactivity"
	"github.com/margince/margince/backend/internal/modules/privacy"
	"github.com/margince/margince/backend/internal/platform/blobstore"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
)

func TestErasureClearsTheRailLinesAboutTheSubject(t *testing.T) {
	e := Setup(t)
	subject := seedSubject(t, e)
	bystander := e.SeedContact(t, "Unrelated Contact", &e.AdminUser)
	note := ids.MustParse(e.WsScalar(t,
		`SELECT activity_id::text FROM activity_link WHERE contact_id = $1`, subject))
	theirFile := seedContactFile(t, e, subject, "selma-passport.pdf")
	otherFile := seedContactFile(t, e, bystander, "unrelated-contract.pdf")

	aboutSubject := projectRailLine(t, e, "contact", subject, "Selma Subject")
	aboutTheirNote := projectRailLine(t, e, "activity", note, "Met Selma")
	aboutTheirFile := projectRailLine(t, e, "attachment", theirFile, "selma-passport.pdf")
	aboutBystander := projectRailLine(t, e, "contact", bystander, "Unrelated Contact")
	aboutOtherFile := projectRailLine(t, e, "attachment", otherFile, "unrelated-contract.pdf")

	// The eraser refuses to delete a file whose object it cannot purge, so an
	// in-memory store stands in for the boundary.
	eraser := privacy.NewEraser(e.DB()).WithBlobstore(blobstore.NewMemory())
	if err := eraser.EraseContact(e.Admin(), subject, "subject request"); err != nil {
		t.Fatalf("EraseContact: %v", err)
	}

	for name, key := range map[string]string{
		"the subject": aboutSubject, "their erased note": aboutTheirNote, "their deleted file": aboutTheirFile,
	} {
		if got := railText(t, e, key); got != "||" {
			t.Errorf("the rail line about %s keeps label|summary|reason %q, want all three cleared", name, got)
		}
	}
	for label, key := range map[string]string{"Unrelated Contact": aboutBystander, "unrelated-contract.pdf": aboutOtherFile} {
		if got := railText(t, e, key); got != label+"|Read the record.|Partial answer." {
			t.Errorf("the bystander's rail line reads %q, want it untouched", got)
		}
	}
}

// A task running during the erasure, or an event relayed after it, carries the
// old name again. The projection keeps it off the rail.
func TestALateRailEventCannotNameAnErasedSubject(t *testing.T) {
	e := Setup(t)
	subject := seedSubject(t, e)
	bystander := e.SeedContact(t, "Unrelated Contact", &e.AdminUser)
	running := projectRailLine(t, e, "contact", subject, "Selma Subject")
	if err := privacy.NewEraser(e.DB()).EraseContact(e.Admin(), subject, "subject request"); err != nil {
		t.Fatalf("EraseContact: %v", err)
	}

	settleRailLine(t, e, running, 2, "contact", subject, "Selma Subject")
	fresh := projectRailLine(t, e, "contact", subject, "Selma Subject")
	aboutBystander := projectRailLine(t, e, "contact", bystander, "Unrelated Contact")

	for name, key := range map[string]string{"a later attempt": running, "a new occurrence": fresh} {
		if got := railText(t, e, key); got != "||" {
			t.Errorf("%s about the erased subject wrote %q back onto the rail", name, got)
		}
	}
	if got := railText(t, e, aboutBystander); got != "Unrelated Contact|Read the record.|Partial answer." {
		t.Errorf("the bystander's new rail line reads %q, want it stored as sent", got)
	}
}

// seedContactFile files one attachment on a contact and answers its id.
func seedContactFile(t *testing.T, e *Env, contact ids.UUID, filename string) ids.UUID {
	t.Helper()
	return ids.MustParse(e.WsScalar(t, `
		INSERT INTO attachment (entity_type, entity_id, filename, storage_key, source, captured_by)
		VALUES ('contact', $1, $2, 'blob/' || $2, 'manual', 'human:test')
		RETURNING id::text`, contact, filename))
}

// projectRailLine writes one settled occurrence about a record through the
// projection's own writer, and answers its occurrence key.
func projectRailLine(t *testing.T, e *Env, subjectType string, subjectID ids.UUID, label string) string {
	t.Helper()
	key := "erasure-rail:" + ids.NewV7().String()
	settleRailLine(t, e, key, 1, subjectType, subjectID, label)
	return key
}

// settleRailLine projects one settled attempt of an occurrence through the
// projection's own writer.
func settleRailLine(t *testing.T, e *Env, key string, attempt int, subjectType string, subjectID ids.UUID, label string) {
	t.Helper()
	var now time.Time
	if err := OwnerConn(t).QueryRow(t.Context(), `SELECT now()`).Scan(&now); err != nil {
		t.Fatalf("reading the database clock: %v", err)
	}
	started, finished := now.Add(time.Second), now.Add(2*time.Second)
	applied, err := aiactivity.NewStore(e.DB()).ApplyStateChange(e.Admin(), aiactivity.Change{
		Source: "relationship_brief", OccurrenceKey: key, Kind: "relationship_brief", Attempt: attempt,
		ActorScope: aiactivity.ScopePersonal, ActorUserID: e.Rep1, State: "degraded",
		QueuedAt: now, StartedAt: &started, FinishedAt: &finished,
		SubjectType: subjectType, SubjectID: subjectID, SubjectLabel: label,
		Summary: "Read the record.", DegradeReason: "Partial answer.", EventID: ids.NewV7(),
	})
	if err != nil || !applied {
		t.Fatalf("projecting the rail line about %s: applied=%v err=%v", label, applied, err)
	}
}

func railText(t *testing.T, e *Env, key string) string {
	t.Helper()
	return e.WsScalar(t, `
		SELECT coalesce(subject_label, '') || '|' || coalesce(summary, '') || '|' || coalesce(degrade_reason, '')
		  FROM ai_task_run WHERE occurrence_key = $1`, key)
}
