// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package privacy

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/margince/margince/backend/internal/platform/database/storekit"
)

// forgetSubjectAddressesInCapture removes the subject's addresses from thread
// verdicts and seat rules. A verdict without the address judges their next
// message again, which holds it.
//
// A rule on the address keeps the seat's decision: the address becomes its
// erasure marker, which capture still matches. Domain rules stay: on a domain
// only the subject uses, the rule still points at them. That residue is accepted.
func forgetSubjectAddressesInCapture(ctx context.Context, tx pgx.Tx, emails []string) error {
	if len(emails) == 0 {
		return nil
	}
	folded := make([]string, 0, len(emails))
	markers := make([]string, 0, len(emails))
	for _, email := range emails {
		folded = append(folded, strings.ToLower(strings.TrimSpace(email)))
		markers = append(markers, storekit.ErasedAddressMarker(email))
	}
	if _, err := tx.Exec(ctx, `
		UPDATE capture_thread_verdict
		   SET seen_addresses = ARRAY(SELECT a FROM unnest(seen_addresses) AS a WHERE lower(a) <> ALL($1::text[])),
		       updated_at = now()
		 WHERE EXISTS (SELECT 1 FROM unnest(seen_addresses) AS a WHERE lower(a) = ANY($1::text[]))`,
		folded); err != nil {
		return fmt.Errorf("privacy: removing the subject's address from the thread verdicts: %w", err)
	}
	return markSubjectAddressRules(ctx, tx, folded, markers)
}

// markSubjectAddressRules moves every seat rule on the subject's address onto
// its marker. Each move is one statement that takes the plain rows and lands
// them on the marker, merging into a marker row an earlier erasure left. A
// seat writing the address meanwhile cannot make it collide.
func markSubjectAddressRules(ctx context.Context, tx pgx.Tx, folded, markers []string) error {
	if _, err := tx.Exec(ctx, `
		WITH moved AS (
		  DELETE FROM capture_counterparty_hold h
		   USING unnest($1::text[], $2::text[]) AS m(address, marker)
		   WHERE h.kind = 'address' AND h.value = m.address
		  RETURNING h.user_id, m.marker, h.created_by)
		INSERT INTO capture_counterparty_hold (user_id, kind, value, created_by)
		SELECT user_id, 'address', marker, created_by FROM moved
		ON CONFLICT (user_id, kind, value) DO UPDATE SET value = EXCLUDED.value`,
		folded, markers); err != nil {
		return fmt.Errorf("privacy: moving the counterparty holds onto the subject's marker: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		WITH moved AS (
		  DELETE FROM capture_sender_override o
		   USING unnest($1::text[], $2::text[]) AS m(address, marker)
		   WHERE o.address = m.address
		  RETURNING o.user_id, m.marker, o.decision, o.overruled_kind)
		INSERT INTO capture_sender_override (user_id, address, decision, overruled_kind)
		SELECT user_id, marker, decision, overruled_kind FROM moved
		ON CONFLICT (user_id, address) DO UPDATE
		   SET address = EXCLUDED.address, decision = EXCLUDED.decision,
		       overruled_kind = EXCLUDED.overruled_kind, updated_at = now()`,
		folded, markers); err != nil {
		return fmt.Errorf("privacy: moving the sender decisions onto the subject's marker: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		WITH moved AS (
		  DELETE FROM capture_exclusion e
		   USING unnest($1::text[], $2::text[]) AS m(address, marker)
		   WHERE e.kind = 'address' AND e.value = m.address
		  RETURNING e.scope, e.user_id, m.marker, e.created_by)
		INSERT INTO capture_exclusion (scope, user_id, kind, value, created_by)
		SELECT scope, user_id, 'address', marker, created_by FROM moved
		ON CONFLICT (scope, user_id, kind, value) DO UPDATE SET value = EXCLUDED.value`,
		folded, markers); err != nil {
		return fmt.Errorf("privacy: moving the exclusions onto the subject's marker: %w", err)
	}
	return nil
}
