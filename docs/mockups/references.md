# References for the board mockups

What the mockups in this folder were drawn from. The "In this repo" paths are relative to the repo root.

## In this repo

| What | Where | Used for |
|---|---|---|
| Decision 0001 | `docs/decisions/0001-core-and-craft-migration.md` | Nine-pillar town, tiles chosen from mockups, the binding Roman brand |
| Widget contract | `docs/widget-contract.md` | Every field the tiles and the map read, including the game fields from PR 9 |
| Event envelope | `docs/event-envelope-v1.md` | The event kinds behind each tap |
| acta commands | `README.md`, the command table | The exact tap commands: `acta quest`, `acta checkin`, `acta rest` |
| OWL runtime | `board/plugin/owl/Owl.js` | The language as built: nodes, Canvas calls, actions (`run`, `screen`, `close`), builtins (`sh`). Read it over the upstream docs, which trail it |
| OWL node layout | `board/plugin/owl/OwlLayout.js` | Text sizes in dp: huge 44, title 20, body 13, small 11, muted 13, label 11 |
| Board | `board/plugin/Board.qml` | The grid (96 px pitch with a 6 px gap), the 0.88 tile background, the 2 dp muted frame, and the `screen { }` layer the town opens |
| Desktop placement | `board/plugin/Desktop.qml` | `board.json` settings: side, columns, cell, margin, opacity |
| Built typing tile | `widgets/60-typing.owl` | Kept unchanged on every board |

## Upstream

| What | Where |
|---|---|
| OWL language | https://github.com/SimonSchubert/omarchy-widgets/blob/main/docs/LANGUAGE.md |
| Tokyo Night palette | https://github.com/basecamp/omarchy/blob/master/themes/tokyo-night/colors.toml |
| Gruvbox palette | https://github.com/basecamp/omarchy/blob/master/themes/gruvbox/colors.toml |

The mockups map each theme's `colors.toml` the way `Owl.themeFromToml` does, with `surface` taken from `lighter_background`. Only the named colors appear: accent, foreground, muted, green, red, yellow, orange, blue, magenta, cyan, background and surface.

## Inspiration

- Stardew Valley's top-down town, for the views from above (Vicus and Tesserae).
- Isometric city builders, for Duae Insulae.
- The Roman camp plan (castra) with its insulae, for the week blocks.

No assets were taken from any of these.
