// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

// Package dispact is the Gradion Dispact bot API boundary: the calls a deal
// channel needs, hand-rolled over net/http like the Telegram client beside it.
// Dispact answers HTTP 200 with an {ok, error} envelope, so every call goes
// through one verdict that turns the envelope into this package's sentinels.
package dispact

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/margince/margince/backend/internal/shared/kernel/textcut"
)

// ProviderName is the channel_connection.provider value of a Dispact bot.
const ProviderName = "dispact"

// ErrTokenRejected marks a token Dispact does not know or has revoked.
var ErrTokenRejected = errors.New("dispact: the bot token was rejected")

// ErrUnreachable marks a transport failure, a 5xx or a 429: a retry may succeed.
var ErrUnreachable = errors.New("dispact: could not reach Dispact")

// ErrRequestRejected marks a request Dispact understood and refused, or a 4xx
// with no Dispact envelope. Either rides an *APIError, which wraps this sentinel.
var ErrRequestRejected = errors.New("dispact: the request was rejected")

// APIError is one refusal, with the code Dispact answered. Callers branch on the
// code; the transport never shows it to a client.
type APIError struct {
	Method string
	Code   string
	// Status is the HTTP status of an answer that carried no Dispact envelope, and
	// 0 for a refusal Dispact itself made.
	Status int
}

func (e *APIError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("dispact: %s answered HTTP %d with no envelope", e.Method, e.Status)
	}
	return fmt.Sprintf("dispact: %s answered %s", e.Method, e.Code)
}

func (e *APIError) Unwrap() error { return ErrRequestRejected }

// CodeOf answers the Dispact error code err carries, or "" when it carries none.
func CodeOf(err error) string {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.Code
	}
	return ""
}

// The refusal codes a caller acts on.
const (
	CodeCreateInProgress  = "create_in_progress"
	CodeChannelNotFound   = "channel_not_found"
	CodeNotInChannel      = "not_in_channel"
	CodeMessageNotFound   = "message_not_found"
	CodeCantUpdateMessage = "cant_update_message"
	CodeIsArchived        = "is_archived"
	CodeNameTaken         = "name_taken"
)

// tokenPrefix starts every Dispact app bot token.
const tokenPrefix = "gwapp_"

// ValidateToken rejects a value that cannot be a Dispact bot token before it is
// spent on a network call. Only GetMe can say whether the token is live.
func ValidateToken(token string) error {
	if !strings.HasPrefix(token, tokenPrefix) || len(token) == len(tokenPrefix) ||
		strings.ContainsAny(token, " \t\r\n") || len(token) > 512 {
		return fmt.Errorf("a Dispact bot token starts with %q and holds no spaces: %w", tokenPrefix, ErrTokenRejected)
	}
	return nil
}

// Bot is what /api/bot/me reports.
type Bot struct {
	// ID is the bot user's id. It survives a token rotation, which keeps the
	// bot's channel keys, so it is what a connection is keyed on.
	ID       string
	Username string
	Team     string
}

// Channel is a channel the bot created or found by its key.
type Channel struct {
	ID          string
	Slug        string
	BotIsMember bool
	IsArchived  bool
}

// CreateChannelRequest is one keyed, private channel with its first members.
type CreateChannelRequest struct {
	Name        string
	ExternalKey string
	Topic       string
	Members     []string
}

// Membership is what adding members answered, per email.
type Membership struct {
	Added         []string
	AlreadyMember []string
	Failed        []FailedMember
}

// FailedMember is one email Dispact did not add, and why.
type FailedMember struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

// API is the Dispact bot API surface. Every call carries the token, because one
// client serves every workspace and a rotation must not need a new client.
type API interface {
	GetMe(ctx context.Context, token string) (Bot, error)
	CreateChannel(ctx context.Context, token string, req CreateChannelRequest) (Channel, Membership, error)
	AddMembers(ctx context.Context, token, slug string, emails []string) (Membership, error)
	// PostMessage posts once per clientMsgID: a repeat returns the first ts.
	PostMessage(ctx context.Context, token, slug, text, clientMsgID string) (ts string, err error)
	UpdateMessage(ctx context.Context, token, slug, ts, text string) error
}

const httpTimeout = 15 * time.Second

const maxResponseBytes = 1 << 20

type httpAPI struct {
	client *http.Client
	base   string
}

// NewAPI builds the client for the Dispact install at base, the origin an
// operator configures. A nil client gets one with a bounded timeout.
//
//nolint:ireturn // returns the API seam by design, so tests substitute a fake
func NewAPI(client *http.Client, base string) API {
	if client == nil {
		client = &http.Client{Timeout: httpTimeout}
	}
	// A redirect would resend the body (deal facts, invited emails) to a host
	// the operator never configured.
	noRedirects := *client
	noRedirects.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &httpAPI{client: &noRedirects, base: strings.TrimRight(base, "/")}
}

// GetMe validates the token and identifies the bot behind it.
func (a *httpAPI) GetMe(ctx context.Context, token string) (Bot, error) {
	var out struct {
		BotID   string `json:"bot_id"`
		BotUser string `json:"bot_user"`
		Team    string `json:"team"`
	}
	if err := a.call(ctx, token, http.MethodGet, "/api/bot/me", nil, &out); err != nil {
		return Bot{}, err
	}
	if out.BotID == "" || len(out.BotID) > maxIDLen {
		return Bot{}, &APIError{Method: "me", Code: "no_bot_id"}
	}
	return Bot{ID: out.BotID, Username: textcut.Runes(out.BotUser, maxLabelLen), Team: textcut.Runes(out.Team, maxLabelLen)}, nil
}

