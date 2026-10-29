"""Direction D · Castra: the town seen from above, quests as a route."""
from common import *
from data import ranked, roman, LATIN, ENG, TITLE, PILLARS, top_pillar, is_night, is_dusk, weather
from town import COLS, ROWS, put, rnd

SLUG = "castra"
BX = [2, 28, 54]          # insula columns (21 wide), roads at 23-27 and 49-53
BY = [3, 8, 13]           # insula rows (3 tall), roads at 6-7 and 11-12
BW = 21


def camp(s):
    g = [[(" ", None) for _ in range(COLS)] for _ in range(ROWS)]
    night, dusk, wx = is_night(s), is_dusk(s), weather(s)
    top = top_pillar(s)
    asks = {c["pillar"] for c in s["checkins"] if c.get("open")}
    pm = {p["pillar"]: p for p in s["pillars"]}
    # header
    put(g, 0, 0, "Castra · the camp from above", "label")
    tod = "night" if night else "dusk" if dusk else "day"
    sky = wx
    right = f"{tod} {s['clock']} · {sky}" + (f" · ? {len(asks)}" if asks else "")
    put(g, 0, COLS - len(right), right, "label")
    if s["errors"]:
        put(g, 0, 30, "●", "red")
    # walls with four gates where the roads leave the camp
    for c in range(1, 76):
        put(g, 2, c, "═", "muted")
        put(g, 16, c, "═", "muted")
    for r in range(2, 17):
        put(g, r, 1, "║", "muted")
        put(g, r, 75, "║", "muted")
    put(g, 2, 1, "╔", "muted"); put(g, 2, 75, "╗", "muted"); put(g, 16, 1, "╚", "muted"); put(g, 16, 75, "╝", "muted")
    for c in (24, 25, 26, 50, 51, 52):
        put(g, 2, c, " "); put(g, 16, c, " ")
    for r in (6, 7, 11, 12):
        put(g, r, 1, " "); put(g, r, 75, " ")
    # roads: dashed centre lines, torches at the crossings by night
    for r in range(1, 18):
        for c in (25, 51):
            if g[r][c][0] == " ":
                put(g, r, c, "┊", "muted")
    for c in range(0, 77):
        if g[6][c][0] in (" ", "┊"):
            put(g, 6, c, "┄", "muted")
        if g[11][c][0] in (" ", "┊"):
            put(g, 11, c, "┄", "muted")
    for r in (6, 11):
        for c in (25, 51):
            put(g, r, c, "✦" if (night or dusk) else "┼", "yellow" if (night or dusk) else "muted")
    # insulae, framework order in a 3x3
    for i, (pid, latin, eng, _f) in enumerate(PILLARS):
        x, y = BX[i % 3], BY[i // 3]
        p = pm[pid]
        head = f" {TITLE[pid]} "
        lv = f" {roman(p['level'])} "
        line = "┌" + head + "─" * (BW - 2 - len(head) - len(lv)) + lv + "┐"
        put(g, y, x, line, "muted")
        put(g, y, x + 1, head, "accent" if pid == top else "foreground")
        put(g, y, x + 2, TITLE[pid][0], "accent")
        put(g, y, x + BW - 1 - len(lv), lv, "accent")
        put(g, y + 1, x, "│", "muted"); put(g, y + 1, x + BW - 1, "│", "muted")
        today, week = p.get("today", 0), max(0, p["events_7d"] - p.get("today", 0))
        cells = BW - 2
        for k in range(cells - 2):
            ch, col = ("▪", "yellow") if k < today else ("▫", "foreground") if k < today + week else ("·", "muted")
            put(g, y + 1, x + 2 + k, ch, col)
        put(g, y + 1, x + 1, " ")
        put(g, y + 1, x + BW - 2, "?" if pid in asks else " ", "magenta")
        el = f" {p['events_7d']} this week "
        bot = "└" + "─" * ((BW - 2 - len(el)) // 2) + el
        bot += "─" * (BW - 1 - len(bot)) + "┘"
        put(g, y + 2, x, bot, "muted")
        put(g, y + 2, x + 1 + (BW - 2 - len(el)) // 2, el, "muted")
        if pid == top:
            put(g, y + 2, x + 2, "@", "accent")
    def inside(row, c):
        return any(y <= row < y + 3 and x <= c < x + BW for x in BX for y in BY)

    # sky outside the walls, rain on open ground
    r = rnd(5)
    for row in (1, 17):
        for c in range(COLS):
            if g[row][c][0] == " " and next(r) < (0.08 if night else 0.0):
                put(g, row, c, "·" if next(r) > 0.2 else "*", "muted")
    if wx != "clear":
        dens = 0.06 if wx == "drizzle" else 0.16
        r = rnd(21)
        for row in list(range(1, 18)):
            for c in range(COLS):
                if g[row][c][0] == " " and not inside(row, c) and next(r) < dens:
                    put(g, row, c, "'", "blue")
    return g


def town_tile(s):
    return Canvas(f'<div class="asc">{ascii_grid(camp(s))}</div>')


def quests_tile(s):
    qs = ranked(s)
    open_ = [q for q in qs if not q["done_today"]]
    done = [q for q in qs if q["done_today"]]
    due = sum(1 for q in open_ if q["due"])
    rows = [Row(T("Iter · the route", "label", weight=True), T(f"{due} due" if open_ else "", "label"))]
    for q in done:
        rows.append(Row(T("●", "small", "green", size=10), T(q["title"], "small", "muted", size=10, weight=True, max_lines=1, cls="strike"),
                        T("↺", "small", "muted", size=10, tap="undo"), gap=6))
        rows.append(T("│", "small", "muted", size=10))
    for i, q in enumerate(open_[:4]):
        mark = "@" if i == 0 else "○"
        rows.append(Col(
            Row(T(mark, "small", "accent" if i == 0 else "muted", size=10, bold=i == 0),
                T(q["title"], "small", "foreground", size=10, weight=True, max_lines=1),
                T(f'{q["overdue_days"]}d', "small", "red", size=9) if q["overdue_days"] else "",
                T("✓", "small", "green", size=10, tap="done"), T("⋯", "small", "muted", size=10, tap="menu"), gap=6),
            T("│ " + " · ".join(q["reasons"]), "small", "muted", size=9, max_lines=2), gap=0))
    if len(open_) > 4:
        rows.append(T(f"┆ +{len(open_) - 4} further", "small", "muted", size=9, tap="scroll"))
    if not open_:
        rows.append(T("@ the road is empty", "small", "foreground", size=10))
        rows.append(T("│ add a stop below", "small", "muted", size=9))
    for p in s["proposed"][:2]:
        rows.append(Row(T("├◇", "small", "cyan", size=10), T(p["title"], "small", "foreground", size=10, weight=True, max_lines=1),
                        T("✓", "small", "green", size=10, tap="accept"), T("✕", "small", "muted", size=10, tap="decline"), gap=6))
    if len(s["proposed"]) > 2:
        rows.append(T(f"├ +{len(s['proposed']) - 2} side roads", "small", "muted", size=9))
    rows.append(T("┴", "small", "muted", size=10))
    return Col(Col(*rows, gap=2), Box(cls="fill"), InputN("add a stop  #Pillar"), gap=4, cls="fill")


def level_tile(s):
    x, st = s["xp"], s["streak"]
    n = 20
    pos = round(x["into"] / x["need"] * n)
    road = (f'<span class="c-accent">{roman(x["level"])} </span><span class="c-accent">{"━" * pos}</span>'
            f'<span class="c-accent b">@</span><span class="c-muted">{"┄" * (n - pos)}</span>'
            f'<span class="c-foreground"> {roman(x["level"] + 1)}</span>')
    stone = Col(T("╭───╮", "small", "muted", size=10), T(f"│{roman(x['level']):^3}│"[:5], "small", "accent", size=10, bold=True),
                T("│MP │", "small", "muted", size=10), T("└───┘", "small", "muted", size=10), gap=0)
    info = Col(T("Miliarium · level", "label"),
               T(f'{x["into"]}/{x["need"]} xp', "small", "foreground", size=9),
               T(f'streak · {st["days"]} days', "small", "foreground", size=9),
               T("today kept", "small", "green", size=9) if st["today"] else T("rest day ›", "small", "accent", size=9, tap="rest"),
               gap=1, cls="w1")
    return Col(Row(stone, info, gap=10, align="flex-start"), T(road, "small", size=10, raw=True, max_lines=1),
               Sparkline(s["history"], "accent", height=14), gap=4, cls="fill")


def strip_tile(s):
    def cell(p):
        n = p["events_7d"]
        return [Row(T("┼", "small", "muted", size=8), T(roman(p["level"]), "small", "accent", size=8), laurel(s, p["pillar"]), gap=1),
                T("·" if not n else "▪" * min(n, 4) + ("+" if n > 4 else ""), "small", "green" if n else "muted", size=8)]
    return pillar_strip(s, cell)


def build_board(theme, s, key, theme_name):
    tiles = [
        tile("Town", 6, 3, 0, 0, town_tile(s)),
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
        ask = (f'<div class="popup" style="left:330px;top:120px">'
               + Col(T("? Recovery asks", "small", "magenta", size=10, bold=True),
                     T("What's your target sleep window?", "small", "foreground", size=10),
                     InputN("e.g. 23:00–07:00"),
                     Row(T("[send]", "small", "accent", size=10, tap="send"),
                         T("[skip]", "small", "muted", size=10, tap="skip"), gap=10), gap=6) + '</div>')
        stage = (f'<div class="pop-stage th-{th}"><div class="board" style="padding:0;background:none;'
                 f'grid-template-rows:repeat(3,96px)">{tile("Town", 6, 3, 0, 0, town_tile(s))}</div>{ask}</div>')
        out.append(('' if only else f'<h3>{th}</h3>') + f'<div class="pop-demo">{stage}</div>')
    return "".join(out)


def tapmaps():
    return "".join([
        tapmap("Town · 6x3", "One Canvas. The tap's x and y pick the insula on the 3×3 grid.", [
            ("Insula with ?", "open the check-in Popup", "Popup"),
            ("Popup · send", "answer", C.answer),
            ("Popup · skip", "hide for today", C.skip),
            ("Any other insula", "show the pillar's numbers", C.pillar),
        ]),
        tapmap("Quests · 3x4", "The route: @ is the next stop, ● stops passed today, ◇ side roads proposed by agents.", [
            ("✓", "mark done", C.done),
            ("↺ on ●", "undo", C.undo),
            ("⋯ → snooze", "snooze", C.snooze1),
            ("⋯ → priority up / down", "priority", C.up),
            ("⋯ → drop", "drop", C.drop),
            ("✓ / ✕ on ◇", "accept or decline", C.accept),
            ("add · Enter", "add a stop", C.add),
        ]),
        tapmap("Level and streak · 3x2", "A milestone stone and the road to the next one.", [
            ("rest day", "rest day (until today is kept)", C.rest),
            ("Tile", "xp by day", C.xp),
        ]),
        tapmap("Pillar strip · 6x1", "Nine waymarks.", [("A waymark", "show its numbers", C.pillar)]),
        tapmap(*TYPING_TAPS),
    ])


NOTES = """
<div><h3>Nodes per tile</h3><ul>
<li><b>Town</b>: one <code>Canvas</code> of text runs, like Forum, but the grid is fixed (walls, roads, nine 21×3 insulae), so only the tesserae, @, ? and rain change. About 200 calls.</li>
<li><b>Quests</b>: Texts on a <code>Column</code>; the route glyphs are plain characters. ⋯ opens the same <code>Popup</code> as Forum.</li>
<li><b>Level</b>: the stone is four Texts; the road to the next milestone is one Text of ━ and ┄ with @ at into/need.</li>
</ul></div>
<div><h3>Why a camp</h3><ul>
<li>A Roman camp is a grid by design, and nine pillars make a clean 3×3: CORE across the top row and a half, CRAFT below.</li>
<li>Each insula shows a week at once: ▪ today, ▫ earlier this week. The skyline directions only show today.</li>
</ul></div>
<div><h3>Watch for</h3><ul>
<li>Levels show only as a numeral, since the plan has no height.</li>
<li>Same width risk as Forum: 77 columns at 9 dp.</li>
</ul></div>
"""
