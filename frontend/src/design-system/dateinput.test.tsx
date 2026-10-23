/** @vitest-environment happy-dom */
// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { type ReactNode, useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { monthAndYear, weekdayInitial } from "../format/format";
import { setDateTimePreferences } from "../format/preferences";
import { type Locale, LocaleProvider } from "../i18n";
import { Button } from "./button";
import { DateInput, type DateInputProps, type ISODate } from "./dateinput";
import { Field } from "./field";
import { Heading } from "./heading";
import { Modal } from "./modal";

afterEach(() => {
  cleanup();
  setDateTimePreferences({ dateFormat: "locale", timeFormat: "locale" });
});

const TODAY = "2026-05-20";

function Due(
  props: Readonly<
    Omit<DateInputProps, "value" | "onChange"> & {
      locale?: Locale;
      initial?: ISODate | "";
      onReport?: (day: string) => void;
    }
  >,
) {
  const { locale = "en", initial = "", onReport, ...rest } = props;
  const [day, setDay] = useState<ISODate | "">(initial);
  return (
    <LocaleProvider initial={locale}>
      <Field label="Due">
        {(control) => (
          <DateInput
            {...control}
            {...rest}
            today={TODAY}
            value={day}
            onChange={(event) => {
              setDay(event.target.value);
              onReport?.(event.target.value);
            }}
          />
        )}
      </Field>
    </LocaleProvider>
  );
}

const box = () =>
  screen.getByRole<HTMLInputElement>("textbox", { name: "Due" });

describe("the face", () => {
  it("writes the day the Vietnamese way and opens a Vietnamese month that starts on Monday", async () => {
    const user = userEvent.setup();
    render(<Due locale="vi" initial="2026-05-06" />);

    expect(box().value).toBe("06/05/2026");
    expect(box().placeholder).toBe("dd/mm/yyyy");

    await user.click(screen.getByRole("button", { name: "Chọn ngày" }));

    const dialog = await screen.findByRole("dialog", { name: "Chọn ngày" });
    const heading = monthAndYear(new Date(2026, 4, 1), "vi");
    expect(heading).toMatch(/tháng/i);
    expect(within(dialog).getByText(heading)).toBeTruthy();
    const legend = dialog.querySelector(".calendar-grid > span");
    expect(legend?.textContent).toBe(
      weekdayInitial(new Date(2026, 4, 4), "vi"),
    );
    expect(
      within(dialog).getByRole("button", { name: "Hôm nay" }),
    ).toBeTruthy();
    expect(within(dialog).getByRole("button", { name: "Xóa" })).toBeTruthy();
  });

  it("follows the installation's month-first format over the UI language", () => {
    setDateTimePreferences({ dateFormat: "mdy", timeFormat: "locale" });
    render(<Due locale="de" initial="2026-05-06" />);

    expect(box().value).toBe("05/06/2026");
  });

  it("submits the ISO day under the caller's name", () => {
    const { container } = render(<Due initial="2026-05-06" name="due" />);

    const sent = container.querySelector<HTMLInputElement>('input[name="due"]');
    expect(sent?.value).toBe("2026-05-06");
    expect(box().name).toBe("");
  });
});

describe("typing", () => {
  it("reports a German day.month.year once typed, and writes it in the pattern on blur", async () => {
    const user = userEvent.setup();
    const report = vi.fn();
    render(<Due locale="de" onReport={report} />);

    await user.type(box(), "6.5.202");
    expect(report).not.toHaveBeenCalled();
    await user.type(box(), "6");
    expect(report).toHaveBeenCalledWith("2026-05-06");
    expect(box().value).toBe("6.5.2026");
    await user.tab();

    expect(report).toHaveBeenCalledTimes(1);
    expect(box().value).toBe("06.05.2026");
  });

  it("reads month first under the mdy preference, on Enter", async () => {
    setDateTimePreferences({ dateFormat: "mdy", timeFormat: "locale" });
    const user = userEvent.setup();
    const report = vi.fn();
    render(<Due onReport={report} />);

    await user.type(box(), "5/6/2026{Enter}");

    expect(report).toHaveBeenCalledWith("2026-05-06");
  });

  it("keeps a day it cannot read, marks it invalid, and leaves the caller no day", async () => {
    const user = userEvent.setup();
    const report = vi.fn();
    render(<Due initial="2026-05-06" onReport={report} />);

    await user.tripleClick(box());
    await user.keyboard("31/02/2026");
    await user.tab();

    expect(report).toHaveBeenLastCalledWith("");
    expect(report).not.toHaveBeenCalledWith("2026-05-06");
    expect(box().value).toBe("31/02/2026");
    expect(box().getAttribute("aria-invalid")).toBe("true");
    expect(box().validationMessage).toBe("Enter a date as DD/MM/YYYY.");
  });

  it("reports an emptied box as no date", async () => {
    const user = userEvent.setup();
    const report = vi.fn();
    render(<Due initial="2026-05-06" onReport={report} />);

    await user.clear(box());
    await user.tab();

    expect(report).toHaveBeenCalledWith("");
  });

  it("refuses a typed day outside min and max", async () => {
    const user = userEvent.setup();
    const report = vi.fn();
    render(<Due min="2026-05-10" max="2026-05-30" onReport={report} />);

    await user.type(box(), "09/05/2026");
    await user.tab();

    expect(report).not.toHaveBeenCalled();
    expect(box().getAttribute("aria-invalid")).toBe("true");
    expect(box().validationMessage).toBe("Earliest date: 10/05/2026");
  });
});

