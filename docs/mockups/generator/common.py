"""Shared page shell, OWL-node helpers, themes, the board grid and the typing tile."""
import html
import json

from data import STATES, typing_strip

DP = 96 / 69          # one launcher dp in board px (cell 96 px = 69 dp)
CELL, GAP = 96, 8

THEMES = {
    # published Omarchy colors.toml values; surface = lighter_background (Owl.themeFromToml)
    "tokyo-night": {"accent": "#7aa2f7", "foreground": "#a9b1d6", "muted": "#414868", "green": "#9ece6a",
                    "red": "#f7768e", "yellow": "#e0af68", "orange": "#eb927b", "blue": "#7aa2f7",
                    "magenta": "#ad8ee6", "cyan": "#449dab", "background": "#1a1b26", "surface": "#24283b",
                    "wall": "#13141c"},
    "gruvbox": {"accent": "#7daea3", "foreground": "#d4be98", "muted": "#665c54", "green": "#a9b665",
                "red": "#ea6962", "yellow": "#d8a657", "orange": "#e1875c", "blue": "#7daea3",
                "magenta": "#d3869b", "cyan": "#89b482", "background": "#282828", "surface": "#3c3836",
                "wall": "#1e1e1e"},
}
NAMED = ["accent", "foreground", "muted", "green", "red", "yellow", "orange", "blue", "magenta", "cyan",
         "background", "surface"]


def esc(s):
    return html.escape(str(s), quote=True)


def attrs(**kw):
    out = []
    for k, v in kw.items():
        if v is None or v is False:
            continue
        k = k.rstrip("_").replace("_", "-")
        out.append(f' {k}="{esc(v)}"')
    return "".join(out)


# ---- OWL node stand-ins. Every element that maps to an OWL node carries data-n,
# so the "show OWL nodes" switch can outline it.

def T(text, style="small", color=None, size=None, bold=False, cls="", tap=None, max_lines=None, weight=False,
      align=None, raw=False):
    """Text. style: huge/title/body/small/muted/label. size overrides fontSize (dp)."""
    c = [f"t-{style}"]
    if color:
        c.append(f"c-{color}")
    if bold:
        c.append("b")
    if weight:
        c.append("w1")
    if max_lines == 1:
        c.append("ml1")
    elif max_lines == 2:
        c.append("ml2")
    if align:
        c.append(f"ta-{align}")
    if tap:
        c.append("tap")
    if cls:
        c.append(cls)
    st = f' style="font-size:{size * DP:.2f}px"' if size else ""
    body = text if raw else esc(text)
    return f'<span class="n {" ".join(c)}" data-n="Text"{st}{attrs(data_tap=tap)}>{body}</span>'


def node(kind, children, cls="", style="", tap=None):
    st = f' style="{style}"' if style else ""
    c = f"n k-{kind.lower()} {cls}".strip()
    if tap:
        c += " tap"
    return f'<div class="{c}" data-n="{kind}"{st}{attrs(data_tap=tap)}>{"".join(children)}</div>'


def Row(*ch, cls="", gap=8, style="", tap=None, align=None):
    s = f"gap:{gap * DP:.1f}px;" + (f"align-items:{align};" if align else "") + style
    return node("Row", ch, cls, s, tap)


def Col(*ch, cls="", gap=4, style="", tap=None):
    return node("Column", ch, cls, f"gap:{gap * DP:.1f}px;" + style, tap)


def Box(*ch, cls="", style="", tap=None):
    return node("Box", ch, cls, style, tap)


def Progress(value, mx, color="accent", height=4, cls=""):
    pct = 0 if not mx else max(0, min(100, value / mx * 100))
    return (f'<div class="n k-progress {cls}" data-n="Progress" style="height:{height * DP:.1f}px">'
            f'<i class="bg-{color}" style="width:{pct:.1f}%"></i></div>')


