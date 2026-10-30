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

### The knowledge graph (v1, additive)

```json
"graph": {"nodes": 74, "edges": 205, "notes": 64, "stubs": 0, "new_7d": 64, "errors": 0}
```

`acta snapshot` also writes `$ACTA_STATE/graph.json` beside this file, the same way (temp file and rename): every node and edge. `new_7d` counts notes first logged in the last seven days, today included. Its shape is in `docs/knowledge-graph.md`. A widget that draws the graph reads `graph.json`; one that only shows counts reads this summary.

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
  "asks": [{"id": "ask:which-meeting-can-go-3c1d", "pillar": "time", "text": "Which meeting can go this week?", "asked": "2026-10-29", "open": false, "answered": "2026-10-29", "answer": "The Thursday sync.", "from": "duet:time"}],
  "xp": {"total": 610, "level": 4, "into": 135, "need": 338},
  "streak": {"days": 4, "today": true},
  "history": [0, 15, "... 30 days of xp, oldest first, ending today"],
  "pillars": [{"pillar": "career", "events_7d": 3, "today": 1, "xp": 40, "level": 1, "into": 40, "need": 100}, "..."],
  "leveled_today": ["competence"]
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
- **`asks`** lists Harlan's questions to the pillar agents (`pillar.asked`), newest first: every open one, plus those answered in the last 7 days (with `answered`, `answer` and the answering agent in `from`). They earn no xp.
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
- **`pillars[].today`** counts the events dated today in that pillar (`today.events` is their sum). The town lights one window per event and colors the day's floor tiles with it.
- **`leveled_today`** names the pillars whose level today's xp raised, in framework order, `[]` when none. The town puts a ✦ on their signs.
- **`streak`** counts consecutive days with any event, or a `day.rested`. A Monkeytype run pulled by `acta-mt` counts, as it earns xp; a run logged by hand and repeated by an import is one run, so it adds neither a streak day nor xp twice. Today joins once it has one; until then the streak runs through yesterday.

### Display rules

- `best`, `target`, `today_best` and `daily[].best` are `null` when there's nothing yet. Render a dash, not 0.
- `daily` always has 30 entries ending today: ready for a heatmap or sparkline.
- `errors` lists invalid log lines; show a warning dot when it's non-empty.

## For a bar pill

`acta status` prints one line, e.g. `typing 88/100 wpm · 5 today`; `acta status --json` prints the snapshot.

## Tiles

`widgets/60-typing.owl` (3x2) reads `file("~/.local/state/core-and-craft/snapshot.json") json`. It shows the best qualifying wpm with the target, `runs_qualifying/runs_total`, today's best and count, a progress bar when a target is set, and a 14-character strip of the last 14 `daily` entries, one character per day (30 clipped the tile). `make check-widgets` renders every tile against the seed, the seed plus a target, and a missing snapshot.

A tile reads the default path. A board running with `ACTA_STATE` set elsewhere has to edit the tile's path.

`widgets/10-town.owl` (6x3) is Provinciae (the Duae Insulae mockup), the town: two provinces joined by the Pons, the bridge. It reads the same snapshot plus `source now = clock` (day, dusk or night on the map). The tile is a still of the two islands from the same renderer, scaled to the tile and framed on the islands: no sailboats or labels, just the towers and the bridge (lit while the Pax, the streak, holds), with the overall level numeral, its xp bar and the streak beside it. A tap opens `screen { }`: one Canvas with the two islands, over a rail with today's route, level, streak and check-ins. Spec: `docs/mockups/duae-insulae/spec.md` (kept as the reference; the widget differs in three places: the streak labels sit above the bridge, not under it, so they clear the Time sign and the boat; the rail's right column is 148 dp, not 196 px; the history sparkline is hidden while every day is zero).

- **Map:** every shape is built once in `let`s, in px at the brief's grid (582 x 470) and in isometric lot space, and the Canvas only walks the list, scaled by `width / 582`. Lots are drawn back to front and every label after the towers. It stays under the 2,000 drawing calls a frame OWL allows (about 480 on a busy day, 660 at night in a storm).
- **Taps:** each lands on a lot, a boat, a `?` bubble or "rest day ›", or on the rail. A lot opens the pillar Popup, a boat or "review" the proposal Popup, a `?` or "? N waiting" the check-in Popup. Every write is `run("acta ... --via board")` with ids and text through `sh()`; `acta` has to be on the board's `PATH`. `board/test/taps.js` runs each tap against the command the spec gives it.
- **Legatus, the pillar's agent:** a lot's Popup ends with "ask <Pillar>'s agent": the latest ask for that pillar, its answer or "waiting for the answer", and an Input with "ask", which runs `acta ask --pillar <id> --text <question> --via board`. `acta relay` sends it to the pillar's Duet channel, and the answer comes back through the inbox (`docs/duet-relay.md`). A typed question or check-in answer belongs to its Popup: closing it, or opening another lot or check-in, clears it, so it never goes to another pillar.
- **Input:** "add a quest  #Pillar" takes a pillar name or any unique start of one (`#eco`). Without a tag, or with one that fits two pillars (`#re`), nothing runs and the rail says why. The Input's font is the shell's own; OWL doesn't let a widget set it.
- **Board.qml** grows a board with fewer than 8 rows to 8 while a screen is open, and hides the other tiles. The rail's list scrolls when the board is shorter than the brief's.
- **Motion and clicks:** anything with an `onTap` (and a tile that opens a screen) gets a pointing hand, a wash on hover and a deeper one while pressed, over 120 ms; a tile's frame turns accent on hover. A screen fades in over 160 ms from 97.5% and fades out the same way, Esc or a right click closes it, and the other tiles fade out and back. Popups rise in over 140 ms. Progress bars spring, and a quest dims at once when you tap done, snooze or drop, until the snapshot arrives. The map itself is still: a `time`-driven Canvas would repaint 60 times a second on a board that sits on the desktop.
