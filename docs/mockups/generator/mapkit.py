"""Shared pieces of the full-screen town maps: Forum buildings, Castra week blocks, the rail, the screen."""
import copy
import hashlib

from common import T, Row, Col, Box, Progress, Sparkline, InputN, Canvas, ascii_grid, esc, attrs, THEMES, DP
from data import STATES as BASE_STATES, TITLE, PILLARS, roman, ranked, top_pillar, is_night, is_dusk, weather
from town import ROOFS, WALL, rnd

W = 77          # characters across the screen at 9 dp (582 px inner width)


# ---- synthetic states, moved to the shape main has after PRs 8 and 9

def _sfx(i):
    return i + "-" + hashlib.sha1(i.encode()).hexdigest()[:4]


def _upgrade(s):
    s = copy.deepcopy(s)
    for q in s["quests"]:
        q["id"] = _sfx(q["id"])
    for p in s["proposed"]:
        p["id"] = _sfx(p["id"])
        p.setdefault("priority", 2)
        p.setdefault("every", "once")
    for c in s["checkins"]:
        c["id"] = _sfx(c["id"])
        c.setdefault("from", "duet:" + c["pillar"])
        c.setdefault("asked", "2026-10-31")
    s["errors"] = [{"line": 412, "reason": "invalid JSON"}] if s["errors"] else []
    for p in s["pillars"]:
        p["xp"] = p["into"]
    return s


STATES = [(k, l, _upgrade(s)) for k, l, s in BASE_STATES]
NORMAL = STATES[1][2]


def qid(state, stem):
    return next(q["id"] for q in state["quests"] + state["proposed"] if q["id"].startswith(stem))


def cid(state, stem):
    return next(c["id"] for c in state["checkins"] if c["id"].startswith(stem))


# ---- the character grid

class Grid:
    def __init__(self, w, h):
        self.w, self.h = w, h
        self.c = [[(" ", None)] * w for _ in range(h)]
        self.occ = [[False] * w for _ in range(h)]

    def put(self, r, c, text, color=None, occ=True):
        for i, ch in enumerate(text):
            if 0 <= r < self.h and 0 <= c + i < self.w:
                self.c[r][c + i] = (ch, color)
                if occ:
                    self.occ[r][c + i] = True

    def mark(self, r0, r1, c0, c1):
        for r in range(max(0, r0), min(self.h, r1 + 1)):
            for c in range(max(0, c0), min(self.w, c1 + 1)):
                self.occ[r][c] = True

    def blank(self, r, c):
        return 0 <= r < self.h and 0 <= c < self.w and not self.occ[r][c] and self.c[r][c][0] == " "


def context(s):
    """What every map reads from the snapshot."""
    open_ = [q for q in ranked(s) if not q["done_today"]]
    stops = {}
    for i, q in enumerate(open_[:4], 1):
        stops.setdefault(q["pillar"], i)
    return dict(
        night=is_night(s), dusk=is_dusk(s), wx=weather(s), top=top_pillar(s), stops=stops,
        asks={c["pillar"] for c in s["checkins"] if c.get("open")},
        pm={p["pillar"]: p for p in s["pillars"]},
        laurel=set(s.get("leveled_today", [])),
    )


def building(g, x, base, pid, floors, today, ctx):
    """A Forum elevation, 7 wide; base is the ground-floor row. Returns the roof's top row."""
    wall = WALL.get(pid, "│")
    slots = []
    for f in range(floors):
        r = base - f
        g.put(r, x, wall + "     " + wall, "muted")
        if f == 0:
            g.put(r, x + 3, "∩", "muted")
            slots += [(r, x + 1), (r, x + 5)]
        else:
            slots += [(r, x + 1), (r, x + 3), (r, x + 5)]
    for k, (r, c) in enumerate(slots):
        lit = k < today
        g.put(r, c, "▪" if lit else "·", "yellow" if lit else "muted")
    roof = ROOFS[pid]
    top = base - floors - len(roof) + 1
    for j, line in enumerate(roof):
        for k, ch in enumerate(line):
            if ch != " ":
                g.put(top + j, x + k, ch, "muted")
    g.mark(top, base, x, x + 6)
    return top