def Ring(value, mx, color="accent", size=64, thickness=6, center=""):
    px = size * DP
    th = thickness * DP
    r = (px - th) / 2
    circ = 2 * 3.14159265 * r
    frac = 0 if not mx else max(0, min(1, value / mx))
    return (f'<div class="n k-ring" data-n="Ring" style="width:{px:.0f}px;height:{px:.0f}px">'
            f'<svg viewBox="0 0 {px:.1f} {px:.1f}" width="{px:.0f}" height="{px:.0f}" aria-hidden="true">'
            f'<circle cx="{px / 2:.1f}" cy="{px / 2:.1f}" r="{r:.1f}" fill="none" class="st-surface" stroke-width="{th:.1f}"/>'
            f'<circle cx="{px / 2:.1f}" cy="{px / 2:.1f}" r="{r:.1f}" fill="none" class="st-{color}" stroke-width="{th:.1f}" '
            f'stroke-linecap="round" stroke-dasharray="{circ * frac:.1f} {circ:.1f}" transform="rotate(-90 {px / 2:.1f} {px / 2:.1f})"/>'
            f'</svg><div class="ring-c">{center}</div></div>')


def Sparkline(values, color="accent", height=28, area=True, cls=""):
    w, h = 100.0, 30.0
    mx = max(values) if values and max(values) > 0 else 1
    n = len(values)
    pts = [(i * w / (n - 1), h - 2 - v / mx * (h - 4)) for i, v in enumerate(values)]
    line = " ".join(f"{x:.1f},{y:.1f}" for x, y in pts)
    poly = f"0,{h} " + line + f" {w},{h}"
    a = f'<polygon points="{poly}" class="fl-{color}" opacity="0.22"/>' if area else ""
    lx, ly = pts[-1]
    return (f'<div class="n k-sparkline {cls}" data-n="Sparkline" style="height:{height * DP:.0f}px">'
            f'<svg viewBox="0 0 {w} {h}" preserveAspectRatio="none" aria-hidden="true">{a}'
            f'<polyline points="{line}" fill="none" class="st-{color}" stroke-width="1.6" vector-effect="non-scaling-stroke"/>'
            f'</svg></div>')


def InputN(placeholder, value="", cls=""):
    v = esc(value) if value else f'<span class="ph">{esc(placeholder)}</span>'
    return f'<div class="n k-input {cls}" data-n="Input"><span class="caret">›</span> {v}<span class="cur">▏</span></div>'


def Canvas(inner, cls=""):
    return f'<div class="n k-canvas {cls}" data-n="Canvas">{inner}</div>'


def ascii_grid(rows):
    """rows: list of lists of (char, color|None). Rendered as one Text per color run (or Canvas text calls)."""
    out = []
    for row in rows:
        line, cur, buf = [], "§", ""
        for ch, col in row:
            if col != cur and buf:
                line.append((buf, cur))
                buf = ""
            cur = col
            buf += ch
        if buf:
            line.append((buf, cur))
        spans = "".join(f'<span class="{"c-" + c if c else ""}">{esc(t)}</span>' for t, c in line)
        out.append(f'<div class="ar">{spans}</div>')
    return "".join(out)


# ---- tiles and the board

def tile(name, w, h, x, y, inner, cls="", label=None):
    return (f'<section class="tile {cls}" style="grid-column:{x + 1}/span {w};grid-row:{y + 1}/span {h}" '
            f'aria-label="{esc(label or name)} tile, {w} by {h}" data-tile="{esc(name)}">'
            f'<div class="tile-in">{inner}</div></section>')


def free_slot(w, h, x, y):
    return (f'<section class="tile free" style="grid-column:{x + 1}/span {w};grid-row:{y + 1}/span {h}" aria-label="free slot">'
            f'<div class="tile-in">{T("free " + str(w) + "x" + str(h), "label")}</div></section>')


