"""Direction A · Forum: the town is the board."""
from common import *
from data import ranked, roman, LATIN, ENG, TITLE, PILLARS, top_pillar
from town import town

SLUG = "forum"


def town_tile(s):
    return Canvas(f'<div class="asc">{ascii_grid(town(s, "forum"))}</div>')


def quests_tile(s):
    qs = ranked(s)
    open_ = [q for q in qs if not q["done_today"]]
    done = [q for q in qs if q["done_today"]]
    due = sum(1 for q in open_ if q["due"])
    head = Row(T("Pensa · quests", "label", weight=True), T(f"{due} due" if open_ else "", "label"))
    items = []
    show = open_[:4]
    for i, q in enumerate(show, 1):
        late = q["overdue_days"] > 0
        items.append(Col(
            Row(T(str(i), "small", "muted", size=10),
                T(q["title"], "small", "foreground", size=10, weight=True, max_lines=1),
                T(f'{q["overdue_days"]}d', "small", "red", size=9) if late else "",
                T("✓", "small", "green", size=10, tap="done"),
                T("⋯", "small", "muted", size=10, tap="menu"), gap=6),
            T("  " + " · ".join(q["reasons"]), "small", "muted", size=9, max_lines=2), gap=0))
    if len(open_) > 4:
        items.append(T(f"  +{len(open_) - 4} more ↓", "small", "muted", size=9, tap="scroll"))
    if not open_:
        items.append(Col(T("nothing due", "small", "foreground", size=10),
                         T("nothing due. add one below, or wait for an agent's proposal.", "small", "muted", size=9), gap=0))
    for q in done:
        items.append(Row(T("✓", "small", "green", size=10),
                         T(q["title"], "small", "muted", size=10, weight=True, max_lines=1, cls="strike"),
                         T("↺", "small", "muted", size=10, tap="undo"), gap=6))
    prop = []
    if s["proposed"]:
        prop.append(T(f"─ proposed {len(s['proposed'])} " + "─" * 24, "label", size=9, max_lines=1))
        for p in s["proposed"][:2]:
            prop.append(Col(
                Row(T("◇", "small", "cyan", size=10),
                    T(p["title"], "small", "foreground", size=10, weight=True, max_lines=1),
                    T("✓", "small", "green", size=10, tap="accept"),
                    T("✕", "small", "muted", size=10, tap="decline"), gap=6),
                T(f'  {TITLE[p["pillar"]]} · from {p["from"]}', "small", "muted", size=9, max_lines=1), gap=0))
        if len(s["proposed"]) > 2:
            prop.append(T(f"  +{len(s['proposed']) - 2} more", "small", "muted", size=9))
    return Col(head, Col(*items, gap=3), Col(*prop, gap=3) if prop else "",
               Box(cls="fill"), InputN("add a quest  #Pillar"), gap=6, cls="fill")


def level_tile(s):
    x, st = s["xp"], s["streak"]
    n = 26
    filled = round(x["into"] / x["need"] * n)
    bar = f'<span class="c-accent">{"▰" * filled}</span><span class="c-muted">{"▱" * (n - filled)}</span>'
    left = Col(T("Gradus · level", "label"), T(roman(x["level"]), "title", "accent"),
               T(f'{x["into"]}/{x["need"]} xp → {roman(x["level"] + 1)}', "small", "muted", size=9), gap=0,
               cls="w1")
    right = Col(T("streak", "label", align="end"),
                T(str(st["days"]), "title", "foreground", align="end"),
                T("days · today ✓", "small", "green", size=9, align="end") if st["today"] else
                T("rest day ›", "small", "accent", size=9, align="end", tap="rest"), gap=0)
    return Col(Row(left, right, align="flex-start"), T(bar, "small", raw=True, max_lines=1),
               Sparkline(s["history"], "accent", height=20),
               Row(T("xp · 30 days", "label", size=9, weight=True), T(f"today +{s['history'][-1]}", "label", size=9)),
               gap=4, cls="fill")


