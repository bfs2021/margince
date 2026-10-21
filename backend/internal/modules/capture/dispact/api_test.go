// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package dispact

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// request is one call the stand-in received.
type request struct {
	method, path, auth string
	body               map[string]any
}

// standIn answers every call with one status and body, and records each call.
// httptest serves each request on its own goroutine, hence the mutex.
type standIn struct {
	mu       sync.Mutex
	requests []request
}

func serve(t *testing.T, status int, body string) (API, *standIn) {
	t.Helper()
	rec := &standIn{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the request: %v", err)
		}
		got := request{method: r.Method, path: r.URL.EscapedPath(), auth: r.Header.Get("Authorization")}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &got.body); err != nil {
				t.Errorf("the request body is not JSON: %v", err)
			}
		}
		rec.mu.Lock()
		rec.requests = append(rec.requests, got)
		rec.mu.Unlock()
		w.WriteHeader(status)
		if _, err := io.WriteString(w, body); err != nil {
			t.Errorf("writing the answer: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return NewAPI(srv.Client(), srv.URL+"/"), rec
}

func (s *standIn) last(t *testing.T) request {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		t.Fatal("no request reached the stand-in")
	}
	return s.requests[len(s.requests)-1]
}

const token = "gwapp_secret"

func TestGetMeKeysTheBotOnItsUser(t *testing.T) {
	api, rec := serve(t, http.StatusOK, `{"ok":true,"bot_id":"u-1","bot_user":"crm","team":"Gradion"}`)
	bot, err := api.GetMe(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if bot != (Bot{ID: "u-1", Username: "crm", Team: "Gradion"}) {
		t.Errorf("bot = %+v", bot)
	}
	got := rec.last(t)
	if got.method != http.MethodGet || got.path != "/api/bot/me" || got.auth != "Bearer "+token {
		t.Errorf("request = %s %s auth %q", got.method, got.path, got.auth)
	}
}

func TestGetMeWithNoBotIDIsARefusal(t *testing.T) {
	api, _ := serve(t, http.StatusOK, `{"ok":true,"bot_user":"crm"}`)
	if _, err := api.GetMe(context.Background(), token); !errors.Is(err, ErrRequestRejected) {
		t.Errorf("err = %v, want ErrRequestRejected", err)
	}
}

func TestVerdictSentinels(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		sentinel error
		code     string
	}{
		{"revoked token", http.StatusOK, `{"ok":false,"error":"invalid_auth"}`, ErrTokenRejected, ""},
		{"missing header", http.StatusOK, `{"ok":false,"error":"not_authed"}`, ErrTokenRejected, ""},
		{"a refusal keeps its code", http.StatusOK, `{"ok":false,"error":"channel_not_found"}`, ErrRequestRejected, CodeChannelNotFound},
		{"an outage", http.StatusBadGateway, `bad gateway`, ErrUnreachable, ""},
		{"a rate limit is retried", http.StatusTooManyRequests, `{"ok":false,"error":"ratelimited"}`, ErrUnreachable, ""},
		{"no envelope", http.StatusOK, `<html>`, ErrUnreachable, ""},
		{"a proxy refuses the token", http.StatusUnauthorized, `Unauthorized`, ErrTokenRejected, ""},
		{"a gateway refuses in JSON", http.StatusForbidden, `{"ok":false,"error":"forbidden"}`, ErrTokenRejected, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			api, _ := serve(t, c.status, c.body)
			err := api.UpdateMessage(context.Background(), token, "deal-x", "1.2", "text")
			if !errors.Is(err, c.sentinel) {
				t.Errorf("err = %v, want %v", err, c.sentinel)
			}
			if CodeOf(err) != c.code {
				t.Errorf("code = %q, want %q", CodeOf(err), c.code)
			}
		})
	}
}

func TestAPathDispactDoesNotServeIsRefusedWithItsStatus(t *testing.T) {
	api, _ := serve(t, http.StatusNotFound, `<html>Not Found</html>`)
	err := api.UpdateMessage(context.Background(), token, "deal-x", "1.2", "text")
	if !errors.Is(err, ErrRequestRejected) {
		t.Errorf("err = %v, want ErrRequestRejected, since no retry repairs a wrong base URL", err)
	}
	if apiErr, ok := errors.AsType[*APIError](err); !ok || apiErr.Status != http.StatusNotFound {
		t.Errorf("err = %#v, want an *APIError naming HTTP 404", err)
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("message = %q, want it to name HTTP 404 for the operator", err.Error())
	}
}

