# Board mockups

These are static HTML mockups of the CORE & CRAFT board, made before any OWL is written, as decision 0001 asks. Open any `index.html` in a browser.

Each page shows Tokyo Night and Gruvbox, an empty, normal and busy state, a tap map with the exact `acta` commands, and build notes. All data on them is synthetic.

## The pick

**[Duae Insulae](duae-insulae/index.html)** was picked on 2026-10-29. It shows the town as an isometric map that fills the board: C.O.R.E. and C.R.A.F.T. sit on two islands joined by a bridge, each pillar is a tower that gains a storey per level, and its Castra week lies in front as seven floor tiles.

- [`duae-insulae/spec.md`](duae-insulae/spec.md) is the build spec: sizes, node mapping, drawing order, taps and theming rules.
- `duae-insulae/preview/` has PNGs of the map.

## How we got there (reference)

| Page | Round | What it shows |
|---|---|---|
| [`reference/tiles-five/`](reference/tiles-five/index.html) | 1 to 3 | Five tile directions at board size: Forum, Tabularium, Hora, Castra and Annales. Harlan chose Forum with Castra's week blocks. |
| [`reference/maps-side/`](reference/maps-side/index.html) | 4 | That direction as a full-board map, three ways, in side view: Duo Fora, Clivus and Rota. |
| [`reference/maps-above/`](reference/maps-above/index.html) | 5 | The same map from above or in isometric: Vicus (text glyphs), Duae Insulae (Canvas shapes) and Tesserae (pixel tiles). Duae Insulae was picked. |

The Roman framing rule from round 3 applies to every page. Pillars use their spec titles in English, under C.O.R.E. and C.R.A.F.T. Latin appears only in eyebrows, as "Latin · English". Roman numerals are for levels only.

## Regenerating

The pages are built by the Python scripts in `generator/`. They use the standard library only, except for `prev.py`, which needs Pillow.

```sh
cd docs/mockups/generator
python3 -I pick.py         # duae-insulae/index.html
python3 -I maps_page2.py   # reference/maps-above/
python3 -I maps_page.py    # reference/maps-side/
python3 -I compare.py      # reference/tiles-five/
python3 -I prev.py tokyo-night duae-insulae && python3 -I prev.py gruvbox duae-insulae   # preview PNGs
```

The synthetic snapshots are in `generator/data.py` and `generator/mapkit.py`.
