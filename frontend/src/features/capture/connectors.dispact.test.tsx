/** @vitest-environment happy-dom */
// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

import "@testing-library/jest-dom/vitest";

import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { components } from "../../api/schema";
import {
  jsonResponse,
  StoryProviders,
  stubWithSession,
} from "../../testing/story-utils";
import { ChannelConnectorsPanel } from "./connectors.telegram";

// A Telegram and a Dispact bot coexist on one list, and each panel owns only
// the rows of its own provider.

type ChannelConnection = components["schemas"]["ChannelConnection"];

const telegramBot: ChannelConnection = {
  id: "018f3a1b-0000-7000-8000-0000000000d1",
  provider: "telegram",
  channelId: "555000111",
  channelLabel: "acme_sales_bot",
  status: "connected",
  version: 1,
};

const dispactBot: ChannelConnection = {
  id: "018f3a1b-0000-7000-8000-0000000000e1",
  provider: "dispact",
  channelId: "u_7f3a",
  channelLabel: "Deal Bot · Acme",
  status: "connected",
  version: 1,
};

const botManager = {
  channel_connection: ["read", "create", "update", "delete"],
} as const;

function stubChannels(
  rows: ChannelConnection[],
  connect: (body: unknown) => Response = () => jsonResponse(dispactBot, 201),
) {
  const posted: unknown[] = [];
  stubWithSession(
    {
      "GET /channel-connections": () => jsonResponse({ data: rows }),
      "POST /channel-connections": (body) => {
        posted.push(body);
        return connect(body);
      },
    },
    { channel_connection: [...botManager.channel_connection] },
  );
  return posted;
}

function renderPanel(provider: ChannelConnection["provider"]) {
  render(
    <StoryProviders>
      <ChannelConnectorsPanel provider={provider} />
    </StoryProviders>,
  );
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("the bot panels split one list by provider", () => {
  it("keeps a Dispact bot out of the Telegram panel", async () => {
    stubChannels([telegramBot, dispactBot]);
    renderPanel("telegram");
    const rows = await screen.findAllByTestId("telegram-connection");
    expect(rows).toHaveLength(1);
    expect(within(rows[0]).getByText("@acme_sales_bot")).toBeInTheDocument();
    expect(screen.queryByText(/Deal Bot/)).toBeNull();
  });

  it("lists only Dispact bots in the Dispact panel, without an @", async () => {
    stubChannels([telegramBot, dispactBot]);
    renderPanel("dispact");
    const rows = await screen.findAllByTestId("dispact-connection");
    expect(rows).toHaveLength(1);
    expect(within(rows[0]).getByText("Deal Bot · Acme")).toBeInTheDocument();
    expect(screen.queryByText(/acme_sales_bot/)).toBeNull();
  });

  it("offers Dispact connect while only a Telegram bot is live", async () => {
    stubChannels([telegramBot]);
    renderPanel("dispact");
    expect(await screen.findByTestId("dispact-connect")).toBeInTheDocument();
    expect(
      screen.getByText(/Each deal gets a private Dispact channel/),
    ).toBeInTheDocument();
  });
});

describe("connecting a Dispact bot", () => {
  async function submitToken(token: string) {
    const user = userEvent.setup();
    await user.click(await screen.findByTestId("dispact-connect"));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(/starts with gwapp_/)).toBeInTheDocument();
    await user.type(within(dialog).getByLabelText("Bot token *"), token);
    await user.click(within(dialog).getByRole("button", { name: "Connect" }));
    return dialog;
  }

  it("posts the token under provider dispact", async () => {
    const posted = stubChannels([]);
    renderPanel("dispact");
    const dialog = await submitToken("gwapp_secret");
    expect(
      await within(dialog).findByText("Connected as Deal Bot · Acme."),
    ).toBeInTheDocument();
    expect(posted).toEqual([{ provider: "dispact", botToken: "gwapp_secret" }]);
  });

  it("says calmly that Dispact is not set up when the server has no address for it", async () => {
    stubChannels([], () =>
      jsonResponse(
        {
          code: "channel_provider_not_configured",
          detail: "dispact_base_url is not set",
        },
        503,
      ),
    );
    renderPanel("dispact");
    const dialog = await submitToken("gwapp_secret");
    expect(
      await within(dialog).findByText(
        "Gradion Dispact is not configured on this installation.",
      ),
    ).toBeInTheDocument();
    expect(within(dialog).queryByText("Bot not connected")).toBeNull();
    expect(within(dialog).queryByRole("alert")).toBeNull();
  });
});
