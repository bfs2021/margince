// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package compose

// The cg:dispact consumer. Once an admin connects a Gradion Dispact bot, every
// deal gets one private channel, keyed on the deal. Its colleagues are invited, and
// a card with its facts is posted, then edited on every change. A stage move,
// owner change or archive also posts one line.
//
// Each Dispact write is idempotent on its own key, so a redelivered event
// repeats nothing.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/margince/margince/backend/internal/compose/companyscan"
	crmcontracts "github.com/margince/margince/backend/internal/contracts"
	"github.com/margince/margince/backend/internal/modules/capture"
	"github.com/margince/margince/backend/internal/modules/capture/dispact"
	"github.com/margince/margince/backend/internal/modules/deals"
	"github.com/margince/margince/backend/internal/platform/mailcopy"
	"github.com/margince/margince/backend/internal/shared/kernel/draftfloor"
	"github.com/margince/margince/backend/internal/shared/kernel/events"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
	"github.com/margince/margince/backend/internal/shared/kernel/textcut"
)

// dispactChannels is the capture surface the sync records its channels through.
type dispactChannels interface {
	LiveDispactBinding(ctx context.Context) (capture.DispactBinding, error)
	DealChannelFor(ctx context.Context, deal, connection ids.UUID) (capture.DealChannel, bool, error)
	SaveDealChannel(ctx context.Context, ch capture.DealChannel) error
}

// dispactDealEntity is the entity type, and the RBAC object, of a deal.
const dispactDealEntity = "deal"

// dispactMaxMembers is how many emails Dispact takes in one call.
const dispactMaxMembers = 100

// dispactMaxProbes is how many candidates one event checks for an invite.
const dispactMaxProbes = 4 * dispactMaxMembers

// DispactDeals keeps each deal's Dispact channel current.
type DispactDeals struct {
	pool      *pgxpool.Pool
	channels  dispactChannels
	api       dispact.API
	authority companyscan.Authority
	// publicBaseURL links the card back to the deal; empty leaves the link out.
	publicBaseURL string
	log           *slog.Logger
}

// NewDispactDeals builds the consumer.
func NewDispactDeals(pool *pgxpool.Pool, channels dispactChannels, api dispact.API,
	authority companyscan.Authority, publicBaseURL string, log *slog.Logger,
) *DispactDeals {
	return &DispactDeals{
		pool: pool, channels: channels, api: api, authority: authority,
		publicBaseURL: publicBaseURL, log: log,
	}
}

var dispactDealEvents = map[string]bool{
	string(crmcontracts.DealCreated):      true,
	string(crmcontracts.DealUpdated):      true,
	deals.EventDealStageChanged:           true,
	string(crmcontracts.DealOwnerChanged): true,
	string(crmcontracts.DealArchived):     true,
}

// HandleEvent syncs the deal one event names. Without a connected Dispact bot,
// or once the deal is gone, there is nothing to do.
func (d *DispactDeals) HandleEvent(ctx context.Context, env events.Envelope) error {
	if env.Entity.ID == ids.Nil || env.Entity.Type != dispactDealEntity || !dispactDealEvents[env.Type] {
		return nil
	}
	ws, err := InstallationDB(d.pool).Workspace(ctx)
	if err != nil {
		return err
	}
	ctx = dispactContext(ctx, env, ws.UUID)
	binding, err := d.channels.LiveDispactBinding(ctx)
	if errors.Is(err, capture.ErrNoConnection) {
		return nil
	}
	if err != nil {
		return err
	}
	deal, found, err := readDispactDeal(ctx, d.pool, env.Entity.ID)
	if err != nil || !found {
		return err
	}
	return d.verdict(ctx, binding, deal, d.sync(ctx, binding, deal, env))
}

// verdict decides what the bus does with a failed sync. An outage or a channel
// another request is still creating is retried. A refusal no retry repairs is
// acked, or the deal's later events would queue behind it forever.
func (d *DispactDeals) verdict(ctx context.Context, binding capture.DispactBinding, deal dispactDeal, err error) error {
	switch {
	case err == nil, dispact.CodeOf(err) == dispact.CodeCreateInProgress:
		return err
	case errors.Is(err, dispact.ErrTokenRejected):
		// Only an admin pasting a new token repairs this, and parking the
		// connection shows them so.
		d.log.WarnContext(ctx, "dispact: the connected bot's token was rejected, so the deal was not synced",
			"deal", deal.ID.String())
		return capture.StopPollingChannelConnection(ctx, d.pool, binding.ConnectionID,
			capture.ChannelPollStoppedByBadToken, "dispact_token_rejected")
	case errors.Is(err, dispact.ErrRequestRejected):
		d.log.WarnContext(ctx, "dispact: Dispact refused the deal's sync", "deal", deal.ID.String(),
			"reason", dispact.CodeOf(err), "error", err.Error())
		return fmt.Errorf("%w: %w", events.ErrUnactionable, err)
	}
	return err
}

