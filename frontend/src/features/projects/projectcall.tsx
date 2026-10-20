// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import type { ReactNode } from "react";
import { Button } from "../../design-system/atoms";
import { Panel, PanelBody } from "../../design-system/panel";
import {
  type Ring,
  RingsLegend,
  type RingTone,
} from "../../design-system/rings";
import { omitted, SurfaceState } from "../../design-system/surfacestate";
import { formatMoneyCompact, formatNumber } from "../../format/format";
import { type Translator, useLocale, useT } from "../../i18n";
import type { MessageKey } from "../../i18n/en";
import {
  RecordSpine,
  type StandingTone,
  VerdictHead,
} from "../records/record360";
import type { ProjectHealthAssessment } from "./projecthealth.queries";
import type { Project360 } from "./projectsections";

type LabelledRing = Ring & { label: string; caption: string };

const DAY_MS = 86_400_000;

const HEALTH: Record<
  ProjectHealthAssessment["state"],
  { word: MessageKey; tone: StandingTone }
> = {
  on_track: { word: "projectHealth.state.onTrack", tone: "calm" },
  at_risk: { word: "projectHealth.state.atRisk", tone: "warning" },
  off_track: { word: "projectHealth.state.offTrack", tone: "danger" },
};

// What delivery has used against what it has. The rings are the money won of
// the deals, the days gone of the days planned, and the commitments on time.
// A share the project cannot state draws no ring.
function projectRings(
  view: Project360,
  t: Translator,
  locale: Parameters<typeof formatNumber>[1],
): LabelledRing[] {
  const rings: LabelledRing[] = [];
  const n = (value: number) => formatNumber(value, locale);
  const won = view.rollups?.won_deal_value;
  const open = view.rollups?.open_deal_value;
  const currency = won?.currency;
  const wonMinor = won?.amount_minor;
  const openMinor = open?.amount_minor;
  if (
    currency &&
    open?.currency === currency &&
    wonMinor != null &&
    openMinor != null &&
    wonMinor + openMinor > 0
  ) {
    const money = (minor: number) =>
      formatMoneyCompact(minor, currency, locale);
    rings.push({
      key: "budget",
      value: wonMinor,
      max: wonMinor + openMinor,
      tone: "calm",
      label: t("project.rings.budget"),
      caption: t("project.rings.budgetOf", {
        won: money(wonMinor),
        total: money(wonMinor + openMinor),
      }),
    });
  }
  const { started_at: start, target_end_date: end } = view.project;
  if (start && end) {
    const days = Math.round((Date.parse(end) - Date.parse(start)) / DAY_MS);
    const day = Math.round(
      (Date.parse(view.as_of) - Date.parse(start)) / DAY_MS,
    );
    if (days > 0 && day >= 0) {
      rings.push({
        key: "time",
        value: Math.min(day, days),
        max: days,
        tone: day > days ? "danger" : "calm",
        label: t("project.rings.time"),
        caption: t("project.rings.timeOf", { day: n(day), days: n(days) }),
      });
    }
  }
  const owed = view.commitments?.data ?? [];
  if (owed.length > 0) {
    const late = owed.filter((row) => row.overdue).length;
    const tone: RingTone = late > 0 ? "warning" : "calm";
    rings.push({
      key: "commitments",
      value: owed.length - late,
      max: owed.length,
      tone,
      label: t("project.rings.commitments"),
      caption: t("project.rings.commitmentsOf", {
        onTime: n(owed.length - late),
        open: n(owed.length),
      }),
    });
  }
  return rings;
}

/**
 * ProjectCall is the project's lead. It holds the delivery health a person
 * last recorded, the shares it is read beside, the phase and the thread. A
 * project nobody has judged says so rather than show a verdict it lacks.
 */
export function ProjectCall({
  view,
  current,
  phase,
  canRecord,
  onRecord,
  onCorrect,
}: Readonly<{
  view: Project360;
  // The reading that stands, if any.
  current?: ProjectHealthAssessment;
  // The phase stepper, under the call.
  phase: ReactNode;
  canRecord: boolean;
  onRecord: () => void;
  onCorrect: (assessment: ProjectHealthAssessment) => void;
}>) {
  const t = useT();
  const { locale } = useLocale();
  const health = current ? HEALTH[current.state] : undefined;
  const rings = projectRings(view, t, locale);
  return (
    <Panel
      className="record-hero"
      title={t("record.howItStandsTitle")}
      titleAction={
        canRecord ? (
          <Button
            variant="link"
            onClick={current ? () => onCorrect(current) : onRecord}
          >
            {t(current ? "projectHealth.correct" : "projectHealth.record")}
          </Button>
        ) : undefined
      }
    >
      <VerdictHead
        label={health ? t(health.word) : t("projectHealth.empty")}
        tone={health ? health.tone : "unknown"}
        because={current?.note}
      />
      {rings.length > 0 ? (
        <PanelBody className="r360-rings">
          <RingsLegend
            rings={rings}
            label={rings.map((r) => `${r.label}: ${r.caption}`).join(", ")}
          />
        </PanelBody>
      ) : null}
      {/* The money share is the one ring a role can withhold, and a withheld
          share says so rather than leaving a ring missing with no reason. */}
      {omitted(view, "rollups") && (
        <PanelBody>
          <SurfaceState
            state="withheld"
            label={t("project.rings.budget")}
            emptyLabel={t("project.rings.budget")}
            loadingLabel={t("project.rings.budget")}
          >
            {null}
          </SurfaceState>
        </PanelBody>
      )}
      <PanelBody>{phase}</PanelBody>
      <RecordSpine
        source={{
          as_of: view.as_of,
          activities: view.activities,
          next_steps: {
            data: (view.commitments?.data ?? []).map((row) => ({
              activity_id: row.activity_id,
              subject: row.subject,
              due_at: row.due_at,
              overdue: row.overdue,
            })),
          },
        }}
      />
    </Panel>
  );
}
