// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package compose

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/margince/margince/backend/internal/compose/integration"
	crmcontracts "github.com/margince/margince/backend/internal/contracts"
	"github.com/margince/margince/backend/internal/modules/assignments"
	"github.com/margince/margince/backend/internal/modules/capture"
	"github.com/margince/margince/backend/internal/modules/capture/dispact"
	"github.com/margince/margince/backend/internal/modules/deals"
	"github.com/margince/margince/backend/internal/modules/identity"
	"github.com/margince/margince/backend/internal/platform/database"
	"github.com/margince/margince/backend/internal/platform/keyvault"
	kevents "github.com/margince/margince/backend/internal/shared/kernel/events"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// fakeDispact is Dispact as its bot API behaves. A key is scoped to the bot that
// made it, and a channel name is unique in the workspace. A repeated client_msg_id
// answers the first ts and keeps the first text; the key of a deleted one answers
// message_not_found.
type fakeDispact struct {
	mu sync.Mutex
	// refuses, when set, is what every channel call answers.
	refuses  error
	channels map[string]*fakeChannel // by bot and external key
	creates  int
	tokens   []string
}

type fakeChannel struct {
	name     string
	slug     string
	archived bool
	members  []string
	messages map[string]string // ts by client_msg_id
	text     map[string]string // text by ts, absent once deleted
	order    []string          // ts in post order
}

func newFakeDispact() *fakeDispact { return &fakeDispact{channels: map[string]*fakeChannel{}} }

// GetMe names the bot by its token, so a test swaps bots by swapping tokens.
func (f *fakeDispact) GetMe(_ context.Context, token string) (dispact.Bot, error) {
	return dispact.Bot{ID: "bot-user-for-" + token, Username: "margince", Team: "Gradion"}, nil
}

func (f *fakeDispact) CreateChannel(_ context.Context, token string, req dispact.CreateChannelRequest) (dispact.Channel, dispact.Membership, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refuses != nil {
		return dispact.Channel{}, dispact.Membership{}, f.refuses
	}
	f.tokens = append(f.tokens, token)
	ch, ok := f.channels[token+"|"+req.ExternalKey]
	if !ok {
		for _, other := range f.channels {
			if other.name == req.Name {
				return dispact.Channel{}, dispact.Membership{}, &dispact.APIError{Method: "create", Code: dispact.CodeNameTaken}
			}
		}
		f.creates++
		ch = &fakeChannel{name: req.Name, slug: fmt.Sprintf("deal-%d", f.creates), messages: map[string]string{}, text: map[string]string{}}
		f.channels[token+"|"+req.ExternalKey] = ch
	}
	membership := f.add(ch, req.Members)
	return dispact.Channel{ID: "id-" + ch.slug, Slug: ch.slug, BotIsMember: true, IsArchived: ch.archived}, membership, nil
}

func (f *fakeDispact) AddMembers(_ context.Context, _, slug string, emails []string) (dispact.Membership, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.add(f.bySlug(slug), emails), nil
}

// add takes only the seeded harness addresses, the way Dispact refuses an
// address no member holds.
func (f *fakeDispact) add(ch *fakeChannel, emails []string) dispact.Membership {
	var out dispact.Membership
	for _, email := range emails {
		switch {
		case !strings.HasSuffix(email, "@authz.test"):
			out.Failed = append(out.Failed, dispact.FailedMember{Email: email, Reason: "user_not_found"})
		case slices.Contains(ch.members, email):
			out.AlreadyMember = append(out.AlreadyMember, email)
		default:
			ch.members = append(ch.members, email)
			out.Added = append(out.Added, email)
		}
	}
	return out
}

func (f *fakeDispact) PostMessage(_ context.Context, _, slug, text, clientMsgID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch := f.bySlug(slug)
	if ch.archived {
		return "", &dispact.APIError{Method: "post", Code: "posting_to_channel_failed"}
	}
	if ts, ok := ch.messages[clientMsgID]; ok {
		if _, live := ch.text[ts]; !live {
			return "", &dispact.APIError{Method: "post", Code: dispact.CodeMessageNotFound}
		}
		return ts, nil
	}
	ts := fmt.Sprintf("1.%d", len(ch.order)+1)
	ch.messages[clientMsgID] = ts
	ch.text[ts] = text
	ch.order = append(ch.order, ts)
	return ts, nil
}

