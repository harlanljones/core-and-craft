# Event envelope v1

`$ACTA_DATA/events.jsonl` (default `~/core-and-craft/events.jsonl`) is the source of truth for CORE & CRAFT. One JSON object per line, append-only. Agents only record what happened; `acta` derives progress. Nothing edits or deletes a line.

## Fields

| field | type | rule |
|---|---|---|
| `id` | string | Native: `evt_` + first 16 hex of sha256 over the canonical JSON of `{agent, at, data, kind, recorded_at, subject}` (keys sorted, `,` and `:` separators, no HTML escaping, UTF-8). Imports: the source's own id, e.g. `mt:<monkeytype _id>`. |
| `v` | int | `1` |
| `at` | string | `second`: RFC 3339 with offset. `day`: local `YYYY-MM-DD`. |
| `at_precision` | string | `second` or `day` |
| `seq` | int | Global, starts at 1, gap-free. Assigned only by `acta` under flock; omit it everywhere else. |
| `recorded_at` | string | RFC 3339 UTC, when it was logged |
| `pillar` | string | Full lowercase word: `career outlook relationships economic competence recovery agents faith time`. Never initials (C and R clash). |
| `kind` | string | See below |
| `subject` | string | `skill:<slug>`, `note:<slug>`, `card:<slug>`, `quest:<slug>` (every `quest.*` kind), `checkin:<slug>` (every `checkin.*` kind) or `ask:<slug>` (every `pillar.*` kind). For `event.retracted`, the target event's id. For `day.rested`, `day:YYYY-MM-DD`. |
| `source.via` | string | `chat`, `manual`, `monkeytype-api`, `keybr`, ... |
| `source.agent` | string | Fixed list: `acta`, `acta-mt`, `claude-code`, `hermes`, `manual`, `duet:<channel>`. `acta`, `acta-mt` and `manual` write only on this machine; an inbox file may not claim them (see below). |
| `source.ref`, `source.note` | string | Optional |
| `data` | object | Kind-specific |
| `supersedes` | string | Optional: id of the event this one corrects |

`seq` is left out of the id hash so a remote writer can name an event before `acta` sequences it. `recorded_at` is in it, so two identical runs logged at different moments never collide, while a retried write of the same line dedupes.

Python one-liner that matches `event.NativeID` (pinned by `TestNativeIDGolden`):

```python
body = {"agent": a, "at": at, "data": data, "kind": kind, "recorded_at": rec, "subject": subj}
"evt_" + hashlib.sha256(json.dumps(body, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()[:16]
```

## Kinds

