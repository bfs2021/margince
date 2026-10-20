// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import { useState } from "react";
import { useT } from "../i18n";
import { isImeCommit } from "../lib/imecommit";
import { Field, SearchField } from "./atoms";
import { DateInput, type ISODate, isISODate } from "./dateinput";
import type { TimelineFilters } from "./recordtimeline";
import "./timelinefilterbar.css";

/**
 * TimelineFilterBar is the ONE row of dials every record timeline narrows
 * by: a search and a date range. The kind is the History card's own cut.
 * Every dial is a server parameter (`q`, `occurred_after` /
 * `occurred_before`). The bar owns no list and filters nothing itself, so
 * "tasks on this contact" never means "tasks among the twenty newest rows".
 *
 * The search commits on Enter or when the field is left, not per keystroke:
 * each commit is a request, and a reader typing a word would otherwise fire
 * one per letter on a phone link.
 *
 * A search answers from a content-gated read (a limited conversation the
 * reader may not open is simply absent from a text match), and the bar says
 * so under the row while a search is active — silence would present the
 * matches as all there are.
 */
export function TimelineFilterBar({
  value,
  onChange,
}: Readonly<{
  value: TimelineFilters;
  onChange: (next: TimelineFilters) => void;
}>) {
  const t = useT();
  const [draft, setDraft] = useState(value.q ?? "");
  // The draft follows a filter reset from outside (a record change): a box
  // still showing the previous record's word would claim a search that is
  // not running.
  const [draftFor, setDraftFor] = useState(value.q ?? "");
  if ((value.q ?? "") !== draftFor) {
    setDraftFor(value.q ?? "");
    setDraft(value.q ?? "");
  }
  const commitSearch = () => {
    if (draft.trim() !== (value.q ?? "")) {
      onChange({ ...value, q: draft.trim() });
    }
  };
  const day = (raw: string): ISODate | "" => (isISODate(raw) ? raw : "");
  return (
    <div className="timeline-filters">
      <div className="timeline-filters-row">
        <SearchField
          value={draft}
          aria-label={t("timeline.filters.search")}
          placeholder={t("timeline.filters.search")}
          onChange={(event) => setDraft(event.target.value)}
          onBlur={commitSearch}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !isImeCommit(event)) {
              event.preventDefault();
              commitSearch();
            }
          }}
        />
        <span className="timeline-filters-range">
          <Field label={t("timeline.filters.from")}>
            {(control) => (
              <DateInput
                {...control}
                value={value.after ?? ""}
                max={value.before || undefined}
                onChange={(event) =>
                  onChange({ ...value, after: day(event.target.value) })
                }
              />
            )}
          </Field>
          <Field label={t("timeline.filters.to")}>
            {(control) => (
              <DateInput
                {...control}
                value={value.before ?? ""}
                min={value.after || undefined}
                onChange={(event) =>
                  onChange({ ...value, before: day(event.target.value) })
                }
              />
            )}
          </Field>
        </span>
      </div>
      {value.q && (
        <p className="t-caption">{t("timeline.filters.searchOmitsLimited")}</p>
      )}
    </div>
  );
}