func (f *fakeDispact) UpdateMessage(_ context.Context, _, slug, ts, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch := f.bySlug(slug)
	if ch.archived {
		return &dispact.APIError{Method: "update", Code: "update_failed"}
	}
	if _, ok := ch.text[ts]; !ok {
		return &dispact.APIError{Method: "update", Code: dispact.CodeMessageNotFound}
	}
	ch.text[ts] = text
	return nil
}

func (f *fakeDispact) bySlug(slug string) *fakeChannel {
	for _, ch := range f.channels {
		if ch.slug == slug {
			return ch
		}
	}
	return &fakeChannel{messages: map[string]string{}, text: map[string]string{}}
}

// only is the one channel the fake holds, and its post order as text.
func (f *fakeDispact) only(t *testing.T) (*fakeChannel, []string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.channels) != 1 {
		t.Fatalf("Dispact holds %d channels, want 1", len(f.channels))
	}
	for _, ch := range f.channels {
		posts := make([]string, 0, len(ch.order))
		for _, ts := range ch.order {
			posts = append(posts, ch.text[ts])
		}
		return ch, posts
	}
	return nil, nil
}

const dealReaderGrants = `{"objects":{"deal":{"read":true}},"row_scope":"all"}`

type dispactEnv struct {
	e     *integration.Env
	fake  *fakeDispact
	sync  *DispactDeals
	deal  ids.UUID
	stage ids.StageID
	role  ids.UUID
	vault keyvault.Vault
}

func setupDispact(t *testing.T, connect bool) dispactEnv {
	t.Helper()
	e := integration.Setup(t)
	vault := keyvault.NewMemory()
	fake := newFakeDispact()
	if connect {
		store := capture.NewChannelStore(e.DB(), vault, &telegramPollFakeAPI{}, quietTestLogger()).WithDispact(fake)
		if _, err := store.Connect(telegramAdminContext(e.WS, e.Rep1), capture.ConnectRequest{
			Provider: capture.ProviderDispact, BotToken: "gwapp_fixture",
		}); err != nil {
			t.Fatalf("connecting the Dispact bot: %v", err)
		}
	}
	d := dispactEnv{
		e: e, fake: fake, vault: vault,
		sync: NewDispactDeals(e.Pool, capture.NewRegistry(e.DB(), nil, nil, vault), fake,
			identity.NewService(e.Pool), "https://crm.example.test", quietTestLogger()),
	}
	for _, seat := range []ids.UUID{e.AdminUser, e.Rep1, e.Rep3} {
		key := "dispact-reader-" + seat.String()
		e.WsExec(t, `INSERT INTO role (key, name, permissions) VALUES ($1, 'Deal reader', $2::jsonb)`, key, dealReaderGrants)
		e.WsExec(t, `INSERT INTO role_assignment (role_id, user_id) SELECT r.id, $1 FROM role r WHERE r.key = $2`, seat, key)
	}
	d.seedDeal(t)
	return d
}

