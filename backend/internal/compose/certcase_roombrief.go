// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package compose

// The certification case for summarize/deal_room_brief. The request comes from
// roombrief.BriefRequest and the reply is read by roombrief.ParseBrief, the two
// the service calls.
//
// ParseBrief already drops what is ungrounded, so a reply saying nothing would
// pass a check that it parsed. What is measured is what production cannot
// guarantee: that the brief cites the thread the room turns on, and names a
// word only that thread said and the floor does not.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/margince/margince/backend/internal/compose/aitasks"
	"github.com/margince/margince/backend/internal/compose/roombrief"
	"github.com/margince/margince/backend/internal/modules/ai"
	"github.com/margince/margince/backend/internal/shared/kernel/ids"
	"github.com/margince/margince/backend/internal/shared/kernel/textlang"
	"github.com/margince/margince/backend/internal/shared/ports/model"
)

type roomBriefCases struct{}

func (roomBriefCases) Site() aitasks.Site {
	return aitasks.Site{Task: ai.TaskSummarize, Variant: "deal_room_brief", Kind: ai.SiteKindOneShot}
}

type roomBriefFixture struct {
	Title     string              `json:"title"`
	State     string              `json:"state"`
	Buyers    []roomBriefBuyer    `json:"buyers"`
	Documents []roomBriefDocument `json:"documents"`
	Threads   []roomBriefThread   `json:"threads"`
}

type roomBriefBuyer struct {
	Label             string   `json:"label"`
	Name              string   `json:"name"`
	SignedIn          bool     `json:"signed_in"`
	LastSeenOn        string   `json:"last_seen_on"`
	AskedForNewLinkOn string   `json:"asked_for_new_link_on"`
	Downloaded        []string `json:"downloaded"`
}

type roomBriefDocument struct {
	Label string `json:"label"`
	Title string `json:"title"`
	Group string `json:"group"`
}

type roomBriefThread struct {
	Label         string                `json:"label"`
	DocumentLabel string                `json:"document_label"`
	Resolved      bool                  `json:"resolved"`
	Comments      []roombrief.CommentIn `json:"comments"`
}

type roomBriefExpectation struct {
	CitesLabel string   `json:"cites_label"`
	NamesToken string   `json:"names_token"`
	Avoids     []string `json:"avoids"`
}

//nolint:ireturn // PreparedCase IS the seam: one implementation per site behind the one interface the cert lane runs.
func (roomBriefCases) Prepare(fixture, expected json.RawMessage) (aitasks.PreparedCase, error) {
	var f roomBriefFixture
	if err := json.Unmarshal(fixture, &f); err != nil {
		return nil, fmt.Errorf("summarize/deal_room_brief: the fixture is not the shape this site takes: %w", err)
	}
	var want roomBriefExpectation
	if err := json.Unmarshal(expected, &want); err != nil {
		return nil, fmt.Errorf("summarize/deal_room_brief: the expected answer is not this site's shape: %w", err)
	}
	in, byLabel, err := roomBriefInput(f)
	if err != nil {
		return nil, err
	}
	if err := refuseUnmeasurableRoomBrief(in, byLabel, want); err != nil {
		return nil, err
	}
	return &roomBriefCase{
		in: in, mustCite: byLabel[want.CitesLabel], citeLabel: want.CitesLabel,
		mustName: want.NamesToken, mustAvoid: want.Avoids,
	}, nil
}

// roomBriefInput mints an id per labelled record and folds the fixture into
// the input production builds. A thread is waiting on the reader when its last
// comment is a buyer's, the rule FromRoom applies.
func roomBriefInput(f roomBriefFixture) (roombrief.Input, map[string]string, error) {
	byLabel := map[string]string{}
	mint := func(label string) (string, error) {
		if strings.TrimSpace(label) == "" || byLabel[label] != "" {
			return "", fmt.Errorf("summarize/deal_room_brief: the label %q is blank or repeated, so an expectation naming it means nothing", label)
		}
		byLabel[label] = ids.NewV7().String()
		return byLabel[label], nil
	}
	in := roombrief.Input{Title: f.Title, State: f.State}
	for _, b := range f.Buyers {
		id, err := mint(b.Label)
		if err != nil {
			return in, nil, err
		}
		in.Buyers = append(in.Buyers, roombrief.BuyerIn{
			ID: id, Name: b.Name, SignedIn: b.SignedIn, LastSeenOn: b.LastSeenOn,
			AskedForNewLinkOn: b.AskedForNewLinkOn, Downloaded: b.Downloaded,
		})
	}
	titles := map[string]string{}
	for _, d := range f.Documents {
		id, err := mint(d.Label)
		if err != nil {
			return in, nil, err
		}
		titles[id] = d.Title
		in.Documents = append(in.Documents, roombrief.DocumentIn{ID: id, Title: d.Title, Group: d.Group})
	}
	for _, t := range f.Threads {
		id, err := mint(t.Label)
		if err != nil {
			return in, nil, err
		}
		if len(t.Comments) == 0 {
			return in, nil, fmt.Errorf("summarize/deal_room_brief: thread %q has no comments", t.Label)
		}
		thread := roombrief.ThreadIn{ID: id, Resolved: t.Resolved, Comments: t.Comments}
		if t.DocumentLabel != "" {
			thread.DocumentID = byLabel[t.DocumentLabel]
			thread.About = titles[thread.DocumentID]
		}
		thread.WaitingOnYou = !t.Resolved && t.Comments[len(t.Comments)-1].Speaker == "buyer"
		in.Threads = append(in.Threads, thread)
	}
	return in, byLabel, nil
}

