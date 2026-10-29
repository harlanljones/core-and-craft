# Event envelope v1

`$LQ_DATA/events.jsonl` (default `~/life/events.jsonl`) is the source of truth for CORE & CRAFT. One JSON object per line, append-only. Agents only record what happened; `lq` derives progress. Nothing edits or deletes a line.

## Fields

| field | type | rule |
|---|---|---|
| `id` | string | Native: `evt_` + first 16 hex of sha256 over the canonical JSON of `{agent, at, data, kind, recorded_at, subject}` (keys sorted, `,` and `:` separators, no HTML escaping, UTF-8). Imports: the source's own id, e.g. `mt:<monkeytype _id>`. |
| `v` | int | `1` |
| `at` | string | `second`: RFC 3339 with offset. `day`: local `YYYY-MM-DD`. |
| `at_precision` | string | `second` or `day` |
| `seq` | int | Global, starts at 1, gap-free. Assigned only by `lq` under flock; omit it everywhere else. |
| `recorded_at` | string | RFC 3339 UTC, when it was logged |
| `pillar` | string | Full lowercase word: `career outlook relationships economic competence recovery agents faith time`. Never initials (C and R clash). |
| `kind` | string | See below |
| `subject` | string | `skill:<slug>`, `note:<slug>` or `card:<slug>`. For `event.retracted`, the target event's id. |
| `source.via` | string | `chat`, `manual`, `monkeytype-api`, `keybr`, ... |
| `source.agent` | string | Fixed list: `lq`, `lq-mt`, `claude-code`, `hermes`, `manual`, `duet:<channel>` |
| `source.ref`, `source.note` | string | Optional |
| `data` | object | Kind-specific |
| `supersedes` | string | Optional: id of the event this one corrects |

`seq` is left out of the id hash so a remote writer can name an event before `lq` sequences it. `recorded_at` is in it, so two identical runs logged at different moments never collide, while a retried write of the same line dedupes.

Python one-liner that matches `event.NativeID` (pinned by `TestNativeIDGolden`):

```python
body = {"agent": a, "at": at, "data": data, "kind": kind, "recorded_at": rec, "subject": subj}
"evt_" + hashlib.sha256(json.dumps(body, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()[:16]
```

## Kinds

- `skill.defined` `{name, domain, metric, unit, direction, baseline}`
- `skill.protocol_set` `{mode, mode2, language, punctuation, numbers, funbox, lazyMode, min_acc, ...}`: the settings a run must match to count
- `skill.target_set` `{metric, target, min_acc?, sustain_runs? (3), sustain_days? (7)}`
- `typing.test`: Monkeytype field names `{wpm, rawWpm, acc, consistency, mode, mode2, language, punctuation, numbers, difficulty, lazyMode, funbox, testDuration, charStats, isPb, bailedOut, restartCount, incompleteTestSeconds}`. Chat-logged runs carry what a screenshot shows, plus `run`. `lq mt pull` fills in the defaults the API leaves out of stored results (`language: english`, `punctuation`/`numbers`/`lazyMode`/`bailedOut: false`, `funbox: []`, `difficulty: normal`), so every archived run states its settings.
- `typing.keystats` `{key, hits, misses, avg_ms}`: reserved; Monkeytype results carry no per-key data
- `study.session` `{minutes, subject, kind}`
- `note.added` `{path, title, sha256, from_pillar, sessions}`
- `review.logged` `{card, rating 1..4, duration_ms}`: FSRS input, later phase
- `event.retracted` `{reason}`

Latest `skill.protocol_set` and `skill.target_set` per subject win, in `seq` order.

## Corrections

Append `event.retracted` (`lq retract <id>`) or a new event with `supersedes`. Retracted import ids stay in the file, so `lq mt pull` never re-adds them.

## Remote writers: the inbox

Cloud agents (Claude Duet channels) can't take the lock. They write whole files to `$LQ_DATA/inbox/<recorded_at>-<agent>.jsonl`: one event per line, `seq` omitted, `id` optional (computed if missing). `lq ingest` validates each file; any bad line rejects the whole file and leaves it in place. Good files are appended (dedupe by id) and removed.

## Mastery (day one: performance skills)

A `typing.test` qualifies when it matches the protocol, has `acc >= min_acc` (default 97), and isn't bailed out. `best` is the best qualifying `wpm`. A skill with a target is `passed` once `sustain_runs` qualifying runs at or above target fall within `sustain_days`. Knowledge nodes (FSRS over `review.logged`) come in a later phase.