// seedDeal makes the deal through the deal store, owned by AdminUser, with Rep1
// and Rep2 assigned. Rep2 holds no grants, so cannot open it. Rep3 can, but is
// not on the deal.
func (d *dispactEnv) seedDeal(t *testing.T) {
	t.Helper()
	ctx := d.e.Admin()
	pipeline, err := d.e.Deals.CreatePipeline(ctx, deals.CreatePipelineInput{Name: "Sales"})
	if err != nil {
		t.Fatal(err)
	}
	pipelineID := ids.From[ids.PipelineKind](ids.UUID(pipeline.Id))
	var stages []ids.StageID
	for i, name := range []string{"Qualified", "Proposal"} {
		stage, err := d.e.Deals.CreateStage(ctx, deals.CreateStageInput{PipelineID: pipelineID, Name: name, Position: i + 1, Semantic: "open"})
		if err != nil {
			t.Fatal(err)
		}
		stages = append(stages, ids.From[ids.StageKind](ids.UUID(stage.Id)))
	}
	owner := ids.From[ids.UserKind](d.e.AdminUser)
	amount, currency := int64(1200000), "EUR"
	deal, err := d.e.Deals.CreateDeal(ctx, deals.CreateDealInput{
		Name: "Fleet renewal", PipelineID: pipelineID, StageID: stages[0], OwnerID: &owner, Source: "manual",
		AmountMinor: &amount, Currency: &currency,
	})
	if err != nil {
		t.Fatal(err)
	}
	d.deal, d.stage = ids.UUID(deal.Id), stages[1]
	var role ids.UUID
	d.read(t, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id FROM record_role WHERE key = 'sales_engineer'`).Scan(&role)
	})
	d.role = role
	d.assign(t, d.e.Rep1)
	d.assign(t, d.e.Rep2)
}

// assign puts rep on the deal through the assignment store.
func (d *dispactEnv) assign(t *testing.T, rep ids.UUID) {
	t.Helper()
	if _, err := assignments.NewStore(d.e.DB()).CreateAssignment(d.e.Admin(), assignments.CreateAssignmentInput{
		RecordType: crmcontracts.AssignmentRecordTypeDeal, RecordID: d.deal,
		SubjectKind: crmcontracts.AssignmentSubjectKindUser, SubjectID: rep, RoleID: d.role,
	}); err != nil {
		t.Fatalf("assigning a rep: %v", err)
	}
}

// envelope is the newest event of eventType the outbox holds for the deal.
func (d *dispactEnv) envelope(t *testing.T, eventType string) kevents.Envelope {
	t.Helper()
	var env kevents.Envelope
	d.read(t, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT envelope FROM event_outbox
			 WHERE envelope->>'type' = $1 AND envelope->'entity'->>'id' = $2
			 ORDER BY seq DESC LIMIT 1`, eventType, d.deal.String()).Scan(&env)
	})
	return env
}

func (d *dispactEnv) deliver(t *testing.T, env kevents.Envelope) {
	t.Helper()
	if err := d.sync.HandleEvent(context.Background(), env); err != nil {
		t.Fatalf("syncing %s: %v", env.Type, err)
	}
}

func (d *dispactEnv) read(t *testing.T, fn func(context.Context, pgx.Tx) error) {
	t.Helper()
	ctx := principal.WithWorkspaceID(context.Background(), d.e.WS)
	if err := database.WithWorkspaceTx(ctx, d.e.Pool, func(tx pgx.Tx) error { return fn(ctx, tx) }); err != nil {
		t.Fatalf("reading the fixture's tables: %v", err)
	}
}