func TestUnreachableHostIsUnreachable(t *testing.T) {
	api := NewAPI(nil, "http://127.0.0.1:1")
	if _, err := api.GetMe(context.Background(), token); !errors.Is(err, ErrUnreachable) {
		t.Errorf("err = %v, want ErrUnreachable", err)
	}
}

func TestCreateChannelIsPrivateAndKeyed(t *testing.T) {
	api, rec := serve(t, http.StatusOK, `{"ok":true,"channel":"deal-acme","channel_id":"c-1","bot_is_member":true,
		"failed":[{"email":"buyer@example.com","reason":"user_not_found"}]}`)
	ch, membership, err := api.CreateChannel(context.Background(), token, CreateChannelRequest{
		Name: "deal Acme", ExternalKey: "margince:deal:1", Topic: "Margince deal: Acme",
		Members: []string{"rep@example.com", "buyer@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ch != (Channel{ID: "c-1", Slug: "deal-acme", BotIsMember: true}) {
		t.Errorf("channel = %+v", ch)
	}
	if len(membership.Failed) != 1 || membership.Failed[0].Reason != "user_not_found" {
		t.Errorf("failed = %+v", membership.Failed)
	}
	body := rec.last(t).body
	if body["is_private"] != true || body["external_key"] != "margince:deal:1" || body["topic"] != "Margince deal: Acme" {
		t.Errorf("body = %v", body)
	}
}

func TestCreateChannelWithNoChannelIsARefusal(t *testing.T) {
	api, _ := serve(t, http.StatusOK, `{"ok":true}`)
	if _, _, err := api.CreateChannel(context.Background(), token, CreateChannelRequest{Name: "x"}); !errors.Is(err, ErrRequestRejected) {
		t.Errorf("err = %v, want ErrRequestRejected", err)
	}
}

func TestAddMembersEscapesTheSlug(t *testing.T) {
	api, rec := serve(t, http.StatusOK, `{"ok":true,"added":["a@example.com"],"already_member":["b@example.com"]}`)
	got, err := api.AddMembers(context.Background(), token, "G-ab/cd", []string{"a@example.com", "b@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Added) != 1 || len(got.AlreadyMember) != 1 {
		t.Errorf("membership = %+v", got)
	}
	if path := rec.last(t).path; path != "/api/bot/channels/G-ab%2Fcd/members" {
		t.Errorf("path = %q", path)
	}
}

func TestPostMessageCarriesItsIdempotencyKey(t *testing.T) {
	api, rec := serve(t, http.StatusOK, `{"ok":true,"ts":"1.42"}`)
	ts, err := api.PostMessage(context.Background(), token, "deal-acme", "card", "margince:card:1")
	if err != nil {
		t.Fatal(err)
	}
	if ts != "1.42" {
		t.Errorf("ts = %q", ts)
	}
	if body := rec.last(t).body; body["client_msg_id"] != "margince:card:1" || body["channel"] != "deal-acme" {
		t.Errorf("body = %v", body)
	}
}

func TestPostMessageWithNoTSIsUnknown(t *testing.T) {
	api, _ := serve(t, http.StatusOK, `{"ok":true}`)
	if _, err := api.PostMessage(context.Background(), token, "deal-acme", "card", "k"); !errors.Is(err, ErrUnreachable) {
		t.Errorf("err = %v, want ErrUnreachable", err)
	}
}

func TestValidateToken(t *testing.T) {
	for token, ok := range map[string]bool{
		"gwapp_abc":        true,
		"gwapp_":           false,
		"123:telegram":     false,
		"gwapp_with space": false,
		"":                 false,
	} {
		if err := ValidateToken(token); (err == nil) != ok {
			t.Errorf("ValidateToken(%q) = %v, want ok=%v", token, err, ok)
		}
		if err := ValidateToken(token); err != nil && !errors.Is(err, ErrTokenRejected) {
			t.Errorf("ValidateToken(%q) = %v, want ErrTokenRejected", token, err)
		}
	}
}
