// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

// The room brief: what the room's buyers did and what is waiting on the rep,
// written from the same participants, documents and threads the tab draws.
// Every sentence names what it was written from, in the room's own words.

import { useQuery } from "@tanstack/react-query";
import { api } from "../../../api/client";
import type { components } from "../../../api/schema";
import { WrittenBy } from "../../../components/citations.list";
import { AiPending } from "../../../design-system/aipending";
import { Badge } from "../../../design-system/atoms";
import { Panel, PanelBody } from "../../../design-system/panel";
import { useT } from "../../../i18n";
import { unwrap } from "../../../lib/problem";
import { useParticipants } from "../dealroomaccess";
import { useRoomThreads } from "../dealroomconversation";
import { useRoomDocuments } from "../dealroomdocuments";
import "./dealroomtab.css";

type DealRoomBrief = components["schemas"]["DealRoomBrief"];
type Evidence = components["schemas"]["DealRoomBriefEvidence"];

// A brief is written by a model on each read, so it is not read again on every
// focus of the window; a minute is as fresh as a reader can tell.
const BRIEF_STALE_MS = 60_000;

export function useRoomBrief(roomId: string) {
  const t = useT();
  return useQuery({
    queryKey: ["deal-room-brief", roomId],
    staleTime: BRIEF_STALE_MS,
    refetchOnWindowFocus: false,
    queryFn: async () => {
      return unwrap(
        await api.GET("/deal-rooms/{id}/brief", {
          params: { path: { id: roomId } },
        }),
        t,
      );
    },
  });
}

/**
 * The brief as an AI panel. It draws nothing for a room with nothing to say,
 * or when the brief cannot be read: the records it summarises are on the tab
 * beside it either way.
 */
export function RoomBrief({ roomId }: Readonly<{ roomId: string }>) {
  const t = useT();
  const brief = useRoomBrief(roomId);
  const sentences = brief.data?.sentences ?? [];
  if (brief.isError || (brief.isSuccess && sentences.length === 0)) {
    return null;
  }
  return (
    <Panel
      title={t("roomtab.brief.title")}
      tone="ai"
      titleAction={<Badge tone="ai">{t("co.assistant.aiTag")}</Badge>}
      footer={brief.data ? <WrittenBy by={brief.data.generated_by} /> : null}
    >
      <PanelBody>
        {brief.data ? (
          <BriefSentences roomId={roomId} brief={brief.data} />
        ) : (
          <AiPending label={t("roomtab.brief.pending")} lines={2} />
        )}
      </PanelBody>
    </Panel>
  );
}

function BriefSentences({
  roomId,
  brief,
}: Readonly<{ roomId: string; brief: DealRoomBrief }>) {
  const t = useT();
  const nameOf = useEvidenceNames(roomId);
  return (
    <ul className="roombrief">
      {brief.sentences.map((sentence) => {
        const sources = sentence.evidence
          .map(nameOf)
          .filter((name): name is string => name !== undefined);
        return (
          <li key={sentence.text}>
            <p>{sentence.text}</p>
            {sources.length > 0 ? (
              <p className="t-caption">
                {t("roomtab.brief.from", {
                  sources: [...new Set(sources)].join(", "),
                })}
              </p>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}

// What a citation names, read from the queries the tab already holds, so the
// brief costs no request of its own to say where it came from.
function useEvidenceNames(roomId: string) {
  const t = useT();
  const participants = useParticipants(roomId).data?.data ?? [];
  const documents = useRoomDocuments(roomId).data?.data ?? [];
  const threads = useRoomThreads(roomId).data?.data ?? [];
  const names = new Map<string, string>();
  for (const p of participants) {
    names.set(`participant:${p.id}`, p.full_name);
  }
  for (const doc of documents) {
    names.set(`document:${doc.id}`, doc.title);
  }
  const titles = new Map(documents.map((doc) => [doc.id, doc.title]));
  for (const thread of threads) {
    const about = thread.document_id
      ? titles.get(thread.document_id)
      : undefined;
    names.set(
      `thread:${thread.id}`,
      about
        ? t("threads.about", { title: about })
        : t("roomtab.brief.roomThread"),
    );
  }
  return (evidence: Evidence) =>
    names.get(`${evidence.entity_type}:${evidence.entity_id}`);
}
