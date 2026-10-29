"""Direction E · Annales: the board reads as a chronicle."""
from common import *
from data import ranked, roman, LATIN, ENG, TITLE, PILLARS, top_pillar, is_night, is_dusk, weather, overdue_count
from town import ROOFS, put, rnd, title_labels

SLUG = "annales"
COLS, SROWS = 77, 8


def skyline(s):
    g = [[(" ", None) for _ in range(COLS)] for _ in range(SROWS)]
    night, dusk, wx = is_night(s), is_dusk(s), weather(s)
    top = top_pillar(s)
    asks = {c["pillar"] for c in s["checkins"] if c.get("open")}
    pm = {p["pillar"]: p for p in s["pillars"]}
    occ = set()
    for i, (pid, latin, eng, _f) in enumerate(PILLARS):
        p, x = pm[pid], 3 + i * 8
        floors = max(1, min(3, p["level"]))
        for f in range(floors):
            r = 4 - f
            put(g, r, x, "│· · ·│", "muted")
            for k in range(3):
                lit = f * 3 + k < p.get("today", 0)
                put(g, r, x + 1 + 2 * k, "▪" if lit else "·", "yellow" if lit else "muted")
        rr = 4 - floors
        put(g, rr, x, ROOFS[pid][1], "muted")
        for r in range(rr, 5):
            for c in range(x, x + 7):
                occ.add((r, c))
        if pid in asks and rr > 0:
            put(g, rr - 1, x + 3, "?", "magenta"); occ.add((rr - 1, x + 3))
        title_labels(g, i, pid, top, row=6)
    put(g, 5, 1, "─" * 75, "muted")
    if top:
        i = [p[0] for p in PILLARS].index(top)
        put(g, 5, 3 + i * 8 + 3, "@", "accent")
    r = rnd(17)
    if night:
        for row in (0, 1):
            for c in range(COLS):
                if (row, c) not in occ and g[row][c][0] == " " and next(r) < 0.06:
                    put(g, row, c, "·", "muted")
        put(g, 0, 72, "◐", "yellow")
    elif dusk:
        put(g, 1, 72, "◑", "yellow")
    else:
        put(g, 0, 72, "☼", "yellow")
    if wx != "clear":
        dens = 0.05 if wx == "drizzle" else 0.13
        r = rnd(31)
        for row in range(0, 5):
            for c in range(COLS):
                if (row, c) not in occ and g[row][c][0] == " " and next(r) < dens:
                    put(g, row, c, "'", "blue")
    return g


def em(t, c="foreground", b=False):
    return f'<span class="c-{c}{" b" if b else ""}">{esc(t)}</span>'


def chronicle(s):
    night, dusk, wx = is_night(s), is_dusk(s), weather(s)
    late = overdue_count(s)
    tod = "Night" if night else "Dusk" if dusk else "Day"
    sky = {"clear": "Clear sky", "drizzle": "A light rain", "storm": "A storm"}[wx]
    nums = ["no", "one", "two", "three", "four", "five", "six"]
    lines = []
    l1 = f'{em(tod, "foreground", True)}, {em(s["clock"], "accent")}. {sky}'
    l1 += f'; {em(nums[late] + (" quest is" if late == 1 else " quests are"), "red")} late.' if late else "."
    if s["errors"]:
        l1 += f' {em("●", "red")} {len(s["errors"])} log line failed to read.'
    lines.append(l1)
    lit = sorted([p for p in s["pillars"] if p.get("today")], key=lambda p: -p["today"])
    if lit:
        parts = [f'{em(TITLE[p["pillar"]])}' + (f' ({em(str(p["today"]), "yellow")})' if p["today"] > 1 else "") for p in lit[:5]]
        if len(lit) > 5:
            parts.append(f"{len(lit) - 5} more")
        lines.append("Lights in " + ", ".join(parts[:-1]) + (" and " if len(parts) > 1 else "") + parts[-1] + ".")
    else:
        lines.append("No lights yet. Nothing is recorded today.")
    open_ = [q for q in ranked(s) if not q["done_today"]]
    if open_:
        q = open_[0]
        lines.append(f'{em("@", "accent", True)} waits at {em(TITLE[q["pillar"]], "accent")}: {em(q["title"])}, '
                     f'{q["reasons"][0]}.')
    else:
        lines.append(f'No quest is ranked, so {em("@", "accent", True)} has nowhere to stand.')
    quiet = [TITLE[p["pillar"]] for p in s["pillars"] if p["events_7d"] == 0]
    if quiet and len(quiet) < 9:
        lines.append("Quiet all week: " + ", ".join(em(q) for q in quiet) + ".")
    cs = [c for c in s["checkins"] if c.get("open")]
    if cs:
        c = cs[0]
        pre = (nums[len(cs)].capitalize() + " questions wait. ") if len(cs) > 1 else ""
        lines.append(f'{pre}{em(TITLE[c["pillar"]], "magenta")} asks: {em(c["question"])}')
    else:
        lines.append("No questions wait.")
    return lines, cs


