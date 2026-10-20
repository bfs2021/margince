// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import type { ReactNode } from "react";
import type { components } from "../../api/schema";
import { Panel, PanelBody, PanelIntro } from "../../design-system/panel";
import { GroupedBars } from "../../design-system/report-charts";
import { StatCard } from "../../design-system/statcard";
import { StatStrip } from "../../design-system/statstrip";
import {
  formatDate,
  formatMoney,
  formatMoneyCompact,
  formatNumber,
  monthName,
} from "../../format/format";
import { type Locale, type Translator, useLocale, useT } from "../../i18n";
import { useRecordZone } from "../../lib/recordzone";
import { RecentInvoices } from "../projects/financeinvoices";
import { OpenSplitMeter, openSplitOf } from "../projects/financeoverdue";
import { medianDaysLabel } from "./company360";
import { amountOf, CoveragePeriod } from "./companyfinance";

type FinanceSummary = components["schemas"]["CompanyFinanceSummary"];
type FinanceInvoice = components["schemas"]["FinanceInvoice"];

/**
 * FinanceTab is a connected account's money, read across and then down. It
 * draws the figures, how invoicing and payment ran, what is open, and the
 * invoices. `source` names where it came from.
 */
export function FinanceTab({
  summary,
  source,
  foot,
}: Readonly<{
  summary: FinanceSummary;
  source?: ReactNode;
  foot?: ReactNode;
}>) {
  const t = useT();
  const invoices = summary.recent_invoices ?? [];
  return (
    <>
      <FinanceReadings summary={summary} />
      <div className="fin-pair">
        <InvoicedByMonth summary={summary} />
        <PaymentBehaviour summary={summary} />
      </div>
      {summary.open_balance?.amount_minor ? (
        <Panel title={t("finance.openBalance")}>
          <PanelBody>
            <OpenSplitMeter summary={summary} />
          </PanelBody>
        </Panel>
      ) : null}
      {invoices.length > 0 && (
        <Panel
          title={t("finance.recentInvoices")}
          titleAction={source}
          footer={foot}
        >
          <RecentInvoices summary={summary} />
        </Panel>
      )}
    </>
  );
}

function FinanceReadings({ summary }: Readonly<{ summary: FinanceSummary }>) {
  const t = useT();
  const { locale } = useLocale();
  const zone = useRecordZone();
  const overdue = amountOf(summary.overdue, locale);
  const median = summary.median_days_after_due;
  // The newest by issue date: the server's order is not the reading's.
  const last = (summary.recent_invoices ?? []).reduce<
    FinanceInvoice | undefined
  >(
    (newest, invoice) =>
      !newest || invoice.issued_at > newest.issued_at ? invoice : newest,
    undefined,
  );
  return (
    <StatStrip
      label={t("finance.title")}
      floor={<CoveragePeriod summary={summary} />}
    >
      <StatCard
        label={t("finance.netInvoiced")}
        value={amountOf(summary.net_invoiced, locale) ?? "—"}
      />
      <StatCard
        label={t("finance.openBalance")}
        value={amountOf(summary.open_balance, locale) ?? "—"}
      />
      <StatCard
        label={t("finance.overdue")}
        value={overdue ?? "—"}
        // A verdict only where there is a figure to judge.
        tone={overdueTone(summary)}
        detail={overdueDetail(summary, locale, t)}
      />
      <StatCard
        label={t("finance.daysToPay")}
        value={median == null ? "—" : formatNumber(Math.abs(median), locale)}
        detail={median == null ? undefined : medianDaysLabel(median, locale, t)}
      />
      <StatCard
        label={t("finance.lastInvoice")}
        value={last ? formatDate(last.issued_at, locale, zone) : "—"}
        detail={
          last
            ? formatMoney(last.gross_minor, last.currency, locale)
            : undefined
        }
      />
    </StatStrip>
  );
}

// What the overdue figure is a share of, or why there is none. The server
// withholds the figure once the relationship has ended, and a bare dash would
// read as "could not compute".
function overdueDetail(
  summary: FinanceSummary,
  locale: Locale,
  t: Translator,
): string | undefined {
  if (summary.overdue?.amount_minor == null) {
    return summary.open_balance
      ? t("finance.overdueRelationshipEnded")
      : undefined;
  }
  const split = openSplitOf(summary);
  return split
    ? t("finance.shareOfOpen", {
        percent: formatNumber(
          Math.round((split.overdue / split.open) * 100),
          locale,
        ),
      })
    : undefined;
}

function overdueTone(
  summary: FinanceSummary,
): "danger" | "success" | undefined {
  const minor = summary.overdue?.amount_minor;
  if (minor == null) {
    return undefined;
  }
  return minor > 0 ? "danger" : "success";
}

// The base currency's net invoiced per calendar month, oldest first, as the
// server sends it.
function InvoicedByMonth({ summary }: Readonly<{ summary: FinanceSummary }>) {
  const t = useT();
  const { locale } = useLocale();
  const months = summary.invoiced_by_month ?? [];
  if (months.length === 0) {
    return null;
  }
  return (
    <Panel title={t("finance.byMonth")}>
      <PanelBody>
        <GroupedBars
          label={t("finance.byMonth")}
          dataLabel={t("reporting.data")}
          valueLabel={t("finance.netInvoicedShort")}
          readings={months.map((month) => ({
            key: month.month,
            label: monthName(Number(month.month.slice(5, 7)), locale, "short"),
            value: month.amount_minor,
            amount: formatMoneyCompact(
              month.amount_minor,
              month.currency,
              locale,
            ),
          }))}
        />
      </PanelBody>
    </Panel>
  );
}

// Days late per settled invoice, oldest first. A run climbing to the right is
// a customer slipping, which the median alone hides.
function PaymentBehaviour({ summary }: Readonly<{ summary: FinanceSummary }>) {
  const t = useT();
  const { locale } = useLocale();
  const series = summary.payment_behaviour ?? [];
  if (series.length < 2) {
    return null;
  }
  return (
    <Panel title={t("finance.behaviour")}>
      <PanelBody>
        <PanelIntro>{t("finance.behaviourShape")}</PanelIntro>
        <GroupedBars
          label={t("finance.behaviour")}
          dataLabel={t("reporting.data")}
          valueLabel={t("finance.daysLate")}
          readings={series.map((days, index) => ({
            // The position is the invoice's identity here: the series names
            // settled invoices by order, oldest first.
            key: String(index),
            label: formatNumber(index + 1, locale),
            // Early payment draws no bar: the axis is lateness.
            value: Math.max(0, days),
            amount: formatNumber(days, locale),
          }))}
        />
      </PanelBody>
    </Panel>
  );
}