def typing_tile(s):
    """widgets/60-typing.owl, rendered with the same rules, unchanged."""
    sk = [k for k in s["skills"] if k["id"] == "skill:typing"][0]
    strip = typing_strip(sk)
    top = Row(
        T("⌨ typing", "muted", weight=True),
        T("●", "small", "red") if s["errors"] else "",
        T("passed", "small", "green") if sk.get("passed") else
        T(f'{sk["runs_qualifying"]}/{sk["runs_total"]} ≥{sk["min_acc"]}%', "small", "muted"),
    )
    mid = Row(
        T("–" if sk["best"] is None else str(sk["best"]), "title", "accent"),
        T("wpm" if sk["target"] is None else f'/{sk["target"]} wpm', "small", "muted", weight=True),
        T(f'today {sk["today_best"] if sk["today_best"] is not None else "–"} · {s["today"]["typing_tests"]}', "small", "foreground"),
        align="flex-end")
    prog = Progress(sk["best"] or 0, sk["target"], "green" if sk.get("passed") else "accent") if sk["target"] else ""
    return Col(top, mid, prog, T(strip, "small", "green", max_lines=1), gap=4, cls="fill")


def board(theme, tiles_html, rows, state_key, theme_name):
    return (f'<div class="board-wrap"><div class="board-cap"><span>{esc(theme_name)}</span>'
            f'<span class="syn">synthetic data · {esc(state_key)}</span></div>'
            f'<div class="scroll"><div class="board th-{theme}" style="grid-template-rows:repeat({rows},{CELL}px)">'
            f'{"".join(tiles_html)}</div></div></div>')


def theme_css():
    out = []
    for name, pal in THEMES.items():
        vars_ = ";".join(f"--{k}:{v}" for k, v in pal.items())
        out.append(f".th-{name}{{{vars_}}}")
    for c in NAMED:
        out.append(f".c-{c}{{color:var(--{c})}} .bg-{c}{{background:var(--{c})}} "
                   f".st-{c}{{stroke:var(--{c})}} .fl-{c}{{fill:var(--{c})}}")
    return "\n".join(out)


