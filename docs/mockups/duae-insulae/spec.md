# Duae Insulae: build spec for the town

The town as an isometric map that fills the board. Harlan picked it on 2026-10-29 from three views from above (`../reference/maps-above/`). The mockup is `index.html` here; `preview/` has PNGs of the map in both themes and all three states.

This spec is for whoever writes the OWL. Mockup coordinates are in px at the brief's grid. 1 dp = 96/69 px, so divide px by 1.391 to get dp.

## Where it lives

- One widget holds both the town tile on the board and the map in its `screen { }` block.
- Tapping the tile runs `screen()`. Board.qml then draws the screen over the whole board, at `background` 0.96 with a 12 dp margin, until `close()` runs. The other tiles don't show while it's open.
- A screen allows up to 1,000 nodes. The map is one `Canvas`, so its drawing calls don't count as nodes. The rail around it is about 60 nodes.
- **Open:** the 6x3 tile on the board has no design yet. The cheapest option is the same Canvas, scaled to the tile, without the signs.

## Size

| | Brief grid (96 px cells, 8 px gap) | Board.qml today (96 px pitch, 6 px gap) |
|---|---|---|
| Board, 6 × 8 | 616 × 824 px | 570 × 762 px |
| Screen content (12 dp margin) | 582 × 790 px | 537 × 729 px |

- The mockup is drawn at the brief's grid. Board.qml's pitch of 96 includes the gap, which makes a 6-wide tile 570 px, not 616.
- Scale the Canvas drawing by `width / 418` (418 dp is 582 px), so it fits either board.
- Text inside the Canvas scales with it. Keep it at 8 dp or more.

## Layout of the screen

`Column { gap: 8 }`, holding:

1. **Header `Row`**
   - `Text` eyebrow "Duae Insulae · two islands" (style label, weight 1).
   - A red `●` when `errors` isn't empty.
   - Status `Text`: "`HH:MM` day|dusk|night · clear|drizzle|storm · ? N".
   - `Button` "✕" that runs `close()`.
2. **`Canvas`**: the map, 582 × 470 px in the mockup (418 × 338 dp).
3. **The rail, a `Row`** (gap 14) with two parts:
   - Left `Column`, weight 1:
     - Label "Iter · today's route" and "N due".
     - Up to four open quests in rank order. Each is a Row with "@" or the stop number, the title (maxLines 1), the pillar title, "Nd" in red when overdue, ✓ and ⋯. Under it is a muted line with the top two `reasons`.
     - "+N more ↓" when there are more.
     - Done-today quests, struck through, each with ↺.
     - "◇ N proposed, moored off the islands", with a "review" tap.
     - An `Input` with the placeholder "add a quest  #Pillar".
   - Right `Column`, 196 px:
     - Label "Gradus · level".
     - The overall level as a Roman numeral (style title, accent), with "into/need xp" beside it.
     - A `Progress` bar.
     - "streak · N days", then "today kept" in green or "rest day ›" in accent.
     - A `Sparkline` of `history`.
     - Label "Rogationes · check-ins", then "? N waiting" in magenta and the waiting pillars' titles (maxLines 2).

## The map (one Canvas)

### Geometry

- Isometric lots with tile width TW = 116 and height TH = 58. A lot at (i, j) is drawn at x = 166 + (i − j) · 58, y = 214 + (i + j) · 29.
- Lot pitch G = 1.32 lots.
- Lots, as (i, j) before multiplying by G:

| C.O.R.E. island | | C.R.A.F.T. island | |
|---|---|---|---|
| Career | (0, 0) | Competence | (2.6, −1) |
| Outlook | (1, 0) | Recovery | (3.6, −1) |
| Relationships | (0, 1) | Agents | (2.6, 0) |
| Economic | (1, 1) | Faith | (3.6, 0) |
| | | Time | (3.1, 1) |

### Drawing order

1. Moon or sun and stars. Night is 19:00 to 06:00 and dusk 17:00 to 19:00, both by the `clock` source.
2. Water strokes in `blue` at 0.3.
3. Islands: each lot's diamond, grown by 0.16, filled `surface`. Under its two front edges is a 14 px cliff, `muted` on the left and `muted` at 0.55 on the right.
4. The bridge from Outlook's lot to Agents' (i from Outlook + 1.1 to Agents − 0.12, j + 0.38 to 0.62). It is `yellow` when `streak.today` is true, otherwise `muted`.
5. Boats, one per `proposed[]`, in `cyan`, off the front corner of their pillar's lot.
6. Every lot, in order of i + j (back to front):
   - A dashed `muted` diamond: Castra's frame.
   - The week: seven small floor tiles along the front right. They are `yellow` for each event today, `green` for each earlier event in the 7 days, and `muted` for the rest.
   - The tower: a box on the back half of the lot, with footprint i + 0.14, j + 0.1 and size 0.46. Its height is 16 + min(7, level) · 14 px. The left face is `muted`, the right face `muted` in a `layer(0.55)`, and the top `surface` with a `muted` outline.
   - Windows: two per storey on the left face, `yellow` for today's events and `background` at 0.7 for the rest.
   - A roof mark per pillar:
     - Career: a mast.
     - Outlook and Recovery: a dome.
     - Relationships and Faith: a pitched roof.
     - Economic: three columns.
     - Competence: a block.
     - Agents: a chimney, with smoke on a day with events.
     - Time: a clock face.
   - An accent dot for @ on the lot of the top-ranked quest.
