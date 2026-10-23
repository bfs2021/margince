/** @vitest-environment happy-dom */
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { weekdayInitial } from "../format/format";
import { LocaleProvider } from "../i18n";
import { Calendar, isoDay, monthGrid } from "./calendar";

afterEach(cleanup);

const AUGUST_2026 = new Date(2026, 7, 1);
const TODAY = new Date(2026, 7, 24);
const MONDAY = 1;

// Six weeks whatever the month: a grid that shrank to five would move every
// control under it as the reader pages.
it("draws six weeks for a month that opens on a Saturday and one that does not", () => {
  expect(monthGrid(new Date(2026, 7, 1), MONDAY)).toHaveLength(42);
  expect(monthGrid(new Date(2026, 1, 1), MONDAY)).toHaveLength(42);
});

it("opens the grid on the week start before the first of the month", () => {
  // 1 August 2026 is a Saturday, so a Monday week opens on 27 July.
  expect(isoDay(monthGrid(AUGUST_2026, MONDAY)[0])).toBe("2026-07-27");
  // 1 June 2026 is a Monday, so the grid opens on the 1st itself.
  expect(isoDay(monthGrid(new Date(2026, 5, 1), MONDAY)[0])).toBe("2026-06-01");
});

it.each(["en", "de", "vi"] as const)(
  "starts the week on Monday in %s, legend and days alike",
  (locale) => {
    const { container } = render(
      <LocaleProvider initial={locale}>
        <Calendar
          month={AUGUST_2026}
          onMonthChange={() => {}}
          selected=""
          onSelect={() => {}}
          today={TODAY}
          locale={locale}
        />
      </LocaleProvider>,
    );
    const cells = container.querySelectorAll(".calendar-grid > *");
    expect(cells[0]?.textContent).toBe(
      weekdayInitial(new Date(2026, 6, 27), locale),
    );
    expect(cells[7]?.getAttribute("data-day")).toBe("2026-07-27");
  },
);

it("hands back the day the reader pressed", async () => {
  const chose = vi.fn();
  render(
    <Calendar
      month={AUGUST_2026}
      onMonthChange={() => {}}
      selected=""
      onSelect={chose}
      today={TODAY}
      locale="en"
    />,
  );

  await userEvent.click(
    screen.getByRole("button", { name: "Tuesday, 25 August 2026" }),
  );

  expect(chose).toHaveBeenCalledWith("2026-08-25");
});

// The month is the caller's, so paging asks rather than moves. Stepping back
// from a 31-day month must not land in the month it started in.
it("steps a month at a time from a date no shorter month holds", async () => {
  const paged = vi.fn();
  render(
    <Calendar
      month={new Date(2026, 6, 31)}
      onMonthChange={paged}
      selected=""
      onSelect={() => {}}
      today={TODAY}
      locale="en"
    />,
  );

  await userEvent.click(screen.getByRole("button", { name: "Previous month" }));

  expect(isoDay(paged.mock.calls[0][0])).toBe("2026-06-01");
});

it("marks the chosen day and nothing else", () => {
  render(
    <Calendar
      month={AUGUST_2026}
      onMonthChange={() => {}}
      selected="2026-08-25"
      onSelect={() => {}}
      today={TODAY}
      locale="en"
    />,
  );

  const pressed = screen
    .getAllByRole("button")
    .filter((button) => button.getAttribute("aria-pressed") === "true");
  expect(pressed).toHaveLength(1);
  expect(pressed[0].textContent).toBe("25");
});