CSS = r"""
/* Layout: page column of state sections; each holds two real-size boards (tokyo-night, gruvbox) that wrap. */
:root{
  --paper:#f3f1ec; --ink:#23212b; --ink-2:#5c5866; --rule:#d9d5cc; --chip:#e7e3da; --link:#3d5a99;
  --wall-page:#e9e6df;
  --serif:"Iowan Old Style","Palatino Linotype",Palatino,"Book Antiqua",Georgia,serif;
  --sans:system-ui,-apple-system,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif;
  --mono:"JetBrainsMono Nerd Font","JetBrains Mono","Cascadia Mono","DejaVu Sans Mono",Menlo,Consolas,monospace;
}
@media (prefers-color-scheme: dark){:root:not([data-theme="light"]){
  --paper:#14141a; --ink:#e4e1ea; --ink-2:#a19dab; --rule:#2c2b35; --chip:#22212b; --link:#9fb4ee; --wall-page:#0d0d12; color-scheme:dark}}
:root[data-theme="dark"]{
  --paper:#14141a; --ink:#e4e1ea; --ink-2:#a19dab; --rule:#2c2b35; --chip:#22212b; --link:#9fb4ee; --wall-page:#0d0d12; color-scheme:dark}
body{background:var(--paper);color:var(--ink);font:15px/1.55 var(--sans)}
.page{max-width:1320px;margin:0 auto;padding-block:28px 64px;
  padding-left:max(16px,env(safe-area-inset-left,0px));padding-right:max(16px,env(safe-area-inset-right,0px))}
a{color:var(--link)}
.eyebrow{font:600 12px/1.3 var(--sans);letter-spacing:.14em;text-transform:uppercase;color:var(--ink-2)}
h1{font:600 44px/1.05 var(--serif);margin:.2em 0 .1em;text-wrap:balance}
h1 small{font:400 20px var(--serif);color:var(--ink-2);margin-left:.4em;font-style:italic}
h2{font:600 26px/1.2 var(--serif);margin:0;text-wrap:balance}
h3{font:600 17px/1.3 var(--serif);margin:0 0 6px}
.lede{max-width:68ch;font-size:16.5px}
.intro{display:grid;grid-template-columns:minmax(0,1.3fr) minmax(0,1fr);gap:32px;align-items:start;margin:8px 0 28px}
@media (max-width:900px){.intro{grid-template-columns:minmax(0,1fr)}}
.aside{border:1px solid var(--rule);border-radius:6px;padding:14px 16px;background:var(--chip);min-width:0}
.aside ul{margin:6px 0 0;padding-left:18px}.aside li{margin:2px 0}
.sigs{display:grid;grid-template-columns:auto minmax(0,1fr);gap:4px 12px;font-size:14px;margin-top:6px}
.sigs code{font-family:var(--mono);font-size:13px}
.bar{display:flex;flex-wrap:wrap;gap:8px 18px;align-items:center;position:sticky;top:env(safe-area-inset-top,0px);
  z-index:5;background:var(--paper);padding:10px 0;border-bottom:1px solid var(--rule);margin-bottom:20px}
body::before{content:"";position:fixed;inset:0 0 auto;height:env(safe-area-inset-top,0px);z-index:10;background:var(--paper)}
.bar nav{display:flex;flex-wrap:wrap;gap:6px}
.bar nav a{font:600 13px var(--sans);text-decoration:none;padding:4px 10px;border-radius:999px;background:var(--chip);color:var(--ink)}
.bar nav a:focus-visible,.bar input:focus-visible{outline:2px solid var(--link);outline-offset:2px}
.bar label{font-size:13.5px;display:flex;gap:6px;align-items:center;cursor:pointer}
.others{margin-left:auto;font-size:13.5px}
.state{margin:8px 0 40px}
.state-h{display:flex;flex-wrap:wrap;gap:6px 14px;align-items:baseline;margin-bottom:12px}
.state-h p{margin:0;color:var(--ink-2);font-size:14.5px;max-width:80ch}
.boards{display:flex;flex-wrap:wrap;gap:24px}
.board-wrap{min-width:0;max-width:100%}
.board-cap{display:flex;justify-content:space-between;gap:12px;font:600 12.5px var(--sans);letter-spacing:.04em;
  color:var(--ink-2);margin:0 2px 6px}
.board-cap .syn{font-weight:500;letter-spacing:.02em}
.scroll{overflow-x:auto;max-width:100%;border-radius:4px}
.board{display:grid;grid-template-columns:repeat(6,96px);gap:8px;padding:12px;width:max-content;background:var(--wall);
  background-image:radial-gradient(circle at 20% 0%,color-mix(in srgb,var(--surface) 55%,transparent),transparent 60%)}

/* tiles: 0.88 background, a 2 dp frame in muted, 12 dp padding, the theme mono font */
.tile{position:relative;background:color-mix(in srgb,var(--background) 88%,transparent);
  box-shadow:inset 0 0 0 3px var(--muted);overflow:hidden;color:var(--foreground);font-family:var(--mono);
  font-variant-numeric:tabular-nums}
.tile-in{position:absolute;inset:3px;padding:14px;display:flex;flex-direction:column;min-width:0}
.tile.free{background:transparent;box-shadow:none;outline:2px dashed color-mix(in srgb,var(--muted) 80%,transparent);outline-offset:-3px}
.tile.free .tile-in{align-items:center;justify-content:center}

.n{min-width:0}
.k-row{display:flex;flex-direction:row;align-items:center}
.k-column{display:flex;flex-direction:column}
.k-box{position:relative;display:grid}
.k-box>*{grid-area:1/1}
.fill{flex:1 1 auto}
.w1{flex:1 1 0;min-width:0}
.ml1{white-space:nowrap;overflow:hidden;text-overflow:clip}
.ml2{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}
.b{font-weight:700}
.ta-end{text-align:right}.ta-center{text-align:center}
[data-n="Text"]{line-height:1.17;white-space:pre-wrap}
.t-huge{font-size:61.2px}.t-title{font-size:27.8px;line-height:1.1}.t-body{font-size:18.1px}
.t-small{font-size:15.3px}.t-muted{font-size:18.1px;color:var(--muted)}
.t-label{font-size:15.3px;color:var(--muted);letter-spacing:.04em}
.c-label{color:var(--muted)}
.asc{font-size:12.52px}
.strike{text-decoration:line-through;text-decoration-color:var(--muted)}
.c-muted{color:var(--muted)}
.k-progress{background:var(--surface);width:100%;flex:0 0 auto}
.k-progress i{display:block;height:100%}
.k-ring{position:relative;flex:0 0 auto}.k-ring svg{position:absolute;inset:0}
.ring-c{position:absolute;inset:0;display:grid;place-items:center}
.k-sparkline{width:100%;flex:0 0 auto}.k-sparkline svg{width:100%;height:100%;display:block}
.k-input{font-size:15.3px;border-bottom:2px solid var(--muted);padding:3px 2px;white-space:nowrap;overflow:hidden}
.k-input .caret{color:var(--accent)}.k-input .ph{color:var(--muted)}
.k-input .cur{color:var(--accent);opacity:.7}
.k-canvas{position:relative;flex:1 1 auto;min-height:0}
.ar{white-space:pre;line-height:1.17}
.tap{cursor:pointer}
.tile .tap:hover{text-decoration:underline;text-decoration-color:var(--muted)}

/* popups: a menu over the window (README: Popup is not a window of its own) */
.pop-demo{display:flex;flex-wrap:wrap;gap:24px;align-items:flex-start}
.pop-stage{position:relative;padding:12px;background:var(--wall);border-radius:4px;width:max-content}
.popup{box-sizing:border-box;position:absolute;z-index:3;background:var(--background);box-shadow:inset 0 0 0 3px var(--accent),0 10px 30px rgba(0,0,0,.45);
  padding:14px 16px;font-family:var(--mono);color:var(--foreground);width:276px;display:flex;flex-direction:column;gap:6px}
.popup .scrim{display:none}

/* OWL node outlines */
.shownodes .tile [data-n]{outline:1px dashed color-mix(in srgb,var(--cyan) 70%,transparent);outline-offset:-1px}
.shownodes .tile [data-n="Row"]{outline-color:color-mix(in srgb,var(--magenta) 70%,transparent)}
.shownodes .tile [data-n="Column"]{outline-color:color-mix(in srgb,var(--yellow) 70%,transparent)}

/* tap maps */
.maps{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,420px),1fr));gap:18px}
.map{border:1px solid var(--rule);border-radius:6px;padding:12px 14px;min-width:0}
.map p{margin:2px 0 8px;color:var(--ink-2);font-size:14px}
.tbl{overflow-x:auto}
table{border-collapse:collapse;width:100%;font-size:13.5px}
th{text-align:left;font:600 11.5px var(--sans);letter-spacing:.08em;text-transform:uppercase;color:var(--ink-2);
  border-bottom:1px solid var(--rule);padding:4px 8px 4px 0}
td{border-bottom:1px solid var(--rule);padding:5px 8px 5px 0;vertical-align:top}
td code,.mono{font-family:var(--mono);font-size:12.5px;word-break:break-word}
section.block{margin:36px 0}
.notes{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,380px),1fr));gap:18px}
.notes div{min-width:0}
details{border:1px solid var(--rule);border-radius:6px;padding:8px 12px;margin:8px 0}
summary{cursor:pointer;font-weight:600}
pre{overflow-x:auto;font:12.5px/1.45 var(--mono);margin:8px 0 0}
footer{margin-top:40px;color:var(--ink-2);font-size:13.5px;border-top:1px solid var(--rule);padding-top:12px}
@media (prefers-reduced-motion: reduce){*{animation:none!important;transition:none!important}}
"""

