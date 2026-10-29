# Widget contract: snapshot.json

Widgets never read `events.jsonl`. They read `$ACTA_STATE/snapshot.json` (default `~/.local/state/core-and-craft/snapshot.json`), which `acta snapshot` replaces atomically (temp file, fsync, rename), so a `file(...) json` watcher in the OWL board or a `FileView` in a bar `Service.qml` always sees a whole file.

`acta-snapshot.path` rebuilds it whenever the log changes; `acta-mt.service` rebuilds it after each Monkeytype pull.

## Shape (v1)

```json
{
  "v": 1,
  "built_at": "2026-10-29T19:00:00Z",
  "tz": "Local",
  "source": {"lines": 8, "sha256": "..."},
  "skills": [{
    "id": "skill:typing", "name": "Typing", "pillar": "competence",
    "metric": "best_wpm_at_acc", "min_acc": 97,
    "best": 88, "best_event": "evt_...", "best_at": "2026-10-29",
    "target": null, "sustain_runs": 3, "sustain_days": 7,
    "passed": false, "runs_qualifying": 2, "runs_total": 5,
    "last_at": "2026-10-29", "today_best": 88,
    "daily": [{"date": "2026-09-30", "best": null, "runs": 0}, "... 30 days, oldest first ..."]
  }],
  "today": {"date": "2026-10-29", "typing_tests": 5, "events": 8},
  "pillars": [{"pillar": "career", "events_7d": 0}, "... all nine, framework order ..."],
  "errors": []
}
```

### The game (v1, additive)

```json
{
  "quests": [{
    "id": "quest:typing-drill-1a2b", "pillar": "competence", "title": "15 min typing drill",
    "priority": 1, "every": "daily", "skill": "skill:typing",
    "due": true, "overdue_days": 2, "done_today": false, "auto": false,
    "score": 8.1, "reasons": ["due today", "priority 1"]
  }],
  "proposed": [{"id": "quest:budget-9f3c", "pillar": "economic", "title": "Review October spend", "priority": 2, "every": "once", "from": "duet:economic"}],
  "checkins": [{"id": "checkin:sleep-window-77aa", "pillar": "recovery", "question": "What's your target sleep window?", "from": "duet:recovery", "asked": "2026-10-28", "open": true}],
  "xp": {"total": 610, "level": 4, "into": 135, "need": 338},
  "streak": {"days": 4, "today": true},
  "history": [0, 15, "... 30 days of xp, oldest first, ending today"],
  "pillars": [{"pillar": "career", "events_7d": 3, "xp": 40, "level": 1, "into": 40, "need": 100}, "..."]
}
```

- **`quests`** lists active quests that aren't snoozed, plus one-off quests finished today, so they can be undone. The order is:
  1. due today, by `score`;
  2. open but not due today (a recurring quest on an off day);
  3. done today.

  Ties break by `id`. The same log always gives the same order.
- **`score`** is a plain sum. Each term is a candidate for `reasons` (the top two):
  - due today: +3
  - overdue: +0.5 per day, up to 7 days
  - priority: +(4 − priority)
  - pillar quietness: +1.5 × (1 − events_7d/7)
  - skill gap: +1 × the share of the target still missing, or 0.5 when the skill has had no qualifying run in 3 days
- **`auto`**: the quest's `rule` (`runs:N` qualifying runs or `minutes:N` of `study.session` for its skill) was met today, so it counts as done without a tap. An auto-done quest earns no quest xp, because the evidence already earned it.
- **`overdue_days`** counts back to the oldest missed due day in the run of misses just before today. One-off quests are never overdue.
- **`checkins`** lists every open check-in, plus those answered in the last 7 days (with `answered` and `answer`).
- **xp table**, applied in the snapshot (no event carries xp):

  | Source | xp |
  |---|---|
  | quest done by hand | 10 (15 at priority 1) |
  | qualifying typing run | 5 each, up to 25 per skill per day |
  | study | 1 per 5 minutes, up to 12 per pillar per day |
  | note | 15 |
  | first answer to a check-in | 5 |
  | skill target passed | 50 |
  | rest day | 0 |

- **Levels**: level *n* costs round(100 × 1.5^(n−1)) xp (100, 150, 225, 338, …), overall and per pillar.
- **`streak`** counts consecutive days with any event that isn't an import, or a `day.rested`. Today joins once it has one; until then the streak runs through yesterday.

### Display rules

- `best`, `target`, `today_best` and `daily[].best` are `null` when there's nothing yet. Render a dash, not 0.
- `daily` always has 30 entries ending today: ready for a heatmap or sparkline.
- `errors` lists invalid log lines; show a warning dot when it's non-empty.

## For a bar pill

`acta status` prints one line, e.g. `typing 88/100 wpm · 5 today`; `acta status --json` prints the snapshot.

## Tiles

`widgets/60-typing.owl` (3x2) reads `file("~/.local/state/core-and-craft/snapshot.json") json`. It shows the best qualifying wpm with the target, `runs_qualifying/runs_total`, today's best and count, a progress bar when a target is set, and a 14-character strip of the last 14 `daily` entries, one character per day (30 clipped the tile). `make check-widgets` renders every tile against the seed, the seed plus a target, and a missing snapshot.

A tile reads the default path. A board running with `ACTA_STATE` set elsewhere has to edit the tile's path.