func (d *DispactDeals) sync(ctx context.Context, binding capture.DispactBinding, deal dispactDeal, env events.Envelope) error {
	ch, known, err := d.channels.DealChannelFor(ctx, deal.ID, binding.ConnectionID)
	if err != nil {
		return err
	}
	if known {
		err = d.invite(ctx, binding, ch, deal)
	} else {
		ch, known, err = d.open(ctx, binding, deal)
	}
	if err != nil || !known {
		return err
	}
	err = d.refreshCard(ctx, binding, ch, deal, env)
	if channelGone(err) {
		// Deleted, archived, or the bot was removed: the key finds the
		// channel again, or a new one once a deletion freed it.
		if ch, known, err = d.open(ctx, binding, deal); err != nil || !known {
			return err
		}
		err = d.refreshCard(ctx, binding, ch, deal, env)
	}
	if err != nil {
		return err
	}
	return d.postLine(ctx, binding, ch, deal, env)
}

// open makes the deal's channel with its colleagues in it, or finds it by its key.
// A deal that existed before the bot was connected is opened on its first
// change. ok is false when the bot cannot post there.
func (d *DispactDeals) open(ctx context.Context, binding capture.DispactBinding, deal dispactDeal) (capture.DealChannel, bool, error) {
	emails, err := d.invitees(ctx, deal)
	if err != nil {
		return capture.DealChannel{}, false, err
	}
	req := dispact.CreateChannelRequest{
		Name: dealChannelName(deal), ExternalKey: dealChannelKey(deal.ID),
		Topic: dealChannelTopic(deal), Members: emails,
	}
	channel, membership, err := d.api.CreateChannel(ctx, binding.Token, req)
	if dispact.CodeOf(err) == dispact.CodeNameTaken {
		// Another bot's channel holds the name, as after a swap to a new bot.
		req.Name += " " + idTail(binding.ConnectionID)
		channel, membership, err = d.api.CreateChannel(ctx, binding.Token, req)
	}
	if err != nil {
		return capture.DealChannel{}, false, err
	}
	d.logFailed(ctx, deal, membership)
	if !channel.BotIsMember || channel.IsArchived {
		d.log.WarnContext(ctx, "dispact: the deal's channel is archived or the bot was removed, so nothing is posted",
			"deal", deal.ID.String(), "channel", channel.Slug)
		return capture.DealChannel{}, false, nil
	}
	return capture.DealChannel{
		DealID: deal.ID, ConnectionID: binding.ConnectionID,
		ChannelID: channel.ID, ChannelSlug: channel.Slug,
	}, true, nil
}

// invite adds the deal's colleagues to its channel; those already in it stay. It
// runs on every change, so a colleague assigned later joins on the next one.
func (d *DispactDeals) invite(ctx context.Context, binding capture.DispactBinding, ch capture.DealChannel, deal dispactDeal) error {
	emails, err := d.invitees(ctx, deal)
	if err != nil || len(emails) == 0 {
		return err
	}
	membership, err := d.api.AddMembers(ctx, binding.Token, ch.ChannelSlug, emails)
	if err != nil && !channelGone(err) {
		return err
	}
	d.logFailed(ctx, deal, membership)
	return nil
}

