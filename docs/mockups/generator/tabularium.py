"""Direction B · Tabularium: every tile is a ledger."""
from common import *
from data import ranked, roman, LATIN, ENG, TITLE, PILLARS
from town import town

SLUG = "tabularium"


def town_tile(s):
    return Canvas(f'<div class="asc">{ascii_grid(town(s, "tabularium"))}</div>')


def quests_tile(s):
    qs = ranked(s)
    open_ = [q for q in qs if not q["done_today"]]
    done = [q for q in qs if q["done_today"]]
    late = sum(1 for q in open_ if q["overdue_days"] > 0)
    due = sum(1 for q in open_ if q["due"])
    head = Row(T("Pensa · quests", "label", size=9, weight=True),
               T(f"{len(open_)} open · {due} due · {late} late", "small", "muted", size=9))
    cols = Row(T("#", "small", "muted", size=8, cls="cw2"), T("quest", "small", "muted", size=8, weight=True),
               T("score", "small", "muted", size=8), T("✓ z ▲▼", "small", "muted", size=8), gap=6)
    rows = []
    for i, q in enumerate(open_, 1):
        rows.append(Col(
            Row(T(str(i), "small", "muted", size=9),
                T(q["title"], "small", "foreground", size=9, weight=True, max_lines=1),
                T(f'{q["overdue_days"]}d', "small", "red", size=9) if q["overdue_days"] else "",
                T(f'{q["score"]:.1f}', "small", "accent" if i == 1 else "foreground", size=9),
                T("✓", "small", "green", size=9, tap="done"), T("z", "small", "muted", size=9, tap="snooze"),
                T("▲", "small", "muted", size=9, tap="up"), T("▼", "small", "muted", size=9, tap="down"), gap=4),
            T("  " + " · ".join(q["reasons"]), "small", "muted", size=8, max_lines=2), gap=0))
    if not open_:
        rows.append(T("   0 quests · nothing ranked yet", "small", "muted", size=9))
    for q in done:
        rows.append(Row(T(" ✓", "small", "green", size=9),
                        T(q["title"], "small", "muted", size=9, weight=True, max_lines=1, cls="strike"),
                        T(q["reasons"][0].replace("done ", ""), "small", "muted", size=8),
                        T("↺", "small", "muted", size=9, tap="undo"), gap=6))
    prop = [T(f"proposed {len(s['proposed'])}", "label", size=9)]
    for p in s["proposed"]:
        prop.append(Row(T(" ◇", "small", "cyan", size=9),
                        T(p["title"], "small", "foreground", size=9, weight=True, max_lines=1),
                        T(p["from"].replace("duet:", "")[:5], "small", "muted", size=8),
                        T("✓", "small", "green", size=9, tap="accept"), T("✕", "small", "muted", size=9, tap="decline"),
                        gap=6))
    if not s["proposed"]:
        prop.append(T("   none waiting", "small", "muted", size=8))
    return Col(head, cols, Col(*rows, gap=2), Col(*prop, gap=2), Box(cls="fill"),
               InputN("add: title #pillar every:daily"), gap=5, cls="fill")


def level_tile(s):
    x, st = s["xp"], s["streak"]
    h = s["history"]
    return Col(
        Row(T("Gradus · level", "label", size=9, weight=True), T("streak", "label", size=9)),
        Row(T(roman(x["level"]), "title", "accent"), T(f'{x["total"]} xp', "body", "foreground", weight=True),
            T(f'{st["days"]}d', "title", "foreground"),
            T("●", "small", "green") if st["today"] else T("[rest]", "small", "accent", size=9, tap="rest"), align="baseline"),
        Progress(x["into"], x["need"], "accent", height=4),
        Row(T(f'{x["into"]}/{x["need"]} → {roman(x["level"] + 1)}', "small", "muted", size=9, weight=True),
            T(f'{x["need"] - x["into"]} to go', "small", "muted", size=9)),
        Sparkline(h, "accent", height=18),
        Row(T("xp · 30d", "small", "muted", size=8, weight=True),
            T(f"Σ{sum(h)} · max {max(h)} · today {h[-1]}", "small", "muted", size=8)),
        gap=3, cls="fill")


def checkins_tile(s):
    cs = [c for c in s["checkins"] if c.get("open")]
    head = Row(T("Rogationes · check-ins", "label", size=9, weight=True), T(f"{len(cs)} open", "small", "muted", size=9))
    if not cs:
        return Col(head, T("no open questions", "small", "foreground", size=9),
                   T("pillar agents ask here", "small", "muted", size=8), gap=4, cls="fill")
    c = cs[0]
    first = Col(Row(T("?", "small", "magenta", size=9, bold=True),
                    title_t(c["pillar"], size=8, color="muted"), gap=6),
                T(c["question"], "small", "foreground", size=9, max_lines=2),
                InputN("answer, Enter sends"),
                Row(T("[skip]", "small", "muted", size=9, tap="skip"), T("[later]", "small", "muted", size=9, tap="later"),
                    gap=10), gap=3)
    rest = [Row(T("?", "small", "magenta", size=9), T(o["question"], "small", "foreground", size=8, weight=True, max_lines=1),
                gap=6, tap="focus") for o in cs[1:]]
    return Col(head, first, Col(*rest, gap=1) if rest else "", gap=5, cls="fill")


