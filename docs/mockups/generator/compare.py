"""One page: all five directions side by side."""
import json, os, sys
sys.path.insert(0, os.path.dirname(__file__))
import common
from common import esc, THEMES, CSS, theme_css
from data import STATES
import forum, tabularium, hora, castra, annales
from build import RULE

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "reference", "tiles-five", "index.html")

D = [
    dict(m=forum, L="A", name="Forum", en="the town is the board", rows=8, new=False,
         pitch="An ASCII skyline of nine buildings leads. Check-ins are ? flags on it; tap the building to answer.",
         s="Most character; pure ASCII; check-ins can't be missed on the skyline.",
         r="The 77-column town needs exact font metrics; answering takes two taps.",
         c="High: one large text Canvas, then simple tiles."),
    dict(m=tabularium, L="B", name="Tabularium", en="every tile is a ledger", rows=10, new=False,
         pitch="Ranks, scores, numeral levels and 7-day counts everywhere. Every action sits on its row; check-ins get a tile.",
         s="Shows every snapshot field; nothing behind a tap; easy to audit acta's arithmetic.",
         r="8–9 dp text, 10 px tap targets, a scoreboard feel; 10 rows with a free 3x2.",
         c="Highest: the same Canvas plus a ~110-node quests tile and the most taps."),
    dict(m=hora, L="C", name="Hora", en="low ink, all day", rows=8, new=False,
         pitch="A quiet silhouette in the surface color. Three quests; proposals and check-ins share one waiting Popup.",
         s="Calmest under windows; large type; fewest nodes.",
         r="Shapes, not characters; questions can sit unseen behind one tap.",
         c="Lowest: ~150 shape calls and ~40 nodes."),
    dict(m=castra, L="D", name="Castra", en="the camp from above", rows=8, new=True,
         pitch="A top-down Roman camp: nine insulae on a 3×3 grid, each showing the whole week. Quests are a route with @ at the next stop; level is a milestone.",
         s="A week per pillar at a glance (▪ today, ▫ this week); the 3×3 grid fits nine pillars; route and milestone read instantly.",
         r="No height, so level is only a numeral; less of a town, more of a map; same 77-column width.",
         c="Medium-high: a text Canvas whose layout never moves, so only cells change."),
    dict(m=annales, L="E", name="Annales", en="the board as a chronicle", rows=8, new=True,
         pitch="A small skyline over five sentences that say what happened and what waits. Quests are sentences; actions are plain words; no Popups.",
         s="Explains itself: the ranking reasons become the sentence; the check-in answer box is on the town; no Popups.",
         r="Prose scans slower than numbers; plurals and lists are fiddly in OWL; long titles clip.",
         c="Medium: a 7-row Canvas and Flow sentences. The wording logic is the work."),
]

GIT = """<h3>What git showed (main at 2f29d48, read only)</h3><ul>
<li>Four PRs merged since the first pass, 13:28 to 13:53 PT: the board and the CLI renamed (<code>lq</code> is now <code>acta</code>, paths under <code>core-and-craft</code>), an install fix, and new event kinds for quests, check-ins and rest days with a recurrence DSL.</li>
<li><code>docs/decisions/0001</code> is accepted: a nine-pillar town replaces the old six-place one, tiles are picked from mockups before they're built, and the Roman brand is binding.</li>
<li>acta has no quest verbs yet, so every tap below is an exact <code>acta log &lt;kind&gt;</code> command. Priority is 1–3, snooze takes a date, undo retracts the done event, and there's no skip kind for check-ins. A new rest-day tap uses <code>day.rested</code>.</li>
<li>Unchanged: the typing tile (path only), the widget contract's shape, and Board.qml's 96 px pitch with a 6 px gap. No mockups or PRODUCT.md in the repo.</li></ul>"""

EXTRA_CSS = r"""
.ctrl{display:flex;flex-wrap:wrap;gap:8px 18px;align-items:center}
.seg{display:inline-flex;border:1px solid var(--rule);border-radius:999px;overflow:hidden}
.seg input{position:absolute;opacity:0;pointer-events:none}
.seg label{padding:4px 12px;font:600 13px var(--sans);cursor:pointer;color:var(--ink-2)}
.seg input:checked+label{background:var(--ink);color:var(--paper)}
.seg input:focus-visible+label{outline:2px solid var(--link);outline-offset:-2px}
.lane-wrap{overflow-x:auto;margin:0 calc(-1 * max(16px,env(safe-area-inset-left,0px)));padding:4px max(16px,env(safe-area-inset-left,0px)) 12px}
.lane{display:flex;gap:28px;width:max-content;align-items:flex-start}
.dir{width:max-content;display:flex;flex-direction:column;gap:10px}
.dir-h{max-width:640px}
.dir-h .tag{display:inline-block;font:700 12px var(--sans);letter-spacing:.1em;padding:2px 8px;border-radius:4px;background:var(--chip);margin-right:6px}
.dir-h .tag.new{background:var(--ink);color:var(--paper)}
.dir-h h2{display:inline}
.dir-h h2 small{font:italic 400 16px var(--serif);color:var(--ink-2);margin-left:8px}
.dir-h p{margin:6px 0 0;font-size:14.5px;max-width:62ch}
.v,.pv{display:none}
.page:has(#zoom:checked) .lane .board-wrap{zoom:.42}
.page:has(#zoom:checked) .dir,.page:has(#zoom:checked) .dir-h{width:270px}
.page:has(#zoom:checked) .dir-h p{display:none}
.cmp{overflow-x:auto}
.cmp table{min-width:760px}
.cmp td:first-child{white-space:nowrap;font-weight:600}
.det{margin:28px 0}
.det>details{margin:10px 0}
.det>details>summary{font:600 20px var(--serif);padding:4px 0}
.det .lede{margin:6px 0 12px}
"""


