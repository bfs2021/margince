/** @vitest-environment happy-dom */
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, describe, expect, it } from "vitest";
import type { TimelineFilters } from "../../design-system/recordtimeline";
import { LocaleProvider } from "../../i18n";
import { useHistoryDials } from "./historydials";
import type { TimelineFilter } from "./recordchronology";

afterEach(() => {
  cleanup();
});

// The dials over a History card, with the state they write printed back so a
// test reads what the page would send.
function Harness({ initial = {} }: Readonly<{ initial?: TimelineFilters }>) {
  const [filter, setFilter] = useState<TimelineFilter>("all");
  const [filters, setFilters] = useState<TimelineFilters>(initial);
  const dials = useHistoryDials({
    filter,
    onFilter: setFilter,
    filters,
    onFilters: setFilters,
  });
  return (
    <>
      {dials.cuts}
      {dials.narrowing}
      <output data-testid="state">
        {JSON.stringify({ filter, ...filters })}
      </output>
    </>
  );
}

function mount(initial?: TimelineFilters) {
  render(
    <LocaleProvider initial="en">
      <Harness initial={initial} />
    </LocaleProvider>,
  );
}

const state = () => JSON.parse(screen.getByTestId("state").textContent ?? "");

describe("a History card's dials", () => {
  it("narrows the exchanges to one kind and back to the whole record", async () => {
    const user = userEvent.setup();
    mount();

    await user.click(screen.getByRole("button", { name: "Meetings" }));
    expect(state()).toEqual({ filter: "all", kind: "meeting" });

    await user.click(screen.getByRole("button", { name: "All" }));
    expect(state()).toEqual({ filter: "all" });
  });

  it("opens Changes as the other feed, and hides the search there", async () => {
    const user = userEvent.setup();
    mount();

    await user.click(screen.getByRole("button", { name: "Changes" }));

    expect(state().filter).toBe("changes");
    expect(
      screen.queryByRole("button", { name: "Search this timeline" }),
    ).toBeNull();
  });

  // A search left set keeps its row in view. Closing the row takes the search
  // back, since a hidden filter would narrow the list with nothing showing.
  it("keeps a running search in view and clears it when closed", async () => {
    const user = userEvent.setup();
    mount({ kind: "email", q: "renewal" });
    expect(screen.getByRole("searchbox")).toBeTruthy();

    await user.click(
      screen.getByRole("button", { name: "Search this timeline" }),
    );

    expect(state()).toEqual({ filter: "all", kind: "email" });
    expect(screen.queryByRole("searchbox")).toBeNull();
  });
});