func (d *dispactEnv) email(t *testing.T, user ids.UUID) string {
	t.Helper()
	var email string
	d.read(t, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT email FROM app_user WHERE id = $1`, user).Scan(&email)
	})
	return email
}

func TestADealCreatedOpensOneChannelWithItsColleaguesAndACard(t *testing.T) {
	d := setupDispact(t, true)
	created := d.envelope(t, string(crmcontracts.DealCreated))
	d.deliver(t, created)
	d.deliver(t, created)

	ch, posts := d.fake.only(t)
	if d.fake.creates != 1 {
		t.Errorf("Dispact made %d channels, want 1", d.fake.creates)
	}
	if _, ok := d.fake.channels["gwapp_fixture|margince:deal:"+d.deal.String()]; !ok {
		t.Errorf("the channel is not keyed on the deal")
	}
	want := []string{d.email(t, d.e.AdminUser), d.email(t, d.e.Rep1)}
	slices.Sort(want)
	got := slices.Sorted(slices.Values(ch.members))
	if !slices.Equal(got, want) {
		t.Errorf("members = %v, want the owner and the assigned rep who can open the deal %v", got, want)
	}
	if slices.Contains(ch.members, d.email(t, d.e.Rep2)) {
		t.Errorf("a rep who cannot open the deal was invited")
	}
	if len(posts) != 1 {
		t.Fatalf("posts = %q, want the card alone", posts)
	}
	for _, fact := range []string{"**Fleet renewal**", "Stage: Qualified", "Amount: 12,000.00 EUR", "https://crm.example.test"} {
		if !strings.Contains(posts[0], fact) {
			t.Errorf("the card %q does not show %q", posts[0], fact)
		}
	}
	if d.fake.tokens[0] != "gwapp_fixture" {
		t.Errorf("the sync spent token %q, not the connected bot's", d.fake.tokens[0])
	}
}

func TestAStageMoveEditsTheCardAndPostsOneLine(t *testing.T) {
	d := setupDispact(t, true)
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	if _, err := d.e.Deals.AdvanceDeal(d.e.Admin(), ids.From[ids.DealKind](d.deal), deals.AdvanceDealInput{ToStageID: d.stage}); err != nil {
		t.Fatal(err)
	}
	moved := d.envelope(t, deals.EventDealStageChanged)
	d.deliver(t, moved)
	d.deliver(t, moved)

	_, posts := d.fake.only(t)
	if len(posts) != 2 {
		t.Fatalf("posts = %q, want the card and one line", posts)
	}
	if !strings.Contains(posts[0], "Stage: Proposal") {
		t.Errorf("the card was not edited to the new stage: %q", posts[0])
	}
	if posts[1] != "Stage moved to Proposal (open)" {
		t.Errorf("line = %q", posts[1])
	}
}

func TestANewOwnerIsInvitedAndAnnounced(t *testing.T) {
	d := setupDispact(t, true)
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	owner := ids.From[ids.UserKind](d.e.Rep3)
	if _, err := d.e.Deals.UpdateDeal(d.e.Admin(), ids.From[ids.DealKind](d.deal), deals.UpdateDealInput{OwnerID: &owner}); err != nil {
		t.Fatal(err)
	}
	d.deliver(t, d.envelope(t, string(crmcontracts.DealOwnerChanged)))

	ch, posts := d.fake.only(t)
	if !slices.Contains(ch.members, d.email(t, d.e.Rep3)) {
		t.Errorf("members = %v, want the new owner among them", ch.members)
	}
	if len(posts) != 2 || posts[1] != "Owner is now Rep" {
		t.Errorf("posts = %q, want the card and the owner line", posts)
	}
}

func TestADealThatPredatesTheBotOpensOnItsFirstChange(t *testing.T) {
	d := setupDispact(t, true)
	if _, err := d.e.Deals.ArchiveDeal(d.e.Admin(), ids.From[ids.DealKind](d.deal), nil); err != nil {
		t.Fatal(err)
	}
	d.deliver(t, d.envelope(t, string(crmcontracts.DealArchived)))

	_, posts := d.fake.only(t)
	if len(posts) != 2 || !strings.Contains(posts[0], "Archived in Margince") || posts[1] != "The deal was archived in Margince" {
		t.Errorf("posts = %q, want the archived card and its line", posts)
	}
}

func TestADeletedCardIsPostedAgain(t *testing.T) {
	d := setupDispact(t, true)
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	ch, _ := d.fake.only(t)
	clear(ch.text)
	ch.order = nil
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	if _, posts := d.fake.only(t); len(posts) != 1 {
		t.Fatalf("a redelivered create after the card was deleted posted %q, want a new card", posts)
	}

	name := "Fleet renewal and services"
	if _, err := d.e.Deals.UpdateDeal(d.e.Admin(), ids.From[ids.DealKind](d.deal), deals.UpdateDealInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	d.deliver(t, d.envelope(t, string(crmcontracts.DealUpdated)))

	_, posts := d.fake.only(t)
	if len(posts) != 1 || !strings.Contains(posts[0], name) {
		t.Errorf("posts = %q, want one new card naming %q", posts, name)
	}
}

func TestReconnectingTheSameBotEditsTheCardToTheDealAsItIsNow(t *testing.T) {
	d := setupDispact(t, true)
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	store := capture.NewChannelStore(d.e.DB(), d.vault, &telegramPollFakeAPI{}, quietTestLogger()).WithDispact(d.fake)
	admin := telegramAdminContext(d.e.WS, d.e.Rep1)
	conns, err := store.List(admin)
	if err != nil || len(conns) != 1 {
		t.Fatalf("listing the connection: %v %v", conns, err)
	}
	if err := store.Disconnect(admin, conns[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Connect(admin, capture.ConnectRequest{Provider: capture.ProviderDispact, BotToken: "gwapp_fixture"}); err != nil {
		t.Fatalf("reconnecting the Dispact bot: %v", err)
	}

	name := "Fleet renewal after the reconnect"
	if _, err := d.e.Deals.UpdateDeal(d.e.Admin(), ids.From[ids.DealKind](d.deal), deals.UpdateDealInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	d.deliver(t, d.envelope(t, string(crmcontracts.DealUpdated)))

	_, posts := d.fake.only(t)
	if len(posts) != 1 || !strings.Contains(posts[0], name) {
		t.Errorf("posts = %q, want the one card edited to name %q", posts, name)
	}
}

func TestAnOwnerWhoseRoleMasksTheAmountIsNotInvited(t *testing.T) {
	d := setupDispact(t, true)
	// A mask binds only a bounded role, so Rep3 reads its own deals alone, and
	// sees this one by owning it.
	role := "dispact-reader-" + d.e.Rep3.String()
	d.e.WsExec(t, `UPDATE role SET permissions = '{"objects":{"deal":{"read":true}},"row_scope":"own"}' WHERE key = $1`, role)
	d.e.WsExec(t, `INSERT INTO field_mask (role_key, object, field, condition) VALUES ($1, 'deal', 'amount_minor', 'always')`, role)
	owner := ids.From[ids.UserKind](d.e.Rep3)
	if _, err := d.e.Deals.UpdateDeal(d.e.Admin(), ids.From[ids.DealKind](d.deal), deals.UpdateDealInput{OwnerID: &owner}); err != nil {
		t.Fatal(err)
	}
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))

	ch, _ := d.fake.only(t)
	if slices.Contains(ch.members, d.email(t, d.e.Rep3)) {
		t.Errorf("a colleague whose role masks the amount was shown the card")
	}
}

func TestARejectedTokenParksTheConnection(t *testing.T) {
	d := setupDispact(t, true)
	d.fake.refuses = dispact.ErrTokenRejected
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))

	var status string
	d.read(t, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT status FROM channel_connection WHERE provider = 'dispact'`).Scan(&status)
	})
	if status != capture.ChannelPollStoppedByBadToken {
		t.Errorf("status = %q, want the connection parked as needing a new token", status)
	}
}