def strip_tile(s):
    def cell(p):
        n = p["events_7d"]
        dots = "·" if n == 0 else "▪" * n if n <= 3 else "▪▪+"
        return [Row(T(roman(p["level"]), "small", "accent", size=8), laurel(s, p["pillar"]),
                    T(dots, "small", "green" if n else "muted", size=8), gap=3)]
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
        # quest menu
        qmenu = (f'<div class="popup" style="left:34px;top:64px;width:262px">'
                 + Col(T("15 min typing drill", "small", "foreground", size=10, bold=True),
                       T("Competence · daily", "small", "muted", size=9),
                       T("✓ done", "small", "green", size=10, tap="done"),
                       T("◷ snooze 1 day", "small", "foreground", size=10, tap="snooze"),
                       T("◷ snooze 1 week", "small", "foreground", size=10, tap="snooze"),
                       T("▲ priority up", "small", "foreground", size=10, tap="up"),
                       T("▼ priority down", "small", "foreground", size=10, tap="down"),
                       T("✕ drop", "small", "muted", size=10, tap="drop"), gap=4)
                 + '</div>')
        stage1 = (f'<div class="pop-stage th-{th}"><div class="board" style="padding:0;background:none;'
                  f'grid-template-columns:repeat(3,96px);grid-template-rows:repeat(4,96px)">'
                  f'{tile("Quests", 3, 4, 0, 0, quests_tile(s))}</div>{qmenu}</div>')
        ask = (f'<div class="popup" style="left:300px;top:70px">'
               + Col(T("? Recovery asks", "small", "magenta", size=10, bold=True),
                     T("What's your target sleep window?", "small", "foreground", size=10),
                     InputN("e.g. 23:00–07:00"),
                     Row(T("[send]", "small", "accent", size=10, tap="send"),
                         T("[skip]", "small", "muted", size=10, tap="skip"), gap=10), gap=6)
               + '</div>')
        stage2 = (f'<div class="pop-stage th-{th}"><div class="board" style="padding:0;background:none;'
                  f'grid-template-rows:repeat(3,96px)">{tile("Town", 6, 3, 0, 0, town_tile(s))}</div>{ask}</div>')
        out.append(('' if only else f'<h3>{th}</h3>') + f'<div class="pop-demo">{stage1}{stage2}</div>')
    return "".join(out)


NOTES = """
<div><h3>Nodes per tile</h3><ul>
<li><b>Town</b>: one <code>Canvas</code>. The 77×18 grid is built as a list of rows in a <code>let</code>, each row drawn with one <code>text()</code> call per color run (about 300 calls a frame at night, under the 2000 limit). Rain reads <code>time</code> to fall; leave it out and the town costs nothing between snapshots.</li>
<li><b>Quests</b>: <code>Column</code> of <code>Row</code>s of <code>Text</code>, the ✓ ⋯ ↺ glyphs are Texts with <code>onTap</code>, <code>Popup</code> for ⋯, <code>Input</code> with <code>onSubmit</code>. About 70 nodes busy.</li>
<li><b>Level</b>: <code>Text</code>, an ASCII bar in one Text, <code>Sparkline</code>.</li>
<li><b>Strip</b>: <code>Row</code> of nine <code>Column</code>s, three Texts each.</li>
</ul></div>
<div><h3>Snapshot fields this needs</h3><ul>
<li><code>pillars[].today</code>: events today per pillar, to light windows. Not in the brief's shape yet.</li>
<li>Night comes from <code>source now = clock</code>; rain from overdue quests (1 = drizzle, 2+ = storm). Both are proposals.</li>
<li>Check-ins are folded into the town, so this board is five tiles and fits in 8 rows.</li>
</ul></div>
<div><h3>Watch for</h3><ul>
<li>The town needs ≥77 columns of the theme mono at 9 dp. On the real board (pitch 96 including a 6 px gap) a 6-wide tile is 570 px, 46 px narrower than here: the town would need fontSize 8.3 dp or a 72-column grid.</li>
<li>Box-drawing and ▪ ∩ ◐ ☼ glyphs must exist in the theme font. JetBrains Mono has them; a theme with another mono may not.</li>
</ul></div>
"""


def tapmaps():
    return "".join([
        tapmap("Town · 6x3", "One Canvas. A tap's x picks the building: index = floor((x − 3 ch) / 8 ch).", [
            ("Building with ?", "open the check-in Popup", "Popup (see Popups)"),
            ("Popup · send", "answer the check-in", C.answer),
            ("Popup · skip", "skip it for today", C.skip),
            ("Any other building", "show the pillar's numbers", C.pillar),
        ]),
        tapmap("Quests · 3x4", "Ranked by score. Two reasons under each title.", [
            ("✓", "mark done", C.done),
            ("↺ on a done row", "undo today's done", C.undo),
            ("⋯ → snooze 1 day", "snooze one day", C.snooze1),
            ("⋯ → snooze 1 week", "snooze one week", C.snooze7),
            ("⋯ → priority up / down", "priority up or down (1..3)", C.up),
            ("⋯ → drop", "drop the quest", C.drop),
            ("✓ on ◇ proposed", "accept the agent's quest", C.accept),
            ("✕ on ◇ proposed", "decline it", C.decline),
            ("add · Enter", "add; #pillar sets the pillar, slug from the title", C.add),
        ]),
        tapmap("Level and streak · 3x2", "Numbers. Rest shows only while today isn't kept yet.", [
            ("rest day", "mark today a rest day, keeps the streak", C.rest),
            ("Tile", "xp by day", C.xp),
        ]),
        tapmap("Pillar strip · 6x1", "Nine cells, one per pillar.", [
            ("A pillar cell", "show its numbers", C.pillar),
        ]),
        tapmap(*TYPING_TAPS),
    ])