// refuseUnmeasurableRoomBrief refuses a scenario no reply could meet, and one
// any reply meets: a token the floor already says passes without the model.
func refuseUnmeasurableRoomBrief(in roombrief.Input, byLabel map[string]string, want roomBriefExpectation) error {
	if byLabel[want.CitesLabel] == "" {
		return fmt.Errorf("summarize/deal_room_brief: the expectation cites %q, which the fixture does not carry", want.CitesLabel)
	}
	token := strings.ToLower(strings.TrimSpace(want.NamesToken))
	if token == "" {
		return errors.New("summarize/deal_room_brief: the expectation names no token, so a generic brief would pass")
	}
	summary, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("summarize/deal_room_brief: encode the summary: %w", err)
	}
	if !strings.Contains(strings.ToLower(string(summary)), token) {
		return fmt.Errorf("summarize/deal_room_brief: the token %q is in nothing the summary carries, so only an invented brief could name it", want.NamesToken)
	}
	if strings.Contains(strings.ToLower(roombrief.Prose(roombrief.Deterministic(in, "en"))), token) {
		return fmt.Errorf("summarize/deal_room_brief: the token %q is already in the floor's own prose, so a reply saying nothing would pass", want.NamesToken)
	}
	for _, avoided := range want.Avoids {
		if strings.TrimSpace(avoided) == "" {
			return errors.New("summarize/deal_room_brief: the expectation forbids a blank phrase, which every reply contains")
		}
	}
	return nil
}

type roomBriefCase struct {
	in        roombrief.Input
	mustCite  string
	citeLabel string
	mustName  string
	mustAvoid []string
}

// Run issues the production request, in English pinned so a record's score
// does not move with an installation's settings.
func (c *roomBriefCase) Run(ctx context.Context, completer aitasks.Completer) (aitasks.Trace, error) {
	req := roombrief.BriefRequest(c.in, string(textlang.English))
	trace := aitasks.Trace{Requests: []model.Request{req}}
	resp, err := completer.Complete(ctx, req)
	if err != nil {
		return trace, fmt.Errorf("summarize/deal_room_brief: %w", err)
	}
	trace.Output = resp.Text
	return trace, nil
}

func (c *roomBriefCase) Evaluate(trace aitasks.Trace) aitasks.Outcome {
	sentences, err := roombrief.ParseBrief(trace.Output, c.in)
	if err != nil {
		return aitasks.Outcome{Result: aitasks.OutcomeInvalid, Detail: fmt.Sprintf("the grounding filter refused the reply: %v", err)}
	}
	if len(sentences) == 0 {
		return aitasks.Outcome{Result: aitasks.OutcomeAbstained, Detail: "no sentence cited a record of this room"}
	}
	var wrong []string
	if !cites(sentences, c.mustCite) {
		wrong = append(wrong, fmt.Sprintf("the brief never cites %q, the thread this room turns on", c.citeLabel))
	}
	folded := strings.ToLower(roombrief.Prose(sentences))
	if !strings.Contains(folded, strings.ToLower(c.mustName)) {
		wrong = append(wrong, fmt.Sprintf("the brief never names %q, so it would read the same about any room", c.mustName))
	}
	for _, avoided := range c.mustAvoid {
		if strings.Contains(folded, strings.ToLower(avoided)) {
			wrong = append(wrong, fmt.Sprintf("the brief says %q", avoided))
		}
	}
	if len(wrong) > 0 {
		return aitasks.Outcome{Result: aitasks.OutcomeWrongAnswer, Detail: strings.Join(wrong, "; ")}
	}
	return aitasks.Outcome{Result: aitasks.OutcomeAccepted}
}