def town_tile(s):
    lines, cs = chronicle(s)
    prose = Col(*[T(l, "small", "muted", size=9, raw=True, max_lines=1) for l in lines], gap=1)
    bottom = (Row(InputN(f"answer {TITLE[cs[0]['pillar']]}, Enter sends", cls="w1"),
                  T("skip", "small", "muted", size=9, tap="skip"), T("›", "small", "muted", size=9, tap="next") if len(cs) > 1 else "",
                  gap=8) if cs else T("the record is up to date", "label", size=9))
    return Col(Row(T("Annales · the chronicle", "label", size=9, weight=True), T(s["today"]["date"], "label", size=9)),
               Canvas(f'<div class="asc">{ascii_grid(skyline(s))}</div>'), prose, Box(cls="fill"), bottom, gap=4, cls="fill")


def sentence(reasons):
    s = "; ".join(reasons)
    return s[:1].upper() + s[1:] + "."


def quests_tile(s):
    qs = ranked(s)
    open_ = [q for q in qs if not q["done_today"]]
    done = [q for q in qs if q["done_today"]]
    rows = [T("Pensa · what's next", "label")]
    for i, q in enumerate(open_[:3], 1):
        taps = (f'{em("done", "green")} · {em("tomorrow", "foreground")} · {em("↑", "muted")} {em("↓", "muted")} · {em("drop", "muted")}')
        rows.append(Col(Row(T(f"{i}.", "small", "accent", size=10),
                            T(q["title"] + ".", "small", "foreground", size=10, bold=True, weight=True, max_lines=1), gap=6),
                        T(sentence(q["reasons"]), "small", "red" if q["overdue_days"] > 1 else "muted", size=9, max_lines=2),
                        T(taps, "small", size=9, raw=True, tap="row"), gap=0))
    if len(open_) > 3:
        rows.append(T(f"And {len(open_) - 3} more, ranked lower. ›", "small", "muted", size=9, tap="more"))
    if not open_:
        rows.append(T("Nothing is due. Add a quest, or accept one an agent proposes.", "small", "muted", size=9))
    for q in done:
        rows.append(T(f'Done today: {em(q["title"], "muted")}. {em("undo", "muted")}', "small", "green", size=9, raw=True, tap="undo"))
    if s["proposed"]:
        p = s["proposed"][0]
        more = f' And {len(s["proposed"]) - 1} more.' if len(s["proposed"]) > 1 else ""
        rows.append(T(f'{em(TITLE[p["pillar"]], "cyan")} proposes {em(p["title"])}. '
                      f'{em("accept", "green")} · {em("decline", "muted")}{esc(more)}', "small", "muted", size=9, raw=True, max_lines=2))
    return Col(Col(*rows, gap=6), Box(cls="fill"), InputN("add · Stretch 10 min #Recovery daily"), gap=4, cls="fill")