func TestATokenOfAnotherBotForgetsTheOldBotsChannels(t *testing.T) {
	d := setupDispact(t, true)
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	store := capture.NewChannelStore(d.e.DB(), d.vault, &telegramPollFakeAPI{}, quietTestLogger()).WithDispact(d.fake)
	admin := telegramAdminContext(d.e.WS, d.e.Rep1)
	conns, err := store.List(admin)
	if err != nil || len(conns) != 1 {
		t.Fatalf("listing the connection: %v %v", conns, err)
	}

	if err := store.ReplaceToken(admin, conns[0].ID, "gwapp_fixture"); err != nil {
		t.Fatal(err)
	}
	if n := d.dealChannels(t); n != 1 {
		t.Errorf("a rotation of the same bot dropped its channels (%d left)", n)
	}
	if err := store.ReplaceToken(admin, conns[0].ID, "gwapp_another_bot"); err != nil {
		t.Fatal(err)
	}
	if n := d.dealChannels(t); n != 0 {
		t.Errorf("%d channels of the old bot are still recorded after a swap to another bot", n)
	}

	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	if d.fake.creates != 2 || d.dealChannels(t) != 1 {
		t.Errorf("the new bot made %d channels in all and recorded %d, want its own channel beside the old bot's",
			d.fake.creates, d.dealChannels(t))
	}
}

func TestReplacingARejectedTokenResumesTheSync(t *testing.T) {
	d := setupDispact(t, true)
	d.fake.refuses = dispact.ErrTokenRejected
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	d.fake.refuses = nil

	store := capture.NewChannelStore(d.e.DB(), d.vault, &telegramPollFakeAPI{}, quietTestLogger()).WithDispact(d.fake)
	admin := telegramAdminContext(d.e.WS, d.e.Rep1)
	conns, err := store.List(admin)
	if err != nil || len(conns) != 1 {
		t.Fatalf("listing the connection: %v %v", conns, err)
	}
	if err := store.ReplaceToken(admin, conns[0].ID, "gwapp_fixture"); err != nil {
		t.Fatal(err)
	}
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	if _, posts := d.fake.only(t); len(posts) != 1 {
		t.Errorf("posts = %q, want the card once the token was replaced", posts)
	}
}

