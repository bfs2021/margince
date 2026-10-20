import { Handshake } from "lucide-react";
import type { ReactNode } from "react";
import type { components } from "../../api/schema";
import { Button } from "../../design-system/atoms";
import { EmailReference } from "../../design-system/emailreference";
import { Panel, PanelBody } from "../../design-system/panel";
import { formatDayMonth } from "../../format/format";
import { useLocale, useT } from "../../i18n";
import { useRecordZone } from "../../lib/recordzone";
import {
  interactionIcon,
  useInteractionLabel,
} from "../records/interactionchrome";
import {
  RecordSpine,
  SentenceList,
  SourcesPill,
  type SpineSource,
  spineTask,
  WrittenBy,
} from "../records/record360";
import { overallVerdict } from "./contactrail";
import { ContactRings } from "./contactrings";

type Contact360 = components["schemas"]["Contact360"];
type ContactBrief = components["schemas"]["ContactBrief"];
type Activity = components["schemas"]["Activity"];
type BriefEvidence = components["schemas"]["CompanyBriefEvidence"];

// --- Relationship brief (§5.6) ---------------------------------------------

export function ContactBriefCard({
  brief,
  loading,
  failed = false,
  onRetry,
  view,
  onOpenEmail,
}: Readonly<{
  brief: ContactBrief | undefined;
  loading: boolean;
  failed?: boolean;
  onRetry?: () => void;
  view: Contact360;
  /**
   * Opens a cited message in the record's email drawer. The page owns the
   * drawer, so it owns the opener; a card that mounted its own would put a
   * second one behind the first.
   */
  onOpenEmail?: (activityId: string) => void;
}>) {
  const t = useT();
  const { locale } = useLocale();
  const recordZone = useRecordZone();
  // Resolved from the timeline the page already read rather than fetched: a
  // chip can never name a record the page beside it is withholding.
  const citedActivities = new Map(
    (view.activities?.data ?? []).map((row) => [row.id, row]),
  );
  const written = brief && brief.sentences.length > 0;
  // One source per distinct record, not per citation: several sentences
  // routinely cite the same thread.
  const sources = [
    ...new Map(
      (brief?.sentences ?? []).flatMap((sentence) =>
        sentence.evidence.map(
          (cited) =>
            [`${cited.entity_type}-${cited.entity_id}`, cited] as const,
        ),
      ),
    ).entries(),
  ];
  if (!loading && !written && !failed) {
    return (
      <Panel title={t("contact.overview.about")}>
        <PanelBody>
          <p className="pe-prose t-body">
            {[
              view.contact.full_name,
              view.contact.title,
              view.contact.employer?.company_name,
              view.contact.address?.city,
            ]
              .filter(Boolean)
              .join(" · ")}
          </p>
          <p className="t-sub">{t("contact.overview.profileOnly")}</p>
        </PanelBody>
      </Panel>
    );
  }
  return (
    <Panel
      // The record's lead, as on every record. A dateline sits over the call
      // once it is written; who wrote it and what it rests on sit at the end.
      tone="ai"
      className="record-hero"
      title={
        written
          ? t("record.howItStands", {
              when: formatDayMonth(brief.generated_at, locale, recordZone),
            })
          : t("contact.brief.title")
      }
      titleAction={
        written ? (
          <>
            <WrittenBy by={brief.generated_by} />
            {/* The messages the reading rests on, behind one pill: a busy
                contact cites dozens, and a list of them buried the call. */}
            <SourcesPill count={sources.length}>
              {sources.map(([key, cited]) => (
                <li key={key}>
                  <SourceChip
                    cited={cited}
                    activity={citedActivities.get(cited.entity_id)}
                    onOpenEmail={onOpenEmail}
                  />
                </li>
              ))}
            </SourcesPill>
          </>
        ) : undefined
      }
    >
      <PanelBody>
        {loading && (
          <p className="pe-prose t-body">{t("contact.brief.reading")}</p>
        )}
        {failed && (
          <p role="alert">
            {t("contact.overview.briefFailed")}{" "}
            <Button variant="ghost" onClick={onRetry}>
              {t("common.retry")}
            </Button>
          </p>
        )}
        {/* The call first with the judgement under it, and beside both the
            shares the relationship is read from. */}
        {written && (
          <div className="pe-brief-lead">
            <div className="pe-brief-call">
              <BriefVerdict view={view} />
              <SentenceList
                sentences={brief.sentences}
                leadWithJudgement
                citations="none"
              />
            </div>
            <ContactRings
              view={view}
              firstName={view.contact.full_name.split(" ")[0] ?? ""}
            />
          </div>
        )}
      </PanelBody>
      {/* The thread the reading was made from: the last word, the silence
          since, and what is dated ahead, as on the company's 360. */}
      {written && (
        <RecordSpine source={spineSource(view)} onOpenEmail={onOpenEmail} />
      )}
    </Panel>
  );
}

