// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import { type LucideIcon, MessagesSquare, Send } from "lucide-react";
import type { components } from "../../api/schema";
import type { MessageKey } from "../../i18n/en";

export type ChannelProvider =
  components["schemas"]["ChannelConnection"]["provider"];

// What one workspace bot panel says, per provider. Keyed by the contract enum,
// so a provider added to the API without its words here fails the typecheck.
type ChannelConnectorCopy = Readonly<{
  name: MessageKey;
  title: MessageKey;
  sub: MessageKey;
  emptyLabel: MessageKey;
  notConfigured: MessageKey;
  connectCta: MessageKey;
  readOnly: MessageKey;
  disconnectBody: MessageKey;
  modalTitle: MessageKey;
  tokenHint: MessageKey;
  connectedAs: MessageKey;
  icon: LucideIcon;
  // A Telegram label is a bare @username; a Dispact one already reads as a
  // name ("bot · workspace") and takes no prefix.
  handle: (channelLabel: string) => string;
}>;

const CHANNEL_CONNECTOR_COPY: Readonly<
  Record<ChannelProvider, ChannelConnectorCopy>
> = {
  telegram: {
    name: "connectors.provTelegram",
    title: "connectors.telegramTitle",
    sub: "connectors.telegramSub",
    emptyLabel: "connectors.telegramEmptyLabel",
    notConfigured: "connectors.telegramNotConfigured",
    connectCta: "connectors.telegramConnectCta",
    readOnly: "connectors.telegramReadOnly",
    disconnectBody: "connectors.telegramDisconnectBody",
    modalTitle: "connectors.telegramModalTitle",
    tokenHint: "connectors.telegramBotTokenHint",
    connectedAs: "connectors.telegramConnectedAs",
    icon: Send,
    handle: (channelLabel) => `@${channelLabel}`,
  },
  dispact: {
    name: "connectors.provDispact",
    title: "connectors.dispactTitle",
    sub: "connectors.dispactSub",
    emptyLabel: "connectors.dispactEmptyLabel",
    notConfigured: "connectors.dispactNotConfigured",
    connectCta: "connectors.dispactConnectCta",
    readOnly: "connectors.dispactReadOnly",
    disconnectBody: "connectors.dispactDisconnectBody",
    modalTitle: "connectors.dispactModalTitle",
    tokenHint: "connectors.dispactBotTokenHint",
    connectedAs: "connectors.dispactConnectedAs",
    icon: MessagesSquare,
    handle: (channelLabel) => channelLabel,
  },
};

export function channelConnectorCopy(
  provider: ChannelProvider,
): ChannelConnectorCopy {
  return CHANNEL_CONNECTOR_COPY[provider];
}
