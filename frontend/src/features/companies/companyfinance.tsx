import { Landmark } from "lucide-react";
import type { components } from "../../api/schema";
import { Panel, PanelBody } from "../../design-system/panel";
import {
  type SectionState,
  SurfaceState,
} from "../../design-system/surfacestate";
import { formatDate, formatMoney } from "../../format/format";
import { useLocale, useT } from "../../i18n";
import type { MessageKey } from "../../i18n/en";
import { problemCodeOf } from "../../lib/problem";
import { useRecordZone } from "../../lib/recordzone";
import { BillingContactsPanel } from "../projects/billingcontacts";
import { useFinanceSummary } from "../projects/financesummary";
// The row and card shapes this file draws — co-rowlink, co-row-meta, co-card —
// are defined in company360.css. Imported HERE rather than left to the caller:
// it works today only because the company record page pulls that stylesheet in
// for its own sake, so this file renders unstyled anywhere else.
import { FinanceTab } from "./companyfinancetab";
import "./company360.css";

// The finance card: does this customer actually pay us, and on time?
//
// THE RULE THIS CARD IS BUILT AROUND: no figure is invented, and the absence
// of one is never drawn as a zero. "€0 open" says the customer is square with
// us; "—" says we do not know. Rendering the second as the first tells a rep
// an account is healthy on the strength of a missing connector, which is the
// one thing §6 State B forbids outright.
//
// So the card renders the STATE first and the figures only where the server
// sent them. Six states, and five of them look identical if you draw only the
// numbers — which is why the server sends the state at all.

type FinanceSummary = components["schemas"]["CompanyFinanceSummary"];
type FinanceState = components["schemas"]["FinanceSummaryState"];

// Which §7 card state each finance state renders as. The mapping is explicit
// rather than derived, because two of them are NOT what they look like:
// `unmapped` is a ready card with an action, not an error, and `error` still
// shows the last good figures.
const CARD_STATE: Record<FinanceState, SectionState> = {
  no_connection: "empty",
  unmapped: "empty",
  // `syncing` is a known state with a name, not a mute wait: the read
  // ANSWERED, and the first sweep has simply not landed yet. Drawn as
  // `loading` it was a skeleton for as long as the sweep ran — minutes on a
  // cold connection — which reads as a section that broke. The empty arm
  // prints the state's own sentence instead, exactly as `no_connection` and
  // `unmapped` do.
  syncing: "empty",
  connected: "ready",
  // The last refresh failed, and the figures beside it are the last ones that
  // succeeded. `stale` rather than `failed`, because `failed` suppresses the
  // body — and a figure from this morning with its date on it is more useful
  // to a rep than an empty card with a retry button. The retry is offered
  // either way.
  error: "stale",
  stale: "stale",
};

/**
 * The lifecycles FIN-AC-3 authorises the card's absence for, and ONLY those.
 *
 * Named as the allowlist of absence rather than as an allowlist of presence,
 * because the two fail in opposite directions. A lifecycle this list forgets
 * gets a card that says "no accounting source connected" — a true statement
 * and a prompt to connect one. A lifecycle wrongly ON it gets NO card, and a
 * reader is never told the money is missing.
 *
 * `unknown` is the case that made this matter: every imported company carries
 * it, so an allowlist of presence hid finance from the majority of the book.
 * `disqualified` is the same shape — an account we stopped selling to may
 * still owe us money.
 */
const NEVER_INVOICED: ReadonlySet<string> = new Set([
  "target",
  "prospect",
  "opportunity",
]);

/**
 * FIN-AC-3: whether we have ever billed this account at all.
 *
 * The tab that holds the card and the card itself have to agree, so both ask
 * here. A tab present over a card that returns null is an empty page a reader
 * clicked for; a tab absent over a card that would have drawn hides money the
 * account owes.
 */
export function hasFinance(lifecycle?: string): boolean {
  return lifecycle == null || !NEVER_INVOICED.has(lifecycle);
}

