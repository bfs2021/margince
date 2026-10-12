// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package deals

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	crmcontracts "github.com/margince/margince/backend/internal/contracts"
)

// An offer body that does not decode is refused before the store is reached,
// so the zero Handlers drives the real CreateOffer path.
func TestCreateOfferRefusesAnUndecodableBodyWithoutNamingTheProgram(t *testing.T) {
	cases := []struct {
		name, body, wantDetail string
	}{
		{
			name:       "a line item quantity sent as an object names the field",
			body:       `{"currency":"EUR","source":"manual","line_items":[{"quantity":{}}]}`,
			wantDetail: "`0.quantity` must be a number, not an object",
		},
		{
			name:       "a buyer company id sent as a number names the shape",
			body:       `{"currency":"EUR","source":"manual","buyer_company_id":7}`,
			wantDetail: "must be a UUID string, not a number",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/deals/x/offers", strings.NewReader(tc.body))
			Handlers{}.CreateOffer(rec, req, uuid.New(), crmcontracts.CreateOfferParams{})

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
			}
			var problem struct {
				Detail  string `json:"detail"`
				Details struct {
					Errors []struct {
						Field string `json:"field"`
						Code  string `json:"code"`
					} `json:"errors"`
				} `json:"details"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decoding problem body %q: %v", rec.Body.String(), err)
			}
			fields := problem.Details.Errors
			if len(fields) != 1 || fields[0].Field != "body" || fields[0].Code != "malformed_json" {
				t.Fatalf("errors = %+v, want one body/malformed_json entry", fields)
			}
			if !strings.Contains(problem.Detail, tc.wantDetail) {
				t.Errorf("detail = %q, want it to say %q", problem.Detail, tc.wantDetail)
			}
			for _, leak := range []string{"crmcontracts.", "Go struct", "Go value", "json: cannot unmarshal", "uuid.UUID"} {
				if strings.Contains(rec.Body.String(), leak) {
					t.Errorf("the refusal carries %q, which describes this program: %s", leak, rec.Body.String())
				}
			}
		})
	}
}