- `skill.defined` `{name, domain, metric, unit, direction, baseline}`
- `skill.protocol_set` `{mode, mode2, language, punctuation, numbers, funbox, lazyMode, min_acc, ...}`: the settings a run must match to count
- `skill.target_set` `{metric, target, min_acc?, sustain_runs? (3), sustain_days? (7)}`
- `typing.test`: Monkeytype field names `{wpm, rawWpm, acc, consistency, mode, mode2, language, punctuation, numbers, difficulty, lazyMode, funbox, testDuration, charStats, isPb, bailedOut, restartCount, incompleteTestSeconds}`. Chat-logged runs carry what a screenshot shows, plus `run`. `acta mt pull` fills in the defaults the API leaves out of stored results (`language: english`, `punctuation`/`numbers`/`lazyMode`/`bailedOut: false`, `funbox: []`, `difficulty: normal`), so every archived run states its settings.
- `typing.keystats` `{key, hits, misses, avg_ms}`: reserved; Monkeytype results carry no per-key data
- `study.session` `{minutes, subject, kind}`
- `note.added` `{path, sha256, title?, type?, from_pillar?, sessions?}`: one version of a note file. `path` is `notes/<pillar>/<slug>.md` or `notes/nodes/<slug>.md`, and its slug must be the subject's (`note:<slug>`). `path` and `sha256` (64 lowercase hex of the file's exact bytes) come together; older lines may carry neither. A new version supersedes the event for the old one. Through the inbox, the body travels as `<sha256>.md` (below). See `docs/knowledge-graph.md`.
- `review.logged` `{card, rating 1..4, duration_ms}`: FSRS input, later phase
- `event.retracted` `{reason}`
- `quest.added` `{title, priority? 1..3, every?, skill?, rule?, status?}`: one concrete next action in the event's pillar.
  - `every`: `once` (default), `daily`, `weekdays`, `weekly:mon,thu`, `every:Nd` (1..365, counted from the day it was added) or `monthly:D` (past a short month's end, its last day).
  - `rule`: `runs:N` or `minutes:N`, and it needs `skill`. Qualifying evidence for that skill finishes the quest for the day.
  - `status`: `active` or `proposed`. Only `manual` quests may start `active`. A quest from any other agent is proposed, with or without the field, until `quest.accepted`.
- `quest.accepted`, `quest.declined` `{reason?}`, `quest.done`, `quest.dropped` `{}`: done on a recurring quest counts for that day. Undo is `event.retracted` on the done event.
- `quest.snoozed` `{until: YYYY-MM-DD}`: hidden until that day.
- `quest.edited` `{title?, priority?, every?}`: at least one field. The latest edit wins.
- `checkin.asked` `{question}` and `checkin.answered` `{text}`: a pillar agent asks, Harlan answers, under the same `checkin:<slug>`.
- `pillar.asked` `{text}` and `pillar.answered` `{text}`: the other way round from a check-in. Harlan asks a pillar's Duet agent (only `manual` writes `pillar.asked`), `acta relay` sends it, and the agent answers under the same `ask:<slug>` (only the pillar's own channel writes `pillar.answered`: its `source.agent` must be `duet:<pillar>` for the event's `pillar`, and the snapshot ignores an answer filed under a pillar other than the ask's). Text is at most 2,000 bytes. Neither earns xp.
- `day.rested` `{}`, subject `day:YYYY-MM-DD`: a rest day keeps the streak.

Xp, levels and scores are never fields on any event. acta computes them in the snapshot.

Latest `skill.protocol_set` and `skill.target_set` per subject win, in `seq` order.

## Corrections

Append `event.retracted` (`acta retract <id>`) or a new event with `supersedes`. Retracted import ids stay in the file, so `acta mt pull` never re-adds them.

## Remote writers: the inbox

Cloud agents (Claude Duet channels) can't take the lock. They write whole files to `$ACTA_DATA/inbox/<recorded_at>-<agent>.jsonl`: one event per line, `seq` omitted, `id` optional (computed if missing). `acta ingest` validates each file; any bad line rejects the whole file.

