// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import { Search } from "lucide-react";
import { type ReactNode, useId, useState } from "react";
import { SegmentedControl } from "../../design-system/atoms";
import { IconAction } from "../../design-system/iconaction";
import type {
  ActivityKind,
  TimelineFilters,
} from "../../design-system/recordtimeline";
import { TimelineFilterBar } from "../../design-system/timelinefilterbar";
import { useT } from "../../i18n";
import type { TimelineFilter } from "./recordchronology";

// The cuts a History card offers in its head: the whole record, one kind of
// exchange, or what changed. A kind is a server parameter; Changes is a
// different feed, so the two share one control and never contradict.
const HISTORY_CUTS = [
  "all",
  "email",
  "meeting",
  "call",
  "note",
  "changes",
] as const;
type HistoryCut = (typeof HISTORY_CUTS)[number];

function isKindCut(cut: string): cut is ActivityKind & HistoryCut {
  return cut !== "all" && cut !== "changes";
}

function cutOf(filter: TimelineFilter, filters: TimelineFilters): HistoryCut {
  if (filter === "changes") {
    return "changes";
  }
  const kind = filters.kind;
  return kind && isKindCut(kind) ? kind : "all";
}

/**
 * useHistoryDials draws a History card's dials. `cuts` stands in the card's
 * head. `narrowing` is the search and date range, under the head while the
 * reader has opened it or while it narrows the list.
 */
export function useHistoryDials({
  filter,
  onFilter,
  filters,
  onFilters,
}: Readonly<{
  filter: TimelineFilter;
  onFilter: (next: TimelineFilter) => void;
  filters: TimelineFilters;
  onFilters: (next: TimelineFilters) => void;
}>): { cuts: ReactNode; narrowing: ReactNode } {
  const t = useT();
  const rowId = useId();
  const [searching, setSearching] = useState(false);
  // A search or a range left set stays in view, so the list never reads as
  // narrowed by nothing.
  const narrowed = Boolean(filters.q || filters.after || filters.before);
  const open = filter !== "changes" && (searching || narrowed);
  const pick = (cut: HistoryCut) => {
    if (cut === "changes") {
      onFilter("changes");
      return;
    }
    onFilter("all");
    onFilters({ ...filters, kind: isKindCut(cut) ? cut : undefined });
  };
  const labels: Record<HistoryCut, string> = {
    all: t("chronology.all"),
    email: t("timeline.filters.kind.email"),
    meeting: t("timeline.filters.kind.meeting"),
    call: t("timeline.filters.kind.call"),
    note: t("timeline.filters.kind.note"),
    changes: t("chronology.changes"),
  };
  // Closing the row takes back what it narrowed: a hidden search still
  // filtering the list would be a filter nobody can see.
  const toggle = () => {
    if (!open) {
      setSearching(true);
      return;
    }
    setSearching(false);
    if (narrowed) {
      onFilters({ kind: filters.kind });
    }
  };
  return {
    cuts: (
      <span className="history-dials">
        {filter !== "changes" && (
          <IconAction
            label={t("timeline.filters.search")}
            icon={<Search aria-hidden />}
            disclosure={{ expanded: open, controls: rowId }}
            onClick={toggle}
          />
        )}
        <SegmentedControl
          options={HISTORY_CUTS}
          value={cutOf(filter, filters)}
          onChange={pick}
          labels={labels}
          label={t("chronology.label")}
        />
      </span>
    ),
    narrowing: open ? (
      <div id={rowId}>
        <TimelineFilterBar value={filters} onChange={onFilters} />
      </div>
    ) : null,
  };
}
