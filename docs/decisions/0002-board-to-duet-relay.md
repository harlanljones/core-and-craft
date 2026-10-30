# 0002: The board reaches the Duet pillar agents through each channel's routine API trigger

Status: proposed, 2026-10-30

## Context

The Duet pillar agents already reach the board: they upload envelope files to the Drive folder "core-and-craft inbox", `acta-inbox.timer` pulls them every 5 minutes and `acta ingest` appends them. The other way had no path. When Harlan answers a check-in, accepts or declines a proposed quest, or wants to ask a pillar something, the board logs it in `events.jsonl`, and the pillar's agent never hears of it unless Harlan retypes it in the channel.

A Duet agent only does work when something wakes it: Harlan's message in its channel, or one of its routines firing. So the question is not where to put the board's message, but what can wake the right channel's Claude with it.

What we know about the pieces (checked 2026-10-30):

- Claude Code routines (now "scheduled tasks") can carry an **API trigger**: `POST https://api.anthropic.com/v1/claude_code/routines/trig_.../fire` with a per-routine bearer token and an optional `text` (up to 65,536 characters). The call returns once the run's session exists. Limits: 30 fires an hour per routine, 100 API fires an hour per account. There is no idempotency key. `text` reaches the run wrapped in a `<routine-fire-payload>` block marked untrusted, so the routine's own prompt has to say what to do with it. Docs: code.claude.com/docs/en/routines, platform.claude.com/docs/en/api/claude-code/routines-fire.
- Duet channel routines are the same kind of object. The channel's existing routines (Pillar watch, Daily typing practice, Sunday stress check) list with `api_token_hint` and `persistent_session_id` set to the channel's own session, so a fire wakes that channel's Claude and its post lands in the channel.
- Remote Control on diocletian (`claude-rc-core-and-craft`) lets a Duet thread read local session transcripts, but local sessions refuse `send_message`, Duet has no "Work locally", and cloud sessions can't reach the tailnet. Nothing local can post into a Duet channel.
- Duet scheduled runs can use Drive but not other channels or GitHub, and a schedule runs at most hourly.

## Decision

Each pillar channel keeps one routine with no schedule, "Board relay", whose prompt says how to act on the board's payload (docs/duet-relay.md). Harlan adds an API trigger to it and puts the fire URL and token on diocletian. `acta relay` sends the board's items to it.

- **What goes out.** Harlan's own events (`source.agent` `manual`) that an agent is waiting on:
  - `pillar.asked`: a new kind, a question for that pillar's agent, typed in the town's lot popup (`acta ask`);
  - `checkin.answered` for a check-in a `duet:<channel>` agent asked, with the question;
  - `quest.accepted` and `quest.declined` for a quest a `duet:<channel>` agent proposed, with the title.
- **What comes back.** The agent answers a `pillar.asked` in its channel and also uploads a `pillar.answered` (same `ask:<slug>` subject) to the inbox, so the answer shows in the lot popup after the next pull.
- **When.** `acta-relay.path` runs `acta relay` whenever the log changes; `acta-relay.timer` retries every 10 minutes. One fire per pillar per run, up to 20 items.
- **State.** What was sent, and to which run, lives in `$ACTA_STATE/relay.json`, written after each fire. The log stays acta's alone and records nothing about delivery. The first run marks everything already in the log as sent, so installing never replays history.
- **Secrets.** The URL must be `https://api.anthropic.com/v1/claude_code/routines/trig_.../fire`, or acta refuses to send the token. The token comes from `ACTA_RELAY_<PILLAR>_TOKEN` in a 0600 `relay.env`, or a systemd credential `relay-<pillar>`, and never appears in output or errors.

Latency: the fire takes about a second; the channel's Claude starts within the minute and posts in the channel. An answer reaches the board on the next inbox pull, so within about 5 minutes.

## Alternatives rejected

- **A Drive outbox that pillar channels read.** Writing is easy (rclone the other way), but nothing wakes the agent: it would read the outbox only when Harlan writes in the channel, or on an hourly routine that mostly finds nothing and still spends a run. Kept as the fallback below.
- **Remote Control sessions on diocletian, one per pillar.** They give a local Claude, not the Duet agent: the channel's memory, routines and voice stay in the cloud. Local sessions can't post into a Duet channel, and a channel can only read them when something else wakes it.
- **Slack or email drops.** Duet doesn't wake on either. Check-ins read mail every 3 hours and treat it as data, never as Harlan's words.
- **A plain Claude Code routine with an API trigger.** It wakes a cloud session outside the channel, with none of the pillar's context, and it can't post into the channel either.
- **Running the pillar agents locally (claude -p or the Agent SDK).** A second copy of each agent beside the Duet one, with its own memory to keep in step.

## Consequences

- Harlan sets up one routine and one token per pillar they want the board to reach (docs/duet-relay.md). A pillar without a route keeps its items waiting, and `acta relay` names it on every run; nothing is lost.
- Not yet verified: that the routine page at claude.ai/code/routines offers the API trigger on a routine created inside a Duet channel. If it doesn't, the fallback is the Drive outbox: `acta relay` keeps the same selection and state, and a sink that writes `<recorded_at>-board.jsonl` files to a "core-and-craft outbox" folder replaces the fire. The board side would not change.
- A fire that succeeds but whose state write fails could be sent again; the routine's prompt treats a repeated event id as already handled.
- Each fire spends one routine run from Harlan's Claude usage. Board taps are rare, so this is a handful a day.
