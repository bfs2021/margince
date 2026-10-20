/** @vitest-environment happy-dom */
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { components } from "../../api/schema";
import { LocaleProvider } from "../../i18n";
import { jsonResponse, stubWithSession } from "../../testing/story-utils";
import { ContactDealsTab } from "./contactdeals";
import { ContactMeetingsTab } from "./contactmeetings";
import { ContactTimelineTab } from "./contacttabs";

type Contact360 = components["schemas"]["Contact360"];
type Deal = components["schemas"]["Deal"];
type Company = components["schemas"]["Company"];
// The 360 carries its OWN spelling of an activity row — the section's element
// type, not the standalone `Activity` schema, which differs in two fields.
// Deriving it from the section is what keeps this fixture honest about the
// payload the page actually reads.
type SectionActivity = NonNullable<Contact360["activities"]>["data"][number];

// Provenance is stamped by the server on every captured row, so a fixture
// that leaves it out is a payload no reader ever receives. These two build a
// row the CONTRACT admits, which is what makes the fixtures below typed
// rather than asserted: an assertion would let a field the page reads drift
// out of the fixture without a word from the compiler.
const CAPTURED = {
  source: "manual",
  captured_by: "human:u-1",
  created_at: "2026-06-01T08:00:00Z",
  updated_at: "2026-08-01T08:00:00Z",
} as const;

function activity(
  row: Pick<SectionActivity, "id" | "kind" | "occurred_at"> &
    Partial<SectionActivity>,
): SectionActivity {
  return { is_done: false, ...CAPTURED, ...row };
}

// The six tabs beside Overview were placeholders for a release, and the thing
// that made them safe to ship as placeholders — a sentence saying so — is
// exactly what makes a regression invisible: a tab that silently renders
// nothing looks like a tab with nothing on it. These pin the two facts a
// reader acts on. That a section WITH rows draws them, and that a section the
// grant withheld says so rather than reading as empty.

// This suite mounts several trees into one document. Without cleanup the
// second assertion reads the first render's DOM.
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

// The one deal every fixture below seats this contact on: `deal_roles` names
// only its title, its stage and the seat, so a deal card fetches the rest
// through the route the default stub below routes to it.
const DEAL_D1: Deal = {
  id: "d-1",
  name: "Fleet renewal 2026",
  pipeline_id: "pl-1",
  stage_id: "s-1",
  status: "open",
  source: "manual",
  captured_by: "human:u-1",
  created_at: "2026-06-01T08:00:00Z",
  updated_at: "2026-08-01T08:00:00Z",
};

// The session probe, routed with no object grants: the ordinary native seat
// this tab is drawn for. Unrouted it is refused, a refused session reads as a
// malformed one, and the tab draws the branch a denied grant produces — which
// a case wins alone and loses under load, on a different name each run.
//
// The stub's fallback answers an empty page, which is also what a narrowed
// read gets here: a kind dial that is a SERVER parameter makes the list its
// own request rather than the 360's seeded page.
beforeEach(() => {
  stubWithSession({ "GET /deals/d-1": () => jsonResponse(DEAL_D1) }, {});
});

function withProviders(node: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <LocaleProvider initial="en">{node}</LocaleProvider>
    </QueryClientProvider>,
  );
}

const view: Contact360 = {
  as_of: "2026-08-13T09:00:00Z",
  contact: { id: "p-1", full_name: "Dana Buyer", ...CAPTURED },
  sections_omitted: [],
  activities: {
    data: [
      activity({
        id: "a-1",
        kind: "email",
        subject: "Fleet renewal",
        direction: "inbound",
        occurred_at: "2026-08-11T12:00:00Z",
      }),
      activity({
        id: "a-2",
        kind: "meeting",
        subject: "Depot walkthrough",
        occurred_at: "2026-08-09T08:00:00Z",
      }),
    ],
    page: { has_more: false },
  },
  deal_roles: {
    data: [
      {
        relationship_id: "r-1",
        deal_id: "d-1",
        deal_title: "Fleet renewal 2026",
        deal_stage: "Proposal",
        role: "economic_buyer",
      },
    ],
    page: { has_more: false },
  },
  next_meeting: {
    activity_id: "a-9",
    starts_at: "2026-08-20T13:00:00Z",
    subject: "Contract review",
    participants: [{ contact_id: "p-1", full_name: "Dana Buyer" }],
  },
};