// Bounds on what a Dispact answer may store. A slug is reused as a path
// segment, so it is held to the characters Dispact derives slugs from.
const (
	maxIDLen    = 64
	maxLabelLen = 100
)

var slugPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

func validSlug(slug string) bool { return slugPattern.MatchString(slug) }

// CreateChannel creates the keyed private channel, or returns the one the key
// already names.
func (a *httpAPI) CreateChannel(ctx context.Context, token string, req CreateChannelRequest) (Channel, Membership, error) {
	body := map[string]any{
		"name": req.Name, "is_private": true, "external_key": req.ExternalKey,
		"topic": req.Topic, "members": req.Members,
	}
	var out struct {
		Channel     string         `json:"channel"`
		ChannelID   string         `json:"channel_id"`
		BotIsMember bool           `json:"bot_is_member"`
		IsArchived  bool           `json:"is_archived"`
		Failed      []FailedMember `json:"failed"`
	}
	if err := a.call(ctx, token, http.MethodPost, "/api/bot/channels", body, &out); err != nil {
		return Channel{}, Membership{}, err
	}
	if !validSlug(out.Channel) || out.ChannelID == "" || len(out.ChannelID) > maxIDLen {
		return Channel{}, Membership{}, &APIError{Method: "channels.create", Code: "no_channel"}
	}
	return Channel{ID: out.ChannelID, Slug: out.Channel, BotIsMember: out.BotIsMember, IsArchived: out.IsArchived},
		Membership{Failed: out.Failed}, nil
}

// AddMembers adds members to a channel the bot is in.
func (a *httpAPI) AddMembers(ctx context.Context, token, slug string, emails []string) (Membership, error) {
	var out struct {
		Added         []string       `json:"added"`
		AlreadyMember []string       `json:"already_member"`
		Failed        []FailedMember `json:"failed"`
	}
	path := "/api/bot/channels/" + url.PathEscape(slug) + "/members"
	if err := a.call(ctx, token, http.MethodPost, path, map[string]any{"emails": emails}, &out); err != nil {
		return Membership{}, err
	}
	return Membership{Added: out.Added, AlreadyMember: out.AlreadyMember, Failed: out.Failed}, nil
}

// PostMessage posts text to the channel.
func (a *httpAPI) PostMessage(ctx context.Context, token, slug, text, clientMsgID string) (string, error) {
	var out struct {
		TS string `json:"ts"`
	}
	body := map[string]any{"channel": slug, "text": text, "client_msg_id": clientMsgID}
	if err := a.call(ctx, token, http.MethodPost, "/api/bot/messages", body, &out); err != nil {
		return "", err
	}
	if out.TS == "" {
		return "", fmt.Errorf("dispact: a post answered without a ts: %w", ErrUnreachable)
	}
	return out.TS, nil
}

// UpdateMessage replaces the text of a message the bot posted.
func (a *httpAPI) UpdateMessage(ctx context.Context, token, slug, ts, text string) error {
	body := map[string]any{"channel": slug, "ts": ts, "text": text}
	return a.call(ctx, token, http.MethodPost, "/api/bot/messages/update", body, nil)
}

// call performs one request and decodes the success body into out (nil when
// only the verdict matters).
//
//craft:ignore naked-any out is the caller's per-endpoint JSON decode target
func (a *httpAPI) call(ctx context.Context, token, method, path string, body map[string]any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("dispact: encoding %s: %w", path, err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, reader)
	if err != nil {
		return fmt.Errorf("dispact: building %s: %w", path, errors.Join(ErrRequestRejected, err))
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("dispact: %s: %w", path, errors.Join(ErrUnreachable, err))
	}
	defer func() { _ = resp.Body.Close() }() //craft:ignore swallowed-errors the body was read in full; a close failure changes no verdict
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("dispact: reading %s: %w", path, errors.Join(ErrUnreachable, err))
	}
	return verdict(path, resp.StatusCode, raw, out)
}

// verdict turns one answer into the decoded body or a sentinel.
//
//craft:ignore naked-any out is the caller's per-endpoint JSON decode target
func verdict(path string, status int, raw []byte, out any) error {
	if status >= http.StatusInternalServerError || status == http.StatusTooManyRequests {
		return fmt.Errorf("dispact: %s answered HTTP %d: %w", path, status, ErrUnreachable)
	}
	// Dispact answers every refusal with HTTP 200, so a 401 or 403 comes from a
	// gateway in front of it, whatever its body says.
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return fmt.Errorf("dispact: %s answered HTTP %d: %w", path, status, ErrTokenRejected)
	}
	var env struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		// A 4xx page that is not Dispact's names a wrong base URL, which no retry repairs.
		if status >= http.StatusBadRequest {
			return &APIError{Method: path, Status: status}
		}
		return fmt.Errorf("dispact: %s answered HTTP %d with no envelope: %w", path, status, ErrUnreachable)
	}
	switch {
	case env.OK && out != nil:
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("dispact: decoding %s: %w", path, errors.Join(ErrUnreachable, err))
		}
		return nil
	case env.OK:
		return nil
	case env.Error == "invalid_auth" || env.Error == "not_authed":
		return fmt.Errorf("dispact: %s: %w", path, ErrTokenRejected)
	default:
		return &APIError{Method: path, Code: env.Error}
	}
}
