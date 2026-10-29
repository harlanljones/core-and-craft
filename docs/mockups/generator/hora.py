"""Direction C · Hora: low ink, sits under windows all day."""
from common import *
from data import ranked, roman, LATIN, ENG, TITLE, PILLARS, top_pillar, is_night, is_dusk, weather
from town import rnd

SLUG = "hora"
W, H = 582, 270


def town_svg(s):
    night, dusk, wx = is_night(s), is_dusk(s), weather(s)
    top = top_pillar(s)
    asks = {c["pillar"] for c in s["checkins"] if c.get("open")}
    pm = {p["pillar"]: p for p in s["pillars"]}
    ground = 226
    el = []
    # sky
    r = rnd(3)
    if night or dusk:
        for _ in range(46 if night else 14):
            x, y = next(r) * W, 18 + next(r) * (110 if night else 60)
            el.append(f'<circle cx="{x:.0f}" cy="{y:.0f}" r="{0.6 + next(r) * 0.9:.1f}" class="fl-foreground" opacity="{0.25 + next(r) * 0.4:.2f}"/>')
    if night:
        el.append(f'<circle cx="520" cy="40" r="9" class="fl-yellow" opacity="0.85"/>'
                  f'<circle cx="525" cy="37" r="8" class="fl-background"/>')
    elif dusk:
        el.append('<circle cx="530" cy="150" r="11" class="fl-yellow" opacity="0.55"/>')
    else:
        el.append('<circle cx="520" cy="44" r="10" class="fl-yellow" opacity="0.7"/>')
    # buildings
    bw, pitch, x0 = 44, 64, 9
    for i, (pid, latin, eng, _f) in enumerate(PILLARS):
        p = pm[pid]
        x = x0 + i * pitch
        floors = max(2, min(7, p["level"] + 1))
        h = 22 + floors * 20
        y = ground - h
        roof = ""
        if pid in ("relationships", "faith"):
            roof = f'<path d="M{x - 2},{y} L{x + bw / 2},{y - 14} L{x + bw + 2},{y} Z" class="fl-surface"/>'
        elif pid in ("recovery", "outlook"):
            roof = f'<path d="M{x + 6},{y} A{bw / 2 - 6},{14} 0 0 1 {x + bw - 6},{y} Z" class="fl-surface"/>'
        elif pid == "career":
            roof = f'<rect x="{x + bw / 2 - 1}" y="{y - 16}" width="2" height="16" class="fl-surface"/>'
        elif pid == "agents":
            roof = f'<rect x="{x + bw - 14}" y="{y - 12}" width="7" height="12" class="fl-surface"/>'
        elif pid == "time":
            roof = f'<rect x="{x + 12}" y="{y - 18}" width="{bw - 24}" height="18" class="fl-surface"/>'
        el.append(roof + f'<rect x="{x}" y="{y}" width="{bw}" height="{h}" class="fl-surface"/>')
        slots = []
        for f in range(floors):
            wy = ground - 16 - f * 20
            cols = [x + 8, x + bw - 14] if f == 0 else [x + 8, x + 19, x + 30]
            slots += [(cx, wy) for cx in cols]
        for k, (cx, wy) in enumerate(slots):
            if k < p.get("today", 0):
                el.append(f'<rect x="{cx}" y="{wy}" width="6" height="8" class="fl-yellow" opacity="0.95"/>')
        t = TITLE[pid]
        el.append(f'<text x="{x + bw / 2}" y="{ground + 17 + (i % 2) * 15}" text-anchor="middle" font-size="12.5">'
                  f'<tspan class="fl-accent" font-weight="700">{t[0]}</tspan>'
                  f'<tspan class="fl-{"accent" if pid == top else "muted"}">{t[1:]}</tspan></text>')
        if pid in asks:
            el.append(f'<text x="{x + bw / 2}" y="{y - 22}" text-anchor="middle" class="fl-magenta" font-size="15" font-weight="700">?</text>')
        if pid == top:
            el.append(f'<text x="{x + bw / 2}" y="{ground - 1}" text-anchor="middle" class="fl-accent" font-size="15" font-weight="700">@</text>')
    el.append(f'<line x1="0" y1="{ground + 0.5}" x2="{W}" y2="{ground + 0.5}" class="st-muted" stroke-width="1"/>')
    # rain
    if wx != "clear":
        r = rnd(11)
        n = 40 if wx == "drizzle" else 110
        for _ in range(n):
            x, y = next(r) * W, 10 + next(r) * (ground - 20)
            el.append(f'<line x1="{x:.0f}" y1="{y:.0f}" x2="{x - 3:.0f}" y2="{y + 9:.0f}" class="st-blue" stroke-width="1" opacity="0.35"/>')
    label = f'<text x="0" y="12" class="fl-muted" font-size="12.5">Urbs · the town</text>'
    if s["errors"]:
        label += '<text x="128" y="12" class="fl-red" font-size="12.5">●</text>'
    return Canvas(f'<svg viewBox="0 0 {W} {H}" width="{W}" height="{H}" font-family="var(--mono)" aria-hidden="true">{"".join(el)}{label}</svg>')


