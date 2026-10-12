// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package consent

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/margince/margince/backend/internal/shared/apperrors"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
	"github.com/margince/margince/backend/internal/shared/kernel/retentionscope"
)

// dsrRetiredByID reads whether a case's subject has been erased, through the
// same tombstone expression the erasure writes.
const dsrRetiredByID = "SELECT subject_ref = " + retentionscope.RetiredSubjectRef +
	" FROM data_subject_request WHERE id = $1"

// ReplayableDSR answers whether a recorded answer naming this case may be served
// again. The caller must still hold the authority that filed it. A case whose
// subject was since erased answers 404, because its recorded body still names them.
func (s *Store) ReplayableDSR(ctx context.Context, id ids.UUID) error {
	if err := requireDSRAdmin(ctx, principal.ActionUpdate); err != nil {
		return err
	}
	return s.db.Tx(ctx, func(tx pgx.Tx) error {
		var retired bool
		err := tx.QueryRow(ctx, dsrRetiredByID, id).Scan(&retired)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return apperrors.ErrNotFound
		case err != nil:
			return err
		case retired:
			return apperrors.ErrNotFound
		}
		return nil
	})
}