7. Every label, after all the towers, so nothing hides one:
   - A sign 18 px tall over each tower, in `background` at 0.8 with a `muted` outline. It holds the pillar's spec title with its first letter in accent and the level numeral in accent. A yellow ✦ follows when the pillar levelled up today.
   - The route stop number in accent, left of the sign.
   - A magenta ? bubble, right of the sign, while a check-in is open.
   - "@" beside the dot.
   - "streak N" and "today kept" or "rest day ›" under the bridge.
8. Rain over everything, in `blue`. One overdue quest gives a drizzle (about 40 strokes) and two or more give a storm (about 110).

### Hit testing

A tap gives (x, y). Undo the iso projection and take the lot whose diamond holds the point. Boats, the bridge's "rest day ›" and ? bubbles get small boxes of their own, tested first.

## Taps

Every write goes through `run()`, with values from the snapshot and shell quoting from `sh()`. `--via board` marks where the write came from. These commands are on main as of PR 8.

| Tap | Does | Runs |
|---|---|---|
| Town tile on the board | open the map | `screen()` |
| ✕ | close the map | `close()` |
| ✓ on a quest | done | `acta quest done ${q.id} --via board` |
| ↺ on a done quest | undo | `acta quest undo ${q.id} --via board` |
| ⋯ → snooze 1d / 3d / 7d | snooze | `acta quest snooze ${q.id} --until YYYY-MM-DD --via board` |
| ⋯ → priority ↑ / ↓ | priority | `acta quest edit ${q.id} --priority 1\|3 --via board` |
| ⋯ → drop | drop | `acta quest drop ${q.id} --via board` |
| A boat, or "review" | proposal Popup | (Popup) |
| Proposal · accept | approve | `acta quest accept ${p.id} --via board` |
| Proposal · decline | turn down | `acta quest decline ${p.id} --via board` |
| Input · Enter | add; `#Pillar` picks the pillar | `acta quest add --pillar ${pillar} --title ${sh(title)} --via board` |
| rest day › | rest day | `acta rest --via board` |
| ? bubble, or "? N waiting" | check-in Popup | (Popup) |
| Check-in · send | answer | `acta checkin answer ${c.id} --text ${sh(text)} --via board` |
| Check-in · later | close it; it stays open | — |
| A lot | pillar Popup: level, xp, 7-day count, open quests, "add a quest" | (Popup) |

- Popups need `onDismiss`.
- Accept and decline run as `manual`, which is the default, because acta refuses them from any other agent.

## Data

The map reads `snapshot.json` as in `docs/widget-contract.md`:

- `quests`, `proposed`, `checkins`
- `xp`, `streak`, `history`
- `pillars[].level`, `events_7d`
- `errors`

The clock comes from `source now = clock`.

Two fields aren't in the contract yet. These mockups propose them:

- `pillars[].today`: events today per pillar. It drives the lit windows and the yellow week tiles. Until it exists, light no windows and draw the week from `events_7d` alone.
- `leveled_today`: the pillars that levelled up today, for the ✦.

## Theming rules (binding, from the mockup rounds)

- Pillars read as the spec writes them, in English: Career, Outlook, Relationships, Economic, Competence, Recovery, Agents, Faith, Time. They are grouped under C.O.R.E. and C.R.A.F.T., with each title's first letter in accent.
- Latin only in eyebrows, as "Latin · English". There are no Latin pillar names, even as subtitles.
- Roman numerals for levels only. Xp, counts, ranks and days stay in digits.
- Only the theme's named colors: `accent`, `foreground`, `muted`, `green`, `red`, `yellow`, `blue`, `magenta`, `cyan`, `background` and `surface`. Color means state, not identity:
  - `yellow` is lit or today.
  - `accent` is you and the top pillar.
  - `magenta` is a question.
  - `cyan` is a proposal.
  - `red` is overdue or an error.
  - `green` is done or kept.
- The mono font for every Text in the rail and on the map's signs. The map itself is shapes, not glyphs. That departs from the ASCII house style, as agreed with the pick.

## Known issues on the board

- **Pitch:** Board.qml uses a 96 px pitch that includes a 6 px gap. The brief's grid is 96 px cells plus 8 px gaps. Everything here is drawn at the brief's grid, so scale it as above.
- **Typing tile:** the built typing tile's 30-day strip is 30 characters at small, about 275 px. Its inner width is 262 px at the brief's grid and 248 px on the current board, so the strip clips. This is unchanged by this work.