// As callers that act on a day write it: an emptied box asks for nothing.
function KeepsItsDay(props: Readonly<{ onReport: (day: string) => void }>) {
  const [day, setDay] = useState<ISODate | "">("2026-05-06");
  return (
    <LocaleProvider initial="en">
      <Field label="Due">
        {(control) => (
          <DateInput
            {...control}
            today={TODAY}
            value={day}
            onChange={(event) => {
              props.onReport(event.target.value);
              if (event.target.value) {
                setDay(event.target.value);
              }
            }}
          />
        )}
      </Field>
    </LocaleProvider>
  );
}

describe("a caller that refuses an empty box", () => {
  it("lets the reader empty the box and type a new day", async () => {
    const user = userEvent.setup();
    const report = vi.fn();
    render(<KeepsItsDay onReport={report} />);

    await user.click(box());
    await user.keyboard("{End}{Backspace>10/}");
    expect(box().value).toBe("");
    await user.keyboard("07/05/2026");

    expect(report).toHaveBeenLastCalledWith("2026-05-07");
    expect(box().value).toBe("07/05/2026");
  });

  it("shows its day again when the box is left empty", async () => {
    const user = userEvent.setup();
    render(<KeepsItsDay onReport={() => {}} />);

    await user.clear(box());
    await user.tab();

    expect(box().value).toBe("06/05/2026");
  });
});

// The shape the app's own forms take: state in the screen, posted on submit.
function PostsOnSubmit(
  props: Readonly<{ noValidate?: boolean; onPost: (day: string) => void }>,
) {
  const [day, setDay] = useState<ISODate | "">("2026-05-06");
  return (
    <LocaleProvider initial="en">
      <form
        noValidate={props.noValidate}
        onSubmit={(event) => {
          event.preventDefault();
          props.onPost(day);
        }}
      >
        <Field label="Due">
          {(control) => (
            <DateInput
              {...control}
              today={TODAY}
              value={day}
              onChange={(event) => setDay(event.target.value)}
            />
          )}
        </Field>
        <Button type="submit">Save</Button>
      </form>
    </LocaleProvider>
  );
}

describe("a form around a day it cannot read", () => {
  it("is invalid by the time the caller's own blur reads it", async () => {
    const user = userEvent.setup();
    const seen = vi.fn();
    render(
      <Due
        initial="2026-05-06"
        onBlur={(event) => seen(event.currentTarget.checkValidity())}
      />,
    );

    await user.tripleClick(box());
    await user.keyboard("31/02/2026");
    await user.tab();

    expect(seen).toHaveBeenCalledWith(false);
  });

  it("is refused by the browser's own check", async () => {
    const user = userEvent.setup();
    const post = vi.fn();
    render(<PostsOnSubmit onPost={post} />);

    await user.tripleClick(box());
    await user.keyboard("31/02/2026");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(post).not.toHaveBeenCalled();
  });

  it("posts no day, never the earlier one, when the form skips that check", async () => {
    const user = userEvent.setup();
    const post = vi.fn();
    render(<PostsOnSubmit noValidate onPost={post} />);

    await user.tripleClick(box());
    await user.keyboard("31/02/2026{Enter}");

    expect(post).toHaveBeenCalledWith("");
  });
});

// The boxes a real layout gives: a field ending at the page gutter of a phone,
// and the calendar's own width. Happy-dom lays nothing out.
function laidOut(field: Readonly<{ left: number; right: number }>) {
  vi.stubGlobal("innerWidth", 390);
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(
    function (this: HTMLElement) {
      return this.classList.contains("date-input")
        ? new DOMRect(field.left, 300, field.right - field.left, 40)
        : new DOMRect(0, 0, 0, 0);
    },
  );
  vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockImplementation(
    function (this: HTMLElement) {
      return this.classList.contains("date-popup") ? 274 : 0;
    },
  );
}

describe("where the calendar opens", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("hangs off the end of a field in the right-hand column, inside the screen", async () => {
    laidOut({ left: 201, right: 374 });
    const user = userEvent.setup();
    render(<Due />);

    await user.click(screen.getByRole("button", { name: "Choose date" }));
    const dialog = await screen.findByRole("dialog", { name: "Choose date" });

    expect(dialog.style.left).toBe("100px");
  });

  it("stays on the screen when the field's end is nearer the start than its width", async () => {
    laidOut({ left: 16, right: 190 });
    const user = userEvent.setup();
    render(<Due />);

    await user.click(screen.getByRole("button", { name: "Choose date" }));
    const dialog = await screen.findByRole("dialog", { name: "Choose date" });

    expect(dialog.style.left).toBe("4px");
  });
});

