// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package identity

// Reading a page of the manual per-record grants.
//
// Split from grants.go because the read answers a different question from the
// writes beside it: those decide whether one share may exist, this one decides
// which of the shares that do the caller may SEE.

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/margince/margince/backend/internal/platform/auth"
	"github.com/margince/margince/backend/internal/platform/database/storekit"
	"github.com/margince/margince/backend/internal/shared/apperrors"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// ListRecordGrants answers one page of the active manual grants whose record
// the caller could read.
//
// Visibility is part of the page's WHERE clause, so the page, HasMore and the
// cursor all come from visible rows. A cursor minted from a row the caller
// cannot see would hand its id and creation time to the caller.
func (s *Service) ListRecordGrants(ctx context.Context, in ListGrantsInput) ([]grantRow, storekit.Page, error) {
	var out []grantRow
	var page storekit.Page
	err := s.db.Tx(ctx, func(tx pgx.Tx) error {
		limit := storekit.ClampLimit(in.Limit)
		var err error
		if out, err = s.grantPage(ctx, tx, in, limit); err != nil {
			return err
		}
		if len(out) <= limit {
			return nil
		}
		out = out[:limit]
		last := out[len(out)-1]
		next, err := storekit.EncodeCursor(last.CreatedAt, last.ID)
		page.NextCursor, page.HasMore = next, true
		return err
	})
	return out, page, err
}

// grantPage reads one page of visible rows, one beyond the limit so the caller
// can tell a full page from the last one.
func (s *Service) grantPage(ctx context.Context, tx pgx.Tx, in ListGrantsInput, limit int) ([]grantRow, error) {
	var args []any
	arg := func(v any) int { args = append(args, v); return len(args) }
	visible, err := grantTargetClause(ctx, arg)
	if err != nil {
		return nil, err
	}
	where := "(expires_at IS NULL OR expires_at > now()) AND " + visible
	var whereSb74 strings.Builder
	for _, f := range []struct {
		column string
		value  any
		set    bool
	}{
		{"record_type", derefOr(in.RecordType), in.RecordType != nil},
		{"record_id", derefOr(in.RecordID), in.RecordID != nil},
		{"subject_type", derefOr(in.SubjectType), in.SubjectType != nil},
		{"subject_id", derefOr(in.SubjectID), in.SubjectID != nil},
	} {
		if f.set {
			whereSb74.WriteString(storekit.SQLf(" AND "+f.column+" = $%d", arg(f.value)))
		}
	}
	where += whereSb74.String()
	if in.Cursor != nil && *in.Cursor != "" {
		cursor, err := storekit.DecodeCursor(*in.Cursor)
		if err != nil {
			return nil, err
		}
		where += storekit.SQLf(" AND (created_at, id) < ($%d, $%d)", arg(cursor.CreatedAt), arg(cursor.ID))
	}
	// id joins the ORDER BY so the keyset is TOTAL: two grants created in the
	// same instant would otherwise order arbitrarily, and a cursor over a
	// non-total order repeats or skips at a page boundary.
	//
	// The LIMIT bounds the READ, not just the answer. Truncating in Go after an
	// unbounded query would serve exactly the same page while still pulling the
	// whole table across — and no test through this API can tell the two apart,
	// because the difference is invisible in the result. That is why it is said
	// here rather than left to a test.
	rows, err := tx.Query(ctx,
		"SELECT "+grantColumns+" FROM record_grant g WHERE "+where+
			storekit.SQLf(" ORDER BY created_at DESC, id DESC LIMIT $%d", arg(limit+1)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var page []grantRow
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, err
		}
		page = append(page, g)
	}
	return page, rows.Err()
}

// grantTargetClause keeps a grant only when the caller could read its record.
// Each arm asks auth.VisibleSubset's two halves: the object read grant on the
// type, then the record's row visibility. A type the seat may not read has no arm.
func grantTargetClause(ctx context.Context, arg func(any) int) (string, error) {
	var arms []string
	for _, table := range slices.Sorted(maps.Keys(shareableRecordTypes)) {
		if err := auth.Require(ctx, table, principal.ActionRead); err != nil {
			if errors.Is(err, apperrors.ErrPermissionDenied) {
				continue
			}
			return "", err
		}
		typed := storekit.SQLf("g.record_type = '%s'", table)
		row, err := auth.ScopeClauseFor(ctx, table, "gt", arg)
		if err != nil {
			return "", err
		}
		if row == "" {
			arms = append(arms, typed)
			continue
		}
		arms = append(arms, storekit.SQLf("(%s AND EXISTS (SELECT 1 FROM %s gt WHERE gt.id = g.record_id AND %s))",
			typed, table, row))
	}
	if len(arms) == 0 {
		return "FALSE", nil
	}
	return "(" + strings.Join(arms, " OR ") + ")", nil
}

// derefOr reads a filter's value for binding, or nil when it is unset — the
// caller has already decided whether the clause is rendered at all.
//
//craft:ignore naked-any the four filters bind different types through one argument registrar; the any is the registrar's own
func derefOr[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
