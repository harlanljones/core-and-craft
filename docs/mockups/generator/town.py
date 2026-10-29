"""ASCII town: 77 x 18 characters at 9 dp, one building per pillar (Forum and Tabularium)."""
from data import PILLARS, LATIN, ENG, TITLE, top_pillar, is_night, is_dusk, weather, roman

COLS, ROWS = 77, 18
GROUND = 15          # ground line row
BASE = 14            # ground floor row
X0, PITCH, BW = 3, 8, 7

ROOFS = {
    "career": ["   ╻   ", "┌──┴──┐"],
    "outlook": ["  ╭─╮  ", "┌─┴─┴─┐"],
    "relationships": ["  ╱▔╲  ", "╱▔▔▔▔▔╲"],
    "economic": ["  ▁▁▁  ", "╒═════╕"],
    "competence": ["┌─────┐", "├─────┤"],
    "recovery": [" ╭───╮ ", "╭┴───┴╮"],
    "agents": ["    ║  ", "┌───╨─┐"],
    "faith": ["  ╱─╲  ", "╱═════╲"],
    "time": [" ┌───┐ ", "┌┤ ○ ├┐"],
}
WALL = {"economic": "║", "faith": "║"}


def rnd(seed):
    x = seed & 0xFFFFFFFF
    while True:
        x = (1103515245 * x + 12345) & 0x7FFFFFFF
        yield x / 0x7FFFFFFF


def put(grid, r, c, s, color=None):
    for i, ch in enumerate(s):
        if 0 <= c + i < COLS and 0 <= r < ROWS:
            grid[r][c + i] = (ch, color)


def s_clock(state):
    return state["clock"]


def title_labels(g, i, pid, top, row=16, x0=X0, pitch=PITCH, bw=BW):
    """Spec titles under the buildings, staggered over two rows so long ones fit."""
    t = TITLE[pid]
    r = row + (i % 2)
    x = x0 + i * pitch + bw // 2 - len(t) // 2
    put(g, r, x, t[0], "accent")
    put(g, r, x + 1, t[1:], "accent" if pid == top else "foreground" if i % 2 == 0 else "label")


def town(state, mode="forum"):
    g = [[(" ", None) for _ in range(COLS)] for _ in range(ROWS)]
    occupied = [[False] * COLS for _ in range(ROWS)]
    night, dusk = is_night(state), is_dusk(state)
    wx = weather(state)
    top = top_pillar(state)
    asks = {c["pillar"] for c in state["checkins"] if c.get("open")}
    pmap = {p["pillar"]: p for p in state["pillars"]}

    # header row
    if mode == "forum":
        left = "Forum · the town"
        tod = "night" if night else "dusk" if dusk else "day"
        sky = {"clear": "clear", "drizzle": "drizzle", "storm": "storm"}[wx]
        right = f"{s_clock(state)} {tod} · {sky}"
        if asks:
            right += f"   ? {len(asks)}"
        put(g, 0, 0, left, "label")
        put(g, 0, COLS - len(right), right, "label")
        if state["errors"]:
            put(g, 0, len(left) + 2, "●", "red")
    else:
        t = state["today"]
        d7 = sum(p["events_7d"] for p in state["pillars"])
        left = f"Census · the town ledger   today {t['events']} · 7d {d7} · xp +{state['history'][-1]}"
        right = f"{state['clock']} {'night' if night else 'day'} · {sum(1 for q in state['quests'] if q['overdue_days'] > 0 and not q['done_today'])} late"
        put(g, 0, 0, left, "label")
        put(g, 0, COLS - len(right), right, "label")
        if state["errors"]:
            put(g, 0, len(left) + 1, "●", "red")

    tops = {}
    for i, (pid, *_rest) in enumerate(PILLARS):
        p = pmap[pid]
        x = X0 + i * PITCH
        floors = max(2, min(6, p["level"] + 1)) if mode == "forum" else max(1, min(6, p["level"]))
        roof = ROOFS[pid] if mode == "forum" else ROOFS[pid][1:]
        wall = WALL.get(pid, "│")
        rcol = "muted"
        # floors, ground first
        slots = []
        for f in range(floors):
            r = BASE - f
            if f == 0:
                put(g, r, x, wall + "     " + wall, rcol)
                put(g, r, x + 3, "∩", rcol)
                slots += [(r, x + 1), (r, x + 5)]
            else:
                put(g, r, x, wall + "     " + wall, rcol)
                slots += [(r, x + 1), (r, x + 3), (r, x + 5)]
        today = p.get("today", 0)
        week = max(0, p["events_7d"] - today)
        for k, (r, c) in enumerate(slots):
            if k < today:
                put(g, r, c, "▪", "yellow")
            elif mode == "tabularium" and k < today + week:
                put(g, r, c, "▫", "foreground")
            else:
                put(g, r, c, "·", "muted")
        rtop = BASE - floors - len(roof) + 1
        for j, line in enumerate(roof):
            for k, ch in enumerate(line):
                if ch != " ":
                    put(g, rtop + j, x + k, ch, rcol)
        for r in range(rtop, BASE + 1):
            for c in range(x, x + BW):
                occupied[r][c] = True
        if mode == "forum" and pid == "agents" and today > 0:
            put(g, rtop - 1, x + 4, "░" if night else "∙", "muted")
            occupied[rtop - 1][x + 4] = True
        tops[pid] = rtop
        if pid in asks:
            put(g, rtop - 1, x + 3, "?", "magenta")
            occupied[rtop - 1][x + 3] = True

    # ground, @, labels
    put(g, GROUND, 1, "─" * (COLS - 2), "muted")
    for i, (pid, latin, eng, _full) in enumerate(PILLARS):
        x = X0 + i * PITCH
        if pid == top:
            put(g, GROUND, x + 3, "@", "accent")
        title_labels(g, i, pid, top)
        if mode != "forum":
            p = pmap[pid]
            s = f"{roman(p['level'])}·{p['events_7d']}"
            put(g, GROUND, x + (BW - len(s)) // 2, s, "label")
            if pid == top:
                put(g, BASE, x + 3, "@", "accent")

    # sky
    rr = rnd(7 + len(state["quests"]) * 13)
    for r in range(1, BASE):
        for c in range(COLS):
            if occupied[r][c] or g[r][c][0] != " ":
                continue
            v = next(rr)
            if night and r < 9 and v < (0.05 if mode == "forum" else 0.025):
                put(g, r, c, "*" if next(rr) < 0.15 else "·", "foreground" if next(rr) < 0.3 else "muted")
            elif dusk and r < 6 and v < 0.018:
                put(g, r, c, "·", "muted")
    if mode == "forum":
        if night:
            put(g, 2, 69, "◐", "yellow")
        elif dusk:
            put(g, 6, 70, "◑", "yellow")
        else:
            put(g, 2, 69, "☼", "yellow")
            put(g, 3, 8, "~~ ~", "muted")
            put(g, 2, 40, "~~~", "muted")
    # rain over open sky and gaps
    if wx != "clear":
        dens = 0.035 if wx == "drizzle" else 0.085
        ch = "'" if wx == "drizzle" else "╱"
        rr = rnd(99)
        for r in range(1, GROUND):
            for c in range(COLS):
                if occupied[r][c] or g[r][c][0] != " ":
                    next(rr)
                    continue
                if next(rr) < dens:
                    put(g, r, c, ch, "blue")
    return g
