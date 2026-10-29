"""Synthetic snapshot data for the CORE & CRAFT tile mockups.

Shape follows Harlan's brief (today, streak, xp, history, pillars, quests,
proposed, checkins, skills, errors). Two fields are proposals of these
mockups and are marked as such on the pages:
  pillars[].today  events today per pillar (lights the town's windows)
  clock            local time, from OWL's `source now = clock`, not the snapshot
"""
import datetime as dt

PILLARS = [
    # id, latin, english label (<= 7 chars), english full
    ("career", "cursus", "career", "Career"),
    ("outlook", "animus", "outlook", "Outlook"),
    ("relationships", "amici", "relat.", "Relationships"),
    ("economic", "arca", "econ.", "Economic"),
    ("competence", "ars", "compet.", "Competence"),
    ("recovery", "otium", "recov.", "Recovery"),
    ("agents", "actores", "agents", "Agents"),
    ("faith", "fides", "faith", "Faith"),
    ("time", "tempus", "time", "Time"),
]
LATIN = {p[0]: p[1] for p in PILLARS}
# Spec titles and acronym letters, used everywhere a pillar is named.
TITLE = {p[0]: p[3] for p in PILLARS}
GROUPS = [("C.O.R.E.", ["career", "outlook", "relationships", "economic"]),
          ("C.R.A.F.T.", ["competence", "recovery", "agents", "faith", "time"])]
ENG = {p[0]: p[2] for p in PILLARS}
ENGFULL = {p[0]: p[3] for p in PILLARS}


def need(level):
    return round(100 * 1.5 ** (level - 1))


def level_of(total):
    level = 1
    while total >= need(level):
        total -= need(level)
        level += 1
    return level, total, need(level)


def roman(n):
    if n == 0:
        return "N"
    out = ""
    for v, s in [(1000, "M"), (900, "CM"), (500, "D"), (400, "CD"), (100, "C"), (90, "XC"), (50, "L"), (40, "XL"),
                 (10, "X"), (9, "IX"), (5, "V"), (4, "IV"), (1, "I")]:
        while n >= v:
            out += s
            n -= v
    return out


TODAY = dt.date(2026, 11, 2)
DATES = [(TODAY - dt.timedelta(days=29 - i)).isoformat() for i in range(30)]


def daily(bests):
    """bests: {days_ago: (best, runs)}"""
    out = []
    for i, d in enumerate(DATES):
        ago = 29 - i
        b, r = bests.get(ago, (None, 0))
        out.append({"date": d, "best": b, "runs": r})
    return out


def pillars(rows):
    out = []
    for (pid, *_), (level, into, d7, today) in zip(PILLARS, rows):
        out.append({"pillar": pid, "level": level, "into": into, "need": need(level),
                    "events_7d": d7, "today": today})
    return out


def xp(total):
    level, into, nd = level_of(total)
    return {"total": total, "level": level, "into": into, "need": nd}


EMPTY = {
    "clock": "10:05",
    "today": {"date": TODAY.isoformat(), "events": 0, "typing_tests": 0},
    "streak": {"days": 0, "today": False},
    "xp": xp(0),
    "history": [0] * 30,
    "pillars": pillars([(1, 0, 0, 0)] * 9),
    "quests": [],
    "proposed": [],
    "checkins": [],
    "skills": [{"id": "skill:typing", "min_acc": 97, "best": None, "target": 100, "today_best": None,
                "passed": False, "runs_qualifying": 0, "runs_total": 0, "daily": daily({})}],
    "errors": [],
    "leveled_today": [],
}

