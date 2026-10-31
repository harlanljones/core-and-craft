package snapshot

import (
	"math"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
	"github.com/harlanljones/core-and-craft/internal/review"
)

// The xp table. Nobody writes xp; these rules turn recorded events into it.
const (
	xpQuestDone      = 10 // a quest marked done by hand
	xpQuestDoneP1    = 15 // ... at priority 1
	xpTypingRun      = 5  // each qualifying typing run
	xpTypingDayCap   = 25 // per skill per day
	xpStudyPer5Min   = 1  // study.session minutes, per pillar per day
	xpStudyDayCap    = 12
	xpNoteAdded      = 15
	xpCheckinAnswer  = 5 // the first answer to each check-in
	xpSkillPassed    = 50
	xpReview         = 2  // the first review of a card each day
	xpReviewDayCap   = 20 // per pillar per day
	xpCardMature     = 10 // once, when a card's stability first reaches 21 days
	levelBase        = 100.0
	levelGrowth      = 1.5
	historyDays      = 30
	streakLookback   = 400
	retractedKind    = "event.retracted"
	restKind         = "day.rested"
	questDoneKind    = "quest.done"
	checkinAnswered  = "checkin.answered"
	studySessionKind = "study.session"
	noteAddedKind    = "note.added"
)

// Level is xp rolled up on the rising curve: level n costs
// round(100 x 1.5^(n-1)) xp to leave.
type Level struct {
	Total int `json:"total"`
	Level int `json:"level"`
	Into  int `json:"into"`
	Need  int `json:"need"`
}

func levelOf(total int) Level {
	l := Level{Total: total, Level: 1, Into: total}
	for {
		cost := int(math.Round(levelBase * math.Pow(levelGrowth, float64(l.Level-1))))
		if l.Into < cost {
			l.Need = cost
			return l
		}
		l.Into -= cost
		l.Level++
	}
}

// Streak counts days in a row with any recorded event (an import from
// acta-mt counts, as it earns xp) or a rest day. Today counts once it has
// one; until then the streak runs through yesterday.
type Streak struct {
	Days  int  `json:"days"`
	Today bool `json:"today"`
}

type xpGain struct {
	date, pillar string
	xp           int
}

// xpInputs is what the skill fold already knows that xp needs.
type xpInputs struct {
	runsByDay map[string]map[string]int // skill id -> date -> qualifying runs
	skills    []Skill
	quests    map[string]Quest
}

// gains lists every xp award from the live log.
func gains(live []event.Event, in xpInputs, loc *time.Location) []xpGain {
	var out []xpGain
	study := map[[2]string]float64{} // date, pillar -> minutes
	answered := map[string]bool{}
	noted := map[string]bool{}
	for _, e := range live {
		date := e.LocalDate(loc)
		switch e.Kind {
		case questDoneKind:
			xp := xpQuestDone
			if q, ok := in.quests[e.Subject]; ok && q.Priority == 1 {
				xp = xpQuestDoneP1
			}
			out = append(out, xpGain{date, e.Pillar, xp})
		case noteAddedKind:
			// Once per note: a new version (supersedes) earns nothing more,
			// and a shared node (notes/nodes/) is filed from a note that
			// already earned it.
			p, _ := decode(e.Data)["path"].(string)
			if folder, _, _ := event.NotePath(p); !noted[e.Subject] && folder != event.NodesDir {
				out = append(out, xpGain{date, e.Pillar, xpNoteAdded})
			}
			noted[e.Subject] = true
		case checkinAnswered:
			if !answered[e.Subject] {
				answered[e.Subject] = true
				out = append(out, xpGain{date, e.Pillar, xpCheckinAnswer})
			}
		case studySessionKind:
			if f, ok := num(decode(e.Data)["minutes"]); ok && f > 0 {
				study[[2]string{date, e.Pillar}] += f
			}
		}
	}
	// Reviews: a card counts once a day, and only while its note is in the
	// log, so a review of a card nobody wrote earns nothing.
	reviewed := map[[2]string]int{} // date, pillar -> cards reviewed
	seen := map[[2]string]bool{}    // card, date
	for _, r := range review.Fold(live, loc).Reviews {
		if !noted["note:"+review.Slug(r.Card)] {
			continue
		}
		if k := [2]string{r.Card, r.Date}; !seen[k] {
			seen[k] = true
			reviewed[[2]string{r.Date, r.Pillar}]++
		}
		if r.FirstMature {
			out = append(out, xpGain{r.Date, r.Pillar, xpCardMature})
		}
	}
	for k, n := range reviewed {
		out = append(out, xpGain{k[0], k[1], min(n*xpReview, xpReviewDayCap)})
	}
	for k, minutes := range study {
		xp := int(minutes/5) * xpStudyPer5Min
		if xp > xpStudyDayCap {
			xp = xpStudyDayCap
		}
		if xp > 0 {
			out = append(out, xpGain{k[0], k[1], xp})
		}
	}
	for _, sk := range in.skills {
		for date, runs := range in.runsByDay[sk.ID] {
			xp := runs * xpTypingRun
			if xp > xpTypingDayCap {
				xp = xpTypingDayCap
			}
			out = append(out, xpGain{date, sk.Pillar, xp})
		}
		if sk.Passed {
			out = append(out, xpGain{dateOf(sk.PassedAt, loc), sk.Pillar, xpSkillPassed})
		}
	}
	return out
}

