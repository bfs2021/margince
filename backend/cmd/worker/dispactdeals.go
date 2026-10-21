// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/margince/margince/backend/internal/compose"
	"github.com/margince/margince/backend/internal/modules/capture"
	"github.com/margince/margince/backend/internal/modules/capture/dispact"
	"github.com/margince/margince/backend/internal/modules/identity"
	"github.com/margince/margince/backend/internal/platform/keyvault"
)

// startDispactDeals subscribes the consumer that keeps each deal's Gradion
// Dispact channel current. It needs Dispact's address and a vault for the bot's
// token. Without either, no Dispact bot can be connected.
func startDispactDeals(
	ctx context.Context,
	cfg workerConfig,
	pool *pgxpool.Pool,
	rdb *redis.Client,
	vault keyvault.Vault,
	background *sync.WaitGroup,
	logger *slog.Logger,
	stdout io.Writer,
) {
	if cfg.dispactBaseURL == "" || vault == nil {
		_, _ = fmt.Fprintln(stdout, "worker not syncing deals to Gradion Dispact (needs dispact_base_url and a credential store)")
		return
	}
	db := compose.InstallationDB(pool)
	consumer := compose.NewDispactDeals(pool, capture.NewRegistry(db, nil, nil, vault),
		dispact.NewAPI(nil, cfg.dispactBaseURL), identity.NewService(pool), cfg.publicBaseURL, logger)
	_, _ = fmt.Fprintln(stdout, "worker syncing deals to Gradion Dispact channels")
	background.Go(func() { runSubscriber(ctx, rdb, "cg:dispact", consumer.HandleEvent, logger, 0) })
}