// The contact's read as the spine takes it: its open tasks are activity rows.
function spineSource(view: Contact360): SpineSource {
  const asOf = Date.parse(view.as_of);
  return {
    ...view,
    next_steps: {
      data: (view.next_steps?.data ?? []).map((task) => spineTask(task, asOf)),
    },
  };
}

// The verdict the brief leads with, in the standing's tone. It is read from
// the touch dates, so a reader they are withheld from gets none.
function BriefVerdict({ view }: Readonly<{ view: Contact360 }>): ReactNode {
  const t = useT();
  const withheld = (view.sections_omitted ?? []).includes("last_touch");
  const overall = withheld ? null : overallVerdict(view);
  const tone = overall ? VERDICT_TONE[overall.tone ?? "none"] : "unknown";
  return (
    <div className="r360-verdict">
      <span className={`r360-standing r360-standing-${tone}`}>
        {overall ? overall.word(t) : t("record.notShown")}
      </span>
    </div>
  );
}

// The record 360's standing tones, keyed by the contact's own verdict tone.
const VERDICT_TONE = {
  success: "calm",
  danger: "danger",
  none: "accent",
} as const;

// One cited source, named for what it is.
//
// A citation carries a RECORD TYPE and an id, never a transport — so an
// activity citation says nothing at all about how the conversation was
// carried, and calling every one of them a mail thread told the reader of a
// contact with no email address that they had been mailed. The activity the
// page ALREADY read is the resolver: it carries the kind, and for a message
// the provider the directory names. A citation whose activity is not on this
// page is named for the one thing it certainly is — a conversation — rather
// than for a transport nobody checked.
function SourceChip({
  cited,
  activity,
  onOpenEmail,
}: Readonly<{
  cited: BriefEvidence;
  activity: Activity | undefined;
  onOpenEmail?: (activityId: string) => void;
}>) {
  const t = useT();
  const { locale } = useLocale();
  const recordZone = useRecordZone();
  const interactionLabel = useInteractionLabel();
  if (cited.entity_type === "activity") {
    // A cited EMAIL is named by its subject and openable, like every other
    // citation of a message in the product: as a plain link in the sources
    // line, because a source is a footnote and not a status. A reference
    // reading "Email" would tell a reader which transport carried the sentence
    // and nothing about which message; the one they want is the one the
    // sentence rests on.
    //
    // `email_summary` is the server's own answer to "is this an email", set
    // only for kind=email, so nothing here decides it from the kind string. A
    // withheld message carries the summary with its subject nulled, and the
    // line says so and opens nothing.
    const summary = activity?.email_summary;
    if (summary) {
      return (
        <EmailReference
          subject={summary.subject}
          withheld={summary.display_status === "withheld"}
          onOpen={
            onOpenEmail ? () => onOpenEmail(summary.activity_id) : undefined
          }
        />
      );
    }
    // A meeting, a call, a chat: the transport's own name and the day, as
    // words, so "Meeting 17 Sept" reads as the record it is.
    // The glyph says what KIND of record the source is, the way the
    // timeline marks its rows, so a meeting and a call read apart before
    // their words do.
    return (
      <span className="t-sub pe-source">
        {activity && interactionIcon(activity.kind)}
        {activity
          ? `${interactionLabel(activity.kind, activity.channel_provider)} ${formatDayMonth(activity.occurred_at, locale, recordZone)}`
          : t("contact.brief.sourceActivity")}
      </span>
    );
  }
  return (
    <span className="t-sub pe-source">
      {cited.entity_type === "deal" && (
        <Handshake size={13} aria-hidden="true" />
      )}
      {cited.entity_type === "deal"
        ? t("contact.brief.sourceDeal")
        : cited.entity_type}
    </span>
  );
}