// The same record read by someone whose grant reaches none of it: the sections
// are absent AND named, which is what separates "you may not see this" from
// "there is none".
const withheld: Contact360 = {
  ...view,
  activities: undefined,
  deal_roles: undefined,
  next_meeting: undefined,
  sections_omitted: ["activities", "deal_roles", "next_meeting"],
};

describe("the timeline tab", () => {
  it("draws the exchanges the 360 carried", () => {
    withProviders(<ContactTimelineTab contactId="p-1" view={view} />);
    expect(screen.getByText("Fleet renewal")).toBeTruthy();
  });

  it("opens on All, the conversation", () => {
    withProviders(<ContactTimelineTab contactId="p-1" view={view} />);
    const pressed = (name: string) =>
      screen.getByRole("button", { name }).getAttribute("aria-pressed");
    expect(pressed("All")).toBe("true");
    expect(pressed("Changes")).toBe("false");
  });

  // The cuts stand in the card's head, as on every History card. The search
  // row is a second dial a reader opens, so it waits under the head.
  it("puts the cuts in the head and opens the search row under it", async () => {
    const user = userEvent.setup();
    const { container } = withProviders(
      <ContactTimelineTab contactId="p-1" view={view} />,
    );
    const cuts = screen.getByRole("button", { name: "All" });
    expect(cuts.closest(".panel-head")).not.toBeNull();
    expect(container.querySelector(".timeline-filters")).toBeNull();

    await user.click(
      screen.getByRole("button", { name: "Search this timeline" }),
    );

    expect(
      container.querySelector(".panel-body .timeline-filters"),
    ).not.toBeNull();
  });

  // Changes is the one cut with no exchanges to narrow, so the filter row
  // stands down — and the cuts themselves must not go with it, or a reader who
  // picked Changes has no way back to the rest of the chronology.
  it("keeps the cuts when the narrowing row stands down", async () => {
    const user = userEvent.setup();
    withProviders(<ContactTimelineTab contactId="p-1" view={view} />);

    await user.click(screen.getByRole("button", { name: "Changes" }));

    expect(
      screen.getByRole("button", { name: "All" }).getAttribute("aria-pressed"),
    ).toBe("false");
    expect(
      screen.queryByRole("button", { name: "Search this timeline" }),
    ).toBeNull();
  });

  it("says the section is withheld rather than drawing it empty", () => {
    withProviders(<ContactTimelineTab contactId="p-1" view={withheld} />);
    expect(screen.queryByText(/Nothing has been logged/)).toBeNull();
    expect(screen.getByText("Hidden for your role")).toBeTruthy();
  });
});