def flags(g, x, top, pid, ctx, today):
    """Above the roof: the route stop number, a check-in ?, and Agents' smoke on a busy day."""
    if pid in ctx["stops"]:
        g.put(top - 1, x + 1, str(ctx["stops"][pid]), "accent")
    if pid in ctx["asks"]:
        g.put(top - 1, x + 3, "?", "magenta")
    if pid == "agents" and today > 0:
        g.put(top - 1, x + 5, "░" if ctx["night"] else "∙", "muted")


def you(g, r, x, pid, ctx):
    if pid == ctx["top"]:
        g.put(r, x + 3, "@", "accent")


def cells(g, r, c, n, p):
    """Castra's week: ▪ today, ▫ earlier in the 7 days, · nothing; + when there's more than fits."""
    today, week = p["today"], max(0, p["events_7d"] - p["today"])
    for k in range(n):
        if k == n - 1 and today + week > n:
            g.put(r, c + k, "+", "foreground")
        elif k < today:
            g.put(r, c + k, "▪", "yellow")
        elif k < today + week:
            g.put(r, c + k, "▫", "foreground")
        else:
            g.put(r, c + k, "·", "muted")


def week_block(g, x, y, bw, pid, ctx, numeral="top"):
    """Castra's insula frame, 3 rows: the title on top, the week inside, the 7-day count below.
    The level numeral sits on the top border, or inside before the week where the border is too short."""
    p = ctx["pm"][pid]
    head = f" {TITLE[pid]} "
    lv = roman(p["level"])
    mark = pid in ctx["laurel"]
    if numeral == "top":
        tail = f" {lv} " + ("✦" if mark else "")
        top_line = "┌" + head + "─" * (bw - 2 - len(head) - len(tail)) + tail + "┐"
    else:
        tail = ""
        top_line = "┌" + head + "─" * (bw - 2 - len(head)) + "┐"
    assert len(top_line) == bw, (pid, top_line)
    g.put(y, x, top_line, "muted")
    g.put(y, x + 1, head, "accent" if pid == ctx["top"] else "foreground")
    g.put(y, x + 2, TITLE[pid][0], "accent")
    c0 = x + 2
    if numeral == "top":
        g.put(y, x + bw - 1 - len(tail) + 1, lv, "accent")
        if mark:
            g.put(y, x + bw - 2, "✦", "yellow")
    else:
        g.put(y + 1, c0, lv, "accent")
        if mark:
            g.put(y + 1, c0 + len(lv), "✦", "yellow")
        c0 += len(lv) + 1 + (1 if mark else 0)
    g.put(y + 1, x, "│", "muted")
    g.put(y + 1, x + bw - 1, "│", "muted")
    cells(g, y + 1, c0, x + bw - 4 - c0, p)
    g.put(y + 1, x + bw - 3, "?" if pid in ctx["asks"] else " ", "magenta")
    el = f" {p['events_7d']} in 7d "
    left = (bw - 2 - len(el)) // 2
    bot = "└" + "─" * left + el + "─" * (bw - 2 - left - len(el)) + "┘"
    g.put(y + 2, x, bot, "muted")
    g.mark(y, y + 2, x, x + bw - 1)


def sky(g, rows, ctx, seed, sun=None):
    r = rnd(seed)
    for row in rows:
        for c in range(g.w):
            if not g.blank(row, c):
                continue
            v = next(r)
            if ctx["night"] and v < 0.045:
                g.put(row, c, "*" if next(r) < 0.15 else "·", "foreground" if next(r) < 0.3 else "muted", occ=False)
            elif ctx["dusk"] and v < 0.015:
                g.put(row, c, "·", "muted", occ=False)
    if sun:
        sr, sc = sun
        g.put(sr, sc, "◐" if ctx["night"] else "◑" if ctx["dusk"] else "☼", "yellow")


def rain(g, rows, ctx, seed):
    if ctx["wx"] == "clear":
        return
    dens = 0.035 if ctx["wx"] == "drizzle" else 0.085
    ch = "'" if ctx["wx"] == "drizzle" else "╱"
    r = rnd(seed)
    for row in rows:
        for c in range(g.w):
            v = next(r)
            if g.blank(row, c) and v < dens:
                g.put(row, c, ch, "blue", occ=False)