func (d *dispactEnv) dealChannels(t *testing.T) int {
	t.Helper()
	var n int
	d.read(t, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM deal_channel`).Scan(&n)
	})
	return n
}

func TestARefusalNoRetryRepairsIsAcked(t *testing.T) {
	d := setupDispact(t, true)
	d.fake.refuses = &dispact.APIError{Method: "create", Code: "invalid_name"}
	err := d.sync.HandleEvent(context.Background(), d.envelope(t, string(crmcontracts.DealCreated)))
	if !errors.Is(err, kevents.ErrUnactionable) {
		t.Errorf("err = %v, want it acked as unactionable", err)
	}
	d.fake.refuses = &dispact.APIError{Method: "create", Code: dispact.CodeCreateInProgress}
	err = d.sync.HandleEvent(context.Background(), d.envelope(t, string(crmcontracts.DealCreated)))
	if err == nil || errors.Is(err, kevents.ErrUnactionable) {
		t.Errorf("err = %v, want a channel still being made retried", err)
	}
}

func TestAnArchivedChannelTakesNoPostsAndNothingWedges(t *testing.T) {
	d := setupDispact(t, true)
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	ch, _ := d.fake.only(t)
	ch.archived = true

	err := d.sync.HandleEvent(context.Background(), d.envelope(t, string(crmcontracts.DealCreated)))
	if !errors.Is(err, kevents.ErrUnactionable) {
		t.Errorf("err = %v, want the refusal acked rather than retried forever", err)
	}
	if _, posts := d.fake.only(t); len(posts) != 1 {
		t.Errorf("posts = %q, want only the card from before the archive", posts)
	}
}

func TestAColleagueAssignedLaterJoinsOnTheNextChange(t *testing.T) {
	d := setupDispact(t, true)
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	d.assign(t, d.e.Rep3)
	name := "Fleet renewal, phase two"
	if _, err := d.e.Deals.UpdateDeal(d.e.Admin(), ids.From[ids.DealKind](d.deal), deals.UpdateDealInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	d.deliver(t, d.envelope(t, string(crmcontracts.DealUpdated)))
	if ch, _ := d.fake.only(t); !slices.Contains(ch.members, d.email(t, d.e.Rep3)) {
		t.Errorf("members = %v, want the colleague assigned after the channel opened", ch.members)
	}
}

func TestWithoutADispactBotNothingIsSynced(t *testing.T) {
	d := setupDispact(t, false)
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))
	if len(d.fake.channels) != 0 {
		t.Errorf("a channel was made with no Dispact bot connected")
	}
}

func TestColleaguesTheProbeRefusesDoNotCrowdOutOneItAdmits(t *testing.T) {
	d := setupDispact(t, true)
	team := ids.NewV7()
	d.e.WsExec(t, `INSERT INTO team (id, name) VALUES ($1, 'Fleet desk')`, team)
	// More grantless members than Dispact takes in one call, each sorting ahead
	// of Rep3, who may open the deal.
	for i := range dispactMaxMembers + 10 {
		d.e.WsExec(t, `
			WITH u AS (INSERT INTO app_user (email, display_name, status) VALUES ($1, 'Desk', 'active') RETURNING id)
			INSERT INTO team_membership (team_id, user_id) SELECT $2, id FROM u`,
			fmt.Sprintf("0desk-%03d@example.test", i), team)
	}
	d.e.WsExec(t, `INSERT INTO team_membership (team_id, user_id) VALUES ($1, $2)`, team, d.e.Rep3)
	var role ids.UUID
	d.read(t, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id FROM record_role WHERE key = 'technical_contact'`).Scan(&role)
	})
	if _, err := assignments.NewStore(d.e.DB()).CreateAssignment(d.e.Admin(), assignments.CreateAssignmentInput{
		RecordType: crmcontracts.AssignmentRecordTypeDeal, RecordID: d.deal,
		SubjectKind: crmcontracts.AssignmentSubjectKindTeam, SubjectID: team, RoleID: role,
	}); err != nil {
		t.Fatalf("assigning the team: %v", err)
	}
	d.deliver(t, d.envelope(t, string(crmcontracts.DealCreated)))

	ch, _ := d.fake.only(t)
	if !slices.Contains(ch.members, d.email(t, d.e.Rep3)) {
		t.Errorf("a team member who may open the deal was crowded out by %d who may not", dispactMaxMembers+10)
	}
}