// dateOf turns an event's at (day or RFC 3339) into its local date.
func dateOf(at string, loc *time.Location) string {
	e := event.Event{At: at, AtPrecision: event.PrecisionSecond}
	if len(at) == len("2006-01-02") {
		e.AtPrecision = event.PrecisionDay
	}
	return e.LocalDate(loc)
}

// rollUp totals gains overall and per pillar, and the last 30 days of xp,
// oldest first, ending today. Gains dated after today don't count.
func rollUp(gs []xpGain, today time.Time) (Level, map[string]Level, []int) {
	todayS := today.Format("2006-01-02")
	total := 0
	per := map[string]int{}
	byDay := map[string]int{}
	for _, g := range gs {
		if g.date > todayS {
			continue
		}
		total += g.xp
		per[g.pillar] += g.xp
		byDay[g.date] += g.xp
	}
	pillars := map[string]Level{}
	for _, p := range event.Pillars {
		pillars[p] = levelOf(per[p])
	}
	hist := make([]int, historyDays)
	for i := range hist {
		hist[i] = byDay[today.AddDate(0, 0, i-historyDays+1).Format("2006-01-02")]
	}
	return levelOf(total), pillars, hist
}

// leveledToday lists the pillars whose level is higher with today's gains
// than without them. Gains dated after today don't count.
func leveledToday(gs []xpGain, today time.Time) []string {
	todayS := today.Format("2006-01-02")
	before, after := map[string]int{}, map[string]int{}
	for _, g := range gs {
		switch {
		case g.date < todayS:
			before[g.pillar] += g.xp
			after[g.pillar] += g.xp
		case g.date == todayS:
			after[g.pillar] += g.xp
		}
	}
	out := []string{}
	for _, p := range event.Pillars {
		if levelOf(after[p]).Level > levelOf(before[p]).Level {
			out = append(out, p)
		}
	}
	return out
}

// streakOf walks back from today over days with an event or a rest. live has
// gone through Unshadowed, so a run logged by hand and repeated by an import
// is one event here, as it is for xp.
func streakOf(live []event.Event, today time.Time, loc *time.Location) Streak {
	counted := map[string]bool{}
	for _, e := range live {
		if e.Kind == retractedKind {
			continue
		}
		counted[e.LocalDate(loc)] = true
	}
	s := Streak{Today: counted[today.Format("2006-01-02")]}
	d := today
	if !s.Today {
		d = d.AddDate(0, 0, -1)
	}
	for i := 0; i < streakLookback && counted[d.Format("2006-01-02")]; i++ {
		s.Days++
		d = d.AddDate(0, 0, -1)
	}
	return s
}