# ---- OWL nodes for the screen around the map

def Btn(text, color="foreground", tap=None):
    return f'<span class="n k-button c-{color} tap" data-n="Button"{attrs(data_tap=tap)}>{esc(text)}</span>'


def status(s):
    ctx = context(s)
    tod = "night" if ctx["night"] else "dusk" if ctx["dusk"] else "day"
    out = f"{s['clock']} {tod} · {ctx['wx']}"
    if ctx["asks"]:
        out += f" · ? {len(ctx['asks'])}"
    return out


def header(s, eyebrow):
    return Row(T(eyebrow, "label", weight=True),
               T("●", "small", "red", size=9) if s["errors"] else "",
               T(status(s), "label", size=9),
               Btn("✕", "muted", tap="close"), gap=8)


def rail(s, where, max_q=4, spark=True):
    """The tiles, tucked under the map: route, level and streak, check-ins, add."""
    qs = ranked(s)
    open_ = [q for q in qs if not q["done_today"]]
    done = [q for q in qs if q["done_today"]]
    due = sum(1 for q in open_ if q["due"])
    rows = [Row(T("Iter · today's route", "label", weight=True), T(f"{due} due" if open_ else "", "label", size=9))]
    for i, q in enumerate(open_[:max_q], 1):
        rows.append(Col(
            Row(T("@" if i == 1 else str(i), "small", "accent" if i == 1 else "muted", size=10, bold=i == 1),
                T(q["title"], "small", "foreground", size=10, weight=True, max_lines=1),
                T(TITLE[q["pillar"]], "small", "muted", size=9, max_lines=1),
                T(f'{q["overdue_days"]}d', "small", "red", size=9) if q["overdue_days"] else "",
                T("✓", "small", "green", size=10, tap="done"),
                T("⋯", "small", "muted", size=10, tap="menu"), gap=6),
            T("  " + " · ".join(q["reasons"]), "small", "muted", size=9, max_lines=1), gap=0))
    if len(open_) > max_q:
        rows.append(T(f"  +{len(open_) - max_q} more ↓", "small", "muted", size=9, tap="scroll"))
    if not open_:
        rows.append(Col(T("@ nothing due", "small", "foreground", size=10),
                        T("  add one below, or wait for an agent's proposal", "small", "muted", size=9), gap=0))
    for q in done:
        rows.append(Row(T("✓", "small", "green", size=10),
                        T(q["title"], "small", "muted", size=10, weight=True, max_lines=1, cls="strike"),
                        T("↺", "small", "muted", size=10, tap="undo"), gap=6))
    if s["proposed"]:
        n = len(s["proposed"])
        rows.append(Row(T("◇", "small", "cyan", size=10),
                        T(f"{n} proposed, {where}", "small", "foreground", size=9, weight=True, max_lines=1),
                        T("review", "small", "accent", size=9, tap="proposal"), gap=6))
    left = Col(Col(*rows, gap=2), Box(cls="fill"), InputN("add a quest  #Pillar"), gap=4, cls="w1 fill")

    x, st = s["xp"], s["streak"]
    asks = [c for c in s["checkins"] if c.get("open")]
    right = [T("Gradus · level", "label"),
             Row(T(roman(x["level"]), "title", "accent"), T(f'{x["into"]}/{x["need"]} xp', "small", "foreground", size=9),
                 gap=8, align="flex-end"),
             Progress(x["into"], x["need"], "accent", height=4),
             T(f'streak · {st["days"]} days', "small", "foreground", size=9),
             T("today kept", "small", "green", size=9) if st["today"] else T("rest day ›", "small", "accent", size=9, tap="rest")]
    if spark:
        right.append(Sparkline(s["history"], "accent", height=14))
    right.append(T("Rogationes · check-ins", "label", size=9))
    if asks:
        right.append(T(f"? {len(asks)} waiting", "small", "magenta", size=9, tap="checkin"))
        right.append(T(" · ".join(TITLE[c["pillar"]] for c in asks), "small", "foreground", size=9, max_lines=2))
    else:
        right.append(T("none waiting", "small", "muted", size=9))
    return Row(left, Col(*right, gap=3, style="width:196px;flex:0 0 auto"), gap=14, align="stretch", cls="fill rail")


