// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package auth

import "github.com/margince/margince/backend/internal/shared/kernel/principal"

// ReadablePredicate is VisiblePredicate for a record another row only points
// at: it also asks the table's object read grant. An identity table's row scope
// admits every seat. So the row predicate alone would name a deal to a seat
// holding no deal.read.
func ReadablePredicate(p principal.Principal, table string, arg func(any) int) func(alias string) string {
	if p.Type != principal.PrincipalSystem && !p.Permissions.Allows(table, principal.ActionRead) {
		return func(string) string { return sqlNoRow }
	}
	return VisiblePredicate(p, table, arg)
}
