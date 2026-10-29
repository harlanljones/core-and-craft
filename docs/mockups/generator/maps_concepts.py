"""Three full-screen town maps: Forum buildings with Castra's week blocks, clustered three ways."""
from mapkit import (Grid, W, context, building, flags, you, week_block, cells, sky, rain, CMD)
from data import TITLE, PILLARS, GROUPS, roman

IDS = [p[0] for p in PILLARS]


def floors_for(level, cap):
    return max(2, min(cap, level + 1))


# ---------------------------------------------------------------- A · Duo Fora

class DuoFora:
    SLUG, L, NAME, EN = "duo-fora", "A", "Duo Fora", "two districts across a river"
    EYEBROW = "Duo Fora · two districts"
    WHERE = "moored on the river"
    MAX_Q, SPARK = 4, True
    H = 32
    NORTH = dict(x=[1, 20, 39, 58], bw=19, base=10)      # C.O.R.E.
    SOUTH = dict(x=[1, 16, 31, 46, 61], bw=15, base=27)  # C.R.A.F.T.
    RIVER = range(15, 19)
    BRIDGE = 35

    @classmethod
    def draw(cls, s):
        g = Grid(W, cls.H)
        ctx = context(s)
        lots = {}
        for (label, ids), d in zip(GROUPS, (cls.NORTH, cls.SOUTH)):
            for pid, x in zip(ids, d["x"]):
                p = ctx["pm"][pid]
                bx = x + (d["bw"] - 7) // 2
                top = building(g, bx, d["base"], pid, floors_for(p["level"], 6), p["today"], ctx)
                flags(g, bx, top, pid, ctx, p["today"])
                g.put(d["base"] + 1, x, "─" * d["bw"], "muted")
                you(g, d["base"] + 1, bx, pid, ctx)
                week_block(g, x, d["base"] + 2, d["bw"], pid, ctx, numeral="inner")
                lots[pid] = (x, d["bw"])
        g.put(0, 1, "C.O.R.E.", "label")
        g.put(19, 1, "C.R.A.F.T.", "label")
        g.mark(0, 0, 1, 8)
        g.mark(19, 19, 1, 10)
        # the river, with a bridge whose planks are the streak
        for r in cls.RIVER:
            for c in range(W):
                if (c + r * 3) % 4 == 0:
                    g.put(r, c, "≈", "blue", occ=False)
                elif (c + r) % 7 == 0:
                    g.put(r, c, "~", "muted", occ=False)
        st = s["streak"]
        for r in cls.RIVER:
            g.put(r, cls.BRIDGE, "╞═════╡", "yellow" if st["today"] else "muted")
        g.put(16, cls.BRIDGE + 7, f" streak {st['days']} days ", "foreground")
        g.put(17, cls.BRIDGE + 7, " today kept " if st["today"] else " rest day › ", "green" if st["today"] else "accent")
        # proposals: boats moored below their district's lot
        for i, p in enumerate(s["proposed"]):
            x, bw = lots[p["pillar"]]
            c = x + bw // 2 + 3
            if cls.BRIDGE - 2 <= c <= cls.BRIDGE + 18:
                c = cls.BRIDGE - 3
            north = p["pillar"] in GROUPS[0][1]
            r = 15 if north else 17
            g.put(r, c, " ◇ ", "cyan")
            g.put(r + 1, c, "╲▁╱", "cyan")
        sky(g, range(0, 10), ctx, 7, sun=(1, 70))
        sky(g, range(19, 27), ctx, 11)
        rain(g, list(range(0, 10)) + list(range(19, 27)), ctx, 99)
        return g

    PITCH = ("C.O.R.E. stands on the far bank and C.R.A.F.T. on the near one, each as a skyline of its own. "
             "The river between them carries agents' proposals as boats moored under their pillar, and the bridge "
             "is the streak.")
    CLUSTER = "By acronym: C.O.R.E.'s four on the far bank, C.R.A.F.T.'s five on the near one."
    UNIQUE = "A river with ◇ boats for proposals; the bridge lights when today is kept; district names are the acronyms."
    S = "The acronym split is the spec's own grouping, so you learn the map once; two skylines leave room for six floors."
    R = "Two ground lines mean two ways to read height; long titles force wide lots, so the far bank has 19 and the near bank 15."
    C = "Medium: fixed lots, so only floors, windows, flags and boats change. About 260 text runs on one Canvas."
    TAPS = [
        ("A building or its block", "open that pillar's Popup", "Popup (reads the snapshot)"),
        ("A ◇ boat", "open the proposal Popup", "Popup"),
        ("Proposal · accept / decline", "approve or turn down", CMD["accept"] + "  |  " + CMD["decline"]),
        ("A ? over a roof", "open the check-in Popup", "Popup"),
        ("The bridge, when not kept", "rest day", CMD["rest"]),
    ]
    NOTES = """<li>Lots are fixed: four 19-wide on the far bank, five 15-wide on the near bank. A tap's x picks the lot in its bank and its y picks the bank.</li>
<li>Height is level plus one floor, from 2 to 6. Lit windows are <code>pillars[].today</code>; the block below is <code>events_7d</code>.</li>
<li>Boats sit on the bank of their pillar's district, nudged off the bridge. With more than four, they'd need a count.</li>"""


