import type { components } from "../../api/schema";
import { Badge } from "../../design-system/atoms";
import { Meter } from "../../design-system/readings";
import { formatMoney } from "../../format/format";
import { useLocale, useT } from "../../i18n";

// The open balance, split into what is overdue and what is not yet due. Its
// own file because what it must not say is as decided as what it says.

type FinanceSummary = components["schemas"]["CompanyFinanceSummary"];

/**
 * OpenSplitMeter is the open balance as a bar: the overdue share against the
 * rest, both named in words beside it.
 */
export function OpenSplitMeter({
  summary,
}: Readonly<{ summary: FinanceSummary }>) {
  const t = useT();
  const { locale } = useLocale();
  const split = openSplitOf(summary);
  if (!split) {
    return null;
  }
  return (
    <>
      <Meter
        value={split.overdue}
        max={split.open}
        label={t("finance.overdueShareLabel")}
        tone="danger"
        restTone="accent"
      />
      {/* The bar's two colours, named: a reader who cannot tell the tones
            apart reads the words. */}
      <p className="fin-legend">
        <Badge tone="danger">
          {t("finance.legendOverdue", {
            amount: formatMoney(split.overdue, split.currency, locale),
          })}
        </Badge>
        <Badge tone="accent">
          {t("finance.legendOpen", {
            amount: formatMoney(split.open, split.currency, locale),
          })}
        </Badge>
      </p>
    </>
  );
}

// The open balance as its two halves in ONE currency: what is overdue, and
// the whole it is part of.
//
// Null unless the halves make a proportion at all. Different currencies is a
// proportion of nothing. Nothing open would put a full bar over "€0 open",
// which reads as an account entirely in arrears rather than one that owes us
// nothing. And overdue ABOVE open is not a share that has run high, it is two
// figures that contradict each other: the mirror says money is late that it
// also says is not outstanding.
//
// The third case is refused rather than clamped because clamping picks a
// winner between two figures the panel cannot choose between, and picks it
// silently — a bar pinned at 100% beside a sentence reading "115% of
// everything open" is the same disagreement with a coat of paint. The overdue
// figure above still draws: it is what the mirror reported, and it is the one
// number a reader can act on. What refuses is only the claim about a
// relationship between the two.
export type OpenSplit = { overdue: number; open: number; currency: string };

export function openSplitOf(summary: FinanceSummary): OpenSplit | null {
  const open = summary.open_balance;
  const overdue = summary.overdue;
  if (
    open?.amount_minor == null ||
    !open.currency ||
    overdue?.amount_minor == null ||
    open.currency !== overdue.currency ||
    open.amount_minor <= 0 ||
    overdue.amount_minor > open.amount_minor
  ) {
    return null;
  }
  return {
    overdue: overdue.amount_minor,
    open: open.amount_minor,
    currency: open.currency,
  };
}