def quests_tile(s):
    qs = ranked(s)
    open_ = [q for q in qs if not q["done_today"]]
    done = [q for q in qs if q["done_today"]]
    items = []
    for q in open_[:3]:
        items.append(Col(Row(T(q["title"], "body", "foreground", weight=True, max_lines=1, tap="menu"),
                             T("○", "body", "muted", tap="done"), gap=8),
                         T(" · ".join(q["reasons"]), "small", "red" if q["overdue_days"] > 1 else "muted", max_lines=2),
                         gap=1))
    if len(open_) > 3:
        items.append(T(f"+{len(open_) - 3} more", "small", "muted", tap="more"))
    if not open_:
        items.append(Col(T("quiet", "body", "foreground"), T("nothing due today", "small", "muted"), gap=1))
    for q in done:
        items.append(Row(T(q["title"], "small", "muted", weight=True, max_lines=1, cls="strike"),
                         T("●", "small", "green"), T("↺", "small", "muted", tap="undo"), gap=8))
    waiting = len(s["proposed"]) + len([c for c in s["checkins"] if c.get("open")])
    foot = Row(T("＋ add", "small", "muted", weight=True, tap="add"),
               T(f"{waiting} waiting ›" if waiting else "", "small", "magenta", tap="waiting"), gap=8)
    return Col(T("Pensa · today", "label"), Col(*items, gap=10), Box(cls="fill"), foot, gap=10, cls="fill")


def level_tile(s):
    x, st = s["xp"], s["streak"]
    ring = Ring(x["into"], x["need"], "accent", size=62, thickness=5, center=T(roman(x["level"]), "title", "accent"))
    right = Col(T("Gradus · level", "label"),
                T(f'{x["into"]} / {x["need"]} xp', "small", "foreground"),
                T(f'streak · {st["days"]} days', "small", "foreground" if st["days"] else "muted"),
                T("today kept", "small", "green") if st["today"] else T("rest day ›", "small", "accent", tap="rest"),
                gap=2, cls="w1")
    return Col(Row(ring, right, gap=12), Box(cls="fill"), Sparkline(s["history"], "muted", height=16),
               gap=6, cls="fill")


GLYPH = "·▁▂▂▃▃▄▄▄▅▅▅▅▆▆▆▆▆▇"


def glyph(n):
    return GLYPH[min(n, len(GLYPH) - 1)] if n < 25 else "█"


def strip_tile(s):
    def cell(p):
        return [Row(T(glyph(p["events_7d"]), "title", "green" if p.get("today") else "muted"),
                    T(roman(p["level"]), "small", "muted", size=8), laurel(s, p["pillar"]), gap=2, align="flex-end")]
    return pillar_strip(s, cell)


def build_board(theme, s, key, theme_name):
    tiles = [
        tile("Town", 6, 3, 0, 0, town_svg(s)),
        tile("Pillar strip", 6, 1, 0, 3, strip_tile(s)),
        tile("Quests", 3, 4, 0, 4, quests_tile(s)),
        tile("Level and streak", 3, 2, 3, 4, level_tile(s)),
        tile("Typing", 3, 2, 3, 6, typing_tile(s)),
    ]
    return board(theme, tiles, 8, key, theme_name)


