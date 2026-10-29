import json, os, sys
sys.path.insert(0, os.path.dirname(__file__))
import common, forum, tabularium, hora

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "reference")
LINKS = {}
try:
    LINKS = json.load(open(os.path.join(os.path.dirname(__file__), "links.json")))
except Exception:
    pass


def link(slug):
    return LINKS.get(slug, f"../{slug}/index.html")


RULE = """<h3>Roman framing</h3><ul>
<li>Pillars always read as the spec writes them, in English, grouped under C.O.R.E. and C.R.A.F.T.; each title's acronym letter is set in the accent color.</li>
<li>Latin appears only in tile eyebrows, as "Latin · English".</li>
<li>Rome lives in the frame: the buildings (a temple for Faith, a colonnaded treasury for Economic, a clock tower for Time), Roman numerals for levels only, a column rule between C.O.R.E. and C.R.A.F.T., and a yellow ✦ when a pillar levels up today.</li>
<li>Xp, counts, ranks and days stay in digits.</li></ul>"""

SIGNALS = """<h3>How to read the town</h3><div class="sigs">
<code>@</code><span>you, at the pillar of the top-ranked quest</span>
<code>▪</code><span>a lit window: one event today in that pillar</span>
<code>?</code><span>an agent's check-in waiting for your answer</span>
<code>night</code><span>after 19:00 by the clock source</span>
<code>rain</code><span>overdue quests: one is drizzle, two or more a storm</span>
<code>height</code><span>the pillar's level</span></div>"""

DIRS = {
    "forum": dict(
        mod=forum, title="Forum tile board", name="Forum", english="the town is the board",
        lede="""<p>The town is the first thing you read. Nine buildings stand for the nine pillars, so the day shows in the skyline before any number does: lit windows where you acted, an @ where the top quest waits, a ? where an agent asked you something, rain when something is overdue.</p>
<p>Everything else stays plain ASCII and small. Quests are a ranked list with their two reasons and one-character taps. Level reads in Roman numerals with an ASCII bar. Check-ins live in the town itself: tap the building with the ?, answer in a Popup. That keeps the board to five tiles in 8 rows.</p>""",
        aside="<h3>Strengths</h3><ul><li>Most character: the board looks like a place, not a dashboard.</li><li>Fewest tiles; check-ins can't be missed because they sit on the skyline.</li><li>Pure text, which is the house style.</li></ul><h3>Risks</h3><ul><li>The town Canvas is the hardest piece to build and needs exact font metrics.</li><li>Answering a check-in takes two taps.</li></ul>" + SIGNALS + RULE,
        blurbs={"empty": "First day. Clear sky at 10:05, every window dark, no @ because nothing is ranked, level I.",
                "normal": "The brief's data at 18:40. Dusk, one overdue quest so a light drizzle, @ on Competence (the typing drill), ? on Recovery.",
                "busy": "22:30 on a full day. Night, two overdue quests so a storm, three questions waiting, Recovery levels up today (✦), one invalid log line (red ●)."}),
    "tabularium": dict(
        mod=tabularium, title="Tabularium tile board", name="Tabularium", english="every tile is a ledger",
        lede="""<p>The record office. Every tile reads like a page of the record: ranks, scores, levels as Roman numerals, 7-day counts and progress bars. The town stays, but as a census plan: floors equal level, yellow windows are today, outlined windows are earlier this week, and the numbers sit under each building.</p>
<p>All actions are inline. Each quest row carries done, snooze and priority; proposals accept or decline on their row; check-ins get their own tile with the answer box already open. Six tiles take 10 rows and leave one 3x2 free.</p>""",
        aside="<h3>Strengths</h3><ul><li>Most information per glance, nothing hidden behind a tap.</li><li>Every snapshot field shows somewhere, which makes it easy to verify acta's arithmetic.</li><li>Check-ins have a home and an open input.</li></ul><h3>Risks</h3><ul><li>Small text (8–9 dp) and tiny tap targets.</li><li>Scores and counts can turn a calm board into a scoreboard.</li><li>Needs 10 rows, with a gap to fill.</li></ul>" + RULE + SIGNALS.replace("a lit window: one event today in that pillar", "today's event (▫ is earlier this week)"),
        blurbs={"empty": "First day. Every count is 0 and every row says so, rather than showing blanks.",
                "normal": "The brief's data at 18:40. One late quest, one proposal, one open check-in with the answer box ready.",
                "busy": "22:30 on a full day. Six open quests, all visible with scores; three proposals; three questions, the oldest one open."}),
    "hora": dict(
        mod=hora, title="Hora tile board", name="Hora", english="low ink, all day",
        lede="""<p>The hour. This board is meant to sit under windows all day and ask for nothing. The town is a quiet silhouette drawn with Canvas shapes in the surface color; only lit windows, the @ and a ? carry color. Rain is a few faint strokes.</p>
<p>The quests tile shows three things, one line of reasons each, and a single magenta "waiting" count that folds proposals and check-ins into one Popup. Level is a ring with a Roman numeral; the strip is nine blocks whose height is the week. Five tiles, 8 rows.</p>""",
        aside="<h3>Strengths</h3><ul><li>Calmest on the desktop; nothing competes with the windows above it.</li><li>Cheapest to build after the town; fewest nodes.</li><li>Big type, easy to read from across a desk.</li></ul><h3>Risks</h3><ul><li>Shapes, not characters: the least ASCII of the three.</li><li>Waiting items hide behind one tap and can be ignored.</li></ul>" + SIGNALS + RULE,
        blurbs={"empty": "First day. A clear morning, dark windows, the strip all dots.",
                "normal": "The brief's data at 18:40. Dusk, a drizzle, @ at Competence, one question on Recovery, \"2 waiting\" on the quests tile.",
                "busy": "22:30 on a full day. Night and a storm, the top three quests only, \"6 waiting\"."}),
}


def main():
    for slug, d in DIRS.items():
        others = [(DIRS[o]["name"], link(o)) for o in DIRS if o != slug]
        html = common.page(d["title"], slug, d["name"], d["english"], d["lede"], d["aside"], d["blurbs"],
                           d["mod"].build_board, None, d["mod"].popups(), d["mod"].tapmaps(), d["mod"].NOTES, others)
        os.makedirs(f"{OUT}/{slug}", exist_ok=True)
        with open(f"{OUT}/{slug}/index.html", "w") as f:
            f.write(html)
        print(slug, len(html))


if __name__ == "__main__":
    main()