JS = r"""
<script>
(function(){
  var cb=document.getElementById('nodes');
  function set(on){document.documentElement.classList.toggle('shownodes',on)}
  try{ if(localStorage.getItem('cc-nodes')==='1'){cb.checked=true;set(true)} }catch(e){}
  cb.addEventListener('change',function(){set(cb.checked);try{localStorage.setItem('cc-nodes',cb.checked?'1':'0')}catch(e){}});
})();
</script>
"""


def page(title, slug, name, english, lede_html, aside_html, state_blurbs, build_board, rows, popups_html,
         maps_html, notes_html, others):
    states = []
    for key, label, s in STATES:
        boards = "".join(build_board(th, s, key, th) for th in THEMES)
        states.append(
            f'<section class="state" id="{key}"><div class="state-h"><h2>{esc(label)}</h2>'
            f'<p>{state_blurbs[key]}</p></div><div class="boards">{boards}</div></section>')
    data_html = "".join(
        f'<details><summary>{esc(label)} snapshot (synthetic)</summary><pre>{esc(json.dumps(s, indent=1, ensure_ascii=False))}</pre></details>'
        for key, label, s in STATES)
    other_links = " · ".join(f'<a href="{esc(u)}">{esc(n)}</a>' for n, u in others)
    return f"""<title>{esc(title)}</title>
<meta name="claude:viewport-fit-sides" content="cover">
<style>{CSS}
{theme_css()}</style>
<div class="page">
<header>
  <div class="eyebrow">CORE &amp; CRAFT · Tabulae · desktop tiles · direction {esc(slug)}</div>
  <h1>{esc(name)}<small>{esc(english)}</small></h1>
</header>
<div class="intro"><div class="lede">{lede_html}</div><aside class="aside">{aside_html}</aside></div>
<div class="bar">
  <nav aria-label="States"><a href="#empty">Empty</a><a href="#normal">Normal</a><a href="#busy">Busy</a>
  <a href="#popups">Popups</a><a href="#taps">Tap map</a><a href="#build">Build notes</a></nav>
  <label for="nodes"><input type="checkbox" id="nodes"> Show OWL nodes</label>
  <span class="others">Other directions: {other_links}</span>
</div>
{"".join(states)}
<section class="block" id="popups"><h2>Popups</h2>
<p class="lede">Normal state. A Popup draws over the board and closes on a tap outside it (OWL needs an onDismiss).</p>
{popups_html}</section>
<section class="block" id="taps"><h2>Tap map</h2>
<p class="lede">Each tap runs one command through <code class="mono">run()</code>, ids from the normal state. {CMD_NOTE}</p>
<div class="maps">{maps_html}</div></section>
<section class="block" id="build"><h2>Build notes</h2><div class="notes">{notes_html}</div>
<h3 style="margin-top:20px">Synthetic data</h3>{data_html}</section>
<footer>Mockup only, no OWL. Every number on this page is synthetic. Tiles are drawn at the brief's grid: 96 px cells, 8 px gaps, 0.88 opacity, 2 dp muted frame, text sizes from OwlLayout.js at 1 dp = 96/69 px.</footer>
</div>
{JS}"""


