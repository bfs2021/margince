// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, within } from "storybook/test";
import {
  jsonResponse,
  type RouteMap,
  StoryProviders,
  stubWithSession,
} from "../../testing/story-utils";
import { ChannelConnectorsPanel } from "./connectors.telegram";
import "./connectors.css";

// The Gradion Dispact bot panel: none yet, one live bot, and an installation
// with no Dispact address that answers a connect with a calm notice.

const dealBot = {
  id: "018f3a1b-0000-7000-8000-0000000000e1",
  provider: "dispact",
  channelId: "u_7f3a",
  channelLabel: "Deal Bot · Acme",
  status: "connected",
  version: 1,
};

function dispactStory(channels: unknown[], routes: RouteMap = {}) {
  return () => {
    stubWithSession(
      {
        "GET /channel-connections": () => jsonResponse({ data: channels }),
        ...routes,
      },
      { channel_connection: ["read", "create", "update", "delete"] },
    );
    return (
      <StoryProviders>
        <ChannelConnectorsPanel provider="dispact" />
      </StoryProviders>
    );
  };
}

const meta: Meta<typeof ChannelConnectorsPanel> = {
  title: "Settings/You/Connections/Dispact bot",
  component: ChannelConnectorsPanel,
  args: { provider: "dispact" },
};
export default meta;
type Story = StoryObj<typeof ChannelConnectorsPanel>;

export const NoBotYet: Story = { render: dispactStory([]) };

export const NoBotYetPhone: Story = {
  globals: { viewport: { value: "phone" } },
  tags: ["uat-phone"],
  render: dispactStory([]),
};

export const Connected: Story = {
  render: dispactStory([dealBot]),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      await canvas.findAllByTestId("dispact-connection"),
    ).toHaveLength(1);
  },
};

export const ConnectedDark: Story = {
  globals: { theme: "dark" },
  render: dispactStory([dealBot]),
};

export const ProviderNotConfigured: Story = {
  render: dispactStory([], {
    "POST /channel-connections": () =>
      jsonResponse({ code: "channel_provider_not_configured" }, 503),
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("dispact-connect"));
    await userEvent.type(
      await screen.findByLabelText("Bot token *"),
      "gwapp_secret",
    );
    await userEvent.click(screen.getByRole("button", { name: "Connect" }));
    await screen.findByText(
      "Gradion Dispact is not configured on this installation.",
    );
  },
};
