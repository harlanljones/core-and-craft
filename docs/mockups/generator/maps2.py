"""Round 5: the town seen from above (Stardew-style) and in isometric, three ways."""
from mapkit import Grid, W, context, cells, week_block, rnd, CMD
from data import TITLE, PILLARS, GROUPS, roman

IDS = [p[0] for p in PILLARS]
CORE = GROUPS[0][1]


def done_today(s):
    return sum(1 for q in s["quests"] if q["done_today"])


# =============================================================== A · Vicus (mono glyphs)

ROOFP = {"career": "═", "outlook": "░", "relationships": "╳", "economic": "▓", "competence": "┼",
         "recovery": "~", "agents": "╬", "faith": "^", "time": "·"}


class Vicus:
    SLUG, L, NAME, EN = "vicus", "A", "Vicus", "a village from above, in text"
    EYEBROW = "Vicus · the village"
    WHERE = "pinned to the notice board"
    MAX_Q, SPARK = 4, True
    H = 32
    TECH = "Mono glyphs, top-down three-quarter view"
    # (lot x, lot y, lot width)
    LOTS = {"career": (1, 0, 18), "outlook": (19, 0, 18), "relationships": (1, 10, 18), "economic": (19, 10, 18),
            "competence": (41, 0, 17), "recovery": (59, 0, 17), "agents": (41, 10, 17), "faith": (59, 10, 17),
            "time": (50, 20, 17)}
    TREES = [(21, 40), (24, 44), (27, 41), (22, 70), (26, 73), (29, 69), (30, 46), (28, 37), (2, 38), (14, 38)]

    @classmethod
    def house(cls, g, x, base, pid, floors, today, ctx):
        """9 wide: a two-row roof seen from above, then one front-wall row per floor."""
        top = base - floors - 1
        pat = ROOFP[pid]
        g.put(top, x, "╭" + pat * 7 + "╮", "muted")
        g.put(top + 1, x, "╰" + pat * 7 + "╯", "muted")
        if pid == "time":
            g.put(top, x + 4, "○", "muted")
        slots = []
        for f in range(floors):
            r = base - f
            g.put(r, x, "│       │", "muted")
            if f == 0:
                g.put(r, x + 4, "∩", "muted")
                slots += [(r, x + 2), (r, x + 6)]
            else:
                slots += [(r, x + 2), (r, x + 4), (r, x + 6)]
        for k, (r, c) in enumerate(slots):
            lit = k < today
            g.put(r, c, "▪" if lit else "·", "yellow" if lit else "muted")
        g.mark(top, base, x, x + 8)
        return top

    @classmethod
    def draw(cls, s):
        g = Grid(W, cls.H)
        ctx = context(s)
        lamp = ctx["night"] or ctx["dusk"]
        for pid, (x, y, w) in cls.LOTS.items():
            p = ctx["pm"][pid]
            floors = max(1, min(4, p["level"]))
            bx = x + (w - 9) // 2
            base = y + 5
            top = cls.house(g, bx, base, pid, floors, p["today"], ctx)
            if pid in ctx["asks"]:
                g.put(top, bx + 10, "?", "magenta")
            if pid in ctx["stops"]:
                g.put(top + 1, bx + 10, str(ctx["stops"][pid]), "accent")
            if pid == ctx["top"]:
                g.put(base, bx + 10, "@", "accent")
            if pid == "agents" and p["today"]:
                g.put(top - 1, bx + 7, "░" if ctx["night"] else "∙", "muted")
            week_block(g, x, y + 6, w, pid, ctx, numeral="inner")
            for c in range(x + 1, x + w - 1):          # the week as a crop field: ▫ growing, ▪ ripe today
                ch, col = g.c[y + 7][c]
                if ch == "▫":
                    g.c[y + 7][c] = (ch, "green")
        # paths between the lots, and the lane down the middle
        for r in (9, 19):
            for c in range(0, W):
                if g.blank(r, c):
                    g.put(r, c, "░", "muted", occ=False)
        for r in range(0, 32):
            for c in (38, 39):
                if g.blank(r, c):
                    g.put(r, c, "░", "muted", occ=False)
        for c in range(40, 77):
            if g.blank(29, c):
                g.put(29, c, "░", "muted", occ=False)
        # the plaza: a notice board for proposals, and a pond
        g.put(21, 2, "┌ notice board ──────┐", "muted")
        g.put(22, 2, "│" + " " * 20 + "│", "muted")
        g.put(23, 2, "│" + " " * 20 + "│", "muted")
        g.put(24, 2, "└┬" + "─" * 18 + "┬┘", "muted")
        g.put(25, 2, " ╵" + " " * 18 + "╵ ", "muted")
        g.mark(21, 25, 2, 23)
        if s["proposed"]:
            for i, p in enumerate(s["proposed"][:3]):
                g.put(22 + i // 2 * 1, 4 + (i % 2) * 10, "◇ " + TITLE[p["pillar"]][:7], "cyan")
        else:
            g.put(22, 4, "nothing posted", "muted")
        g.put(23, 4 if not s["proposed"] else 4, "" if s["proposed"] else "", None)
        dn = done_today(s)
        g.put(23, 4, f"✓ {dn} done today", "green" if dn else "muted")
        for r in range(26, 31):
            for c in range(5, 20):
                edge = r in (26, 30) or c in (5, 19)
                if edge and (r, c) in ((26, 5), (26, 19), (30, 5), (30, 19)):
                    continue
                g.put(r, c, "▁" if r == 26 and not c in (5, 19) else "▔" if r == 30 and c not in (5, 19)
                      else "▏" if c == 5 else "▕" if c == 19 else ("≈" if (r + c) % 3 == 0 else " "),
                      "muted" if edge else "blue", occ=True)
        st = s["streak"]
        g.put(28, 22, f"streak {st['days']}", "foreground")
        g.put(29, 22, "today kept" if st["today"] else "rest day ›", "green" if st["today"] else "accent")
        g.mark(28, 29, 22, 33)
        # trees, grass, lamps
        for (r, c) in cls.TREES:
            if g.blank(r, c):
                g.put(r, c, "♣", "green")
        if lamp:
            for (r, c) in [(9, 18), (9, 58), (19, 18), (19, 58), (5, 39), (24, 39), (29, 49)]:
                g.put(r, c, "¡", "yellow")
        rr = rnd(13)
        for r in range(g.h):
            for c in range(g.w):
                v = next(rr)
                if g.blank(r, c) and v < 0.045:
                    g.put(r, c, "," if next(rr) < 0.6 else "'", "green" if not ctx["night"] else "muted", occ=False)
        if ctx["wx"] != "clear":
            dens = 0.03 if ctx["wx"] == "drizzle" else 0.075
            rr = rnd(77)
            for r in range(g.h):
                for c in range(g.w):
                    if g.blank(r, c) and next(rr) < dens:
                        g.put(r, c, "'" if ctx["wx"] == "drizzle" else "╱", "blue", occ=False)
        return g

    PITCH = ("A village seen from above, Stardew-style, drawn in the same mono glyphs as the board. Each house shows its roof and "
             "front wall, grows a floor per level, and has its Castra week as a fenced crop field in front: ▪ ripe today, ▫ growing.")
    CLUSTER = "Two neighbourhoods on either side of a lane: C.O.R.E. to the west, C.R.A.F.T. to the east, Time at the end of the lane."
    UNIQUE = "Crop fields for the week, a notice board where proposals are pinned, a pond, trees, and lamps along the paths by night."
    S = "Keeps the ASCII house style exactly, so it sits with the typing tile; cheapest of the three and the fonts already work."
    R = "Glyphs can only hint at Stardew: no real perspective, and roofs tell pillars apart by texture, which is subtle."
    C = "Low-medium: the same text-run Canvas as Forum, about 280 runs. No new drawing code."
    TAPS = [
        ("A house or its field", "open that pillar's Popup", "Popup (reads the snapshot)"),
        ("The notice board", "open the proposal Popup", "Popup"),
        ("Proposal · accept / decline", "approve or turn down", CMD["accept"] + "  |  " + CMD["decline"]),
        ("A ? beside a roof", "open the check-in Popup", "Popup"),
        ("rest day › by the pond", "rest day", CMD["rest"]),
    ]
    NOTES = """<li><b>Drawn with</b> one <code>Canvas</code> of <code>text()</code> runs, 77 characters by 32 rows at 9 dp: the house style, unchanged.</li>
<li>Lots are fixed (C.O.R.E. 18 wide at x 1 and 19; C.R.A.F.T. 17 wide at x 41 and 59; Time at x 50, y 20), so a tap's x and y pick the lot.</li>
<li>A house has one front-wall row per level, up to four, like Stardew's house upgrades. The fence is Castra's block: level numeral, the week as crops, and <code>events_7d</code>.</li>"""


# =============================================================== B · Duae Insulae (isometric shapes)

def svg(w, h, body):
    return (f'<svg class="mapsvg" viewBox="0 0 {w} {h}" width="{w}" height="{h}" font-family="var(--mono)" '
            f'aria-hidden="true">{body}</svg>')


def poly(pts, cls, extra=""):
    return f'<polygon points="{" ".join(f"{x:.1f},{y:.1f}" for x, y in pts)}" class="{cls}" {extra}/>'


def text(x, y, s, cls="fl-foreground", size=12.5, anchor="middle", bold=False, extra=""):
    b = ' font-weight="700"' if bold else ""
    return (f'<text x="{x:.1f}" y="{y:.1f}" font-size="{size}" text-anchor="{anchor}" dominant-baseline="middle" '
            f'class="{cls}"{b} {extra}>{s}</text>')


def title_svg(x, y, pid, ctx, size=12.5):
    t = TITLE[pid]
    rest = "fl-accent" if pid == ctx["top"] else "fl-foreground"
    return (f'<text x="{x:.1f}" y="{y:.1f}" font-size="{size}" text-anchor="middle" dominant-baseline="middle">'
            f'<tspan class="fl-accent" font-weight="700">{t[0]}</tspan><tspan class="{rest}">{t[1:]}</tspan></text>')


def week_svg(x, y, p, cell=7, gap=2, n=7, iso=None):
    """Castra's week as n small squares: yellow today, green earlier in 7 days, muted nothing."""
    out = []
    today, week = p["today"], max(0, p["events_7d"] - p["today"])
    for k in range(n):
        if k < today:
            c, o = "fl-yellow", 1
        elif k < today + week:
            c, o = "fl-green", 0.9
        else:
            c, o = "fl-muted", 0.7
        if iso:
            ox, oy = iso(k)
            out.append(poly([(x + ox, y + oy), (x + ox + cell, y + oy - cell / 2), (x + ox + cell * 2, y + oy),
                             (x + ox + cell, y + oy + cell / 2)], c, f'opacity="{o}"'))
        else:
            out.append(f'<rect x="{x + k * (cell + gap):.1f}" y="{y:.1f}" width="{cell}" height="{cell}" class="{c}" opacity="{o}"/>')
    if today + week > n:
        out.append(text(x + n * (cell + gap) + 4, y + cell / 2, "+", "fl-foreground", 11, "start"))
    return "".join(out)


def stars(w, h, ctx, seed, n_night=46):
    r = rnd(seed)
    out = []
    if ctx["night"] or ctx["dusk"]:
        for _ in range(n_night if ctx["night"] else 12):
            x, y = next(r) * w, next(r) * h
            out.append(f'<circle cx="{x:.0f}" cy="{y:.0f}" r="{0.6 + next(r) * 0.8:.1f}" class="fl-foreground" opacity="{0.2 + next(r) * 0.35:.2f}"/>')
    return "".join(out)


def rain_svg(w, h, ctx, seed):
    if ctx["wx"] == "clear":
        return ""
    r = rnd(seed)
    n = 40 if ctx["wx"] == "drizzle" else 110
    out = []
    for _ in range(n):
        x, y = next(r) * w, next(r) * h
        out.append(f'<line x1="{x:.0f}" y1="{y:.0f}" x2="{x - 4:.0f}" y2="{y + 9:.0f}" class="st-blue" stroke-width="1.2" opacity="0.6"/>')
    return "".join(out)


class DuaeInsulae:
    SLUG, L, NAME, EN = "duae-insulae", "B", "Duae Insulae", "two islands in isometric"
    EYEBROW = "Duae Insulae · two islands"
    WHERE = "moored off the islands"
    MAX_Q, SPARK = 4, True
    TECH = "Canvas shapes, isometric"
    WPX, HPX = 582, 470
    TW, TH = 116, 58            # one lot's diamond
    OX, OY = 166, 214
    G = 1.32                    # lot pitch, in lots
    LOTS = {"career": (0, 0), "outlook": (1, 0), "relationships": (0, 1), "economic": (1, 1),
            "competence": (2.6, -1.0), "recovery": (3.6, -1.0), "agents": (2.6, 0), "faith": (3.6, 0), "time": (3.1, 1)}

    @classmethod
    def ij(cls, pid):
        i, j = cls.LOTS[pid]
        return i * cls.G, j * cls.G

    @classmethod
    def at(cls, i, j):
        return cls.OX + (i - j) * cls.TW / 2, cls.OY + (i + j) * cls.TH / 2

    @classmethod
    def diamond(cls, i, j, grow=0.0, size=1.0):
        return [cls.at(i - grow, j - grow), cls.at(i + size + grow, j - grow),
                cls.at(i + size + grow, j + size + grow), cls.at(i - grow, j + size + grow)]

    @classmethod
    def canvas(cls, s):
        ctx = context(s)
        out = []
        if ctx["night"]:
            out.append('<circle cx="545" cy="30" r="10" class="fl-yellow" opacity="0.85"/><circle cx="550" cy="27" r="9" class="fl-background"/>')
        elif ctx["dusk"]:
            out.append('<circle cx="545" cy="54" r="11" class="fl-yellow" opacity="0.55"/>')
        else:
            out.append('<circle cx="545" cy="30" r="10" class="fl-yellow" opacity="0.7"/>')
        out.append(stars(cls.WPX, 110, ctx, 3))
        r = rnd(9)
        for _ in range(60):
            x, y = next(r) * cls.WPX, 40 + next(r) * (cls.HPX - 50)
            out.append(f'<line x1="{x:.0f}" y1="{y:.0f}" x2="{x + 10:.0f}" y2="{y:.0f}" class="st-blue" stroke-width="1" opacity="0.3"/>')
        # islands: one grown diamond per lot with a cliff under its front edges, filled per island
        for group in (CORE, GROUPS[1][1]):
            for pid in group:
                i, j = cls.ij(pid)
                a, b, c, d = cls.diamond(i, j, 0.16)
                out.append(poly([d, c, (c[0], c[1] + 14), (d[0], d[1] + 14)], "fl-muted", 'opacity="0.9"'))
                out.append(poly([c, b, (b[0], b[1] + 14), (c[0], c[1] + 14)], "fl-muted", 'opacity="0.55"'))
            for pid in group:
                i, j = cls.ij(pid)
                out.append(poly(cls.diamond(i, j, 0.16), "fl-surface"))
            # fill the gaps between the lots of one island
            ii = [cls.ij(p)[0] for p in group]; jj = [cls.ij(p)[1] for p in group]
        # the bridge from Outlook's island to Agents': lit when today is kept
        st = s["streak"]
        oi, oj = cls.ij("outlook"); ai, aj = cls.ij("agents")
        bc = "fl-yellow" if st["today"] else "fl-muted"
        out.append(poly([cls.at(oi + 1.1, oj + 0.38), cls.at(ai - 0.12, aj + 0.38), cls.at(ai - 0.12, aj + 0.62), cls.at(oi + 1.1, oj + 0.62)], bc, 'opacity="0.85"'))
        for k in range(1, 6):
            t = oi + 1.1 + (ai - 0.12 - oi - 1.1) * k / 6
            out.append(f'<line x1="{cls.at(t, oj + 0.38)[0]:.1f}" y1="{cls.at(t, oj + 0.38)[1]:.1f}" x2="{cls.at(t, oj + 0.62)[0]:.1f}" y2="{cls.at(t, oj + 0.62)[1]:.1f}" class="st-background" stroke-width="1.2" opacity="0.6"/>')
        # boats: proposals, moored off the front of their island
        for k, p in enumerate(s["proposed"]):
            i, j = cls.ij(p["pillar"])
            x, y = cls.at(i + 1.25, j + 1.0)
            x += 18 + k * 30
            y += 26
            out.append(poly([(x - 11, y), (x + 11, y), (x + 7, y + 6), (x - 7, y + 6)], "fl-cyan"))
            out.append(poly([(x, y - 15), (x + 9, y - 2), (x, y - 2)], "fl-cyan", 'opacity="0.8"'))
        # lots, back to front, then every label on top so nothing hides it
        order = sorted(IDS, key=lambda pid: sum(cls.ij(pid)))
        for pid in order:
            out.append(cls.tower(pid, ctx))
        for pid in order:
            out.append(cls.labels(pid, ctx))
        mx, my = cls.at((oi + 1.1 + ai) / 2, oj + 0.5)
        out.append(text(mx + 4, my + 26, f"streak {st['days']}", "fl-foreground", 11))
        out.append(text(mx + 4, my + 39, "today kept" if st["today"] else "rest day ›", "fl-green" if st["today"] else "fl-accent", 11))
        out.append(text(14, 20, "C.O.R.E.", "fl-muted", 11, "start"))
        out.append(text(330, 20, "C.R.A.F.T.", "fl-muted", 11, "start"))
        out.append(rain_svg(cls.WPX, cls.HPX, ctx, 31))
        return svg(cls.WPX, cls.HPX, "".join(out))

    @classmethod
    def geom(cls, pid, p):
        i, j = cls.ij(pid)
        fi, fj, fw = i + 0.14, j + 0.1, 0.46
        base = [cls.at(fi, fj + fw), cls.at(fi + fw, fj + fw), cls.at(fi + fw, fj), cls.at(fi, fj)]
        hgt = 16 + min(7, p["level"]) * 14
        return i, j, base, hgt

    @classmethod
    def tower(cls, pid, ctx):
        p = ctx["pm"][pid]
        i, j, (L, F, R, B), hgt = cls.geom(pid, p)
        up = lambda pt: (pt[0], pt[1] - hgt)
        out = [poly(cls.diamond(i, j, -0.05), "st-muted", 'fill="none" stroke-width="1.2" stroke-dasharray="3 3"')]
        # Castra's week, on the lot's front right: seven small floor tiles
        today, week = p["today"], max(0, p["events_7d"] - p["today"])
        for k in range(7):
            c, o = ("fl-yellow", 1) if k < today else ("fl-green", 0.9) if k < today + week else ("fl-muted", 0.8)
            a = 0.68 + (k % 4) * 0.08
            b = 0.1 + (k // 4) * 0.1 + (k % 4) * 0.0
            bi, bj = i + 0.66, j + 0.12 + k * 0.11
            out.append(poly(cls.diamond(bi, bj, 0, 0.09), c, f'opacity="{o}"'))
        out.append(poly([L, F, up(F), up(L)], "fl-muted"))
        out.append(poly([F, R, up(R), up(F)], "fl-muted", 'opacity="0.55"'))
        out.append(poly([up(L), up(F), up(R), up(B)], "fl-surface"))
        out.append(poly([up(L), up(F), up(R), up(B)], "st-muted", 'fill="none" stroke-width="1"'))
        storeys = max(1, min(7, p["level"]))
        k = 0
        for sidx in range(storeys):
            for t in (0.3, 0.7):
                x0 = L[0] + (F[0] - L[0]) * t
                y0 = L[1] + (F[1] - L[1]) * t - 9 - sidx * 14
                on = k < p["today"]
                out.append(poly([(x0 - 3, y0 - 1.5), (x0 + 3, y0 + 1.5), (x0 + 3, y0 - 5.5), (x0 - 3, y0 - 8.5)],
                                "fl-yellow" if on else "fl-background", f'opacity="{1 if on else 0.7}"'))
                k += 1
        Lt, Ft, Rt, Bt = up(L), up(F), up(R), up(B)
        cx, cy = (Lt[0] + Rt[0]) / 2, (Ft[1] + Bt[1]) / 2
        if pid in ("relationships", "faith"):
            out.append(poly([Lt, Ft, (cx, cy - 18)], "fl-muted"))
            out.append(poly([Ft, Rt, (cx, cy - 18)], "fl-muted", 'opacity="0.55"'))
        elif pid in ("outlook", "recovery"):
            out.append(f'<ellipse cx="{cx:.1f}" cy="{cy - 2:.1f}" rx="13" ry="8" class="fl-muted"/>')
        elif pid == "career":
            out.append(f'<line x1="{cx:.1f}" y1="{cy:.1f}" x2="{cx:.1f}" y2="{cy - 18:.1f}" class="st-muted" stroke-width="2"/>')
        elif pid == "agents":
            out.append(f'<rect x="{cx + 4:.1f}" y="{cy - 14:.1f}" width="5" height="12" class="fl-muted"/>')
            if p["today"]:
                out.append(f'<circle cx="{cx + 7:.1f}" cy="{cy - 20:.1f}" r="3" class="fl-muted" opacity="0.5"/>')
        elif pid == "time":
            out.append(f'<circle cx="{cx:.1f}" cy="{cy - 10:.1f}" r="8" class="fl-surface"/><circle cx="{cx:.1f}" cy="{cy - 10:.1f}" r="8" class="st-muted" fill="none" stroke-width="1.4"/>'
                       f'<line x1="{cx:.1f}" y1="{cy - 10:.1f}" x2="{cx:.1f}" y2="{cy - 15:.1f}" class="st-muted" stroke-width="1.4"/>')
        elif pid == "economic":
            for d in (-8, 0, 8):
                out.append(f'<line x1="{cx + d:.1f}" y1="{cy + 2:.1f}" x2="{cx + d:.1f}" y2="{cy - 8:.1f}" class="st-muted" stroke-width="2"/>')
        elif pid == "competence":
            out.append(poly([(cx - 8, cy - 4), (cx + 8, cy - 4), (cx + 8, cy - 12), (cx - 8, cy - 12)], "fl-muted"))
        if pid == ctx["top"]:
            ax, ay = cls.at(i + 0.45, j + 0.8)
            out.append(f'<circle cx="{ax:.1f}" cy="{ay:.1f}" r="5" class="fl-accent"/>')
        return "".join(out)

    @classmethod
    def labels(cls, pid, ctx):
        """A small sign over each tower: title, then numeral and 7-day count; flags beside it."""
        p = ctx["pm"][pid]
        i, j, (L, F, R, B), hgt = cls.geom(pid, p)
        out = []
        cx = (L[0] + R[0]) / 2
        roof = B[1] - hgt - (18 if pid in ("relationships", "faith", "career") else 12)
        ty = roof - 14
        mark = pid in ctx["laurel"]
        lv = roman(p["level"])
        wch = len(TITLE[pid]) + 1 + len(lv) + (1 if mark else 0)
        pw = wch * 6.9 + 12
        x0 = min(max(cx - pw / 2, 4), cls.WPX - 26 - pw)
        out.append(f'<rect x="{x0:.1f}" y="{ty - 9:.1f}" width="{pw:.1f}" height="18" class="fl-background" opacity="0.8"/>')
        out.append(f'<rect x="{x0:.1f}" y="{ty - 9:.1f}" width="{pw:.1f}" height="18" fill="none" class="st-muted" stroke-width="1"/>')
        t = TITLE[pid]
        rest = "fl-accent" if pid == ctx["top"] else "fl-foreground"
        out.append(f'<text x="{x0 + 6:.1f}" y="{ty:.1f}" font-size="11.5" text-anchor="start" dominant-baseline="middle">'
                   f'<tspan class="fl-accent" font-weight="700">{t[0]}</tspan><tspan class="{rest}">{t[1:]} </tspan>'
                   f'<tspan class="fl-accent">{lv}</tspan>' + ('<tspan class="fl-yellow">✦</tspan>' if mark else "") + '</text>')
        if pid in ctx["stops"]:
            out.append(text(x0 - 8, ty, str(ctx["stops"][pid]), "fl-accent", 13, bold=True))
        if pid in ctx["asks"]:
            out.append(f'<circle cx="{x0 + pw + 10:.1f}" cy="{ty:.1f}" r="7.5" class="fl-magenta"/>' + text(x0 + pw + 10, ty + 0.5, "?", "fl-background", 11, bold=True))
        if pid == ctx["top"]:
            ax, ay = cls.at(i + 0.45, j + 0.8)
            out.append(text(ax - 11, ay - 2, "@", "fl-accent", 12, bold=True))
        return "".join(out)

    PITCH = ("The town in true isometric, drawn with Canvas shapes: each pillar is a tower on its own lot that gains a storey per level, "
             "windows lit for today, and its Castra week laid in front as seven small floor tiles.")
    CLUSTER = "Two islands: C.O.R.E. as a 2×2 block on the left, C.R.A.F.T. as four plus Time on the right, joined by a bridge."
    UNIQUE = "Real height for level, a bridge that lights when the streak holds, and proposals as sailboats moored off their island."
    S = "The most like a game: depth, height and overlap read instantly, and it scales up cleanly to a whole monitor later."
    R = "Leaves the ASCII house style for shapes; overlap means drawing back to front, and small text in iso needs care."
    C = "High: about 350 path, rect and text calls with iso math per lot, plus hit testing by diamond instead of by grid cell."
    TAPS = [
        ("A lot (tower or tiles)", "open that pillar's Popup", "Popup (reads the snapshot)"),
        ("A sailboat", "open the proposal Popup", "Popup"),
        ("Proposal · accept / decline", "approve or turn down", CMD["accept"] + "  |  " + CMD["decline"]),
        ("A ? over a tower", "open the check-in Popup", "Popup"),
        ("rest day › by the bridge", "rest day", CMD["rest"]),
    ]
    NOTES = """<li><b>Drawn with</b> Canvas <code>path(points, closed: true)</code> for every face and tile, <code>circle</code> and <code>line</code> for roofs and rain, and <code>text()</code> for labels. No glyph art at all: this one leaves the ASCII house style.</li>
<li>Lots sit on an iso grid: x = ox + (i − j) × 56, y = oy + (i + j) × 28 (in px; ÷ 1.39 for dp). Draw lots in order of i + j so nearer towers cover farther ones. A tap maps back with the inverse and picks the lot whose diamond holds it.</li>
<li>Faces use named colors only: left <code>muted</code>, right <code>muted</code> in a <code>layer(0.55)</code>, top <code>surface</code>. Night is the same drawing with stars and the moon; only windows and the @ glow.</li>"""


# =============================================================== C · Tesserae (pixel tiles)

class Tesserae:
    SLUG, L, NAME, EN = "tesserae", "C", "Tesserae", "a pixel-tile town around Time"
    EYEBROW = "Tesserae · the tiled town"
    WHERE = "waiting at the gates"
    MAX_Q, SPARK = 4, True
    TECH = "Canvas rects on a 16 px tile grid, top-down"
    WPX, HPX = 582, 470
    T = 16
    # block origins in tiles; blocks are 10 by 9, paths 2 wide; the ring reads C O R E C R A F clockwise, Time at the hub
    BX, BY = [1, 13, 25], [0, 10, 20]
    RING = {(0, 0): "career", (1, 0): "outlook", (2, 0): "relationships", (2, 1): "economic",
            (2, 2): "competence", (1, 2): "recovery", (0, 2): "agents", (0, 1): "faith", (1, 1): "time"}

    @classmethod
    def r(cls, tx, ty, tw, th, c, o=1.0, extra=""):
        t = cls.T
        return f'<rect x="{tx * t:.1f}" y="{ty * t:.1f}" width="{tw * t:.1f}" height="{th * t:.1f}" class="{c}" opacity="{o}" {extra}/>'

    @classmethod
    def canvas(cls, s):
        ctx = context(s)
        t = cls.T
        out = []
        # ground: grass in two tones of surface, with tufts
        out.append(f'<rect x="0" y="0" width="{cls.WPX}" height="{cls.HPX}" class="fl-surface" opacity="0.35"/>')
        rr = rnd(17)
        for ty in range(0, 30):
            for tx in range(0, 37):
                if (tx + ty) % 2 == 0:
                    out.append(cls.r(tx, ty, 1, 1, "fl-surface", 0.25))
                if next(rr) < 0.06:
                    out.append(f'<rect x="{tx * t + 5}" y="{ty * t + 7}" width="2" height="4" class="fl-green" opacity="0.55"/>'
                               f'<rect x="{tx * t + 9}" y="{ty * t + 5}" width="2" height="6" class="fl-green" opacity="0.55"/>')
        # paths: the ring road between blocks
        for x in (11, 23):
            out.append(cls.r(x, 0, 2, 30, "fl-muted", 0.55))
        for y in (9, 19):
            out.append(cls.r(0, y, 37, 1, "fl-muted", 0.55))
        # blocks
        for (gx, gy), pid in cls.RING.items():
            out.append(cls.block(cls.BX[gx], cls.BY[gy], pid, ctx, s))
        # the shipping bin at the hub's corner: quests done today
        dn = done_today(s)
        out.append(cls.r(23.15, 9.1, 1.7, 0.8, "fl-orange", 0.9))
        out.append(f'<text x="{24 * t:.0f}" y="{9.5 * t:.0f}" font-size="10.5" text-anchor="middle" dominant-baseline="middle" class="fl-background" font-weight="700">✓{dn}</text>')
        # proposals: carts waiting at the gates, the path ends nearest their block
        pos = {pid: xy for xy, pid in cls.RING.items()}
        gates = {"N": [(11.5, 0.1), (23.5, 0.1)], "S": [(11.5, 28.9), (23.5, 28.9)], "W": [(0.1, 9), (0.1, 19)], "E": [(35.9, 9), (35.9, 19)]}
        used = set()
        for p in s["proposed"]:
            gx, gy = pos[p["pillar"]]
            side = "N" if gy == 0 or (gx, gy) == (1, 1) else "S" if gy == 2 else "W" if gx == 0 else "E"
            for g_ in gates[side]:
                if g_ not in used:
                    used.add(g_)
                    x, y = g_
                    x = min(max(x, 0.2), 35.2)
                    y = min(max(y, 0.1), 28.2)
                    out.append(cls.r(x, y, 0.9, 0.7, "fl-cyan") + f'<circle cx="{(x + 0.2) * t:.1f}" cy="{(y + 0.85) * t:.1f}" r="2.5" class="fl-cyan"/>'
                               f'<circle cx="{(x + 0.7) * t:.1f}" cy="{(y + 0.85) * t:.1f}" r="2.5" class="fl-cyan"/>')
                    break
        # night: dim the ground, then light the lamps and windows again on top
        if ctx["night"] or ctx["dusk"]:
            out.append(f'<rect x="0" y="0" width="{cls.WPX}" height="{cls.HPX}" class="fl-background" opacity="{0.45 if ctx["night"] else 0.22}"/>')
            for (lx, ly) in [(11, 4), (23, 4), (11, 14), (23, 14), (11, 24), (23, 24), (5, 9), (17, 9), (29, 9), (5, 19), (17, 19), (29, 19)]:
                out.append(f'<circle cx="{lx * t + 8}" cy="{ly * t + 8}" r="9" class="fl-yellow" opacity="0.18"/>'
                           f'<rect x="{lx * t + 6}" y="{ly * t + 4}" width="4" height="8" class="fl-yellow"/>')
            for (gx, gy), pid in cls.RING.items():
                out.append(cls.windows(cls.BX[gx], cls.BY[gy], pid, ctx))
        out.append(rain_svg(cls.WPX, cls.HPX, ctx, 51))
        return svg(cls.WPX, cls.HPX, "".join(out))

    @classmethod
    def house_geom(cls, bx, by, p):
        floors = max(1, min(3, p["level"]))
        top = by + 1 + (3 - floors)
        return floors, top

    @classmethod
    def windows(cls, bx, by, pid, ctx):
        p = ctx["pm"][pid]
        floors, top = cls.house_geom(bx, by, p)
        t = cls.T
        out = []
        k = 0
        hx = bx + 2
        for f in range(floors):
            wy = by + 5 - f
            for wxo in ((0.6, 4.6) if f == 0 else (0.6, 2.6, 4.6)):
                on = k < p["today"]
                out.append(f'<rect x="{(hx + wxo) * t + 2:.1f}" y="{wy * t + 4:.1f}" width="{t * 0.8 - 4:.1f}" height="{t - 8:.1f}" '
                           f'class="{"fl-yellow" if on else "fl-background"}" opacity="{1 if on else 0.75}"/>')
                k += 1
        return "".join(out)

    @classmethod
    def block(cls, bx, by, pid, ctx, s):
        p = ctx["pm"][pid]
        t = cls.T
        out = []
        # Castra's frame around the whole yard
        out.append(f'<rect x="{bx * t + 1}" y="{by * t + 1}" width="{10 * t - 2}" height="{9 * t - 2}" fill="none" class="st-muted" stroke-width="2"/>')
        # title and numeral on a sign along the top
        out.append(title_svg((bx + 4) * t, by * t + 10, pid, ctx, 11.5).replace('text-anchor="middle"', 'text-anchor="middle"'))
        mark = "✦" if pid in ctx["laurel"] else ""
        out.append(text((bx + 9.3) * t, by * t + 10, roman(p["level"]) + mark, "fl-accent", 11, "end", bold=True))
        # the house: roof tiles, then one wall row per level (up to three)
        floors, top = cls.house_geom(bx, by, p)
        hx = bx + 2
        out.append(cls.r(hx - 0.2, top, 6.4, 1.3, "fl-muted"))
        out.append(cls.r(hx - 0.2, top + 1.0, 6.4, 0.3, "fl-background", 0.5))
        out.append(cls.roof_mark(hx, top, pid))
        out.append(cls.r(hx, top + 1.3, 6, floors - 0.3 + (1 - 1.0), "fl-surface", 1))
        out.append(cls.r(hx, by + 5, 6, 1, "fl-surface", 1))
        out.append(f'<rect x="{(hx + 2.6) * t:.1f}" y="{(by + 5) * t + 3:.1f}" width="{0.8 * t:.1f}" height="{t - 3:.1f}" class="fl-muted"/>')
        out.append(cls.windows(bx, by, pid, ctx))
        # mailbox by the door: its flag is up while a check-in waits
        mx = hx + 6.5
        out.append(f'<rect x="{mx * t:.1f}" y="{(by + 5) * t + 4:.1f}" width="7" height="6" class="fl-muted"/>'
                   f'<rect x="{mx * t + 3:.1f}" y="{(by + 5) * t + 10:.1f}" width="1.5" height="6" class="fl-muted"/>')
        if pid in ctx["asks"]:
            out.append(f'<rect x="{mx * t + 7:.1f}" y="{(by + 5) * t - 2:.1f}" width="2" height="9" class="fl-magenta"/>'
                       f'<rect x="{mx * t + 9:.1f}" y="{(by + 5) * t - 2:.1f}" width="5" height="4" class="fl-magenta"/>')
        # route stop and @
        if pid in ctx["stops"]:
            out.append(text((hx - 0.9) * t, (top + 0.6) * t, str(ctx["stops"][pid]), "fl-accent", 12, bold=True))
        if pid == ctx["top"]:
            out.append(f'<circle cx="{(hx + 3) * t:.1f}" cy="{(by + 6.5) * t:.1f}" r="5" class="fl-accent"/>')
        if pid == "agents" and p["today"]:
            out.append(f'<circle cx="{(hx + 4.6) * t:.1f}" cy="{(top - 0.4) * t:.1f}" r="3.5" class="fl-muted" opacity="0.5"/>')
        # Castra's week as a crop row: seven plots, then the 7-day count
        out.append(week_svg((bx + 0.6) * t, (by + 7.1) * t, p, cell=12, gap=3, n=7))
        out.append(text((bx + 0.6) * t, (by + 8.45) * t, f"{p['events_7d']} in 7d", "fl-muted", 10.5, "start"))
        return "".join(out)

    @classmethod
    def roof_mark(cls, hx, top, pid):
        t = cls.T
        x, y = hx * t, top * t
        if pid == "time":
            return f'<circle cx="{x + 3 * t:.1f}" cy="{y + 9:.1f}" r="7" class="fl-surface"/><line x1="{x + 3 * t:.1f}" y1="{y + 9:.1f}" x2="{x + 3 * t:.1f}" y2="{y + 4:.1f}" class="st-muted" stroke-width="1.5"/>'
        if pid in ("faith", "relationships"):
            return f'<polygon points="{x + 3 * t - 8},{y + 4} {x + 3 * t + 8},{y + 4} {x + 3 * t},{y - 8}" class="fl-muted"/>'
        if pid == "career":
            return f'<rect x="{x + 3 * t - 1.5:.1f}" y="{y - 10:.1f}" width="3" height="10" class="fl-muted"/>'
        if pid == "economic":
            return "".join(f'<rect x="{x + k * 16 + 6:.1f}" y="{y + 4:.1f}" width="3" height="9" class="fl-background" opacity="0.5"/>' for k in range(6))
        if pid in ("outlook", "recovery"):
            return f'<ellipse cx="{x + 3 * t:.1f}" cy="{y + 2:.1f}" rx="16" ry="7" class="fl-muted"/>'
        if pid == "agents":
            return f'<rect x="{x + 4.4 * t:.1f}" y="{y - 8:.1f}" width="6" height="10" class="fl-muted"/>'
        if pid == "competence":
            return "".join(f'<rect x="{x + 6 + k * 18:.1f}" y="{y + 3:.1f}" width="12" height="3" class="fl-background" opacity="0.45"/>' for k in range(5))
        return ""

    PITCH = ("Closest to Stardew itself: a top-down town of 16 px tiles. Each block is a fenced yard with a house that gains a storey per level, "
             "a mailbox whose flag goes up when a check-in waits, and seven crop plots for the week.")
    CLUSTER = "A ring of yards read clockwise from the top left, C O R E C R A F, with Time's clock house at the hub."
    UNIQUE = "Mailbox flags for check-ins, a shipping bin counting today's done quests, carts at the gates for proposals, lamps by night."
    S = "Feels like a game, and every signal has a familiar object; the 3×3 plan keeps hit testing as simple as a grid."
    R = "Leaves the ASCII house style; tiles at board size are small, so houses are simple and the night dimming must stay readable."
    C = "Medium-high: about 450 rect calls, but all on a fixed 16 px grid with no perspective math."
    TAPS = [
        ("A yard", "open that pillar's Popup", "Popup (reads the snapshot)"),
        ("A raised mailbox flag", "open the check-in Popup", "Popup"),
        ("A cart at a gate", "open the proposal Popup", "Popup"),
        ("Proposal · accept / decline", "approve or turn down", CMD["accept"] + "  |  " + CMD["decline"]),
        ("The shipping bin", "show today's done quests, each with ↺", CMD["undo"]),
    ]
    NOTES = """<li><b>Drawn with</b> Canvas <code>rect()</code> on a 16 px (11.5 dp) grid, a few <code>circle</code>s and <code>path</code>s for roofs, and <code>text()</code> for titles, numerals and counts. It leaves the glyph style for pixel tiles.</li>
<li>Blocks are 10 by 9 tiles at x = 1, 13, 25 and y = 0, 10, 20; paths are 2 tiles. A tap's tile picks the yard, and inside it the mailbox and bin have their own small boxes.</li>
<li>Night lays a <code>background</code> rect at 0.45 over the ground, then draws lamps and windows again on top, so only the lit things stay bright.</li>"""


CONCEPTS = [Vicus, DuaeInsulae, Tesserae]