NORMAL = {
    "clock": "18:40",
    "today": {"date": TODAY.isoformat(), "events": 6, "typing_tests": 3},
    "streak": {"days": 4, "today": True},
    "xp": xp(610),
    "history": [0, 15, 30, 5, 0, 25, 40, 10, 0, 20, 35, 15, 5, 0, 30, 45, 20, 10, 0, 25,
                15, 40, 30, 0, 10, 35, 20, 45, 25, 55],
    "pillars": pillars([(2, 40, 3, 1), (1, 60, 1, 0), (1, 20, 0, 0), (1, 35, 2, 0), (3, 120, 9, 3),
                        (1, 10, 1, 0), (2, 85, 5, 1), (2, 15, 4, 1), (1, 70, 2, 0)]),
    "quests": [
        {"id": "quest:typing-drill", "pillar": "competence", "title": "15 min typing drill", "priority": 1,
         "every": "daily", "due": True, "overdue_days": 0, "done_today": False, "auto": True, "score": 4.2,
         "reasons": ["due today", "typing 12 under target"]},
        {"id": "quest:call-mom", "pillar": "relationships", "title": "Call mom", "priority": 2,
         "every": "weekly:sun", "due": True, "overdue_days": 2, "done_today": False, "auto": False, "score": 3.9,
         "reasons": ["2 days overdue", "relationships quiet 6d"]},
        {"id": "quest:shutdown-note", "pillar": "outlook", "title": "Evening shutdown note", "priority": 3,
         "every": "daily", "due": True, "overdue_days": 0, "done_today": True, "auto": False, "score": 0,
         "reasons": ["done 17:55", "daily"]},
    ],
    "proposed": [{"id": "quest:budget-review", "pillar": "economic", "title": "Review October spend",
                  "from": "duet:economic"}],
    "checkins": [{"id": "checkin:sleep-window", "pillar": "recovery",
                  "question": "What's your target sleep window?", "open": True}],
    "skills": [{"id": "skill:typing", "min_acc": 97, "best": 88, "target": 100, "today_best": 88, "passed": False,
                "runs_qualifying": 2, "runs_total": 5,
                "daily": daily({12: (76, 2), 9: (79, 3), 7: (81, 1), 5: (84, 2), 3: (85, 2), 0: (88, 3)})}],
    "errors": [],
    "leveled_today": [],
}

