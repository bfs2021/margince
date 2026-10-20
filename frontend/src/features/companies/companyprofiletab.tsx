// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import type { ReactNode } from "react";
import { useId, useState } from "react";
import type { components } from "../../api/schema";
import { Button } from "../../design-system/atoms";
import { Callout } from "../../design-system/callout";
import { FieldGrid } from "../../design-system/fieldgrid";
import { Panel, PanelBody } from "../../design-system/panel";
import { formatNumber } from "../../format/format";
import { useLocale, usePlural, useT } from "../../i18n";
import { useCanWriteRecord } from "../../lib/capability";
import { useCompanyProfileFields } from "../records/evidenceverdict";
import type { OpenReceipt } from "../records/record360";
import { profileFieldLabel } from "./companies";
import { DossierPanel } from "./companydossier";
import { CompanyFactsPanel } from "./companyfactspanel";
import { useCompanyReadOnlyReason } from "./companyheader";
import { SidecarFieldRow } from "./companyraildetails";

type Company = components["schemas"]["Company"];
type ProfileFieldKey = components["parameters"]["ProfileFieldKey"];
type ProfileField = components["schemas"]["CompanyProfileField"];

// The account's own story, in the order a rep reads it: what the company
// sells, who it sells to, and how the sale actually happens. These eleven
// fields have no column on `company` — the profile-field row IS the
// record of them — so every one is written through the same PATCH the rail's
// registration rows use.
//
// They were previously read-only: the tab listed whichever the crawl had
// found and offered a "correct" button beside each, which meant a field the
// crawl never produced could not be entered at all. The row is now the same
// editable row the rail draws, so an empty field reads as an invitation
// rather than as an absence with no remedy.
const NARRATIVE_FIELDS = [
  "offer_summary",
  "icp",
  "buying_center",
  "value_proposition",
  "usp",
  "customer_pains",
  "desired_outcomes",
  "buying_intents",
  "common_objections",
  "sales_motion",
  "history",
] as const satisfies readonly ProfileFieldKey[];
type NarrativeField = (typeof NARRATIVE_FIELDS)[number];

/**
 * CompanyProfileForm is the Profile tab's own body: the account's details as
 * one form, rather than the four collapsed sections this tab used to be.
 *
 * WHY A FORM AND NOT DISCLOSURES. Every section here describes the same
 * subject — this company, as the record holds it — so a reader looking for
 * one field had to guess which of four folds it was behind, open it, and
 * find that half the fields on the tab were not editable there anyway. The
 * genuinely editable ones lived in the record rail two tabs away. A record's
 * own fields belong in one place that can be read top to bottom.
 *
 * WRITE STATE IS DERIVED ONCE, HERE. `useCanWriteRecord` folds the three
 * independent axes — the object grant, the seat ceiling, and the server's own
 * per-row `writable` — where the tab previously consulted only the first.
 * A rep who cannot write a colleague's account was still shown every verb,
 * and found out at the save. The reason sentence is rendered ONCE at the top
 * and every refused control points at it by id, rather than each control
 * carrying its own copy: withholding twelve buttons individually is noise,
 * and withholding the page's one explanation is the defect.
 */
