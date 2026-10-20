import { useQueryClient } from "@tanstack/react-query";
import type { components } from "../../api/schema";
import { Panel, PanelBody } from "../../design-system/panel";
import {
  hasTimelineFilters,
  useRecordTimeline,
  useTimelineFilters,
} from "../../design-system/recordtimeline";
import { SurfaceState } from "../../design-system/surfacestate";
import { GroupedTimelineList } from "../../design-system/timeline";
import { groupChronology } from "../../design-system/timelinegroups";
import { useLocale, useT } from "../../i18n";
import { useRecordZone } from "../../lib/recordzone";
import { RecordHistoryTab } from "../records/history";
import { useHistoryDials } from "../records/historydials";
import {
  CHRONOLOGY_EMPTY_KEYS,
  ChronologyFooter,
  hasChronologyFooter,
  useChronologyFilter,
  useRecordChronology,
} from "../records/recordchronology";
import { TimelineActions } from "../records/timelineactions";
import { MeetingBriefAction } from "./contactmeetings";
import { timelineState } from "./contacttimelinestate";
import "./contact360.css";
import { invalidateRecord } from "../../lib/recordwritekeys";

// The tabs beside Overview that read what the 360 already assembled — the same
// rule the overview cards hold to (contactcards.tsx): a tab can never show a
// record the tab beside it is withholding. The Timeline's CHANGES half and the
// Files tab are the exceptions, and they fetch because the 360 carries
// neither.

type Contact360 = components["schemas"]["Contact360"];

// --- Timeline ---------------------------------------------------------------

/**
 * ContactTimelineTab is the contact's History, read through the same
 * `useRecordChronology` every record page uses. All and each kind show what
 * was said to them, and Changes what was changed about them.
 *
 * The exchanges are the 360's own section, already fetched, and one page of
 * it. When the server says there is more, the tab says so, so a cut list does
 * not read as the whole ledger.
 */
export function ContactTimelineTab({
  contactId,
  view,
  loading = false,
  onBriefMeeting,
  onOpenEmail,
}: Readonly<{
  contactId: string;
  /** Opens one message in the record's drawer, which the page owns. */
  onOpenEmail?: (activityId: string) => void;
  view?: Contact360;
  loading?: boolean;
  // Opens the pre-meeting brief for one meeting row. The drawer lives on the
  // page, so the tab asks rather than renders it.
  onBriefMeeting?: (activityId: string) => void;
}>) {
  const t = useT();
  const { locale } = useLocale();
  const queryClient = useQueryClient();
  const recordZone = useRecordZone();
  const [filter, setFilter] = useChronologyFilter(contactId);
  const [filters, setFilters] = useTimelineFilters(contactId);
  const dials = useHistoryDials({
    filter,
    onFilter: setFilter,
    filters,
    onFilters: setFilters,
  });
  // The 360's own page seeds the list; older pages and every narrowed read
  // come from the activity list itself.
  const timeline = useRecordTimeline("contact", contactId, {
    filters,
    firstPage: view?.activities,
  });
  const chronology = useRecordChronology({
    onOpenEmail,
    kind: "contact",
    recordId: contactId,
    filter,
    // RecordHistoryTab draws Changes here and pages its own feed.
    changesInPanel: true,
    activities: timeline.activities,
    activitiesHaveMore: timeline.hasNextPage,
    loadMore: timeline,
    // No name resolver yet: a uuid in a change row still renders untouched
    // rather than guessed at, which is the correct fallback while nothing
    // here resolves contacts/company ids to names.
    // What a stored value needs to be read as what it MEANS: this record holds
    // no money of its own, so the currency is absent and a minor-unit column
    // says so rather than printing a bare integer.
    values: { currency: null, locale, zone: recordZone },
    renderActions: (activity) => (
      <TimelineActions
        activity={activity}
        entityType="contact"
        entityId={contactId}
        contactId={contactId}
        extra={(row) => (
          <MeetingBriefAction activity={row} onBriefMeeting={onBriefMeeting} />
        )}
      />
    ),
  });
  return (
    <Panel
      title={t("tab.timeline")}
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
            changed on this record, and the one that can put a change back. A
            second rendering of the same audit rows beside it would be two
            answers to one question, and only one of them would ever carry the
            control. */}
        {filter === "changes" ? (
          <RecordHistoryTab
            kind="contact"
            id={contactId}
            restore={{
              // Handed over whether or not it is there. `RecordRestore.version`
              // is optional and the panel withholds the button when it is
              // absent, so a second guard here would be the same policy in two
              // places — and the company surface, which does not repeat it,
              // would be the one that looked wrong.
              version: view?.contact.version,
              onRestored: () =>
                invalidateRecord(queryClient, "contact", contactId),
            }}
          />
        ) : (
          <SurfaceState
            loadingLabel={t("tab.timeline")}
            state={timelineState(
              view,
              chronology,
              timeline,
              hasTimelineFilters(filters),
              loading,
            )}
            emptyLabel={
              filter === "all"
                ? t("contact.timeline.empty")
                : t(CHRONOLOGY_EMPTY_KEYS[filter])
            }
          >
            <GroupedTimelineList
              groups={groupChronology(chronology.entries, timeline.hasNextPage)}
              zone={recordZone}
            />
          </SurfaceState>
        )}
      </PanelBody>
    </Panel>
  );
}
