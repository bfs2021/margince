// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package privacy

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/margince/margince/backend/internal/platform/database/storekit"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
)

// redactRailOccurrences clears what the AI-activity rail kept about the
// subject: the name it stored for the record and the prose about the work.
//
// An occurrence names its record by subject_type and subject_id, the same
// citation ai_call carries, so it is reached by the same three keys. The row
// itself stays, because it records that work ran and names nobody once cleared.
// The records are listed as forgotten, so a late event cannot write the name back.
func redactRailOccurrences(ctx context.Context, tx pgx.Tx, contactID ids.ContactID, erased erasedCitations) error {
	if err := storekit.LockRailRedaction(ctx, tx, true); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_task_run_forgotten_subject (subject_type, subject_id)
		SELECT 'contact', $1::uuid
		 UNION SELECT 'activity', unnest($2::uuid[])
		 UNION SELECT 'lead', unnest($3::uuid[])
		ON CONFLICT DO NOTHING`,
		contactID, erased.activities, erased.leads); err != nil {
		return fmt.Errorf("privacy: listing the subject's records as forgotten on the AI activity rail: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE ai_task_run
		   SET subject_label = NULL, summary = NULL, degrade_reason = NULL
		 WHERE (subject_type = 'contact' AND subject_id = $1)
		    OR (subject_type = 'activity' AND subject_id = ANY($2))
		    OR (subject_type = 'lead' AND subject_id = ANY($3))`,
		contactID, erased.activities, erased.leads); err != nil {
		return fmt.Errorf("privacy: clearing the AI activity rail's lines about the subject: %w", err)
	}
	return nil
}

// redactAttachmentRailOccurrences clears the rail lines about attachments the
// caller just deleted. An extraction's label is the file's name, which often
// names the subject.
func redactAttachmentRailOccurrences(ctx context.Context, tx pgx.Tx, attachments []ids.UUID) error {
	if len(attachments) == 0 {
		return nil
	}
	if err := storekit.LockRailRedaction(ctx, tx, true); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_task_run_forgotten_subject (subject_type, subject_id)
		SELECT 'attachment', unnest($1::uuid[])
		ON CONFLICT DO NOTHING`, attachments); err != nil {
		return fmt.Errorf("privacy: listing deleted attachments as forgotten on the AI activity rail: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE ai_task_run
		   SET subject_label = NULL, summary = NULL, degrade_reason = NULL
		 WHERE subject_type = 'attachment' AND subject_id = ANY($1)`, attachments); err != nil {
		return fmt.Errorf("privacy: clearing the AI activity rail's lines about deleted attachments: %w", err)
	}
	return nil
}
