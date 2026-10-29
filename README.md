# core-and-craft

LifeQuest, rebuilt as the interface for the CORE & CRAFT framework (Career, Outlook, Relationships, Economic; Competence, Recovery, Agents, Faith, Time).

One rule: agents only record what happened, and arithmetic decides progress. An append-only `events.jsonl` plus markdown notes is the source of truth. `lq` is the only writer and folds the log into `snapshot.json` for the widgets.

## Quick start (diocletian)

```sh
make test install           # builds ./cmd/lq into ~/.local/bin/lq
lq init                     # ~/life (data) and ~/.local/state/lifequest (derived)
cp seed/*.jsonl ~/life/inbox/ && lq ingest
mkdir -p ~/life/notes/competence && cp seed/notes/competence/*.md ~/life/notes/competence/
lq snapshot && lq status
```

Monkeytype archiver:

```sh
install -Dm600 /dev/null ~/.config/lifequest/monkeytype.env
$EDITOR ~/.config/lifequest/monkeytype.env   # MONKEYTYPE_APE_KEY=...
lq mt pull --all                             # one-time backfill
make install-units                           # daily 23:30 pull + rebuild on change
```

Board (Omarchy 4):

```sh
make check-widgets install-board   # plugin harlan.lifequest + ~/.config/lifequest/widgets/
```

Create the ApeKey in Monkeytype's account settings and enable it. The key is read from the env file or a systemd credential named `monkeytype`; it is never put in a URL, a log line or an error.

## Commands

| command | does |
|---|---|
| `lq init` | create data, inbox and state directories |
| `lq mt pull [--all]` | archive new Monkeytype results as `typing.test` (`mt:<_id>`, deduped) |
| `lq log <kind> --subject S --agent A [--data JSON] [--at T]` | append one event |
| `lq retract <id> --agent A [--reason R]` | append `event.retracted` |
| `lq ingest [file...]` | append inbox files from remote agents |
| `lq snapshot` | rebuild `snapshot.json` |
| `lq status [--json]` | one line for a bar widget |
| `lq doctor` | duplicate ids, seq gaps, future times, dangling corrections |

Environment: `LQ_DATA` (default `~/life`), `LQ_STATE` (default `~/.local/state/lifequest`), `LQ_TZ` (default local).

## Layout

```
cmd/lq/               CLI
internal/event/       v1 envelope, validation, ids
internal/store/       flock append, scanning
internal/monkeytype/  ApeKey client and result mapping
internal/snapshot/    fold into snapshot.json (gate, sustain, daily series, pillars)
systemd/              user units: daily pull, rebuild on change
board/plugin/         Omarchy 4 shell plugin: the board, vendored OWL runtime (owl/)
board/test/           check.sh renders every widget under node against lq-built snapshots
widgets/              OWL tiles the board shows (60-typing.owl)
docs/                 event envelope, widget contract
seed/                 Oct 29 events and the first study note, as an inbox file
```

## Roadmap

1. Day one (this): archiver, ingest, snapshot, status.
2. Board plugin ported from `~/dev/omarchy-widgets` (runtime only; the old quest widgets and bash `lq` stay there), typing tile.
3. Week one: `lq build` into SQLite (modernc.org/sqlite), pillar strip widget, bar pill.
4. FSRS reviews over knowledge notes (go-fsrs), then the knowledge graph.