// refreshCard edits the card to the deal as it is now, or posts it when there
// is none. A card someone deleted is posted again, keyed on this event, since
// Dispact answers a reused key of a deleted message with message_not_found.
func (d *DispactDeals) refreshCard(ctx context.Context, binding capture.DispactBinding, ch capture.DealChannel, deal dispactDeal, env events.Envelope) error {
	card := dealCard(deal, d.publicBaseURL)
	if ch.CardTS != "" {
		err := d.api.UpdateMessage(ctx, binding.Token, ch.ChannelSlug, ch.CardTS, card)
		if err == nil {
			return nil
		}
		if code := dispact.CodeOf(err); code != dispact.CodeMessageNotFound && code != dispact.CodeCantUpdateMessage {
			return err
		}
	}
	ts, err := d.api.PostMessage(ctx, binding.Token, ch.ChannelSlug, card, cardMessageKey(deal.ID, ch.ChannelID, ""))
	if dispact.CodeOf(err) == dispact.CodeMessageNotFound {
		ts, err = d.api.PostMessage(ctx, binding.Token, ch.ChannelSlug, card, cardMessageKey(deal.ID, ch.ChannelID, env.EventID.String()))
	}
	if err != nil {
		return err
	}
	// A key Dispact already holds answers its first message unchanged, as after the
	// same bot is connected again. The edit brings that card up to date.
	if err := d.api.UpdateMessage(ctx, binding.Token, ch.ChannelSlug, ts, card); err != nil {
		return err
	}
	ch.CardTS = ts
	return d.channels.SaveDealChannel(ctx, ch)
}

// postLine posts the one line a stage move, owner change or archive earns.
func (d *DispactDeals) postLine(ctx context.Context, binding capture.DispactBinding, ch capture.DealChannel, deal dispactDeal, env events.Envelope) error {
	line := dealEventLine(env.Type, deal)
	if line == "" {
		return nil
	}
	_, err := d.api.PostMessage(ctx, binding.Token, ch.ChannelSlug, line, "margince:event:"+env.EventID.String())
	if channelGone(err) {
		return nil
	}
	return err
}

func (d *DispactDeals) invitees(ctx context.Context, deal dispactDeal) ([]string, error) {
	return dispactInvitees(ctx, d.pool, d.authority, deal.ID)
}

// logFailed counts the colleagues Dispact did not add, by reason. The addresses stay
// out of the log.
func (d *DispactDeals) logFailed(ctx context.Context, deal dispactDeal, membership dispact.Membership) {
	if len(membership.Failed) == 0 {
		return
	}
	reasons := map[string]int{}
	for _, f := range membership.Failed {
		reasons[f.Reason]++
	}
	d.log.InfoContext(ctx, "dispact: some of the deal's colleagues were not added to its channel",
		"deal", deal.ID.String(), "not_added", len(membership.Failed), "reasons", reasons)
}

// channelGone reports a refusal that means the channel itself takes no posts:
// deleted, archived, or the bot is no longer in it.
func channelGone(err error) bool {
	switch dispact.CodeOf(err) {
	case dispact.CodeChannelNotFound, dispact.CodeNotInChannel, dispact.CodeIsArchived:
		return true
	}
	return false
}

func dealChannelKey(deal ids.UUID) string { return "margince:deal:" + deal.String() }

func cardMessageKey(deal ids.UUID, channelID, repost string) string {
	key := "margince:card:" + deal.String() + ":" + channelID
	if repost != "" {
		key += ":" + repost
	}
	return key
}

// dealChannelNameRunes leaves room under Dispact's 80-character name limit for
// the deal's id tail and, after a name collision, the connection's.
const dealChannelNameRunes = 48

func dealChannelName(deal dispactDeal) string {
	return fmt.Sprintf("deal %s %s", textcut.Runes(draftfloor.OneLine(deal.Name), dealChannelNameRunes), idTail(deal.ID))
}

// idTail is the random end of a v7 id. Its head is a timestamp, which two ids
// made in the same minute share.
func idTail(id ids.UUID) string {
	s := id.String()
	return s[len(s)-12:]
}

// dealChannelTopicRunes is Dispact's limit on a channel topic.
const dealChannelTopicRunes = 250

func dealChannelTopic(deal dispactDeal) string {
	return textcut.Runes("Margince deal: "+draftfloor.OneLine(deal.Name), dealChannelTopicRunes)
}

func dealLink(deal ids.UUID, base string) string {
	return mailcopy.Destination(base, mailcopy.RecordFragment("deal", deal.String()))
}

// dispactContext binds the workspace, the trace and the system actor the sync
// reads as.
func dispactContext(ctx context.Context, env events.Envelope, ws ids.UUID) context.Context {
	ctx = principal.WithWorkspaceID(ctx, ws)
	ctx = principal.WithCorrelationID(ctx, env.Trace.CorrelationID)
	return principal.WithActor(ctx, principal.Principal{
		Type: principal.PrincipalSystem, ID: "system:dispact_sync",
		Permissions: principal.Permissions{RowScope: principal.RowScopeAll},
	})
}