def screen(theme, s, key, concept):
    if hasattr(concept, "canvas"):
        art = concept.canvas(s)
    else:
        art = f'<div class="asc">{ascii_grid(concept.draw(s).c)}</div>'
    inner = Col(header(s, concept.EYEBROW),
                Canvas(art, cls="mapc tap"),
                rail(s, concept.WHERE, concept.MAX_Q, concept.SPARK), gap=8, cls="fill")
    return (f'<div class="board-wrap"><div class="board-cap"><span>{esc(theme)}</span>'
            f'<span class="syn">synthetic data · {esc(key)}</span></div>'
            f'<div class="scroll"><div class="sboard th-{theme}"><div class="screen" data-n="screen">{inner}</div>'
            f'</div></div></div>')


# ---- Popups, shared by all three (they open from the map and the rail)

def popups(theme):
    s = NORMAL

    def box(*nodes):
        return f'<div class="popup st">{Col(*nodes, gap=6)}</div>'
    p = s["proposed"][0]
    return f'<div class="pop-stage th-{theme} pops">' + "".join([
        box(T("? Recovery asks", "small", "magenta", size=10, bold=True),
            T("What's your target sleep window?", "small", "foreground", size=10),
            InputN("e.g. 23:00–07:00"),
            Row(T("[send]", "small", "accent", size=10, tap="send"), T("[later]", "small", "muted", size=10, tap="later"), gap=10)),
        box(T(f"◇ {TITLE[p['pillar']]} · from {p['from']}", "small", "cyan", size=10, bold=True),
            T(p["title"], "small", "foreground", size=10),
            T(f"priority {p['priority']} · {p['every']}", "small", "muted", size=9),
            Row(T("[accept]", "small", "green", size=10, tap="accept"), T("[decline]", "small", "muted", size=10, tap="decline"), gap=10)),
        box(T("15 min typing drill", "small", "foreground", size=10, bold=True),
            Row(T("snooze", "small", "muted", size=9), T("1d", "small", "accent", size=10, tap="s1"),
                T("3d", "small", "accent", size=10, tap="s3"), T("7d", "small", "accent", size=10, tap="s7"), gap=8),
            Row(T("priority", "small", "muted", size=9), T("↑", "small", "accent", size=10, tap="up"),
                T("↓", "small", "accent", size=10, tap="down"), gap=8),
            T("drop", "small", "red", size=10, tap="drop")),
        box(T("Career · II", "small", "accent", size=10, bold=True),
            T("40/150 xp · 3 in 7d · 1 today", "small", "foreground", size=9),
            Progress(40, 150, "accent", height=4),
            T("no open quests", "small", "muted", size=9),
            T("[add a Career quest]", "small", "accent", size=10, tap="add")),
    ]) + "</div>"


def screen_cmd(s):
    n = s
    return dict(
        done=f"acta quest done {qid(n, 'quest:typing-drill')} --via board",
        undo=f"acta quest undo {qid(n, 'quest:shutdown-note')} --via board",
        s1=f"acta quest snooze {qid(n, 'quest:typing-drill')} --until 2026-11-03 --via board",
        s3=f"acta quest snooze {qid(n, 'quest:typing-drill')} --until 2026-11-05 --via board",
        s7=f"acta quest snooze {qid(n, 'quest:typing-drill')} --until 2026-11-09 --via board",
        up=f"acta quest edit {qid(n, 'quest:call-mom')} --priority 1 --via board",
        down=f"acta quest edit {qid(n, 'quest:call-mom')} --priority 3 --via board",
        drop=f"acta quest drop {qid(n, 'quest:typing-drill')} --via board",
        accept=f"acta quest accept {qid(n, 'quest:budget-review')} --via board",
        decline=f"acta quest decline {qid(n, 'quest:budget-review')} --via board",
        add="acta quest add --pillar recovery --title 'Stretch 10 min' --via board",
        addp="acta quest add --pillar career --title '<typed>' --via board",
        answer=f"acta checkin answer {cid(n, 'checkin:sleep-window')} --text '23:00–07:00' --via board",
        rest="acta rest --via board",
    )


CMD = screen_cmd(NORMAL)
