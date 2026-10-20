import type { ReactNode } from "react";
import { Badge } from "../../design-system/atoms";
import { IdentityLine } from "../../design-system/identityline";
import { RecordAccess } from "../records/recordaccess";
import { readableRole } from "./contactcards";
import type { Contact360 } from "./contactpage";

// The mark under the name: who may read the record. How the relationship
// stands is the 360's verdict, so the head does not repeat it.
export function ContactMarks({
  view,
}: Readonly<{ view: Contact360 }>): ReactNode {
  return (
    <IdentityLine separator="space">
      <RecordAccess
        key={view.contact.id}
        kind="contact"
        record={view.contact}
      />
    </IdentityLine>
  );
}

// The seat this contact holds in the buying, read off the relationship edge
// and never off a title, in the words the deal card uses. None when no seat
// is recorded.
export function ContactRoleBadge({
  role,
}: Readonly<{ role?: string | null }>): ReactNode {
  return role ? <Badge>{readableRole(role)}</Badge> : null;
}