A `note.added` with a `path` needs its body: a file named `<sha256>.md` in the inbox, holding exactly the bytes that hash to it. Ingest writes the body to `path` and removes it from the inbox with the event file. Until every body a file names has arrived, the file waits in `inbox/` (it isn't rejected, and later runs retry it). A body that doesn't hash to its name, or a `path` that holds another version the event doesn't supersede, rejects the file. A file named on the command line may instead keep each body at `<its folder>/<path>`, and one it can't find is logged with a warning. A line in `inbox/` whose `source.agent` is `manual`, `acta` or `acta-mt` is bad (`line N: source.agent "manual" writes only on this machine, never through the inbox`): those writers are Harlan and acta on diocletian, and `acta relay` forwards `manual` events to the pillar agents as Harlan's own words, so a remote file must not claim them. A file named on the command line (`acta ingest <file>`) is Harlan's own and may hold any writer. A rejected file moves to `$ACTA_DATA/inbox/rejected/<name>`, with `<name>.err` next to it holding the file name and each `line N: ...` error; if that name is taken, acta adds a suffix (`<base>-1.jsonl`, `<base>-2.jsonl`, ...) and never overwrites or deletes. The `*.jsonl` glob does not enter `rejected/`, so a rejected file is not retried. To retry, fix the file and move it back to `inbox/`. With `--keep`, acta moves nothing. Good files are appended (dedupe by id) and removed.

### Duet route: Drive to inbox

Duet channels run in the cloud and save finished files to the Google Drive folder "core-and-craft inbox" (raw `.jsonl`, not converted to a Google Doc). On the desktop, `acta-inbox.timer` runs `acta-inbox.service` every 5 minutes. The service runs `acta inbox pull`, which does these steps:

1. `rclone lsjson ccinbox: --files-only --include "*.jsonl" --include "*.md" --drive-skip-gdocs` lists the folder. A `.md` must be named `<sha256>.md` (a note body); any other name is refused. If the listing fails, acta changes nothing and exits 1.
2. For each Drive file ID that is not in `$ACTA_STATE/inbox-seen.json`, `rclone copyto ccinbox:<name> ...` copies the file into `inbox/`. rclone writes to a hidden temp name that the `*.jsonl` glob ignores, then acta links it into place. acta never overwrites a local file: if `inbox/<name>` exists with different bytes, the copy is `<base>.<first 8 chars of the ID>.jsonl`. A name with a path separator, or one that does not end in `.jsonl`, is refused. acta records the ID right after each copy. A failed copy is not recorded, so the next run tries it again; the other files continue and the run exits 1.
3. IDs that are no longer in Drive are removed from `inbox-seen.json`, so the file stays small.
4. acta ingests `inbox/` exactly as `acta ingest` does, as above.

The folder is only read. acta never moves or deletes a file in Drive, so the files stay there; delete old ones in Drive by hand when you want. (The remote uses a Google service account. It can read the shared folder but cannot delete files it does not own, so `rclone move` failed with 403 `insufficientFilePermissions`.) The first run on a full folder fetches every file it has not seen. That is safe, because ingest dedupes by event id. `acta inbox pull --dry-run` lists what it would fetch and changes nothing. `ACTA_INBOX_REMOTE` replaces `ccinbox:`.

A file that fails validation moves to `inbox/rejected/` with a `.err` file and fails that run of the unit. Its ID is already recorded, so later runs do not fetch it again. Read the reason in the `.err` file, with `acta doctor`, or with `journalctl --user -u acta-inbox`. To retry, fix the file and move it back to `inbox/`.

One-time setup: share the Drive folder with the service account, then `rclone config create ccinbox drive scope=drive root_folder_id=<folder id> service_account_file=<key.json>` (key file mode 600). The remote stays in `~/.config/rclone/rclone.conf`.

## Outbound: acta relay

The inbox brings the Duet agents' events in; `acta relay` takes Harlan's out. It sends, to the channel of the agent that is waiting:

- `pillar.asked`, to `duet:<pillar>`;
- `checkin.answered`, to the agent that asked the check-in, with its question;
- `quest.accepted` and `quest.declined`, to the agent that proposed the quest, with its title.

Only events from `manual` go, and only once: `$ACTA_STATE/relay.json` records each sent id with the run it started. Nothing about delivery is written to the log. Each pillar channel's "Board relay" routine has an API trigger; acta POSTs the items to its fire URL as one text, a header line `acta relay v1 · to duet:<pillar> · N item(s) ...` and then one `{"event": ..., "re": ...}` JSON object per line. `acta-relay.path` runs it on every log change and `acta-relay.timer` retries every 10 minutes. Setup and the routine's prompt: `docs/duet-relay.md`. Why: `docs/decisions/0002-board-to-duet-relay.md`.

## Mastery (day one: performance skills)

A hand-logged `typing.test` (a screenshot read in chat) that an imported Monkeytype run repeats, with the same subject, local date, and wpm and acc rounded to whole numbers, is counted once, as the import. Each import stands in for at most one hand-logged run. Both stay in the log, so nothing needs retracting after the first pull.

A `typing.test` qualifies when it matches the protocol, has `acc >= min_acc` (default 97), and isn't bailed out. `best` is the best qualifying `wpm`. A skill with a target is `passed` once `sustain_runs` qualifying runs at or above target fall within `sustain_days`. Knowledge nodes (FSRS over `review.logged`) come in a later phase.