def popups(only=None):
    from data import NORMAL as s
    out = []
    for th in ([only] if only else THEMES):
        wait = (f'<div class="popup" style="left:20px;top:120px;width:284px">'
                + Col(T("waiting", "label"),
                      T("◇ Review October spend", "small", "foreground", bold=True),
                      T("Economic · proposed by duet:economic", "small", "muted", size=9),
                      Row(T("[accept]", "small", "green", tap="accept"), T("[decline]", "small", "muted", tap="decline"), gap=10),
                      T("? What's your target sleep window?", "small", "magenta", bold=True),
                      T("Recovery asks", "small", "muted", size=9),
                      InputN("answer, Enter sends"),
                      Row(T("[skip]", "small", "muted", tap="skip"), T("[tomorrow]", "small", "muted", tap="later"), gap=10),
                      gap=4)
                + '</div>')
        menu = (f'<div class="popup" style="left:36px;top:50px;width:262px">'
                + Col(T("15 min typing drill", "small", "foreground", bold=True),
                      T("due today · typing 12 under target", "small", "muted", size=9),
                      T("◷ snooze 1 day", "small", "foreground", tap="snooze"),
                      T("▲ priority up", "small", "foreground", tap="up"),
                      T("▼ priority down", "small", "foreground", tap="down"),
                      T("✕ drop", "small", "muted", tap="drop"), gap=5)
                + '</div>')
        stage = lambda pop: (f'<div class="pop-stage th-{th}"><div class="board" style="padding:0;background:none;'
                             f'grid-template-columns:repeat(3,96px);grid-template-rows:repeat(4,96px)">'
                             f'{tile("Quests", 3, 4, 0, 0, quests_tile(s))}</div>{pop}</div>')
        out.append(('' if only else f'<h3>{th}</h3>') + f'<div class="pop-demo">{stage(menu)}{stage(wait)}</div>')
    return ('<p class="lede">Proposed quests and check-ins fold into one "waiting" line on the quests tile. '
            'Tapping a title opens its menu; tapping ○ marks it done.</p>' + "".join(out))


NOTES = """
<div><h3>Nodes per tile</h3><ul>
<li><b>Town</b>: one <code>Canvas</code> with <code>rect</code>, <code>path</code> and <code>circle</code> calls in <code>surface</code>, lit windows as 6×8 dp rects in <code>yellow</code>, <code>?</code> and <code>@</code> as <code>text()</code>. About 150 calls, no per-character layout. Rain is 40–110 <code>line</code>s that can read <code>time</code>.</li>
<li><b>Quests</b>: Texts and one <code>Popup</code> that switches between "menu", "more", "waiting" and "add" through a <code>state</code>. About 40 nodes.</li>
<li><b>Level</b>: <code>Box</code> holding a <code>Ring</code> and a Text, a <code>Sparkline</code> in muted.</li>
<li><b>Strip</b>: nine <code>Column</code>s, a title-size block glyph and two small Texts each.</li>
</ul></div>
<div><h3>Snapshot fields this needs</h3><ul>
<li><code>pillars[].today</code>, as in the other two.</li>
<li>Nothing else new. Check-ins and proposals are counted into one "waiting" number.</li>
</ul></div>
<div><h3>Watch for</h3><ul>
<li>It is the least ASCII of the three: the town is shapes, not characters, which drifts from the house style.</li>
<li>Waiting items hide behind a tap, so an agent's question can sit unseen. The magenta count is the only nudge.</li>
<li>The muted Sparkline on tokyo-night (#414868 on #1a1b26) is faint by design and may vanish on a bright wallpaper.</li>
</ul></div>
"""


def tapmaps():
    return "".join([
        tapmap("Town · 6x3", "Ambient only. The whole tile is one tap.", [
            ("Tile", "today's numbers", 'terminal("acta status --json | jq .today")'),
        ]),
        tapmap("Quests · 3x4", "Three quests, one waiting line, one add.", [
            ("○", "mark done", C.done),
            ("↺ on a done row", "undo", C.undo),
            ("Title → snooze 1 day", "snooze a day", C.snooze1),
            ("Title → priority up / down", "priority up or down", C.up),
            ("Title → drop", "drop", C.drop),
            ("waiting → accept / decline", "accept or decline a proposal", C.accept),
            ("waiting → Input · Enter", "answer a check-in", C.answer),
            ("waiting → skip / tomorrow", "hide for today", C.skip),
            ("＋ add → Input · Enter", "add a quest", C.add),
        ]),
        tapmap("Level and streak · 3x2", "A ring and a faint line.", [
            ("rest day", "rest day (until today is kept)", C.rest),
            ("Tile", "xp by day", C.xp),
        ]),
        tapmap("Pillar strip · 6x1", "Block height is 7-day activity, green if active today.", [
            ("A pillar", "show its numbers", C.pillar),
        ]),
        tapmap(*TYPING_TAPS),
    ])