// The grid is six weeks whatever the month, so up to eleven of its forty-two
// cells belong to the month either side. Undistinguished they read as the month
// on show, which is how a reader picks the 1st of the wrong one — and the class
// is the only thing that separates them, so nothing else can fail if it goes.
it("marks the days that belong to the month either side", () => {
  render(
    <Calendar
      month={AUGUST_2026}
      onMonthChange={() => {}}
      selected=""
      onSelect={() => {}}
      today={TODAY}
      locale="en"
    />,
  );

  // 27 July, the Monday the grid opens on, is the month before.
  expect(
    screen.getByRole("button", { name: "Monday, 27 July 2026" }).className,
  ).toContain("calendar-day-outside");
  // 25 August is the month on show, and must NOT carry it — a class on every
  // day would satisfy the line above while distinguishing nothing.
  expect(
    screen.getByRole("button", { name: "Tuesday, 25 August 2026" }).className,
  ).not.toContain("calendar-day-outside");
});

// A refused day stays in the grid and cannot be chosen, while the days either
// side of it still can: the predicate is asked per day, not once per month.
it("refuses the days the caller says cannot be chosen", async () => {
  const user = userEvent.setup();
  const chose = vi.fn();
  render(
    <Calendar
      month={AUGUST_2026}
      onMonthChange={() => {}}
      selected=""
      onSelect={chose}
      today={TODAY}
      locale="en"
      refusal={(day) => (day === "2026-08-25" ? "nothing free" : undefined)}
    />,
  );

  // The reason is in the name: struck through says it only to the eye.
  const refused = screen.getByRole("button", {
    name: "Tuesday, 25 August 2026, nothing free",
  });
  expect(refused.getAttribute("aria-disabled")).toBe("true");
  await user.click(refused);
  expect(chose).not.toHaveBeenCalled();

  await user.click(
    screen.getByRole("button", { name: "Wednesday, 26 August 2026" }),
  );
  expect(chose).toHaveBeenCalledWith("2026-08-26");
});

function Paged({ onSelect }: Readonly<{ onSelect: (day: string) => void }>) {
  const [month, setMonth] = useState(AUGUST_2026);
  return (
    <Calendar
      month={month}
      onMonthChange={setMonth}
      selected="2026-08-31"
      onSelect={onSelect}
      today={TODAY}
      locale="en"
    />
  );
}

// One day is a tab stop; the arrows walk the month and page past its edges.
it("walks days, weeks, months and years from the keyboard", async () => {
  const user = userEvent.setup();
  const chose = vi.fn();
  render(<Paged onSelect={chose} />);
  const day = (name: string) => screen.getByRole("button", { name });

  expect(day("Monday, 31 August 2026").tabIndex).toBe(0);
  expect(day("Sunday, 30 August 2026").tabIndex).toBe(-1);
  day("Monday, 31 August 2026").focus();

  await user.keyboard("{ArrowRight}");
  expect(document.activeElement).toBe(day("Tuesday, 1 September 2026"));
  expect(screen.getByText("September 2026")).toBeTruthy();

  await user.keyboard("{ArrowUp}{End}");
  expect(document.activeElement).toBe(day("Sunday, 30 August 2026"));

  await user.keyboard("{Home}{PageDown}");
  expect(document.activeElement).toBe(day("Thursday, 24 September 2026"));

  await user.keyboard("{Shift>}{PageUp}{/Shift}");
  expect(document.activeElement).toBe(day("Wednesday, 24 September 2025"));

  await user.keyboard("{Enter}");
  expect(chose).toHaveBeenCalledWith("2025-09-24");
});

it("keeps a refused day reachable by the arrows and refuses its press", async () => {
  const user = userEvent.setup();
  const chose = vi.fn();
  render(
    <Calendar
      month={AUGUST_2026}
      onMonthChange={() => {}}
      selected="2026-08-24"
      onSelect={chose}
      today={TODAY}
      locale="en"
      refusal={(day) => (day === "2026-08-25" ? "nothing free" : undefined)}
    />,
  );
  screen.getByRole("button", { name: "Monday, 24 August 2026" }).focus();

  await user.keyboard("{ArrowRight}{Enter}");

  expect(document.activeElement?.getAttribute("data-day")).toBe("2026-08-25");
  expect(chose).not.toHaveBeenCalled();
});
