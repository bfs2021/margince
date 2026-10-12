// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package compose

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	crmcontracts "github.com/margince/margince/backend/internal/contracts"
	"github.com/margince/margince/backend/internal/modules/agents"
	"github.com/margince/margince/backend/internal/platform/agentvolume"
	"github.com/margince/margince/backend/internal/platform/auth"
	"github.com/margince/margince/backend/internal/shared/apperrors"
)

// readCeiling is the volume meter at a fixed read ceiling: it refuses a charge
// that would cross it and records the ones it admits.
type readCeiling struct {
	ceiling int
	spent   map[agentvolume.Counter]int
}

func (c *readCeiling) Consume(_ context.Context, counter agentvolume.Counter, n int) error {
	if c.spent[counter]+n > c.ceiling {
		return fmt.Errorf("read ceiling %d reached: %w", c.ceiling, apperrors.ErrBudgetExceeded)
	}
	c.spent[counter] += n
	return nil
}

// serveReportingCSVToAgent writes one evaluation's CSV through the meter an
// agent read carries, and answers the recorder and what the read bound was charged.
func serveReportingCSVToAgent(t *testing.T, ceiling int) (*httptest.ResponseRecorder, int) {
	t.Helper()
	charger := &readCeiling{ceiling: ceiling, spent: map[agentvolume.Counter]int{}}
	reg := agents.NewRegistry(nil, auth.NewGate(fullSeat{}), agents.WithVolumeCharger(charger))
	r := httptest.NewRequest(http.MethodGet, "/v1/analytics/evaluate.csv", nil)
	recorder := httptest.NewRecorder()
	meter := &servedMeter{ResponseWriter: recorder, r: r, reg: reg, mayRefuse: nothingHasHappenedYet}

	writeReportingCSV(meter, r, crmcontracts.ReportingEvaluation{
		Metrics: []crmcontracts.ReportingMetric{{Id: "pipeline_value", Unit: "count"}},
	})
	return recorder, charger.spent[agentvolume.Reads]
}

func TestAReportingCSVIsChargedAgainstTheAgentReadBound(t *testing.T) {
	recorder, charged := serveReportingCSVToAgent(t, 10)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 with room under the ceiling", recorder.Code)
	}
	if !strings.HasPrefix(recorder.Body.String(), "metric,block,label") {
		t.Errorf("body %q, want the evaluation's CSV", recorder.Body.String())
	}
	if charged != 1 {
		t.Errorf("the read bound was charged %d, want 1, the count the JSON twin pays", charged)
	}
}

func TestAReportingCSVPastTheReadCeilingIsWithheld(t *testing.T) {
	recorder, charged := serveReportingCSVToAgent(t, 0)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429 from the read bound", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"rate_limited"`) {
		t.Errorf("body %q, want the budget refusal's rate_limited code", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "pipeline_value") {
		t.Errorf("a refused export still handed over its rows: %q", recorder.Body.String())
	}
	if ct := recorder.Header().Get("Content-Type"); strings.HasPrefix(ct, "text/csv") {
		t.Errorf("a refused export answered as %q", ct)
	}
	if charged != 0 {
		t.Errorf("a refused export charged %d", charged)
	}
}

// A refused download must not carry the validator or cache life its handler set
// for the success, or a cache could keep the refusal against the object.
func TestARefusedDownloadDropsTheHeadersSetForItsSuccess(t *testing.T) {
	charger := &readCeiling{ceiling: 0, spent: map[agentvolume.Counter]int{}}
	reg := agents.NewRegistry(nil, auth.NewGate(fullSeat{}), agents.WithVolumeCharger(charger))
	r := httptest.NewRequest(http.MethodGet, "/v1/companies/logo", nil)
	recorder := httptest.NewRecorder()
	meter := &servedMeter{ResponseWriter: recorder, r: r, reg: reg, mayRefuse: nothingHasHappenedYet}
	meter.Header().Set("ETag", `"logo-v1"`)
	meter.Header().Set("Cache-Control", "private, max-age=300")

	if meter.NoteServed(1) {
		t.Fatal("NoteServed admitted a read past the ceiling")
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429 from the read bound", recorder.Code)
	}
	for _, name := range []string{"ETag", "Cache-Control"} {
		if got := recorder.Header().Get(name); got != "" {
			t.Errorf("the refusal carried %s %q set for the success", name, got)
		}
	}
}
