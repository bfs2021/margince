// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package compose

// What the Dispact sync reads about a deal: the facts its card shows, and the
// colleagues its channel invites.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/margince/margince/backend/internal/compose/companyscan"
	"github.com/margince/margince/backend/internal/platform/auth"
	"github.com/margince/margince/backend/internal/platform/database"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
	"github.com/margince/margince/backend/internal/shared/kernel/workforce"
)

// dispactDeal is one deal as its card shows it.
type dispactDeal struct {
	ID          ids.UUID
	Name        string
	Status      string
	Stage       string
	AmountMinor *int64
	Currency    *string
	CloseDate   *time.Time
	Owner       *string
	Archived    bool
}

// readDispactDeal reads the deal, archived or not; found is false when it is gone.
func readDispactDeal(ctx context.Context, pool *pgxpool.Pool, id ids.UUID) (dispactDeal, bool, error) {
	out := dispactDeal{ID: id}
	var archivedAt *time.Time
	err := database.WithWorkspaceTx(ctx, pool, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT d.name, d.status, s.name, d.amount_minor, d.currency, d.expected_close_date,
			       u.display_name, d.archived_at
			  FROM deal d
			  JOIN stage s ON s.id = d.stage_id
			  LEFT JOIN app_user u ON u.id = d.owner_id
			 WHERE d.id = $1`, id).
			Scan(&out.Name, &out.Status, &out.Stage, &out.AmountMinor, &out.Currency, &out.CloseDate,
				&out.Owner, &archivedAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dispactDeal{}, false, nil
	}
	if err != nil {
		return dispactDeal{}, false, fmt.Errorf("dispact: reading deal %s: %w", id, err)
	}
	out.Archived = archivedAt != nil
	return out, true, nil
}

// dealCardFields are the deal columns the card shows. A colleague whose role
// withholds any of them is not invited, because the channel would show it.
//
//nolint:goconst // the deal's column names, spelled as a mask names them
var dealCardFields = []string{"name", "status", "stage_id", "amount_minor", "currency", "expected_close_date", "owner_id"}

type dispactColleague struct {
	ID    ids.UUID
	Email string
}

// dispactInvitees answers the emails the deal's channel invites: the colleagues
// whose own grants let them open the deal and read every fact the card shows.
// Contacts are never invited, since the channel shows internal pipeline facts.
func dispactInvitees(ctx context.Context, pool *pgxpool.Pool, authority companyscan.Authority, deal ids.UUID) ([]string, error) {
	// dispactCandidates are the colleagues responsible for the deal: its owner, the
	// colleagues assigned to it, and the members of the teams assigned to it. Agents and
	// members who cannot sign in are left out. The limit bounds the authority
	// probes one event costs. It is wider than Dispact's cap, so colleagues the
	// probe refuses do not crowd out the ones it admits.
	const dispactCandidates = `
		SELECT u.id, u.email FROM app_user u
		 WHERE NOT u.is_agent AND %s AND (
		       u.id = (SELECT owner_id FROM deal WHERE id = $1)
		    OR u.id IN (SELECT user_id FROM record_assignment
		                 WHERE deal_id = $1 AND archived_at IS NULL AND user_id IS NOT NULL)
		    OR u.id IN (SELECT m.user_id FROM record_assignment a
		                  JOIN team t ON t.id = a.team_id AND t.archived_at IS NULL
		                  JOIN team_membership m ON m.team_id = t.id
		                 WHERE a.deal_id = $1 AND a.archived_at IS NULL))
		 ORDER BY u.id = (SELECT owner_id FROM deal WHERE id = $1) DESC, u.email
		 LIMIT $2`
	var colleagues []dispactColleague
	err := database.WithWorkspaceTx(ctx, pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, fmt.Sprintf(dispactCandidates, workforce.LiveMemberSQL("u")), deal, dispactMaxProbes)
		if err != nil {
			return err
		}
		colleagues, err = pgx.CollectRows(rows, pgx.RowToStructByPos[dispactColleague])
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("dispact: reading who deal %s invites: %w", deal, err)
	}
	var emails []string
	for _, c := range colleagues {
		if len(emails) == dispactMaxMembers {
			break
		}
		shown, err := cardShownTo(ctx, pool, authority, c.ID, deal)
		if err != nil {
			return nil, err
		}
		if shown {
			emails = append(emails, c.Email)
		}
	}
	return emails, nil
}

// cardShownTo answers whether colleague, under their own grants, may open deal
// and read every field its card shows.
func cardShownTo(ctx context.Context, pool *pgxpool.Pool, authority companyscan.Authority, colleague, deal ids.UUID) (bool, error) {
	ownCtx, err := companyscan.WorkerContext(ctx, authority, ids.From[ids.UserKind](colleague), deal)
	if err != nil {
		return false, err
	}
	own, ok := principal.Actor(ownCtx)
	if !ok {
		return false, nil
	}
	masked := auth.MaskedFields(own, dispactDealEntity, false)
	if slices.ContainsFunc(dealCardFields, func(f string) bool { return slices.Contains(masked, f) }) {
		return false, nil
	}
	var visible bool
	err = database.WithWorkspaceTx(ownCtx, pool, func(tx pgx.Tx) error {
		visible, err = auth.VisibleTo(ownCtx, tx, dispactDealEntity, deal)
		return err
	})
	return visible, err
}
