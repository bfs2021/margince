// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

/** @vitest-environment happy-dom */
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { LocaleProvider } from "../i18n";
import { DateInput } from "./dateinput";
import { refuseUnreadableField } from "./formvalidity";

afterEach(cleanup);

function InlineEditor({
  name,
  onSave,
}: Readonly<{ name: string; onSave: (refused: boolean) => void }>) {
  return (
    <section>
      <DateInput aria-label={name} defaultValue="2026-05-04" />
      <div className="form-actions">
        <button
          type="button"
          onClick={(event) =>
            onSave(refuseUnreadableField(event.currentTarget))
          }
        >
          {`Save ${name}`}
        </button>
      </div>
    </section>
  );
}

it("reads the box around a save outside any form, and only that box", async () => {
  const onSave = vi.fn();
  render(
    <LocaleProvider initial="en">
      <InlineEditor name="Due" onSave={onSave} />
      <InlineEditor name="Start" onSave={onSave} />
    </LocaleProvider>,
  );
  const user = userEvent.setup();
  const due = screen.getByRole("textbox", { name: "Due" });
  await user.clear(due);
  await user.type(due, "31/02/2027");

  await user.click(screen.getByRole("button", { name: "Save Start" }));
  await user.click(screen.getByRole("button", { name: "Save Due" }));

  expect(onSave.mock.calls).toEqual([[false], [true]]);
});

it("reads every field of the form it is handed", async () => {
  const onSubmit = vi.fn();
  render(
    <LocaleProvider initial="en">
      <form
        onSubmit={(event) => {
          event.preventDefault();
          onSubmit(refuseUnreadableField(event.currentTarget));
        }}
        noValidate
      >
        <fieldset>
          <DateInput aria-label="Signed" />
        </fieldset>
        <button type="submit">Save</button>
      </form>
    </LocaleProvider>,
  );
  const user = userEvent.setup();
  await user.type(screen.getByRole("textbox", { name: "Signed" }), "99/99");

  await user.click(screen.getByRole("button", { name: "Save" }));

  expect(onSubmit).toHaveBeenCalledWith(true);
});
