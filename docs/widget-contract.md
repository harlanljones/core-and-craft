# Widget contract: snapshot.json

Widgets never read `events.jsonl`. They read `$LQ_STATE/snapshot.json` (default `~/.local/state/lifequest/snapshot.json`), which `lq snapshot` replaces atomically (temp file, fsync, rename), so a `file(...) json` watcher in the OWL board or a `FileView` in a bar `Service.qml` always sees a whole file.

`lifequest-snapshot.path` rebuilds it whenever the log changes; `lifequest-mt.service` rebuilds it after each Monkeytype pull.

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

- `best`, `target`, `today_best` and `daily[].best` are `null` when there's nothing yet. Render a dash, not 0.
- `daily` always has 30 entries ending today: ready for a heatmap or sparkline.
- `errors` lists invalid log lines; show a warning dot when it's non-empty.

## For a bar pill

`lq status` prints one line, e.g. `typing 88/100 wpm · 5 today`; `lq status --json` prints the snapshot.

## Tiles

`widgets/60-typing.owl` (3x2) reads `file("~/.local/state/lifequest/snapshot.json") json`. It shows the best qualifying wpm with the target, `runs_qualifying/runs_total`, today's best and count, a progress bar when a target is set, and a 30-character strip of `daily`, one character per day. `make check-widgets` renders every tile against the seed, the seed plus a target, and a missing snapshot.

A tile reads the default path. A board running with `LQ_STATE` set elsewhere has to edit the tile's path.
