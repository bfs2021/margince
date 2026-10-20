// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import type { ReactNode } from "react";
import {
  type Ring,
  RingsLegend,
  type RingTone,
} from "../../design-system/rings";
import { formatNumber } from "../../format/format";
import { useLocale, useT } from "../../i18n";
import { leadStatusLabel } from "./leadpresentation";
import type { Lead } from "./leads";

type LabelledRing = Ring & { label: string; caption: string };

// The rungs a lead climbs, in order. Promotion is the top; a disqualified
// lead has left the ladder and fills none of it.
const LADDER = ["new", "contacted", "engaged", "promoted"] as const;

const SLA_STEP = {
  within_target: { value: 1, tone: "calm", word: "lead.sla.withinTarget" },
  at_risk: { value: 2, tone: "warning", word: "lead.sla.atRisk" },
  breached: { value: 3, tone: "danger", word: "lead.sla.breached" },
} as const;

function scoreTone(score: number): RingTone {
  if (score >= 60) return "calm";
  return score >= 30 ? "warning" : "danger";
}

/**
 * LeadRings is what a lead's call is read from, as the 360's rings. They are
 * the score out of a hundred, the first response against its clock, and the
 * rung the lead has reached. Every value is a fact already on the lead; a
 * clock the lead does not carry draws no ring.
 */
export function LeadRings({ lead }: Readonly<{ lead: Lead }>): ReactNode {
  const t = useT();
  const { locale } = useLocale();
  const n = (value: number) => formatNumber(value, locale);
  const score = Math.max(0, Math.min(100, lead.score));
  const rings: LabelledRing[] = [
    {
      key: "score",
      value: score,
      max: 100,
      tone: scoreTone(score),
      label: t("lead.score"),
      caption: t("lead.rings.scoreOf", { score: n(score), max: n(100) }),
    },
  ];
  if (lead.first_response_at) {
    rings.push({
      key: "response",
      value: 1,
      max: 1,
      tone: "calm",
      label: t("lead.readings.firstResponse"),
      caption: t("lead.readings.answered"),
    });
  } else if (lead.sla_state) {
    const step = SLA_STEP[lead.sla_state];
    rings.push({
      key: "response",
      value: step.value,
      max: 3,
      tone: step.tone,
      label: t("lead.readings.firstResponse"),
      caption: t(step.word),
    });
  }
  const rung = LADDER.indexOf(lead.status as (typeof LADDER)[number]);
  rings.push({
    key: "ladder",
    value: rung + 1,
    max: LADDER.length,
    tone: rung < 0 ? "danger" : "calm",
    label: t("lead.rings.ladder"),
    caption: t(leadStatusLabel(lead.status)),
  });
  return (
    <RingsLegend
      rings={rings}
      label={rings.map((ring) => `${ring.label}: ${ring.caption}`).join(", ")}
    />
  );
}
