// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

// The deal's History tab, read off the same hook as the lead's and the
// contact's. All shows what was said about the deal, Changes what changed.
//
// The Changes cut swaps in the record-change audit (RecordHistoryTab), the one
// surface that can put a value back. A second rendering of the same changes
// beside it would be a second answer without the restore.

import { useQueryClient } from "@tanstack/react-query";
import type { components } from "../../../api/schema";
import { Panel, PanelBody } from "../../../design-system/panel";
import {
  useRecordTimeline,
  useTimelineFilters,
} from "../../../design-system/recordtimeline";
import { GroupedTimelineList } from "../../../design-system/timeline";
import { groupChronology } from "../../../design-system/timelinegroups";
import { useLocale, useT } from "../../../i18n";
import { invalidateRecord } from "../../../lib/recordwritekeys";
import { useRecordZone } from "../../../lib/recordzone";
import { RecordHistoryTab } from "../../records/history";
import { useHistoryDials } from "../../records/historydials";
import {
  ChronologyFooter,
  chronologyNotice,
  hasChronologyFooter,
  useChronologyFilter,
  useRecordChronology,
} from "../../records/recordchronology";
import { TimelineActions } from "../../records/timelineactions";

type Deal = components["schemas"]["Deal"];

export function DealHistoryTab({
  deal,
  onOpenEmail,
}: Readonly<{
  deal: Deal;
  // Opens one message in the record's drawer, which the page owns — the
  // page-level drawer stays mounted whichever tab is open, so a message
  // opened from here reads the same as one opened from the Overview call.
  onOpenEmail: (activityId: string) => void;
}>) {
  const t = useT();
  const { locale } = useLocale();
  const queryClient = useQueryClient();
  const recordZone = useRecordZone();
  const [filter, setFilter] = useChronologyFilter(deal.id);
  const [filters, setFilters] = useTimelineFilters(deal.id);
  const timeline = useRecordTimeline("deal", deal.id, { filters });
  const chronology = useRecordChronology({
    onOpenEmail,
    kind: "deal",
    recordId: deal.id,
    filter,
    // RecordHistoryTab draws Changes here and pages its own feed.
    changesInPanel: true,
    activities: timeline.activities,
    activitiesHaveMore: timeline.hasNextPage,
    loadMore: timeline,
    values: { currency: deal.currency ?? null, locale, zone: recordZone },
    renderActions: (activity) => (
      <TimelineActions
        activity={activity}
        entityType="deal"
        entityId={deal.id}
      />
    ),
  });
  const dials = useHistoryDials({
    filter,
    onFilter: setFilter,
    filters,
    onFilters: setFilters,
  });

  return (
    <Panel
      title={t("tab.history")}
      titleAction={dials.cuts}
      actions={
        hasChronologyFooter(filter, chronology) ? (
          <ChronologyFooter filter={filter} chronology={chronology} />
        ) : undefined
      }
    >
      <PanelBody>
        {dials.narrowing}
        {/* The Changes view IS the record's history: one reading of what
            changed on this deal, and the one that can put a change back. */}
        {filter === "changes" ? (
          <RecordHistoryTab
            kind="deal"
            id={deal.id}
            currency={deal.currency}
            restore={{
              version: deal.version,
              onRestored: () => invalidateRecord(queryClient, "deal", deal.id),
            }}
          />
        ) : (
          <div className="timeline-card">
            {chronologyNotice(
              "deal.timeline.empty",
              filter,
              chronology,
              {
                loading: timeline.isPending,
                failed: timeline.isError,
                assembled: timeline.isSuccess,
              },
              t,
            ) ?? (
              <GroupedTimelineList
                groups={groupChronology(
                  chronology.entries,
                  timeline.hasNextPage,
                )}
                zone={recordZone}
              />
            )}
          </div>
        )}
      </PanelBody>
    </Panel>
  );
}