export function CompanyFinanceCard({
  companyId,
  lifecycle,
}: Readonly<{
  companyId: string;
  // The account's lifecycle. A target, a prospect or an opportunity has never
  // been invoiced, so the card is ABSENT for them rather than empty (FIN-AC-3)
  // — an empty finance card on a company we have never billed is a question
  // nobody asked.
  lifecycle?: string;
}>) {
  const t = useT();
  const { locale } = useLocale();
  const recordZone = useRecordZone();
  const query = useFinanceSummary(companyId);

  if (!hasFinance(lifecycle)) {
    return null;
  }
  // Resolved ONCE, above the branches. A former customer's money is history in
  // every state the card can be in, and the `error` state is where the mislabel
  // would mislead most: it keeps showing the last good figures, so a title
  // saying "Finance" there puts real money from a finished relationship under a
  // heading that reads as current.
  const title =
    lifecycle === "former_customer"
      ? t("finance.titleHistorical")
      : t("finance.title");
  if (query.isPending) {
    return (
      <Panel title={title}>
        <PanelBody>
          <SurfaceState
            state="loading"
            emptyLabel={t("finance.none")}
            loadingLabel={t("finance.loading")}
          >
            {null}
          </SurfaceState>
        </PanelBody>
      </Panel>
    );
  }
  if (query.isError) {
    // A refusal is not a failure. A reader whose role cannot see finance is
    // told so; retrying would refuse again, and a retry button that always
    // fails teaches them the card is broken.
    const withheld = problemCodeOf(query.error) === "permission_denied";
    return (
      <Panel title={title}>
        <PanelBody>
          <SurfaceState
            loadingLabel={title}
            state={withheld ? "withheld" : "failed"}
            emptyLabel={t("finance.none")}
            detail={withheld ? {} : { onRetry: () => void query.refetch() }}
          >
            {null}
          </SurfaceState>
        </PanelBody>
      </Panel>
    );
  }
  const summary = query.data;
  const cardState = CARD_STATE[summary.state];
  // `stale` and `partial` still carry real rows, so the figures belong with
  // them: a stale figure from this morning is still a figure, with its as-of.
  const figures =
    cardState === "ready" || cardState === "stale" || cardState === "partial";
  const states = {
    loadingLabel: title,
    state: cardState,
    emptyLabel: t(EMPTY_LABEL[summary.state] ?? "finance.none"),
    detail: {
      onRetry: () => void query.refetch(),
      staleAsOf: summary.last_synced_at
        ? formatDate(summary.last_synced_at, locale, recordZone)
        : undefined,
    },
  };
  return (
    <div className="co-overview-stack">
      {figures ? (
        <SurfaceState {...states}>
          <FinanceTab
            summary={summary}
            source={<FinanceProvenance summary={summary} />}
            foot={
              summary.truncated ? (
                <span className="t-caption">{t("finance.moreInvoices")}</span>
              ) : undefined
            }
          />
        </SurfaceState>
      ) : (
        <Panel title={title}>
          <PanelBody>
            <SurfaceState {...states}>{null}</SurfaceState>
          </PanelBody>
        </Panel>
      )}
      {/* OUTSIDE the SurfaceState above, deliberately. Who to send the invoice
          to is the installation's own record and has nothing to do with whether
          an accounting source is connected, mapped or still syncing — and an
          account with no connector at all is exactly when a reader wants it.
          Inside the state wrapper it would disappear behind "no financial
          source connected", which says nothing about the recipient. */}
      <BillingContactsPanel
        contacts={summary.billing_contacts}
        companyId={companyId}
      />
    </div>
  );
}

// What the card says when it has no figures. Two different sentences, because
// "nobody has connected an accounting system" and "this customer is not mapped
// to one of its customers" have different fixes.
const EMPTY_LABEL: Partial<Record<FinanceState, MessageKey>> = {
  no_connection: "finance.noConnection",
  unmapped: "finance.unmapped",
  syncing: "finance.syncing",
};

// WHAT PERIOD these figures are drawn from, which is a different question from
// when we last looked.
//
// `last_synced_at` is already on the panel and answers the second. Without the
// first, a card can say its numbers are fresh without saying that the account
// it describes stopped buying two years ago — which is the reading FIN-AC-3
// asks for on a former customer's section, and which no field carried.
//
// The trailing figure's own label is NOT derived from this. `net_invoiced` is a
// true rolling 365-day fold, so "12 months" is what it is over whatever the
// coverage says, and on an account that stopped buying it honestly reads €0.
// What the coverage names is the mirror — and with it the lifetime figure,
// which has no window of its own.
//
// Absent when the mirror holds no invoice: a period minted from nothing would
// name a window nothing is in.
export function CoveragePeriod({
  summary,
}: Readonly<{ summary: FinanceSummary }>) {
  const t = useT();
  const { locale } = useLocale();
  const recordZone = useRecordZone();
  if (!summary.coverage_start || !summary.coverage_end) {
    return null;
  }
  return (
    <p className="fin-coverage t-caption">
      {t("finance.coveragePeriod", {
        from: formatDate(summary.coverage_start, locale, recordZone),
        to: formatDate(summary.coverage_end, locale, recordZone),
      })}
    </p>
  );
}

// A money reading, or nothing.
//
// BOTH halves are required. An amount with no currency cannot be rendered —
// defaulting to EUR would put a euro sign on a figure that might be dollars,
// which is a worse error than showing no figure. And a null amount is the
// server saying it could not compute one, so it must not become a zero: this
// card's whole rule is that "€0 open" and "we do not know" are different
// claims about a customer.
export function amountOf(
  money: components["schemas"]["Money"] | undefined,
  locale: ReturnType<typeof useLocale>["locale"],
): string | undefined {
  if (money?.amount_minor == null || !money.currency) {
    return undefined;
  }
  return formatMoney(money.amount_minor, money.currency, locale);
}

function FinanceProvenance({ summary }: Readonly<{ summary: FinanceSummary }>) {
  const t = useT();
  const { locale } = useLocale();
  const recordZone = useRecordZone();
  if (!summary.provider) {
    return null;
  }
  return (
    <p className="t-caption fin-provenance">
      <Landmark size={12} aria-hidden="true" />
      {summary.last_synced_at
        ? t("finance.syncedFrom", {
            provider: summary.provider,
            when: formatDate(summary.last_synced_at, locale, recordZone),
          })
        : t("finance.fromNeverSynced", { provider: summary.provider })}
    </p>
  );
}