export function CompanyProfileForm({
  company,
  onOpenHistory,
  nameOf,
  onOpenRecord,
  onOpenReceipt,
  onOpenEmail,
  tools,
}: Readonly<{
  company: Company;
  // Opens the record's own history drawer, for a reader following an evidence
  // mark back to what changed.
  onOpenHistory?: () => void;
  // The dossier's own props, threaded straight from the page: this tab's copy
  // reads the same names and opens through the same receipt as the
  // overview's, so a chip cited in either place lands in the same drawer.
  nameOf?: (entityType: string, entityId: string) => string | undefined;
  onOpenRecord?: (entityType: string, entityId: string) => void;
  onOpenReceipt: OpenReceipt;
  onOpenEmail?: (activityId: string) => void;
  // The account's own tooling — custom fields, group rollup, the site read,
  // the technical profile. Passed in rather than built here: they are the
  // caller's existing cards and this file has no business knowing what is in
  // the set.
  tools: ReactNode;
}>): ReactNode {
  const t = useT();
  const reasonId = useId();
  const writable = useCanWriteRecord("company", company);
  // The archived/not-yours sentence, when there is one. It is the
  // more specific answer, so it wins over the generic refusal below whenever
  // it applies.
  const specificReason = useCompanyReadOnlyReason(company);
  // Every denial owes the reader a sentence. `useCanWriteRecord` also refuses
  // a read seat and a missing grant, neither of which the specific reason
  // knows about, and a control pointing at an id that renders nothing is a
  // refusal the reader cannot resolve.
  const reason = writable
    ? undefined
    : (specificReason ?? t("record.notYoursToChange"));
  const canEdit = writable && !specificReason;
  const profileQuery = useCompanyProfileFields(company.id);
  const fields = profileQuery.data ?? [];
  const narrative = useNarrativeFold(fields, profileQuery.isSuccess);

  return (
    <div className="record-stack">
      {/* The account in prose, leading the tab the same way it leads the
          overview: what the account IS, before its fields and facts. The
          overview keeps its own copy so a reader who never opens this tab
          still meets it. */}
      <DossierPanel
        companyId={company.id}
        nameOf={nameOf}
        onOpenRecord={onOpenRecord}
        onOpenReceipt={onOpenReceipt}
        onOpenEmail={onOpenEmail}
      />
      {reason && (
        <Callout
          tone="info"
          kind="standing"
          // The sentence carries the id every refused control below points at,
          // so the heading IS the description rather than a copy of it.
          title={<span id={reasonId}>{reason}</span>}
        />
      )}
      <Panel title={t("co.narrative.title")}>
        <PanelBody>
          <FieldGrid>
            {narrative.shown.map((field) => (
              <SidecarFieldRow
                key={field}
                companyId={company.id}
                anchor={company.is_anchor === true}
                fields={fields}
                // A pending or failed read is not the same claim as "this
                // field has no row": editing on that guess sends no If-Match
                // and overwrites a value the reader never saw.
                fieldsLoaded={profileQuery.isSuccess}
                field={field}
                // The label comes from the ONE vocabulary map
                // (PROFILE_FIELD_LABELS), not a second list here: a field
                // renamed there must not keep its old name on this tab.
                label={profileFieldLabel(field, t)}
                placeholderKey="co.narrative.add"
                canEdit={canEdit}
                readOnlyReason={reason}
                multiline
              />
            ))}
          </FieldGrid>
          {narrative.toggle}
        </PanelBody>
      </Panel>
      {/* Facts are their own panel: many-valued, correctable one row at a
          time, and now addable and removable — which is a different act from
          stating a field, and needs the same write state this form derived. */}
      <CompanyFactsPanel
        companyId={company.id}
        anchor={company.is_anchor === true}
        canEdit={canEdit}
        reasonId={reason ? reasonId : undefined}
        onOpenHistory={onOpenHistory}
      />
      {tools}
    </div>
  );
}

/**
 * useNarrativeFold keeps the account's empty description fields behind one
 * link, as the details pane does, so the card reads as what is known. Until
 * the fields have loaded every row stands: an empty-looking row is not yet a
 * fact about the account.
 */
function useNarrativeFold(
  fields: readonly ProfileField[],
  loaded: boolean,
): { shown: readonly NarrativeField[]; toggle: ReactNode } {
  const t = useT();
  const plural = usePlural();
  const { locale } = useLocale();
  const [open, setOpen] = useState(false);
  const filled = NARRATIVE_FIELDS.filter((key) =>
    Boolean(fields.find((one) => one.field === key)?.value.trim()),
  );
  const empty = NARRATIVE_FIELDS.length - filled.length;
  if (!loaded || empty === 0) {
    return { shown: NARRATIVE_FIELDS, toggle: null };
  }
  return {
    shown: open ? NARRATIVE_FIELDS : filled,
    toggle: (
      <Button variant="link" onClick={() => setOpen(!open)}>
        {open
          ? t("record.hideEmptyFields")
          : plural("record.showEmptyFields", empty, {
              count: formatNumber(empty, locale),
            })}
      </Button>
    ),
  };
}
