// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

//go:build integration

package compose

// A recorded DSR intake names a data subject, so replaying it is a read of the
// case: the officer must still work the queue, and an erased subject's case
// must not be served from the claim table.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/margince/margince/backend/internal/compose/integration"
	crmcontracts "github.com/margince/margince/backend/internal/contracts"
	"github.com/margince/margince/backend/internal/modules/consent"
	"github.com/margince/margince/backend/internal/modules/privacy"
)

const dsrIntakePath = "/v1/data-subject-requests"

// keyedDSRRouter mounts the real intake handler behind the middleware, with the
// probes the composition root wires.
func keyedDSRRouter(e *integration.Env, filed *int) chi.Router {
	h := consent.NewHandlers(e.DB())
	r := chi.NewRouter()
	r.With(idempotency(e.Pool, replayProbes(nil, nil, nil, nil, e.DB()), nil)).Post(dsrIntakePath,
		func(w http.ResponseWriter, req *http.Request) {
			*filed++
			h.CreateDataSubjectRequest(w, req, crmcontracts.CreateDataSubjectRequestParams{})
		})
	return r
}

func keyedDSRCall(ctx context.Context, r chi.Router, subjectRef, key string) *httptest.ResponseRecorder {
	body := `{"kind":"access","subject_ref":"` + subjectRef + `"}`
	req := httptest.NewRequest(http.MethodPost, dsrIntakePath, strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// fileAndReplay files one case and proves the same key replays it unchanged
// without filing a second, so a later refusal is the gate and not a broken replay.
func fileAndReplay(ctx context.Context, t *testing.T, r chi.Router, filed *int, subjectRef, key string) {
	t.Helper()
	first := keyedDSRCall(ctx, r, subjectRef, key)
	if first.Code != http.StatusCreated {
		t.Fatalf("filing = %d %s, want 201", first.Code, first.Body)
	}
	replay := keyedDSRCall(ctx, r, subjectRef, key)
	if replay.Code != http.StatusCreated || replay.Body.String() != first.Body.String() {
		t.Fatalf("replay while still allowed = %d %s, want the recorded 201 %s", replay.Code, replay.Body, first.Body)
	}
	if *filed != 1 {
		t.Fatalf("intake ran %d times, want 1: the second call must be a replay", *filed)
	}
}

func assertRefusedReplay(t *testing.T, rec *httptest.ResponseRecorder, status int, code, subjectRef string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("replay = %d %s, want %d", rec.Code, rec.Body, status)
	}
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil || problem.Code != code {
		t.Fatalf("replay problem code = %q (decode err %v), want %q: %s", problem.Code, err, code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), subjectRef) {
		t.Fatalf("refused replay leaked the subject: %s", rec.Body)
	}
}

func TestDSRIntakeReplayRefusesAnOfficerWhoLostTheQueue(t *testing.T) {
	e := integration.Setup(t)
	officer := e.As(e.AdminUser, nil, integration.AdminPerms)
	filed := 0
	r := keyedDSRRouter(e, &filed)
	const subjectRef = "replay-subject@example.com"
	fileAndReplay(officer, t, r, &filed, subjectRef, "dsr-replay-authority")

	// Same seat, same key, same body: only the privacy_request grant is gone.
	demoted := e.As(e.AdminUser, nil, integration.RepPerms)
	assertRefusedReplay(t, keyedDSRCall(demoted, r, subjectRef, "dsr-replay-authority"),
		http.StatusForbidden, "permission_denied", subjectRef)
	if filed != 1 {
		t.Fatalf("intake ran %d times, want 1: a refused replay must not file again", filed)
	}
}

func TestDSRIntakeReplayRefusesOnceTheSubjectIsErased(t *testing.T) {
	e := integration.Setup(t)
	officer := e.Admin()
	filed := 0
	r := keyedDSRRouter(e, &filed)
	subject := e.SeedContact(t, "Erased Before Replay", nil)
	fileAndReplay(officer, t, r, &filed, subject.String(), "dsr-replay-erasure")

	if err := privacy.NewEraser(e.DB()).EraseContact(officer, subject, "manual"); err != nil {
		t.Fatalf("erasing the subject: %v", err)
	}
	assertRefusedReplay(t, keyedDSRCall(officer, r, subject.String(), "dsr-replay-erasure"),
		http.StatusNotFound, "not_found", subject.String())
}
