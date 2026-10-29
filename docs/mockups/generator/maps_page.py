"""One page: three full-screen town maps, Forum with Castra, side by side."""
import json, os, sys
sys.path.insert(0, os.path.dirname(__file__))
import common
from common import esc, THEMES, CSS, theme_css, tapmap
from mapkit import STATES, screen, popups, CMD
from maps_concepts import CONCEPTS

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "reference", "maps-side", "index.html")

RULES = """<h3>Rules kept from the tiles</h3><ul>
<li>Pillars read as the spec writes them, in English, under C.O.R.E. and C.R.A.F.T., with each acronym letter in the accent color.</li>
<li>Latin only in eyebrows, as "Latin · English". Roman numerals for levels only; xp, counts and days in digits.</li>
<li>Only the theme's named colors, the mono font, and OWL's nodes: one Canvas for the map, Rows, Columns, Texts, a Progress, a Sparkline, an Input, Buttons and Popups around it.</li>
<li>From Forum: the nine buildings, lit windows for today, @ at the top quest, ? for a check-in, night by the clock, rain for overdue quests.</li>
<li>From Castra: every building stands on its insula block, which shows the level, the week (▪ today, ▫ earlier in 7 days) and the 7-day count; the route's stops are numbered on the roofs.</li></ul>"""

GIT = """<h3>What git showed (main at 586fd1e, read only)</h3><ul>
<li>PR 8 added <code>acta quest</code>, <code>acta checkin</code> and <code>acta rest</code>, so every tap below is now a real verb with <code>--via board</code>, and undo finds the done event itself.</li>
<li>PR 9 put the game into <code>snapshot.json</code>: ranked quests with score and reasons, proposals, check-ins, xp, levels, streak, 30 days of history, and per-pillar level, xp and <code>events_7d</code>.</li>
<li>Board.qml already has the hook for this: a widget's <code>screen { }</code> covers the board until <code>close()</code>, drawn at 0.96 over the tiles with a 12 dp margin and up to 1,000 nodes. Its comment says "The town uses this."</li></ul>"""

FULL = """<h3>What "full screen" means here</h3>
<p>The town tile stays the 6x3 Forum tile on the board. Tapping it runs <code>screen()</code>, and the map covers the whole board, 6 columns by 8 rows (616 by 824 px at the brief's grid), until <code>✕</code> runs <code>close()</code>. The other tiles can't show while it's open, so their essentials are tucked into a rail under the map: the route, level and streak, check-ins and the add box.</p>
<p>If you meant the whole monitor, the board's <code>columns</code> setting would have to grow first, and these would be laid out wide.</p>"""