def variant_rules():
    out = []
    for th in THEMES:
        for st, _l, _s in STATES:
            out.append(f'.page:has(#th-{th}:checked):has(#st-{st}:checked) .v.th_{th}.st_{st}{{display:block}}')
        out.append(f'.page:has(#th-{th}:checked) .pv.th_{th}{{display:block}}')
    return "\n".join(out)


def lane():
    cols = []
    for d in D:
        boards = []
        for th in THEMES:
            for st, label, s in STATES:
                boards.append(f'<div class="v th_{th} st_{st}">{d["m"].build_board(th, s, st, th)}</div>')
        cols.append(f'''<div class="dir" id="lane-{d["m"].SLUG}"><div class="dir-h">
<span class="tag{" new" if d["new"] else ""}">{d["L"]}{" · new" if d["new"] else ""}</span><h2>{esc(d["name"])}<small>{esc(d["en"])}</small></h2>
<p>{esc(d["pitch"])}</p></div>{"".join(boards)}</div>''')
    return f'<div class="lane-wrap"><div class="lane">{"".join(cols)}</div></div>'


def table():
    rows = "".join(f'<tr><td>{d["L"]} · {esc(d["name"])}</td><td>{esc(d["s"])}</td><td>{esc(d["r"])}</td>'
                   f'<td>{esc(d["c"])}</td><td>{d["rows"]}</td></tr>' for d in D)
    return (f'<div class="cmp"><table><thead><tr><th>Direction</th><th>Strengths</th><th>Risks</th><th>OWL build cost</th>'
            f'<th>Rows</th></tr></thead><tbody>{rows}</tbody></table></div>')


def details():
    out = []
    for d in D:
        m = d["m"]
        pops = "".join(f'<div class="pv th_{th}">{m.popups(th)}</div>' for th in THEMES)
        out.append(f'''<details id="d-{m.SLUG}"{" open" if d["new"] else ""}><summary>{d["L"]} · {esc(d["name"])}: popups, tap map, build notes</summary>
<p class="lede">{esc(d["pitch"])}</p>
<h3>Popups</h3>{pops}
<h3 style="margin-top:16px">Tap map</h3><div class="maps">{m.tapmaps()}</div>
<h3 style="margin-top:16px">Build notes</h3><div class="notes">{m.NOTES}</div></details>''')
    return "".join(out)


def radios(name, items, default):
    return '<span class="seg" role="radiogroup">' + "".join(
        f'<input type="radio" name="{name}" id="{name}-{v}"{" checked" if v == default else ""}><label for="{name}-{v}">{esc(l)}</label>'
        for v, l in items) + '</span>'


def main():
    data_html = "".join(
        f'<details><summary>{esc(label)} snapshot (synthetic)</summary><pre>{esc(json.dumps(s, indent=1, ensure_ascii=False))}</pre></details>'
        for key, label, s in STATES)
    html = f"""<title>CORE &amp; CRAFT Board Directions</title>
<meta name="claude:viewport-fit-sides" content="cover">
<style>{CSS}
{EXTRA_CSS}
{theme_css()}
{variant_rules()}</style>
<div class="page">
<header>
  <div class="eyebrow">CORE &amp; CRAFT · Tabulae · desktop tiles · five directions</div>
  <h1>Five boards, side by side</h1>
</header>
<div class="intro"><div class="lede">
<p>Forum, Tabularium and Hora from the first round, plus two new ones: Castra, the town seen from above as a Roman camp, and Annales, the board as a written chronicle. Every board is drawn at the brief's grid size with the same synthetic data, so they differ only in how they show it.</p>
<p>Switch the theme and the state for all five at once. "Fit five" shrinks the boards to see them together; at 100% they're real size, and the row scrolls sideways. Below the boards: a comparison, then each direction's Popups, tap map with the exact acta command, and build notes.</p>
</div><aside class="aside">{RULE}{GIT}</aside></div>
<div class="bar"><div class="ctrl">
  {radios("th", [(t, t) for t in THEMES], "tokyo-night")}
  {radios("st", [(k, l) for k, l, _ in STATES], "normal")}
  <label for="zoom"><input type="checkbox" id="zoom"> Fit five</label>
  <label for="nodes"><input type="checkbox" id="nodes"> Show OWL nodes</label>
</div></div>
{lane()}
<section class="block" id="compare"><h2>Comparison</h2>
<p class="lede">The first three are as before, with taps updated to today's acta. Castra and Annales are new. My pick is still Forum: it keeps the ASCII house style, and its town does the most work.</p>
{table()}</section>
<section class="block det" id="details"><h2>Per direction</h2>
<p class="lede">Taps run through <code class="mono">run()</code>, with ids from the normal state. {common.CMD_NOTE}</p>
{details()}</section>
<section class="block"><h2>Synthetic data</h2>
<p class="lede">The same three snapshots drive all five boards. <code class="mono">pillars[].today</code>, <code class="mono">quests[].done_event</code> and <code class="mono">clock</code> are proposals of these mockups; the rest follows the brief's shape.</p>{data_html}</section>
<footer>Mockup only, no OWL. Every number is synthetic. Tiles use the brief's grid: 96 px cells, 8 px gaps, 0.88 opacity, a 2 dp muted frame, and OwlLayout.js text sizes at 1 dp = 96/69 px. The current Board.qml uses a 96 px pitch with a 6 px gap, which makes a 6-wide tile 570 px instead of 616.</footer>
</div>
{common.JS}"""
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    open(OUT, "w").write(html)
    print(len(html))


main()