def tapmap(title, desc, rows):
    trs = "".join(f"<tr><td>{esc(a)}</td><td>{esc(b)}</td><td><code>{esc(c)}</code></td></tr>" for a, b, c in rows)
    return (f'<div class="map"><h3>{esc(title)}</h3><p>{desc}</p><div class="tbl"><table><thead><tr>'
            f'<th>Tap</th><th>Does</th><th>Runs</th></tr></thead><tbody>{trs}</tbody></table></div></div>')


TYPING_TAPS = ("Typing · 3x2", "The built tile, unchanged. It has no tap today.",
               [("Tile", "nothing (as built)", "—")])


# ---- acta commands. acta has no quest verbs yet (main, Oct 29): every tap is an
# `acta log <kind>` with the kinds from docs/event-envelope-v1.md, agent manual.

def acta_log(kind, subject, pillar, data=None):
    s = f"acta log {kind} --subject {subject} --pillar {pillar} --agent manual"
    if data is not None:
        s += " --data '" + json.dumps(data, ensure_ascii=False, separators=(",", ":")) + "'"
    return s


class C:
    done = acta_log("quest.done", "quest:typing-drill", "competence")
    undo = "acta retract <done_event> --agent manual --reason undo"
    snooze1 = acta_log("quest.snoozed", "quest:typing-drill", "competence", {"until": "2026-11-03"})
    snooze3 = acta_log("quest.snoozed", "quest:typing-drill", "competence", {"until": "2026-11-05"})
    snooze7 = acta_log("quest.snoozed", "quest:typing-drill", "competence", {"until": "2026-11-09"})
    up = acta_log("quest.edited", "quest:call-mom", "relationships", {"priority": 1})
    down = acta_log("quest.edited", "quest:call-mom", "relationships", {"priority": 3})
    drop = acta_log("quest.dropped", "quest:typing-drill", "competence")
    accept = acta_log("quest.accepted", "quest:budget-review", "economic")
    decline = acta_log("quest.declined", "quest:budget-review", "economic")
    add = acta_log("quest.added", "quest:stretch-10-min", "recovery", {"title": "Stretch 10 min", "every": "daily"})
    answer = acta_log("checkin.answered", "checkin:sleep-window", "recovery", {"text": "23:00–07:00"})
    skip = "no event kind yet (checkin.skipped proposed); hides it for today in state"
    rest = acta_log("day.rested", "day:2026-11-02", "recovery")
    pillar = 'terminal("acta status --json | jq \'.pillars[] | select(.pillar==\\"career\\")\'")'
    xp = 'terminal("acta status --json | jq .history")'


