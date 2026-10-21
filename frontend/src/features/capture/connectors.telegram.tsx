// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../../api/client";
import type { components } from "../../api/schema";
import {
  Badge,
  Button,
  EmptyState,
  OverflowMenu,
} from "../../design-system/atoms";
import { Callout } from "../../design-system/callout";
import { ConfirmModal } from "../../design-system/confirmmodal";
import { KeyedName } from "../../design-system/keyedname";
import { Panel, PanelBody, PanelIntro } from "../../design-system/panel";
import { SettingList, SettingRow } from "../../design-system/settingrow";
import { useT } from "../../i18n";
import { useCanWrite } from "../../lib/capability";
import { statusLabel, statusTone } from "../../lib/connector-status";
import {
  problemCode,
  problemMessageOf,
  throwProblem,
  unwrap,
} from "../../lib/problem";
import { useMe } from "../../lib/session";
import {
  type ChannelProvider,
  channelConnectorCopy,
} from "./channelconnectorcopy";
import { ChannelConnectForm } from "./telegram-connect-form";

// The workspace's messaging bot, as its own file. It shares the connections
// card with the mail roster and nothing else: a different endpoint, a different
// scope — one bot for the workspace against a roster per user — and its own
// three grants. Only ConnectorsCard, which renders the pair, spans both.

type ChannelConnection = components["schemas"]["ChannelConnection"];

type ChannelConnectionsResult = {
  // GET /channel-connections answers 503 when this deployment serves no
  // messaging channels, or has no credential store to seal a bot token in — a
  // calm, documented feature-off state, mirroring the mail card's 501
  // not_implemented treatment above rather than an error card.
  notConfigured: boolean;
  data: ChannelConnection[];
};

function useChannelConnections() {
  return useQuery({
    queryKey: ["channel-connections"],
    queryFn: async (): Promise<ChannelConnectionsResult> => {
      const { data, error, response } = await api.GET("/channel-connections");
      if (
        response.status === 503 &&
        (problemCode(error) === "channel_connections_not_configured" ||
          problemCode(error) === "channel_credentials_not_configured")
      ) {
        return { notConfigured: true, data: [] };
      }
      if (error) {
        throwProblem(error);
      }
      return { notConfigured: false, data: data.data };
    },
  });
}

// One live bot as a row: its handle, a badge only when it is not live, and
// whichever of the two verbs this reader holds.
function ChannelConnectionRow({
  connection,
  onEdit,
  onDisconnect,
}: Readonly<{
  connection: ChannelConnection;
  onEdit?: () => void;
  onDisconnect?: () => void;
}>) {
  const t = useT();
  const copy = channelConnectorCopy(connection.provider);
  const handle = copy.handle(connection.channelLabel);
  return (
    <SettingRow
      testId={`${connection.provider}-connection`}
      label={
        <span className="connector-head">
          <KeyedName name={t(copy.name)} code={handle} />
          {connection.status !== "connected" && (
            <Badge tone={statusTone(connection.status)}>
              {t(statusLabel(connection.status))}
            </Badge>
          )}
        </span>
      }
      control={
        (onEdit || onDisconnect) && (
          <span className="cell-actions">
            <OverflowMenu label={t("table.rowActions", { name: handle })}>
              {onEdit && (
                <Button aria-haspopup="dialog" onClick={onEdit}>
                  {t("connectors.channelEditToken")}
                </Button>
              )}
              {onDisconnect && (
                <Button
                  variant="danger"
                  aria-haspopup="dialog"
                  onClick={onDisconnect}
                >
                  {t("connectors.disconnect")}
                </Button>
              )}
            </OverflowMenu>
          </span>
        )
      }
    />
  );
}

// What a bot panel shows in place of its rows, split out so the panel keeps
// one return and unconditional hooks.
function ChannelNotice({
  provider,
  query,
}: Readonly<{
  provider: ChannelProvider;
  query: ReturnType<typeof useChannelConnections>;
}>) {
  const t = useT();
  if (query.isPending) {
    return <p>{t("connectors.loading")}</p>;
  }
  if (query.isError) {
    return (
      <Callout tone="danger" kind="outcome" title={t("connectors.loadFailed")}>
        {problemMessageOf(query.error, t)}
      </Callout>
    );
  }
  if (query.data.notConfigured) {
    return (
      <EmptyState>
        <p>{t(channelConnectorCopy(provider).notConfigured)}</p>
      </EmptyState>
    );
  }
  return null;
}