def level_tile(s):
    x, st = s["xp"], s["streak"]
    return Col(
        T("Gradus · level", "label", align="center"),
        T(f"· {roman(x['level'])} ·", "title", "accent", align="center"),
        T(f"{x['into']} of {x['need']} xp to {roman(x['level'] + 1)}", "small", "foreground", size=9, align="center"),
        Progress(x["into"], x["need"], "accent", height=2),
        T(f"streak · {st['days']} days", "small",
          "foreground" if st["days"] else "muted", size=9, align="center") if st["today"] else
        T(f"streak · {st['days']} days · rest day ›", "small", "accent", size=9, align="center", tap="rest"),
        Sparkline(s["history"], "muted", height=10),
        gap=2, cls="fill")


def strip_tile(s):
    def cell(p):
        n = p["events_7d"]
        return [Row(T(roman(p["level"]), "small", "accent", size=8), laurel(s, p["pillar"]), gap=3),
                T(f"{n} / 7d", "small", "green" if n else "muted", size=8)]
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
    return ('<p class="lede">Annales has no Popup. Every action is a word in a sentence: done, tomorrow, accept, drop. '
            'The check-in input sits on the last line of the town.</p>')


def tapmaps():
    return "".join([
        tapmap("Town · 6x3", "A small skyline over a few sentences. The last line answers the oldest open check-in.", [
            ("Input · Enter", "answer the check-in named on the line", C.answer),
            ("skip", "hide it for today", C.skip),
            ("›", "show the next waiting question", "state: ask = next id"),
            ("A pillar title", "show its numbers", C.pillar),
        ]),
        tapmap("Quests · 3x4", "Three quests as sentences, the actions as words under each.", [
            ("done", "mark done", C.done),
            ("tomorrow", "snooze a day", C.snooze1),
            ("↑ / ↓", "priority up or down", C.up),
            ("drop", "drop", C.drop),
            ("undo", "undo today's done", C.undo),
            ("accept / decline", "accept or decline the proposal", C.accept),
            ("And N more ›", "page to the next three (state)", "state: page = page + 1"),
            ("add · Enter", "add; #pillar and a cadence word", C.add),
        ]),
        tapmap("Level and streak · 3x2", "An inscription: the level as a Roman numeral, xp and days in digits.", [
            ("rest day", "rest day (until today is kept)", C.rest),
            ("Tile", "xp by day", C.xp),
        ]),
        tapmap("Pillar strip · 6x1", "Level and things done this week.", [("A pillar", "show its numbers", C.pillar)]),
        tapmap(*TYPING_TAPS),
    ])


NOTES = """
<div><h3>Nodes per tile</h3><ul>
<li><b>Town</b>: a 7-row <code>Canvas</code> skyline (about 80 text calls), then a <code>Column</code> of sentences. A sentence with colored words is a <code>Flow</code> of <code>Text</code>s, built with <code>for</code> from a list of [word, color] pairs in a <code>let</code>. Then one <code>Input</code>.</li>
<li><b>Quests</b>: Texts only. The action words are Texts with <code>onTap</code> inside a <code>Flow</code>.</li>
<li><b>Level</b>: the numeral comes from a small <code>let</code> over a lookup list; only levels use it.</li>
</ul></div>
<div><h3>Why a chronicle</h3><ul>
<li>acta is "things done", the Roman record. Here the board writes that record back to you in sentences.</li>
<li>It is the only direction that explains itself: the reasons that rank a quest become the sentence under it.</li>
</ul></div>
<div><h3>Watch for</h3><ul>
<li>Sentences need wording rules in OWL (plurals, lists with "and"), which is fiddly in an expression language.</li>
<li>Prose is slower to scan than numbers, and long titles clip.</li>
</ul></div>
"""
