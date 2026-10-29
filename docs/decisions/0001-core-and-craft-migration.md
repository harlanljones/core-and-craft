# 0001: Migrate fully to CORE & CRAFT; rebuild the game on acta

Status: accepted, 2026-10-29

## Context

LifeQuest (`~/dev/omarchy-widgets`) was a quest game. A bash `lq` wrote everything to one `state.json`, and OWL tiles plus a Qt town app read it. CORE & CRAFT replaced its core with an append-only event log, a single Go writer and a derived `snapshot.json`. The first pass kept the old name in the CLI (`lq`), the units, the state and config paths, and the board plugin. The decision was a full migration: no LifeQuest naming, and the game rebuilt on the new model.

The project may come under the empire-product-web brand. Its Roman world is binding: Latin product names with plain-English labels, and emperor names reserved for machines.

Nothing needed migrating. The live log held 8 events, all from `duet:competence`. No `state.json` existed, and the Qt app was never installed.

## Decisions

### Naming

| Thing | Was | Now |
|---|---|---|
| Project and board | LifeQuest | CORE & CRAFT |
| Writer CLI | `lq` | `acta` ("things done": the record) |
| Writer agent ids | `lq`, `lq-mt` | `acta`, `acta-mt` |
| Env | `LQ_DATA`, `LQ_STATE`, `LQ_TZ`, `LIFEQUEST_WIDGETS` | `ACTA_DATA`, `ACTA_STATE`, `ACTA_TZ`, `ACTA_WIDGETS` |
| Data | `~/life` | `~/core-and-craft` |
| State | `~/.local/state/lifequest` | `~/.local/state/core-and-craft` |
| Config | `~/.config/lifequest` | `~/.config/core-and-craft` |
| Units | `lifequest-*` | `acta-*` |
| Plugin | `harlan.lifequest` | `harlan.core-and-craft` |

### The game, rebuilt as events

- **Quests** are concrete next actions under one pillar.
  - They can come from the CLI, the board, pillar Duet agents (inbox) or Claude Code.
  - Agent quests arrive as proposed until Harlan accepts.
  - Recurrence uses a small DSL (`once`, `daily`, `weekdays`, `weekly:mon,thu`, `every:3d`, `monthly:1`) and is computed per day.
  - Skill-linked quests finish themselves from evidence.
- **Ranking** is a new deterministic engine that explains itself. It weighs pillar balance, due/overdue, priority and skill gap. Each quest carries its score and reasons in the snapshot.
- **Progress** has two layers:
  - Skills keep mastery (target, sustain).
  - Xp comes from a fixed table per event kind, so no writer sets xp.
  - Levels are per pillar and overall, on a rising curve.
  - One global streak counts rest days; a missed day breaks it.
- **Reflection** happens through pillar check-ins: an agent asks, Harlan answers, and the answer is an event.
- **The board** is the only interface. It has full tap control, and every tap calls `acta`. A nine-pillar town replaces the six-place one. Tile designs are chosen from mockups before they're built.
- **Retired:** bash `lq`, `state.json`, the Qt app, the repo survey, and the Ollama chapel quiz. `~/dev/omarchy-widgets` is archived to a tarball, then deleted.

## Alternatives rejected

- **The old six-trait weighted engine with dice and reroll.** Its ordering was random and it couldn't explain itself.
- **Xp declared on each quest.** It is self-reported, so agents would decide progress.
- **A standalone daily journal and the chapel question.** Check-ins tie reflection to a pillar.
- **Porting the Qt app.** A second interface to maintain; the board covers it.
- **Keeping the game beside the log under another name.** Two sources of truth.
- **CLI names.** `tally`, `scribe`, `reckon` (not in the Roman world); `fasti`, `cursus`, `ratio` (fit less well than `acta`, Rome's record of things done); `cc` (taken by the C compiler).

## Consequences

- Duet pillar agents must write to `~/core-and-craft/inbox/` and use the new kinds only once they ship.
- Each machine runs a one-time local move: data, config and plugin. There's no compatibility code, because no event ever used the old agent ids.