D = [
    dict(c=CONCEPTS[0], blurbs="Two skylines, one per acronym, with the river between."),
    dict(c=CONCEPTS[1], blurbs="One skyline on a hill, where level is altitude."),
    dict(c=CONCEPTS[2], blurbs="A walled camp read clockwise, Time at the hub."),
]

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
.dir-h h2{display:inline}
.dir-h h2 small{font:italic 400 16px var(--serif);color:var(--ink-2);margin-left:8px}
.dir-h p{margin:6px 0 0;font-size:14.5px;max-width:62ch}
.v,.pv{display:none}
.page:has(#zoom:checked) .lane .board-wrap{zoom:.6}
.page:has(#zoom:checked) .dir,.page:has(#zoom:checked) .dir-h{width:386px}
.page:has(#zoom:checked) .dir-h p{display:none}
.sboard{padding:12px;width:max-content;background:var(--wall);
  background-image:radial-gradient(circle at 20% 0%,color-mix(in srgb,var(--surface) 55%,transparent),transparent 60%)}
.screen{box-sizing:border-box;width:616px;height:824px;padding:16.7px;display:flex;flex-direction:column;
  background:color-mix(in srgb,var(--background) 96%,transparent);box-shadow:inset 0 0 0 3px var(--muted);
  color:var(--foreground);font-family:var(--mono);font-variant-numeric:tabular-nums;overflow:hidden}
.screen>.k-column{flex:1 1 auto;min-height:0}
.screen .k-row>[data-n="Text"]:first-child.w1{flex:1 1 0}
.mapc{flex:0 0 auto}
.rail{min-height:0;overflow:hidden}
.k-button{font-size:15.3px;line-height:1.17;padding:0 7px;background:var(--surface)}
.screen .tap:hover{text-decoration:underline;text-decoration-color:var(--muted)}
.popup.st{position:relative;left:auto;top:auto}
.pops{display:flex;flex-wrap:wrap;gap:16px;padding:16px}
.shownodes .screen [data-n]{outline:1px dashed color-mix(in srgb,var(--cyan) 70%,transparent);outline-offset:-1px}
.shownodes .screen [data-n="Row"]{outline-color:color-mix(in srgb,var(--magenta) 70%,transparent)}
.shownodes .screen [data-n="Column"]{outline-color:color-mix(in srgb,var(--yellow) 70%,transparent)}
.cmp{overflow-x:auto}
.cmp table{min-width:900px}
.cmp td:first-child{white-space:nowrap;font-weight:600}
.det>details{margin:10px 0}
.det>details>summary{font:600 20px var(--serif);padding:4px 0}
.three{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,360px),1fr));gap:18px}
.three>div{min-width:0}
"""


def variant_rules():
    out = []
    for th in THEMES:
        for st, _l, _s in STATES:
            out.append(f'.page:has(#th-{th}:checked):has(#st-{st}:checked) .v.th_{th}.st_{st}{{display:block}}')
        out.append(f'.page:has(#th-{th}:checked) .pv.th_{th}{{display:block}}')
    return "\n".join(out)


def radios(name, items, default):
    return '<span class="seg" role="radiogroup">' + "".join(
        f'<input type="radio" name="{name}" id="{name}-{v}"{" checked" if v == default else ""}><label for="{name}-{v}">{esc(l)}</label>'
        for v, l in items) + '</span>'


def lane():
    cols = []
    for d in D:
        c = d["c"]
        boards = "".join(f'<div class="v th_{th} st_{st}">{screen(th, s, st, c)}</div>'
                         for th in THEMES for st, _l, s in STATES)
        cols.append(f'''<div class="dir" id="lane-{c.SLUG}"><div class="dir-h">
<span class="tag">{c.L}</span><h2>{esc(c.NAME)}<small>{esc(c.EN)}</small></h2>
<p>{esc(c.PITCH)}</p></div>{boards}</div>''')
    return f'<div class="lane-wrap"><div class="lane">{"".join(cols)}</div></div>'


def table():
    rows = "".join(f'<tr><td>{c.L} · {esc(c.NAME)}</td><td>{esc(c.CLUSTER)}</td><td>{esc(c.UNIQUE)}</td>'
                   f'<td>{esc(c.S)}</td><td>{esc(c.R)}</td><td>{esc(c.C)}</td></tr>' for c in CONCEPTS)
    return (f'<div class="cmp"><table><thead><tr><th>Concept</th><th>Clusters by</th><th>Its own elements</th>'
            f'<th>Strength</th><th>Risk</th><th>OWL build cost</th></tr></thead><tbody>{rows}</tbody></table></div>')


SHARED_TAPS = [
    ("Town tile (6x3, on the board)", "open the map over the board", "screen()"),
    ("✕", "close the map", "close()"),
    ("✓ on a quest", "mark done", CMD["done"]),
    ("↺ on a done quest", "undo", CMD["undo"]),
    ("⋯ → snooze 1d / 3d / 7d", "snooze", CMD["s1"] + "  (3d: 2026-11-05, 7d: 2026-11-09)"),
    ("⋯ → priority ↑ / ↓", "priority", CMD["up"] + "  (↓: --priority 3)"),
    ("⋯ → drop", "drop", CMD["drop"]),
    ("review, on the ◇ line", "open the proposal Popup", "Popup"),
    ("Proposal · accept", "approve", CMD["accept"]),
    ("Proposal · decline", "turn down", CMD["decline"]),
    ("add · Enter", "add a quest; #Pillar sets the pillar", CMD["add"]),
    ("rest day ›", "rest day (until today is kept)", CMD["rest"]),
    ("? N waiting", "open the oldest check-in Popup", "Popup"),
    ("Check-in · send", "answer", CMD["answer"]),
    ("Check-in · later", "close it; it stays open in the snapshot", "—"),
    ("Pillar Popup · add a quest", "add a quest in that pillar", CMD["addp"]),
]


def details():
    out = []
    for c in CONCEPTS:
        out.append(f'''<details id="d-{c.SLUG}" open><summary>{c.L} · {esc(c.NAME)}: map taps and build notes</summary>
<div class="three"><div>{tapmap(c.NAME + " · the map", "Taps on the Canvas, by x and y.", c.TAPS)}</div>
<div><h3>Build notes</h3><ul>{c.NOTES}</ul></div></div></details>''')
    return "".join(out)


def main():
    data_html = "".join(
        f'<details><summary>{esc(label)} snapshot (synthetic)</summary><pre>{esc(json.dumps(s, indent=1, ensure_ascii=False))}</pre></details>'
        for key, label, s in STATES)
    pops = "".join(f'<div class="pv th_{th}">{popups(th)}</div>' for th in THEMES)
    html = f"""<title>CORE &amp; CRAFT Town Maps</title>