// One provider's workspace bot as its own panel. Telegram and Dispact bots
// coexist, so each panel lists only its own provider's rows.
//
// A send needs one live bot per provider, and this panel is the only place to
// remove a second one. So it lists every row of its provider.
//
// Editing reuses ChannelConnectForm, whose PATCH replaces a disconnect and
// reconnect. One form instance is keyed to the row that opened it.
export function ChannelConnectorsPanel({
  provider,
}: Readonly<{ provider: ChannelProvider }>) {
  const t = useT();
  const copy = channelConnectorCopy(provider);
  const Icon = copy.icon;
  const qc = useQueryClient();
  const query = useChannelConnections();
  const [connectOpen, setConnectOpen] = useState(false);
  const [editingConnection, setEditingConnection] =
    useState<ChannelConnection | null>(null);
  const [disconnecting, setDisconnecting] = useState<ChannelConnection | null>(
    null,
  );

  const disconnect = useMutation({
    mutationFn: async (connection: ChannelConnection) => {
      unwrap(
        await api.DELETE("/channel-connections/{id}", {
          params: { path: { id: connection.id } },
        }),
      );
    },
    onSuccess: () => {
      setDisconnecting(null);
      void qc.invalidateQueries({ queryKey: ["channel-connections"] });
    },
  });

  // Three grants, three verbs: the seeded matrix moves them together for
  // admin and ops, but an operator may narrow one alone.
  const canCreate = useCanWrite("channel_connection", "create");
  const canEdit = useCanWrite("channel_connection", "update");
  const canDisconnect = useCanWrite("channel_connection", "delete");
  // The probe, not just its answer: every grant reads false while /me is in
  // flight, and the read-only line must not flash at an admin on each load.
  const me = useMe();
  const live = query.isSuccess && !query.data.notConfigured;
  const connections = live
    ? query.data.data.filter((connection) => connection.provider === provider)
    : [];
  // A deployment with no messaging channels withholds the verbs from every
  // seat, so the not-configured notice is the reason and this line stays quiet.
  const readOnly =
    me.isSuccess && live && !canCreate && !canEdit && !canDisconnect;
  const closeForms = () => {
    setConnectOpen(false);
    setEditingConnection(null);
  };

  return (
    <Panel title={t(copy.title)}>
      {(!live || readOnly) && (
        <PanelBody>
          {readOnly && <PanelIntro>{t(copy.readOnly)}</PanelIntro>}
          <ChannelNotice provider={provider} query={query} />
        </PanelBody>
      )}
      {live && (
        <SettingList bleed={connections.length > 0 ? "records" : "settings"}>
          {connections.length === 0 ? (
            <SettingRow
              testId={`${provider}-empty`}
              label={t(copy.emptyLabel)}
              description={t(copy.sub)}
              control={
                canCreate && (
                  <Button
                    data-testid={`${provider}-connect`}
                    onClick={() => setConnectOpen(true)}
                  >
                    <Icon aria-hidden /> {t(copy.connectCta)}
                  </Button>
                )
              }
            />
          ) : (
            connections.map((connection) => (
              <ChannelConnectionRow
                key={connection.id}
                connection={connection}
                onEdit={
                  canEdit ? () => setEditingConnection(connection) : undefined
                }
                onDisconnect={
                  canDisconnect ? () => setDisconnecting(connection) : undefined
                }
              />
            ))
          )}
        </SettingList>
      )}
      <ChannelConnectForm
        provider={provider}
        // Keyed to the row that opened it, so the form never carries one
        // connection's in-progress state onto another's rotation.
        key={editingConnection?.id ?? "new"}
        open={connectOpen || editingConnection !== null}
        connection={editingConnection ?? undefined}
        onClose={closeForms}
        onConnected={closeForms}
      />
      <ConfirmModal
        open={disconnecting !== null}
        onClose={() => setDisconnecting(null)}
        title={t("connectors.channelDisconnectTitle")}
        confirmLabel={t("connectors.disconnect")}
        confirmVariant="danger"
        pending={disconnect.isPending}
        error={
          disconnect.isError ? problemMessageOf(disconnect.error, t) : null
        }
        onConfirm={() => {
          if (disconnecting) {
            disconnect.mutate(disconnecting);
          }
        }}
      >
        <p>{t(copy.disconnectBody)}</p>
      </ConfirmModal>
    </Panel>
  );
}
