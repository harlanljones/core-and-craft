# 0004: FSRS reviews with go-fsrs, folded from review.logged

Status: proposed, 2026-10-31

## Context

0003 made every note with a `recall:` line a future card, and `review.logged` was already in the envelope. Harlan's rule for each mechanic is to prove it on real data first, then build it through log, acta, snapshot, board and docs. The seed has 13 notes with `recall:` lines.

## Decision

- **go-fsrs v3.3.1, pinned.** It's the repo's first outside dependency, and the roadmap already named it. Its scheduler is the reference implementation for Go, so acta's fold is tested against go-fsrs's own reference sequence instead of a port that could drift. v4 needs Go 1.26; the repo is on Go 1.24, so v3 it is until the toolchain moves.
- **Long-term scheduler, no fuzz.** Reviews come a day at a time, from the board or a quiz in chat, so same-day learning steps don't fit. Fuzz off keeps the fold deterministic.
- **Day granularity.** A review counts at noon on its local date, so a chat review (a day) and a board tap (a second) schedule alike, and due dates are whole days.
- **No new event kind, no card registry.** A card exists because a note's `recall:` line says so. A review of a card no note names is kept, earns nothing, and is listed as a problem, so a quiz filed before its note arrives isn't lost.
- **Xp.** 2 for the first review of a card each day, capped at 20 a day per pillar; 10 once when a card matures (stability of 21 days). Both need the note in the log.

## Alternatives rejected

- **Port the formulas to stay stdlib only.** About 150 lines, but every FSRS release would need re-porting by hand, and the reference values would be ours, not upstream's.
- **The short-term scheduler.** Its minute-scale learning steps want an app open all day; here a card answered Again simply comes back tomorrow.
- **Rejecting reviews of unknown cards at ingest.** One bad line rejects a whole file, and a channel's note body can arrive after its quiz.

## Consequences

- `go.sum` now matters; CI and diocletian fetch the module through the Go proxy on first build.
- `acta review` works on the command line now. The board's buttons come with the Library tile, after its mockup.