<meta name="claude:viewport-fit-sides" content="cover">
<style>{CSS}
{EXTRA_CSS}
{theme_css()}
{variant_rules()}</style>
<div class="page">
<header>
  <div class="eyebrow">CORE &amp; CRAFT · Tabulae · the town, full screen · three concepts</div>
  <h1>Three town maps</h1>
</header>
<div class="intro"><div class="lede">
<p>Forum with Castra, as a map that fills the board. Each concept keeps Forum's buildings and puts every one on its Castra block with its whole week, but clusters the nine pillars its own way and adds something of its own: a river, a hill, a walled wheel.</p>
<p>Switch the theme and the state for all three at once. "Fit three" shrinks them to see together; at 100% they're real size and the row scrolls sideways. Below: a comparison, the Popups and rail they share, each map's own taps and build notes, and the synthetic data.</p>
{FULL}
</div><aside class="aside">{RULES}{GIT}</aside></div>
<div class="bar"><div class="ctrl">
  {radios("th", [(t, t) for t in THEMES], "tokyo-night")}
  {radios("st", [(k, l) for k, l, _ in STATES], "normal")}
  <label for="zoom"><input type="checkbox" id="zoom"> Fit three</label>
  <label for="nodes"><input type="checkbox" id="nodes"> Show OWL nodes</label>
</div></div>
{lane()}
<section class="block" id="compare"><h2>Comparison</h2>
<p class="lede">My pick is Clivus. Its columns never move, so taps and titles stay put, and level becomes something you can see from across the room. The aqueduct gives the streak and xp history a place on the map.</p>
{table()}</section>
<section class="block" id="shared"><h2>What all three share</h2>
<p class="lede">The rail under each map and these Popups are the same in every concept. Each tap runs one command through <code class="mono">run()</code>, with ids from the normal state and values shell-quoted from OWL.</p>
{pops}
<div class="maps" style="margin-top:16px">{tapmap("Rail and Popups", "The route lists the top quests in rank order; @ and the numbers match the stops on the map.", SHARED_TAPS)}</div>
<div class="notes" style="margin-top:16px"><div><h3>Nodes</h3><ul>
<li>One <code>Canvas</code> of text runs for the map, under 300 per concept. The rail is about 60 nodes, so a screen stays far under OWL's 1,000.</li>
<li>Map taps use the Canvas's <code>onTap(x, y)</code>; the concepts' build notes give the hit boxes.</li></ul></div>
<div><h3>Snapshot fields these still need</h3><ul>
<li><code>pillars[].today</code>: events today per pillar, for lit windows and Castra's ▪. PR 9 has only <code>events_7d</code>.</li>
<li><code>leveled_today</code>: pillars that levelled up today, for the yellow ✦.</li>
<li><code>clock</code> comes from OWL's clock source, not the snapshot.</li></ul></div></div>
</section>
<section class="block det" id="details"><h2>Per concept</h2>{details()}</section>
<section class="block"><h2>Synthetic data</h2>
<p class="lede">The same three snapshots drive all three maps, in the shape main has after PR 9, with the two proposed fields above.</p>{data_html}</section>
<footer>Mockup only, no OWL. Every number is synthetic. Drawn at the brief's grid (96 px cells, 8 px gaps), so the board is 616 by 824 px; Board.qml's 6 px gap makes it 570 by 762, and the 77-character map would need 8 dp text there. Text sizes from OwlLayout.js at 1 dp = 96/69 px; no browser was available here to render a preview, so fit was checked by character counts.</footer>
</div>
{common.JS}"""
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    open(OUT, "w").write(html)
    print(len(html))


if __name__ == "__main__":
    main()