CMD_NOTE = ("acta has no quest verbs yet, so each tap runs <code class=\"mono\">acta log</code> with a kind from "
            "event-envelope-v1 (agent <code class=\"mono\">manual</code>). Undo retracts the done event, so the snapshot "
            "needs <code class=\"mono\">quests[].done_event</code>. Values go in as <code class=\"mono\">${sh(value)}</code>.")


# ---- pillars as the spec names them, grouped under C.O.R.E. and C.R.A.F.T.
from data import TITLE, GROUPS

SIZE8 = 8 * DP * 0.6            # px per mono character at 8 dp


def title_t(pid, size=8, color="foreground", top=None, align=None):
    """A pillar's spec title; its acronym letter in accent as the quiet Roman ornament."""
    t = TITLE[pid]
    c = "accent" if pid == top else color
    html_ = f'<span class="c-accent b">{esc(t[0])}</span><span class="c-{c}">{esc(t[1:])}</span>'
    return T(html_, "small", size=size, raw=True, max_lines=1, align=align)


def laurel(s, pid):
    """A level-up mark: a yellow ✦ beside the numeral on the day a pillar levels up."""
    return T("✦", "small", "yellow", size=8) if pid in s.get("leveled_today", []) else ""


def pillar_strip(s, cell):
    """cell(p) -> list of nodes under the title. Cells are weighted by the chars they need."""
    pm = {p["pillar"]: p for p in s["pillars"]}
    groups = []
    for label, ids in GROUPS:
        cells = []
        for pid in ids:
            w = max(len(TITLE[pid]), 8)
            cells.append(Col(title_t(pid), *cell(pm[pid]), gap=0, style=f"flex:{w} 1 0", tap="pillar"))
        w = sum(max(len(TITLE[i]), 8) for i in ids)
        groups.append(Col(Row(T(label, "label", size=8), T("─" * 60, "small", "muted", size=8, max_lines=1, weight=True), gap=4),
                          Row(*cells, gap=3), gap=1, style=f"flex:{w} 1 0"))
    return Row(groups[0], T("┃", "small", "muted", size=9, cls="colrule"), groups[1], gap=6, cls="fill", align="stretch")