def strip_tile(s):
    def cell(p):
        return [Row(T(roman(p["level"]), "small", "accent", size=9), laurel(s, p["pillar"]),
                    T(str(p["events_7d"]), "small", "green" if p["events_7d"] else "muted", size=9), gap=3),
                Progress(p["into"], p["need"], "accent", height=2)]
    return pillar_strip(s, cell)


def build_board(theme, s, key, theme_name):
    tiles = [
        tile("Town", 6, 3, 0, 0, town_tile(s)),
        tile("Pillar strip", 6, 1, 0, 3, strip_tile(s)),
        tile("Quests", 3, 4, 0, 4, quests_tile(s)),
        tile("Level and streak", 3, 2, 3, 4, level_tile(s)),
        tile("Check-ins", 3, 2, 3, 6, checkins_tile(s)),
        tile("Typing", 3, 2, 0, 8, typing_tile(s)),
        free_slot(3, 2, 3, 8),
    ]
    return board(theme, tiles, 10, key, theme_name)


def popups(only=None):
    from data import NORMAL as s
    out = []
    for th in ([only] if only else THEMES):
        snooze = (f'<div class="popup" style="left:56px;top:96px;width:240px">'
                  + Col(T("z · snooze 15 min typing drill", "small", "foreground", size=9, bold=True),
                        Row(T("[1d]", "small", "accent", size=9, tap="1d"), T("[3d]", "small", "accent", size=9, tap="3d"),
                            T("[1w]", "small", "accent", size=9, tap="1w"), T("[drop]", "small", "muted", size=9, tap="drop"), gap=8),
                        T("streak is safe: a snooze is not a miss", "small", "muted", size=8), gap=5)
                  + '</div>')
        stage = (f'<div class="pop-stage th-{th}"><div class="board" style="padding:0;background:none;'
                 f'grid-template-columns:repeat(3,96px);grid-template-rows:repeat(4,96px)">'
                 f'{tile("Quests", 3, 4, 0, 0, quests_tile(s))}</div>{snooze}</div>')
        out.append(('' if only else f'<h3>{th}</h3>') + f'<div class="pop-demo">{stage}</div>')
    return ('<p class="lede">This direction keeps every action inline, so it needs one Popup: the snooze choice.</p>'
            + "".join(out))


NOTES = """
<div><h3>Nodes per tile</h3><ul>
<li><b>Town</b>: the same Canvas as Forum, one roof row per building and floors equal to level. ▪ is today, ▫ earlier this week, · empty.</li>
<li><b>Quests</b>: one <code>Row</code> per quest with eight children (rank, title, late, score, four taps) plus a reasons Text. About 110 nodes busy, the biggest tile on any board.</li>
<li><b>Level</b>: <code>Text</code>, <code>Progress</code>, <code>Sparkline</code>.</li>
<li><b>Check-ins</b>: <code>Text</code>, one <code>Input</code>, a <code>state focus</code> for which question it answers.</li>
<li><b>Strip</b>: nine <code>Column</code>s with a 2 dp <code>Progress</code> each.</li>
</ul></div>
<div><h3>Snapshot fields this needs</h3><ul>
<li><code>pillars[].today</code> for the town, as in Forum.</li>
<li>Everything else is already in the brief's shape: score, reasons, level, into, need, events_7d.</li>
<li>Six tiles take 10 rows; a 3x2 is left free. It could hold a later tile (a review queue, the bar pill's big brother).</li>
</ul></div>
<div><h3>Watch for</h3><ul>
<li>8–9 dp text everywhere. Legible on a 27" screen, tiring on a laptop.</li>
<li>Four 1-character tap targets per quest are about 10 px wide. Fine with a mouse, poor for touch.</li>
<li>Scores on screen invite second-guessing the ranking.</li>
</ul></div>
"""


def tapmaps():
    return "".join([
        tapmap("Town · 6x3", "One Canvas, read like a census. Taps show records, not actions.", [
            ("A building", "show the pillar's numbers", C.pillar),
        ]),
        tapmap("Quests · 3x4", "Every action sits on the row.", [
            ("✓", "mark done", C.done),
            ("z → 1d / 3d / 1w", "snooze until that day", C.snooze3),
            ("z → drop", "drop the quest", C.drop),
            ("▲ / ▼", "priority up or down (1..3)", C.down),
            ("↺ on a done row", "undo today's done", C.undo),
            ("✓ on ◇", "accept", C.accept),
            ("✕ on ◇", "decline", C.decline),
            ("add · Enter", "add with #pillar and every:", C.add),
        ]),
        tapmap("Level and streak · 3x2", "Numbers only.", [
            ("[rest]", "rest day (shown until today is kept)", C.rest),
            ("Tile", "xp by day", C.xp),
        ]),
        tapmap("Check-ins · 3x2", "The oldest open question gets the input.", [
            ("Input · Enter", "answer", C.answer),
            ("[skip] / [later]", "hide for today", C.skip),
            ("Another ? row", "move the input to it", "state: focus = id"),
        ]),
        tapmap("Pillar strip · 6x1", "Level, 7-day events, progress to next level.", [
            ("A pillar cell", "show its numbers", C.pillar),
        ]),
        tapmap(*TYPING_TAPS),
    ])
