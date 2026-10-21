// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import type { Meta, StoryObj } from "@storybook/react-vite";
import { screen, userEvent } from "storybook/test";
import type { components } from "../../api/schema";
import {
  installFetchStub,
  jsonResponse,
  StoryProviders,
} from "../../testing/story-utils";
import { ChannelConnectForm } from "./telegram-connect-form";

// The Telegram connect form, each state reached by a play(). They cover idle,
// a resolved username, a token replace on a pending bot and a conflict.

type ChannelConnection = components["schemas"]["ChannelConnection"];

const pendingConnection: ChannelConnection = {
  id: "018f3a1b-0000-7000-8000-0000000000d1",
  provider: "telegram",
  channelId: "555000111",
  channelLabel: "acme_sales_bot",
  status: "pending",
  version: 1,
};

const connectedConnection: ChannelConnection = {
  ...pendingConnection,
  status: "connected",
};

const meta: Meta<typeof ChannelConnectForm> = {
  title: "Settings/You/Connections/Telegram connect form",
  component: ChannelConnectForm,
};
export default meta;
type Story = StoryObj<typeof ChannelConnectForm>;

// `screen`, not the story canvas: the form is inside a portalled Modal.
async function fillAndSubmit(cta: string) {
  const canvas = screen;
  await userEvent.type(
    canvas.getByLabelText("Bot token *"),
    "555000111:AAG-fake-bot-father-token",
  );
  await userEvent.click(canvas.getByRole("button", { name: cta }));
}

export const Idle: Story = {
  render: () => {
    installFetchStub({});
    return (
      <StoryProviders>
        <ChannelConnectForm provider="telegram" open onClose={() => {}} />
      </StoryProviders>
    );
  },
};

export const Connected: Story = {
  render: () => {
    installFetchStub({
      "POST /channel-connections": () => jsonResponse(connectedConnection, 201),
    });
    return (
      <StoryProviders>
        <ChannelConnectForm provider="telegram" open onClose={() => {}} />
      </StoryProviders>
    );
  },
  play: async () => {
    await fillAndSubmit("Connect");
    await screen.findByText(/@acme_sales_bot/);
  },
};

export const EditPendingConnection: Story = {
  render: () => {
    installFetchStub({});
    return (
      <StoryProviders>
        <ChannelConnectForm
          provider="telegram"
          open
          onClose={() => {}}
          connection={pendingConnection}
        />
      </StoryProviders>
    );
  },
};

export const WorkspaceAlreadyBound: Story = {
  render: () => {
    installFetchStub({
      "POST /channel-connections": () =>
        jsonResponse(
          {
            code: "channel_workspace_already_bound",
            detail:
              "Another bot is already connected to this workspace. Disconnect it first, or replace its token to point it at a different bot.",
          },
          409,
        ),
    });
    return (
      <StoryProviders>
        <ChannelConnectForm provider="telegram" open onClose={() => {}} />
      </StoryProviders>
    );
  },
  play: async () => {
    await fillAndSubmit("Connect");
    await screen.findByText(/already connected to this workspace/i);
  },
};