BUSY = {
    "clock": "22:30",
    "today": {"date": TODAY.isoformat(), "events": 19, "typing_tests": 9},
    "streak": {"days": 23, "today": True},
    "xp": xp(2890),
    "history": [40, 55, 30, 65, 70, 45, 90, 60, 75, 50, 85, 95, 40, 70, 110, 80, 65, 90, 120, 75,
                95, 60, 105, 130, 85, 100, 115, 90, 140, 155],
    "pillars": pillars([(4, 200, 12, 3), (2, 90, 5, 1), (2, 20, 0, 0), (3, 60, 4, 2), (5, 300, 21, 9),
                        (3, 10, 6, 1), (4, 90, 14, 2), (3, 200, 8, 1), (2, 140, 6, 0)]),
    "quests": [
        {"id": "quest:call-mom", "pillar": "relationships", "title": "Call mom", "priority": 1,
         "every": "weekly:sun", "due": True, "overdue_days": 4, "done_today": False, "auto": False, "score": 4.8,
         "reasons": ["4 days overdue", "relationships quiet 9d"]},
        {"id": "quest:typing-drill", "pillar": "competence", "title": "15 min typing drill", "priority": 2,
         "every": "daily", "due": True, "overdue_days": 0, "done_today": False, "auto": True, "score": 4.2,
         "reasons": ["due today", "typing 6 under target"]},
        {"id": "quest:receipts", "pillar": "economic", "title": "File expense receipts", "priority": 2,
         "every": "weekly:mon", "due": True, "overdue_days": 0, "done_today": False, "auto": False, "score": 3.6,
         "reasons": ["due today", "economic quiet 5d"]},
        {"id": "quest:wind-down", "pillar": "recovery", "title": "Screens off by 22:45", "priority": 3,
         "every": "daily", "due": True, "overdue_days": 0, "done_today": False, "auto": False, "score": 3.4,
         "reasons": ["due tonight", "2 short nights"]},
        {"id": "quest:stress-check", "pillar": "outlook", "title": "Sunday stress check", "priority": 3,
         "every": "weekly:sun", "due": True, "overdue_days": 1, "done_today": False, "auto": True, "score": 3.1,
         "reasons": ["1 day overdue", "outlook quiet 4d"]},
        {"id": "quest:agent-review", "pillar": "agents", "title": "Weekly agent review", "priority": 3,
         "every": "weekly:wed", "due": False, "overdue_days": 0, "done_today": False, "auto": False, "score": 2.2,
         "reasons": ["due in 2d", "friction backlog 4"]},
        {"id": "quest:sprint-2", "pillar": "career", "title": "Focus sprint 2", "priority": 2,
         "every": "daily", "due": True, "overdue_days": 0, "done_today": True, "auto": False, "score": 0,
         "reasons": ["done 15:02", "daily"]},
    ],
    "proposed": [
        {"id": "quest:budget-review", "pillar": "economic", "title": "Review October spend", "from": "duet:economic"},
        {"id": "quest:two-catchups", "pillar": "relationships", "title": "Plan two catch-ups", "from": "duet:relationships"},
        {"id": "quest:sprint-block", "pillar": "time", "title": "Block Sprint 1 daily", "from": "duet:time"},
    ],
    "checkins": [
        {"id": "checkin:sleep-window", "pillar": "recovery", "question": "What's your target sleep window?", "open": True},
        {"id": "checkin:shipped", "pillar": "career", "question": "What shipped this week?", "open": True},
        {"id": "checkin:stay-close", "pillar": "relationships", "question": "Who do you want to stay close to?", "open": True},
    ],
    "skills": [{"id": "skill:typing", "min_acc": 97, "best": 94, "target": 100, "today_best": 92, "passed": False,
                "runs_qualifying": 6, "runs_total": 9,
                "daily": daily({ago: (b, 2) for ago, b in zip(
                    [27, 25, 24, 22, 21, 20, 18, 17, 15, 14, 13, 11, 10, 9, 8, 6, 5, 4, 3, 2, 1, 0],
                    [78, 80, 79, 82, 81, 84, 83, 85, 86, 85, 87, 88, 88, 89, 90, 91, 90, 92, 93, 94, 91, 92])})}],
    "errors": [{"line": 412, "reason": "unknown kind quest.snoozed"}],
    "leveled_today": ["recovery"],
}

STATES = [("empty", "Empty", EMPTY), ("normal", "Normal", NORMAL), ("busy", "Busy", BUSY)]


def ranked(state):
    open_ = [q for q in state["quests"] if not q["done_today"]]
    done = [q for q in state["quests"] if q["done_today"]]
    return sorted(open_, key=lambda q: -q["score"]) + done


def top_pillar(state):
    r = [q for q in state["quests"] if not q["done_today"]]
    return max(r, key=lambda q: q["score"])["pillar"] if r else None


def is_night(state):
    h = int(state["clock"].split(":")[0])
    return h >= 19 or h < 6


def is_dusk(state):
    h = int(state["clock"].split(":")[0])
    return 17 <= h < 19


def overdue_count(state):
    return sum(1 for q in state["quests"] if q["overdue_days"] > 0 and not q["done_today"])


def weather(state):
    n = overdue_count(state)
    return "clear" if n == 0 else "drizzle" if n == 1 else "storm"


def typing_strip(sk):
    """Same arithmetic as widgets/60-typing.owl."""
    d = sk["daily"]
    bests = [x["best"] for x in d if x["best"] is not None]
    lo = min(bests) if bests else 0
    hi = max(bests) if bests else 0
    blocks = "▁▂▃▄▅▆▇█"
    out = ""
    for x in d:
        if x["best"] is None:
            out += "·"
        else:
            lvl = 7 if hi == lo else max(0, min(7, int((x["best"] - lo) / (hi - lo) * 7.99)))
            out += blocks[lvl]
    return out