describe("the calendar", () => {
  it("opens on the chosen day, moves with the arrows and picks with Enter", async () => {
    const user = userEvent.setup();
    const report = vi.fn();
    render(<Due initial="2026-05-06" onReport={report} />);
    const opener = screen.getByRole("button", { name: "Choose date" });
    expect(opener.getAttribute("aria-haspopup")).toBe("dialog");
    expect(opener.getAttribute("aria-expanded")).toBe("false");

    await user.click(opener);
    const dialog = await screen.findByRole("dialog", { name: "Choose date" });
    expect(opener.getAttribute("aria-expanded")).toBe("true");
    expect(document.activeElement?.getAttribute("data-day")).toBe("2026-05-06");

    await user.keyboard("{ArrowRight}{Enter}");

    expect(report).toHaveBeenCalledWith("2026-05-07");
    expect(dialog.isConnected).toBe(false);
    expect(document.activeElement).toBe(opener);
    expect(box().value).toBe("07/05/2026");
  });

  it("opens on today with nothing chosen, and Escape closes it back onto the button", async () => {
    const user = userEvent.setup();
    const report = vi.fn();
    render(<Due onReport={report} />);
    const opener = screen.getByRole("button", { name: "Choose date" });

    await user.click(opener);
    await screen.findByRole("dialog", { name: "Choose date" });
    expect(document.activeElement?.getAttribute("data-day")).toBe("2026-05-20");

    await user.keyboard("{Escape}");

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(opener);
    expect(report).not.toHaveBeenCalled();
  });

  it("keeps Tab inside its own controls", async () => {
    const user = userEvent.setup();
    render(<Due initial="2026-05-06" />);

    await user.click(screen.getByRole("button", { name: "Choose date" }));
    const dialog = await screen.findByRole("dialog");
    await user.tab();
    expect(document.activeElement?.textContent).toBe("Today");
    await user.tab();
    await user.tab();

    expect(document.activeElement?.getAttribute("aria-label")).toBe(
      "Previous month",
    );
    expect(dialog.contains(document.activeElement)).toBe(true);
  });

  it("refuses the days outside min and max, with the reason in their name", async () => {
    const user = userEvent.setup();
    const report = vi.fn();
    render(<Due min="2026-05-10" max="2026-05-30" onReport={report} />);

    await user.click(screen.getByRole("button", { name: "Choose date" }));
    const early = await screen.findByRole("button", {
      name: "Saturday, 9 May 2026, Earliest date: 10/05/2026",
    });
    await user.click(early);

    expect(early.getAttribute("aria-disabled")).toBe("true");
    expect(report).not.toHaveBeenCalled();
    await user.click(
      screen.getByRole("button", { name: "Sunday, 10 May 2026" }),
    );
    expect(report).toHaveBeenCalledWith("2026-05-10");
  });

  it("clears the day with Clear and picks today with Today", async () => {
    const user = userEvent.setup();
    const report = vi.fn();
    render(<Due initial="2026-05-06" onReport={report} />);
    const opener = screen.getByRole("button", { name: "Choose date" });

    await user.click(opener);
    await user.click(await screen.findByRole("button", { name: "Clear" }));
    expect(report).toHaveBeenLastCalledWith("");
    expect(box().value).toBe("");

    await user.click(opener);
    await user.click(await screen.findByRole("button", { name: "Today" }));
    expect(report).toHaveBeenLastCalledWith("2026-05-20");
  });

  it("closes alone on Escape inside a dialog, which stays open", async () => {
    const user = userEvent.setup();
    const closeDialog = vi.fn();
    render(
      <InDialog onClose={closeDialog}>
        <Due />
      </InDialog>,
    );

    await user.click(screen.getByRole("button", { name: "Choose date" }));
    await screen.findByRole("dialog", { name: "Choose date" });
    await user.keyboard("{Escape}");

    expect(screen.queryByRole("dialog", { name: "Choose date" })).toBeNull();
    expect(closeDialog).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "Log activity" })).toBeTruthy();
  });

  it("does not open while the field is disabled or read-only", () => {
    render(
      <>
        <Due disabled />
        <Due readOnly />
      </>,
    );

    for (const opener of screen.getAllByRole("button", {
      name: "Choose date",
    })) {
      expect(opener).toHaveProperty("disabled", true);
    }
  });
});

function InDialog(
  props: Readonly<{ onClose: () => void; children: ReactNode }>,
) {
  return (
    <>
      <Button>Behind</Button>
      <Modal open onClose={props.onClose} labelledBy="log" intent="form">
        <Heading size="large" id="log">
          Log activity
        </Heading>
        {props.children}
      </Modal>
    </>
  );
}