describe("the deals tab", () => {
  it("names every deal the contact is recorded on, with their seat", () => {
    withProviders(<ContactDealsTab view={view} />);
    expect(screen.getByText("Fleet renewal 2026")).toBeTruthy();
    expect(screen.getByText("Proposal")).toBeTruthy();
    expect(screen.getByText("Economic buyer")).toBeTruthy();
  });

  // The card draws its money, its close date and its company off the deal
  // itself: `deal_roles` names neither. The committee used to live a second
  // time in a panel of its own below this list, read off `commercial.deal`;
  // it is folded into the matching card instead, so the panel is gone rather
  // than repeating a deal this list already names.
  it("draws the deal's money, close date and company on its card, and folds the committee in", async () => {
    stubWithSession(
      {
        "GET /deals/d-1": () =>
          jsonResponse({
            ...DEAL_D1,
            amount_minor: 42_000_000,
            currency: "EUR",
            expected_close_date: "2026-09-30",
            company_id: "co-1",
          } satisfies Deal),
        "GET /companies/co-1": () =>
          jsonResponse({
            id: "co-1",
            display_name: "straight. GmbH",
            source: "manual",
            captured_by: "human:u-1",
            created_at: "2026-06-01T08:00:00Z",
            updated_at: "2026-06-01T08:00:00Z",
          } satisfies Company),
      },
      {},
    );
    const withCommercial: Contact360 = {
      ...view,
      commercial: {
        deal: { deal_id: "d-1", title: "Fleet renewal 2026" },
        role: "economic_buyer",
        committee: [
          { contact_id: "c-2", full_name: "Sam Ops", role: "champion" },
        ],
      },
    };
    withProviders(<ContactDealsTab view={withCommercial} />);
    expect(await screen.findByText(/€420k/i)).toBeTruthy();
    expect(screen.getByText(/closes/i)).toBeTruthy();
    expect(screen.getByText(/straight\. GmbH/)).toBeTruthy();
    expect(screen.getByText("Sam Ops")).toBeTruthy();
    expect(screen.getByText("Champion")).toBeTruthy();
    expect(
      screen.queryByRole("heading", { name: "Open deal and buying role" }),
    ).toBeNull();
  });

  // A seat may record no role — the schema requires one only for
  // billing_contact — so the row shows the name with nothing where the role
  // would be, rather than a word the data does not support.
  it("names a committee member whose seat records no role", () => {
    const noRole: Contact360 = {
      ...view,
      commercial: {
        deal: { deal_id: "d-1", title: "Fleet renewal 2026" },
        role: null,
        committee: [
          { contact_id: "c-2", full_name: "Sacha Brandt", role: "" },
          { contact_id: "c-3", full_name: "Rae Toft", role: "champion" },
        ],
      },
    };
    withProviders(<ContactDealsTab view={noRole} />);
    // The row carries the name and NO role caption — not a word standing in for
    // one, which is what a reader would have to trust otherwise.
    const row = screen
      .getByText("Sacha Brandt")
      .closest(".pe-deal-card-committee-row");
    expect(row).toBeTruthy();
    expect(row?.querySelector(".t-caption")).toBeNull();
    // The roled seat beside it still reads: one roleless seat took the whole
    // list down before, so this says the list survived rather than emptied.
    expect(screen.getByText("Rae Toft")).toBeTruthy();
    expect(screen.getByText("Champion")).toBeTruthy();
  });

  it("does not report an absent grant as an absence of deals", () => {
    withProviders(<ContactDealsTab view={withheld} />);
    expect(screen.queryByText(/not recorded on any deal/)).toBeNull();
  });

  // THE TAB'S OWN VERB. A reader asking which deals this contact is on is the
  // reader who wants to put them on one, and before this the only path was the
  // relationships tab — a different question, reached from a different place.
  it("offers the verb that seats this contact on a deal", async () => {
    const searched: string[] = [];
    const page = (rows: { id: string; name: string }[]) =>
      new Response(
        JSON.stringify({
          data: rows,
          page: { has_more: false, next_cursor: null },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    stubWithSession(
      {
        "GET /deals": () => {
          searched.push("deals");
          // A name NOT already on the tab. "Fleet renewal 2026" is the row the
          // page draws, so waiting for it would pass on the rendering the
          // picker had nothing to do with.
          return page([{ id: "d-9", name: "Depot expansion 2027" }]);
        },
        "GET /companies": () => {
          searched.push("companies");
          return page([]);
        },
      },
      { relationship: ["create"] },
    );
    withProviders(<ContactDealsTab view={view} />);

    const add = await screen.findByRole("button", { name: "Add to a deal" });
    await userEvent.click(add);

    expect(screen.getByRole("heading", { name: "Add to a deal" })).toBeTruthy();

    // NARROWED TO THE DEAL EDGE, asserted on what the picker SEARCHES.
    //
    // Two weaker assertions were tried and both passed with the narrowing
    // removed: the kind selector is hidden either way, and no option label is
    // rendered when it is hidden. What the narrowing actually decides is which
    // endpoint the dialog picks first — unnarrowed, a contact scope leads with
    // `employment` and the picker searches COMPANIES, so a reader typing a deal
    // name would find nothing and never learn why.
    await userEvent.type(
      screen.getByRole("searchbox", { name: /search/i }),
      "Depot",
    );
    await waitFor(
      () => {
        expect(screen.getByText("Depot expansion 2027")).toBeTruthy();
      },
      { timeout: 2000 },
    );
    expect(searched).toContain("deals");
    expect(searched).not.toContain("companies");
  });

  // Withheld rather than disabled: a grant the role does not hold is not a fact
  // about this contact, so there is nothing for a disabled button to explain.
  it("withholds the verb from a reader who may not write an edge", async () => {
    stubWithSession({}, {});
    withProviders(<ContactDealsTab view={view} />);

    // Awaited through the rows, so the absence is read AFTER the grant probe
    // has answered rather than before it has run.
    await screen.findByText("Fleet renewal 2026");
    expect(screen.queryByRole("button", { name: "Add to a deal" })).toBeNull();
  });

  // The contact's own door into the rooms they sit in: an admin removing
  // somebody who left the buyer knows the name, not the deals.
  const withAddress: Contact360 = {
    ...view,
    contact: {
      ...view.contact,
      emails: [
        {
          id: "e-1",
          email: "dana@buyer.example",
          email_type: "work",
          is_primary: true,
          position: 0,
          source: "manual",
          captured_by: "human:u-1",
        },
      ],
    },
  };
  const ROOM = {
    id: "room-1",
    deal_id: "d-1",
    title: "Fleet renewal room",
    state: "live",
    ...CAPTURED,
  } satisfies components["schemas"]["DealRoom"];

  it("lists the Deal Rooms the contact can still enter, and opens one", async () => {
    stubWithSession(
      {
        "GET /deals/d-1": () => jsonResponse(DEAL_D1),
        "GET /deal-rooms": () =>
          jsonResponse({
            data: [ROOM],
            page: { has_more: false, next_cursor: null },
          }),
      },
      { deal_room: ["read"] },
    );
    const user = userEvent.setup();
    withProviders(<ContactDealsTab view={withAddress} />);

    const panel = (await screen.findByText("Fleet renewal room")).closest(
      ".panel",
    );
    expect(panel).not.toBeNull();
    await user.click(
      within(panel as HTMLElement).getByRole("button", { name: "Open" }),
    );
    await waitFor(() => expect(window.location.hash).toBe("#/deals/d-1/room"));
  });

  it("asks for no rooms from a reader without the room grant", async () => {
    let asked = false;
    stubWithSession(
      {
        "GET /deals/d-1": () => jsonResponse(DEAL_D1),
        "GET /deal-rooms": () => {
          asked = true;
          return jsonResponse({ data: [ROOM], page: { has_more: false } });
        },
      },
      { relationship: ["create"] },
    );
    withProviders(<ContactDealsTab view={withAddress} />);

    // Awaited through a control the grant probe draws, so the absence below
    // is read after the probe has answered rather than before it ran.
    await screen.findByRole("button", { name: "Add to a deal" });
    expect(screen.queryByText("Fleet renewal room")).toBeNull();
    expect(asked).toBe(false);
  });
});

describe("the meetings tab", () => {
  const held = activity({
    id: "a-2",
    kind: "meeting",
    subject: "Depot walkthrough",
    occurred_at: "2026-08-09T08:00:00Z",
  });

  // The tab lists the contact's meetings from their own read, narrowed on the
  // server, rather than from the 360's first page of every kind.
  function serveMeetings(rows: SectionActivity[]) {
    stubWithSession(
      {
        "GET /activities": () =>
          jsonResponse({ data: rows, page: { has_more: false } }),
      },
      {},
    );
  }

  it("puts the booked meeting above the ones already held", async () => {
    serveMeetings([held]);
    withProviders(<ContactMeetingsTab view={view} />);
    const booked = screen.getByText("Contract review");
    const past = await screen.findByText("Depot walkthrough");
    expect(
      booked.compareDocumentPosition(past) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("asks the server for this contact's meetings alone", async () => {
    // The email in the same 360 page belongs to the Activity tab. A list that
    // let it through here would make this tab a second, worse spelling of
    // that one.
    serveMeetings([held]);
    const requests = vi.spyOn(globalThis, "fetch");
    withProviders(<ContactMeetingsTab view={view} />);
    await screen.findByText("Depot walkthrough");
    const asked = requests.mock.calls
      .map(([input, init]) =>
        input instanceof Request ? input : new Request(input, init),
      )
      .map((request) => new URL(request.url))
      .find((url) => url.pathname.endsWith("/activities"));
    expect(asked?.searchParams.get("kind")).toBe("meeting");
    expect(asked?.searchParams.get("entity_type")).toBe("contact");
    expect(asked?.searchParams.get("entity_id")).toBe("p-1");
    expect(screen.queryByText("Fleet renewal")).toBeNull();
  });

  it("files a meeting booked after the next one under Upcoming, not Held", async () => {
    // Only the soonest booking arrives as next_meeting. A second one further
    // out is still ahead of the read, and listing it as held told the reader
    // a meeting had happened that had not.
    serveMeetings([
      activity({
        id: "a-10",
        kind: "meeting",
        subject: "Rollout planning",
        occurred_at: "2026-08-27T13:00:00Z",
      }),
      held,
    ]);
    withProviders(<ContactMeetingsTab view={view} />);
    const later = await screen.findByText("Rollout planning");
    const upcoming = screen.getByRole("region", { name: "Upcoming" });
    expect(upcoming.contains(later)).toBe(true);
    const past = screen.getByRole("region", { name: "Held" });
    expect(within(past).queryByText("Rollout planning")).toBeNull();
    expect(within(past).getByText("Depot walkthrough")).toBeTruthy();
  });

  it("offers a brief for the booked meeting and for one already held", async () => {
    // The backend assembles a brief for ANY meeting activity. Reaching it only
    // through the next meeting's prep moment left every other meeting on the
    // record with a brief nothing could ask for.
    serveMeetings([held]);
    const briefed: string[] = [];
    withProviders(
      <ContactMeetingsTab
        view={view}
        onBriefMeeting={(id) => briefed.push(id)}
      />,
    );
    await screen.findByText("Depot walkthrough");
    const actions = screen.getAllByRole("button", { name: "Prepare brief" });
    expect(actions.length).toBe(2);
    await userEvent.setup().click(actions[0]);
    expect(briefed.length).toBe(1);
  });

  it("offers no brief for a meeting the reader may find but not read", async () => {
    // The timeline carries discoverable-but-withheld rows on purpose, so the
    // reader knows a conversation happened. The brief endpoint applies the
    // stricter content gate, so a verb here would promise what their own grant
    // refuses — and answer 404 when they took it up.
    serveMeetings([
      activity({
        id: "a-secret",
        kind: "meeting",
        subject: "Board session",
        occurred_at: "2026-08-10T08:00:00Z",
        content_state: "withheld",
      }),
    ]);
    withProviders(
      <ContactMeetingsTab
        view={{ ...view, next_meeting: undefined }}
        onBriefMeeting={() => {}}
      />,
    );
    // The row is DRAWN — the reader learns a meeting happened — and carries no
    // verb. Its subject is redacted, which is the whole point of the state.
    expect(
      await screen.findByText("Content for participants only"),
    ).toBeTruthy();
    expect(screen.queryByText("Board session")).toBeNull();
    expect(screen.queryByRole("button", { name: "Prepare brief" })).toBeNull();
  });

  it("offers no brief when the surface cannot open one", async () => {
    // Without the callback the verb would be a button that does nothing, which
    // teaches a reader the feature is broken rather than absent.
    serveMeetings([held]);
    withProviders(<ContactMeetingsTab view={view} />);
    await screen.findByText("Depot walkthrough");
    expect(screen.queryByRole("button", { name: "Prepare brief" })).toBeNull();
  });

  it("names the meeting the reader picked, not the soonest one", async () => {
    // The defect this replaces: the drawer always read next_meeting, so a
    // brief opened from a past meeting described a different room.
    serveMeetings([held]);
    const briefed: string[] = [];
    withProviders(
      <ContactMeetingsTab
        view={view}
        onBriefMeeting={(id) => briefed.push(id)}
      />,
    );
    await screen.findByText("Depot walkthrough");
    const actions = screen.getAllByRole("button", { name: "Prepare brief" });
    // The booked meeting leads the tab; the held one follows it.
    await userEvent.setup().click(actions[1]);
    expect(briefed).toEqual(["a-2"]);
  });

  it("draws the prep note and its agenda verb on the booked meeting", async () => {
    // The rung fires off the next meeting alone: a moment about a different
    // claim (re-engagement, an overdue promise) has no meeting to sit on.
    const draftAgenda: components["schemas"]["ContactMomentAction"] = {
      kind: "draft_reply",
      label: "Draft agenda",
      state: "will_confirm",
      destination: { surface: "composer", prefill: { intent: "agenda" } },
    };
    const withMoment: Contact360 = {
      ...view,
      moment: {
        claim_key: "moment:meeting_prep",
        evidence_fingerprint: "fp-1",
        rule: "meeting_prep",
        headline: "Prepare for Contract review",
        why_now: "Prepare before the meeting, not after.",
        confidence: "observed_fact",
        evidence: [],
        recommended_action: {
          kind: "open_meeting_brief",
          label: "Open meeting brief",
          state: "available",
        },
        secondary_actions: [draftAgenda],
      },
    };
    const acted: components["schemas"]["ContactMomentAction"][] = [];
    withProviders(
      <ContactMeetingsTab
        view={withMoment}
        onBriefMeeting={() => {}}
        onAction={(action) => acted.push(action)}
      />,
    );
    expect(screen.getByText("Upcoming")).toBeTruthy();
    expect(screen.getByText(/Prepare before the meeting/)).toBeTruthy();
    const draft = screen.getByRole("button", { name: "Draft agenda" });
    await userEvent.setup().click(draft);
    expect(acted).toEqual([draftAgenda]);
  });
});
