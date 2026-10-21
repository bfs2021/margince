// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package migrations_test

import (
	"context"
	"strings"
	"testing"

	"github.com/margince/margince/backend/internal/platform/dbmigrate"
	"github.com/margince/margince/backend/migrations"
)

// Rolling back the Dispact provider leaves the provider CHECK a fresh install
// of the version before it has: Telegram alone, and validated.
func TestTheDispactDownRestoresAValidatedTelegramProviderCheck(t *testing.T) {
	ownerDSN, _ := dsns(t)
	conn := connect(t, ownerDSN)
	resetSchema(t, conn)
	ctx := context.Background()

	core, err := migrations.Core()
	if err != nil {
		t.Fatalf("loading core: %v", err)
	}
	through := namespaceThroughVersion(t, core, "1792544164")
	if _, err := dbmigrate.Up(ctx, conn, through); err != nil {
		t.Fatalf("up: %v", err)
	}
	if _, err := dbmigrate.Down(ctx, conn, through, 2); err != nil {
		t.Fatalf("down: %v", err)
	}
	rows, err := conn.Query(ctx, `
		SELECT conname, convalidated, pg_get_constraintdef(oid) FROM pg_constraint
		 WHERE conrelid = 'channel_connection'::regclass AND conname LIKE 'channel_connection_provider_check%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var found int
	for rows.Next() {
		var name, definition string
		var validated bool
		if err := rows.Scan(&name, &validated, &definition); err != nil {
			t.Fatal(err)
		}
		found++
		if name != "channel_connection_provider_check" || !validated || strings.Contains(definition, "dispact") {
			t.Errorf("%s (validated %t) is %s, want channel_connection_provider_check alone, validated, Telegram only",
				name, validated, definition)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if found != 1 {
		t.Errorf("the rollback left %d provider CHECKs, want 1", found)
	}
}