# ---------------------------------------------------------------- B · Clivus

class Clivus:
    SLUG, L, NAME, EN = "clivus", "B", "Clivus", "the town climbs as you level"
    EYEBROW = "Clivus · the hill"
    WHERE = "waiting on the low road"
    MAX_Q, SPARK = 4, False
    H = 32
    X0, PITCH_X = 3, 8
    VALLEY = 24          # ground line of the lowest terrace
    STEP = 4

    @staticmethod
    def band(level):
        return min(3, (level - 1) // 2)

    @classmethod
    def draw(cls, s):
        g = Grid(W, cls.H)
        ctx = context(s)
        # the aqueduct: thirty days of xp, the run of the streak in bright water, gaps where a day earned nothing
        hist = s["history"]
        g.put(0, 1, "xp · 30 days", "label")
        today_s = f"today +{hist[-1]}"
        g.put(0, W - len(today_s) - 1, today_s, "label")
        g.mark(0, 0, 0, W - 1)
        streak = s["streak"]["days"]
        x0 = 6
        for d, v in enumerate(hist):
            c = x0 + d * 2
            in_run = d >= 30 - streak
            if v > 0:
                g.put(1, c, "≈≈", "blue" if in_run else "muted")
                g.put(2, c, "╭╮", "muted")
                g.put(3, c, "│ ", "muted")
            else:
                g.put(1, c, "  ")
                g.put(2, c, " ╴", "muted")
                g.put(3, c, "  ")
        end = x0 + 60
        g.put(1, end, "╗", "muted")
        g.put(2, end, "╠═╗", "muted")
        g.put(3, end, "║" + ("▒" if hist[-1] else " ") + "║", "blue" if hist[-1] else "muted")
        g.mark(1, 3, x0, end + 2)
        # terraces: each pillar keeps its column; its terrace rises with its level
        grounds = {}
        for i, pid in enumerate(IDS):
            p = ctx["pm"][pid]
            gr = cls.VALLEY - cls.STEP * cls.band(p["level"])
            grounds[pid] = gr
            x = cls.X0 + i * cls.PITCH_X
            top = building(g, x, gr - 1, pid, 2, p["today"], ctx)
            flags(g, x, top, pid, ctx, p["today"])
            if pid in ctx["laurel"]:
                g.put(top - 1, x + 5, "✦", "yellow")
            g.put(gr, x, "▔" * 7, "muted")
            you(g, gr, x, pid, ctx)
            # Castra's block, as the terrace's retaining wall
            lv = roman(p["level"])
            g.put(gr + 1, x, "┌" + lv.center(5, "─") + "┐", "muted")
            g.put(gr + 1, x + 1 + lv.center(5, "─").index(lv), lv, "accent")
            g.put(gr + 2, x, "│", "muted")
            cells(g, gr + 2, x + 1, 5, p)
            g.put(gr + 2, x + 6, "│", "muted")
            n = str(p["events_7d"])
            g.put(gr + 3, x, "└" + n.center(5, "─") + "┘", "muted")
            g.mark(gr + 1, gr + 3, x, x + 6)
            # hillside under the wall, down to the valley floor
            for r in range(gr + 4, cls.VALLEY + 4):
                for c in range(x, x + 7):
                    if (r + c) % 3 == 0:
                        g.put(r, c, "·", "muted")
                    g.mark(r, r, c, c)
        # the gaps between columns slope from one terrace to the next
        for i in range(len(IDS) - 1):
            a, b = grounds[IDS[i]], grounds[IDS[i + 1]]
            c = cls.X0 + i * cls.PITCH_X + 7
            r = min(a, b)
            g.put(r, c, "─" if a == b else "╲" if a < b else "╱", "muted")
            for rr in range(r + 1, cls.VALLEY + 4):
                if (rr + c) % 3 == 0:
                    g.put(rr, c, "·", "muted")
                g.mark(rr, rr, c, c)
        g.put(cls.VALLEY, 0, "▔▔", "muted")
        g.put(cls.VALLEY, 74, "▔▔▔", "muted")
        # titles stay at the foot, under their column, staggered so long ones fit
        for i, pid in enumerate(IDS):
            t = TITLE[pid]
            r = 28 + (i % 2)
            x = cls.X0 + i * cls.PITCH_X + 3 - len(t) // 2
            g.put(r, x, t[0], "accent")
            g.put(r, x + 1, t[1:], "accent" if pid == ctx["top"] else "foreground")
        # the low road, where proposals wait under their column
        g.put(31, 0, "┄" * W, "muted")
        for p in s["proposed"]:
            i = IDS.index(p["pillar"])
            g.put(31, cls.X0 + i * cls.PITCH_X + 2, "◇▸", "cyan")
        sky(g, range(4, cls.VALLEY), ctx, 5, sun=(5, 72))
        rain(g, range(4, cls.VALLEY), ctx, 41)
        return g

    PITCH = ("Every pillar keeps its column in spec order, and its terrace climbs the hill as it levels: I–II in the valley, "
             "III–IV on the slope, V–VI up high, VII and on at the top. An aqueduct across the sky carries thirty days of xp.")
    CLUSTER = "By level: the same nine columns, raised onto four terraces. Your strongest pillars stand highest."
    UNIQUE = "The aqueduct: bright water is the streak, a broken arch is a day with no xp; proposals wait on the low road."
    S = "Level reads at a glance as altitude, and each pillar's column never moves, so taps and titles stay put."
    R = "Early on everything sits in the valley and the hill is flat; a pillar moves only on level-up, so it changes slowly."
    C = "Medium-high: per-column terrace math and a 60-character aqueduct. Under 300 text runs on one Canvas."
    TAPS = [
        ("A column (building, wall or title)", "open that pillar's Popup", "Popup (reads the snapshot)"),
        ("The aqueduct", "show xp by day in a Popup", "Popup (reads history)"),
        ("A ◇ on the low road", "open the proposal Popup", "Popup"),
        ("Proposal · accept / decline", "approve or turn down", CMD["accept"] + "  |  " + CMD["decline"]),
        ("A ? over a roof", "open the check-in Popup", "Popup"),
    ]
    NOTES = """<li>Columns sit at x = 3 + 8 × index, so a tap's x alone picks the pillar. Terrace height is <code>min(3, (level − 1) // 2)</code> steps of four rows.</li>
<li>Every building is two floors here, since height is the terrace's job. The wall under it shows the level numeral, the week and <code>events_7d</code>.</li>
<li>The aqueduct is two characters per day of <code>history</code>; the last <code>streak.days</code> days are bright. The level tile drops its sparkline, since the aqueduct is one.</li>"""


# ---------------------------------------------------------------- C · Rota

class Rota:
    SLUG, L, NAME, EN = "rota", "C", "Rota", "a walled wheel with Time at its hub"
    EYEBROW = "Rota · the wheel"
    WHERE = "waiting at the gates"
    MAX_Q, SPARK = 3, True
    H = 36
    BX = [1, 27, 53]
    BY = [1, 13, 25]
    BW, BH = 23, 10
    # clockwise from the top left, spelling C O R E C R A F, with Time at the hub
    RING = {(0, 0): "career", (1, 0): "outlook", (2, 0): "relationships", (2, 1): "economic",
            (2, 2): "competence", (1, 2): "recovery", (0, 2): "agents", (0, 1): "faith", (1, 1): "time"}
    GATES = {"N": [(0, 24), (0, 50)], "S": [(35, 24), (35, 50)], "W": [(11, 0), (23, 0)], "E": [(11, 76), (23, 76)]}

    @classmethod
    def draw(cls, s):
        g = Grid(W, cls.H)
        ctx = context(s)
        lit = ctx["night"] or ctx["dusk"]
        # walls and gates
        for c in range(1, 76):
            g.put(0, c, "═", "muted")
            g.put(35, c, "═", "muted")
        for r in range(1, 35):
            g.put(r, 0, "║", "muted")
            g.put(r, 76, "║", "muted")
        g.put(0, 0, "╔", "muted"); g.put(0, 76, "╗", "muted"); g.put(35, 0, "╚", "muted"); g.put(35, 76, "╝", "muted")
        for c in (24, 25, 26, 50, 51, 52):
            g.put(0, c, " "); g.put(35, c, " ")
        for r in (11, 12, 23, 24):
            g.put(r, 0, " "); g.put(r, 76, " ")
        # roads with dashed centre lines; torches at the four crossings by night
        for r in range(0, 36):
            for c in (25, 51):
                if g.c[r][c][0] == " ":
                    g.put(r, c, "┊", "muted", occ=False)
        for c in range(0, 77):
            for r in (11, 23):
                if g.c[r][c][0] in (" ", "┊"):
                    g.put(r, c, "┄", "muted", occ=False)
        for r in (11, 23):
            for c in (25, 51):
                g.put(r, c, "•" if lit else "┼", "yellow" if lit else "muted", occ=False)
        # insulae: a Forum building on each, Castra's frame and week around it
        for (gx, gy), pid in cls.RING.items():
            x, y = cls.BX[gx], cls.BY[gy]
            p = ctx["pm"][pid]
            week_block(g, x, y, cls.BW, pid, ctx)          # draws the frame's top row and marks it
            # re-draw as a 10-row insula: sides, building, ground, week, count
            top_row = g.c[y][:]
            for r in range(y, y + cls.BH):
                g.put(r, x, " " * cls.BW, None)
            g.c[y] = top_row
            for r in range(y + 1, y + cls.BH - 1):
                g.put(r, x, "│", "muted")
                g.put(r, x + cls.BW - 1, "│", "muted")
            bx = x + 8
            floors = floors_for(p["level"], 4)
            top = building(g, bx, y + 6, pid, floors, p["today"], ctx)
            if pid in ctx["stops"]:
                g.put(top, bx + 8, str(ctx["stops"][pid]), "accent")
            if pid in ctx["asks"]:
                g.put(top, bx + 10, "?", "magenta")
            if pid == "agents" and p["today"] > 0:
                g.put(top, bx - 2, "░" if ctx["night"] else "∙", "muted")
            g.put(y + 7, x + 2, "─" * (cls.BW - 4), "muted")
            you(g, y + 7, bx, pid, ctx)
            cells(g, y + 8, x + 2, cls.BW - 4, p)
            el = f" {p['events_7d']} in 7d "
            left = (cls.BW - 2 - len(el)) // 2
            g.put(y + 9, x, "└" + "─" * left + el + "─" * (cls.BW - 2 - left - len(el)) + "┘", "muted")
            g.mark(y, y + cls.BH - 1, x, x + cls.BW - 1)
        # the reading order, clockwise and into the hub
        for (r, c, ch) in [(5, 25, "→"), (5, 51, "→"), (12, 64, "↓"), (24, 64, "↓"), (29, 51, "←"),
                           (29, 25, "←"), (24, 12, "↑"), (17, 25, "→")]:
            g.put(r, c, ch, "muted")
        # proposals: carts at the gate nearest their insula
        pos = {pid: xy for xy, pid in cls.RING.items()}
        used = set()
        for p in s["proposed"]:
            gx, gy = pos[p["pillar"]]
            side = "N" if gy == 0 else "S" if gy == 2 else "W" if gx == 0 else "E"
            if gx == 1 and gy == 1:
                side = "N"
            for (r, c) in cls.GATES[side]:
                if (r, c) not in used:
                    used.add((r, c))
                    if side in "NS":
                        g.put(r, c, "◇", "cyan")
                    else:
                        g.put(r, c, "◇", "cyan")
                    break
        rain(g, range(1, 35), ctx, 23)
        return g

    PITCH = ("A walled camp whose eight outer blocks run clockwise from the top left, spelling C O R E C R A F, "
             "with Time in the middle as the hub the rest turn on. Each block is a Castra insula with a Forum building inside.")
    CLUSTER = "By reading order: a ring that spells the acronyms clockwise and ends at Time, in the centre."
    UNIQUE = "Walls and four gates where proposals wait as ◇ carts; torches at the crossings by night; arrows that trace the ring."
    S = "The richest Castra: every pillar gets a full 23-wide block with its whole week, and Time's place says what it is."
    R = "Height caps at four floors, so levels past III look alike; the ring is a new idea of mine, not in the spec."
    C = "Medium: a fixed 3×3 plan like the Castra tile, scaled up. About 300 text runs; the rail is one quest shorter."
    TAPS = [
        ("A block", "open that pillar's Popup", "Popup (reads the snapshot)"),
        ("A ◇ cart at a gate", "open the proposal Popup", "Popup"),
        ("Proposal · accept / decline", "approve or turn down", CMD["accept"] + "  |  " + CMD["decline"]),
        ("A ? in a block", "open the check-in Popup", "Popup"),
    ]
    NOTES = """<li>The plan is fixed: blocks at x = 1, 27, 53 and y = 1, 13, 25, each 23 by 10. A tap's x and y pick the block.</li>
<li>The ring's order is a design choice, not spec: it keeps the acronyms in reading order and puts Time at the hub. If it reads as too clever, the Castra tile's row order drops in with no other change.</li>
<li>Gates take the cart for the side their pillar faces (Time's go north). Two per side; a third waits off the map in the rail's count.</li>"""


CONCEPTS = [DuoFora, Clivus, Rota]
