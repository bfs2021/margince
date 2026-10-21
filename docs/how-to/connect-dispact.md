# Connect a Gradion Dispact bot

Gradion Dispact is a team chat product. Once an admin connects a Dispact app bot, a new deal gets a
private Dispact channel when it is created, and an older deal gets one on its next change. The
colleagues responsible for the deal are invited to it, and a card with the deal's facts is posted there
and kept current. This page is for the operator: what the installation needs, what the sync does on
each deal change, and what each failure means. The REST surface is the contract
(`backend/api/crm.yaml`, `/channel-connections*`, with `provider: dispact`).

A Dispact bot is a channel connection like the Telegram bot in [connect-telegram.md](connect-telegram.md).
The same rules hold: one live bot per provider, admin and ops connect it, every role reads its status,
and every action is `x-agent-access: human-only`. A Dispact bot and a Telegram bot can be connected at
the same time. A Dispact bot only posts; nothing from its channels comes into Margince.

## Before you start

You need three things:

- **Where Dispact lives.** Set `dispact_base_url` (`MARGINCE_DISPACT_BASE_URL`) to the origin of the
  Dispact install, on **both** the API and the worker. The API calls Dispact to check the token on
  connect, and the worker calls it on every deal change. It is an operator setting, never a field on the
  screen, so no admin can point the server at an address of their choosing.
- **The vault key**, `MARGINCE_KEYVAULT_ROOT_KEY`, on both programs, as for Telegram. The API seals the
  token, and the worker opens it.
- **An app bot token from Dispact.** Install an app in Dispact whose manifest asks for the scopes
  `channel:create`, `channel:members:write` and `chat:write:channel`. Its install shows the bot token
  once, with the shape `gwapp_<secret>`. The server checks that shape before it spends a call on it.

```sh
export MARGINCE_DISPACT_BASE_URL="https://dispact.example.com"
export MARGINCE_KEYVAULT_ROOT_KEY="$(openssl rand -base64 32)"  # base64 of exactly 32 bytes
```

Without `dispact_base_url`, a connect answers `503 channel_provider_not_configured`, and the worker
prints `worker not syncing deals to Gradion Dispact`. Without the vault, the worker prints the same line,
and the API refuses every connect as it does for Telegram. Set `public_base_url` too if the card should
link back to the deal.

## What connect does

1. `GET /api/bot/me` checks the token, and returns the bot user's ID, its name and the Dispact workspace.
2. Margince seals the token in the vault.
3. Margince adds the row as `connected`, in one transaction with its audit row.

The row's `channel_id` is the bot **user's** ID. A token rotated in Dispact keeps that user, so the
channels the bot already made stay reachable by their keys after **Replace token**. A token of a
different bot makes Margince forget the channels it recorded, and each deal gets a new channel on its
next change.

## What the sync does

The worker's `cg:dispact` consumer reads the deal stream. It acts on `deal.created`, `deal.updated`,
`deal.stage_changed`, `deal.owner_changed` and `deal.archived`, and skips an event an import wrote.

| Event | What happens in Dispact |
|---|---|
| The deal's first event | A private channel is made with the external key `margince:deal:<deal id>`. Its colleagues are invited, and the card is posted. |
| `deal.updated` | The card is edited to the deal as it is now. |
| `deal.stage_changed` | The card is edited, and one line is posted: "Stage moved to …". |
| `deal.owner_changed` | The card is edited, and one line is posted: "Owner is now …". |
| `deal.archived` | The card is edited, and one line is posted: "The deal was archived in Margince". |

Every event also invites the deal's colleagues again, so a colleague assigned later, or a new owner, joins
on the next change. A deal made before the bot was connected gets its channel on its first change. The channel is named
`deal <deal name> <last 12 characters of the deal id>`, so two deals of one name never collide. The
deal name is cut to 48 characters, so the name stays under Dispact's limit of 80. When another bot's
channel already holds that name, a space and the last 12 characters of the connection ID are added,
which still fits.

**Who is invited.** The deal's owner, the colleagues assigned to it, and the members of the teams assigned
to it, at most 100. A colleague is invited only if their own grants let them open the deal and read
every fact the card shows, because the channel shows them to everyone in it. A role with a field mask on
the amount, the close date or the owner keeps its members out. Contacts are never invited. Dispact adds
an address only when it belongs to an active member inside the app's audience. The worker logs how many
were not added and why, never the addresses.

Nothing takes a member out. Dispact's bot API has no call to remove one. A colleague who later loses
access to the deal stays in the channel until someone removes them in Dispact: after an owner change, an
unassignment, a revoked grant or a new field mask.

**Retries repeat nothing.** The channel is keyed on the deal, the card on the deal and the channel, and
each line on the event ID, through Dispact's `external_key` and `client_msg_id`. A redelivered event
makes no second channel and posts no second line. `deal_channel` records each deal's channel and card
message, so the next change edits the card instead of posting a new one.

## When something goes wrong

| What you see | Meaning | What to do |
|---|---|---|
| `channel_token_rejected` on connect | Dispact does not know the token, or it was rotated. | Paste the token from the latest install or rotation. |
| `channel_provider_unreachable` on connect | Dispact could not be reached. Nothing was written. | Check `dispact_base_url` and try again. |
| `channel_provider_rejected` on connect | Dispact refused the request, or `dispact_base_url` answered a 4xx page that is not Dispact's. Nothing was written. | Check the app is still installed and `dispact_base_url` is right. |
| **Needs reconnect** on the card | Dispact refused the sealed token while syncing. The sync stops. | **Replace token** with a new one; the sync starts again. |
| Worker log: "the deal's channel is archived or the bot was removed" | The bot cannot post in the channel. | Unarchive it, or add the bot back, in Dispact; the next change posts again. |
| Worker log: "Dispact refused the deal's sync" | Dispact refused a call for a reason no retry repairs, named in `reason`. An `error` that names an HTTP status with no envelope is a 4xx page that is not Dispact's. The event is acked. | Fix what the reason names. For an HTTP status, check `dispact_base_url`. The next change tries again. |
| A deleted card | Someone deleted the card message. | Nothing: the next change posts a new card. |
| A deleted channel | Someone deleted the channel, which frees its key. | Nothing: the next change makes a new channel. |

An outage, or a channel another request is still making (`create_in_progress`), returns the event to the
bus, which delivers it again.

## Disconnect

**Disconnect** archives the row and deletes the token, as for Telegram. No new sync starts once the row
is archived; one already running may finish. The Dispact channels stay in Dispact, and `deal_channel`
keeps its rows. A bot connected later records its
channels afresh: the same bot user finds each deal's channel again by its key, and another bot makes
channels of its own.
