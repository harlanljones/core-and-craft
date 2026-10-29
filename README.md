# core-and-craft

CORE & CRAFT, a life framework of nine pillars (Career, Outlook, Relationships, Economic; Competence, Recovery, Agents, Faith, Time), with **acta** ("things done": the record) as its one writer and an Omarchy desktop board as its face.

One rule: agents only record what happened, and arithmetic decides progress. An append-only `events.jsonl` plus markdown notes is the source of truth. `acta` is the only writer and folds the log into `snapshot.json` for the widgets.

## Quick start (diocletian)

```sh
make test install             # builds ./cmd/acta into ~/.local/bin/acta
acta init                     # ~/core-and-craft (data) and ~/.local/state/core-and-craft (derived)
cp seed/*.jsonl ~/core-and-craft/inbox/ && acta ingest
mkdir -p ~/core-and-craft/notes/competence && cp seed/notes/competence/*.md ~/core-and-craft/notes/competence/
acta snapshot && acta status
```

Monkeytype archiver:

```sh
install -Dm600 /dev/null ~/.config/core-and-craft/monkeytype.env
$EDITOR ~/.config/core-and-craft/monkeytype.env   # MONKEYTYPE_APE_KEY=...
acta mt pull --all                               # one-time backfill
make install-units                               # daily 23:30 pull + rebuild on change
```

Board (Omarchy 4):

```sh
make check-widgets install-board   # plugin harlan.core-and-craft + ~/.config/core-and-craft/widgets/
```

Create the ApeKey in Monkeytype's account settings and enable it. The key is read from the env file or a systemd credential named `monkeytype`; it is never put in a URL, a log line or an error.

## Commands

| command | does |
|---|---|
| `acta init` | create data, inbox and state directories |
| `acta mt pull [--all]` | archive new Monkeytype results as `typing.test` (`mt:<_id>`, deduped) |
| `acta log <kind> --subject S --agent A [--data JSON] [--at T]` | append one event |
| `acta retract <id> --agent A [--reason R]` | append `event.retracted` |
| `acta ingest [file...]` | append inbox files from remote agents |
| `acta snapshot` | rebuild `snapshot.json` |
| `acta status [--json]` | one line for a bar widget |
| `acta doctor` | duplicate ids, seq gaps, future times, dangling corrections |
| `acta quest add --pillar P --title T [--priority N] [--every R] [--skill S --rule R]` | add a quest (active from you, proposed from any other `--agent`) |
| `acta quest done\|undo\|accept\|decline\|drop <id>` | finish (undo retracts the latest done), approve or turn down a proposal, drop |
| `acta quest snooze <id> --until D`, `acta quest edit <id> ...` | hide until a day; change title, priority or recurrence |
| `acta quest list [--json] [--all]` | open and proposed quests (`--all` adds done, declined, dropped) |
| `acta checkin ask --pillar P --question Q`, `acta checkin answer <id> --text T` | a pillar's open question and your answer |
| `acta rest [--at D]` | a rest day, which keeps the streak |

Environment: `ACTA_DATA` (default `~/core-and-craft`), `ACTA_STATE` (default `~/.local/state/core-and-craft`), `ACTA_TZ` (default local).

## Layout

```
cmd/acta/             CLI
internal/event/       v1 envelope, validation, ids
internal/store/       flock append, scanning
internal/monkeytype/  ApeKey client and result mapping
internal/snapshot/    fold into snapshot.json (gate, sustain, daily series, pillars)
systemd/              user units: daily pull, Drive inbox pull, rebuild on change
board/plugin/         Omarchy 4 shell plugin: the board, vendored OWL runtime (owl/)
board/test/           check.sh renders every widget under node against acta-built snapshots
widgets/              OWL tiles the board shows (60-typing.owl)
docs/                 event envelope, widget contract
seed/                 Oct 29 events and the first study note, as an inbox file
```

## Roadmap

1. Day one (this): archiver, ingest, snapshot, status.
2. Board plugin (vendored OWL runtime) and the typing tile.
3. Rename to CORE & CRAFT and acta; quests, check-ins, xp, levels and streak as events (see `docs/decisions/0001-core-and-craft-migration.md`).
4. `acta build` into SQLite (modernc.org/sqlite), pillar strip widget, bar pill.
5. FSRS reviews over knowledge notes (go-fsrs), then the knowledge graph.
