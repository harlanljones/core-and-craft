package snapshot

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
	"github.com/harlanljones/core-and-craft/internal/quest"
)

// Engine weights. The score is a plain sum, so the same log always gives
// the same order, and each term can be named as a reason.
const (
	wDue        = 3.0 // due today
	wOverdue    = 0.5 // per day overdue, up to maxOverdue
	wPriority   = 1.0 // x (4 - priority): p1 = 3, p3 = 1
	wQuiet      = 1.5 // x how quiet the pillar was over 7 days (0..1)
	wGap        = 1.0 // x how far the quest's skill is from where it should be (0..1)
	maxOverdue  = 7
	overdueScan = 60
	quietAt     = 7.0 // 7 or more events in 7 days is not quiet at all
	staleDays   = 3   // a skill with no qualifying run in this many days has a gap
)

// QuestView is one quest as the board shows it.
type QuestView struct {
	ID          string   `json:"id"`
	Pillar      string   `json:"pillar"`
	Title       string   `json:"title"`
	Priority    int      `json:"priority"`
	Every       string   `json:"every"`
	Skill       string   `json:"skill,omitempty"`
	Due         bool     `json:"due"`
	OverdueDays int      `json:"overdue_days"`
	DoneToday   bool     `json:"done_today"`
	Auto        bool     `json:"auto"`
	Score       float64  `json:"score"`
	Reasons     []string `json:"reasons"`
}

// Proposal is a quest an agent proposed, waiting for accept or decline.
type Proposal struct {
	ID       string `json:"id"`
	Pillar   string `json:"pillar"`
	Title    string `json:"title"`
	Priority int    `json:"priority"`
	Every    string `json:"every"`
	From     string `json:"from"`
}

// Checkin is a pillar question; open until answered.
type Checkin struct {
	ID       string `json:"id"`
	Pillar   string `json:"pillar"`
	Question string `json:"question"`
	From     string `json:"from"`
	Asked    string `json:"asked"`
	Open     bool   `json:"open"`
	Answered string `json:"answered,omitempty"`
	Answer   string `json:"answer,omitempty"`
}

var ruleParts = regexp.MustCompile(`^(runs|minutes):(\d+)$`)

type engineInputs struct {
	today    time.Time // local midnight
	quiet    map[string]int
	skills   map[string]Skill
	runs     map[string]map[string]int     // skill -> date -> qualifying runs
	minutes  map[string]map[string]float64 // skill -> date -> study minutes
	timeZone *time.Location
}

type term struct {
	value  float64
	reason string
}

// rank turns folded quests into the board's list and the proposals.
// Order: open quests due today by score, then open quests not due today,
// then quests already done today. Ties break by id.
func rank(qs []Quest, in engineInputs) ([]QuestView, []Proposal) {
	todayS := in.today.Format("2006-01-02")
	views := []QuestView{}
	proposals := []Proposal{}
	for _, q := range qs {
		switch {
		case q.Status == "proposed":
			proposals = append(proposals, Proposal{ID: q.ID, Pillar: q.Pillar, Title: q.Title, Priority: q.Priority, Every: q.Every, From: q.From})
			continue
		case q.Status == "done" && q.LastDone == todayS:
			// a one-off finished today stays visible, so it can be undone
		case q.Status != "active":
			continue
		}
		if q.SnoozedUntil > todayS {
			continue
		}
		ev, err := quest.Parse(q.Every)
		if err != nil {
			continue
		}
		added, _ := time.ParseInLocation("2006-01-02", q.Added, in.timeZone)
		v := QuestView{ID: q.ID, Pillar: q.Pillar, Title: q.Title, Priority: q.Priority, Every: q.Every, Skill: q.Skill, Reasons: []string{}}
		v.Due = ev.Due(added, in.today)
		v.Auto = autoDone(q, in, todayS)
		v.DoneToday = q.DoneOn(todayS) || v.Auto
		if ev.Recurring() {
			v.OverdueDays = overdue(ev, q, added, in.today)
		}
		if v.DoneToday {
			v.Due = false
		}

		var terms []term
		if v.Due {
			terms = append(terms, term{wDue, "due today"})
		}
		if v.OverdueDays > 0 {
			n := v.OverdueDays
			if n > maxOverdue {
				n = maxOverdue
			}
			terms = append(terms, term{wOverdue * float64(n), plural(v.OverdueDays, "day") + " overdue"})
		}
		terms = append(terms, term{wPriority * float64(4-q.Priority), "priority " + strconv.Itoa(q.Priority)})
		if quiet := 1 - math.Min(float64(in.quiet[q.Pillar]), quietAt)/quietAt; quiet > 0 {
			terms = append(terms, term{wQuiet * quiet, fmt.Sprintf("%s quiet (%d in 7d)", q.Pillar, in.quiet[q.Pillar])})
		}
		if g, why := gap(q.Skill, in); g > 0 {
			terms = append(terms, term{wGap * g, why})
		}
		for _, t := range terms {
			v.Score += t.value
		}
		v.Score = math.Round(v.Score*100) / 100
		sort.SliceStable(terms, func(i, j int) bool { return terms[i].value > terms[j].value })
		for i := 0; i < len(terms) && i < 2; i++ {
			v.Reasons = append(v.Reasons, terms[i].reason)
		}
		views = append(views, v)
	}
	group := func(v QuestView) int {
		switch {
		case v.DoneToday:
			return 2
		case v.Due:
			return 0
		}
		return 1
	}
	sort.SliceStable(views, func(i, j int) bool {
		gi, gj := group(views[i]), group(views[j])
		if gi != gj {
			return gi < gj
		}
		if views[i].Score != views[j].Score {
			return views[i].Score > views[j].Score
		}
		return views[i].ID < views[j].ID
	})
	return views, proposals
}

// overdue counts days since the oldest missed due day in the run of
// missed due days just before today. A done due day ends the run.
func overdue(ev quest.Every, q Quest, added, today time.Time) int {
	oldest := 0
	for i := 1; i <= overdueScan; i++ {
		d := today.AddDate(0, 0, -i)
		if d.Before(added) {
			break
		}
		if !ev.Due(added, d) {
			continue
		}
		if q.DoneOn(d.Format("2006-01-02")) {
			break
		}
		oldest = i
	}
	return oldest
}

// autoDone reports whether today's evidence meets a skill quest's rule.
func autoDone(q Quest, in engineInputs, today string) bool {
	m := ruleParts.FindStringSubmatch(q.Rule)
	if q.Skill == "" || m == nil {
		return false
	}
	n, _ := strconv.Atoi(m[2])
	if m[1] == "runs" {
		return in.runs[q.Skill][today] >= n
	}
	return in.minutes[q.Skill][today] >= float64(n)
}

// gap is 0..1: how far a quest's skill is from its target, or, without a
// target, 0.5 when it has had no qualifying run for staleDays.
func gap(skillID string, in engineInputs) (float64, string) {
	sk, ok := in.skills[skillID]
	if skillID == "" || !ok {
		return 0, ""
	}
	name := strings.ToLower(sk.Name)
	if name == "" {
		name = strings.TrimPrefix(skillID, "skill:")
	}
	if sk.Target != nil && !sk.Passed {
		if sk.Best == nil {
			return 1, name + " not started"
		}
		if g := (*sk.Target - *sk.Best) / *sk.Target; g > 0 {
			return math.Min(g, 1), fmt.Sprintf("%s %s under target", name, strconv.FormatFloat(*sk.Target-*sk.Best, 'f', -1, 64))
		}
	}
	for i := 0; i < staleDays; i++ {
		if in.runs[skillID][in.today.AddDate(0, 0, -i).Format("2006-01-02")] > 0 {
			return 0, ""
		}
	}
	return 0.5, fmt.Sprintf("%s idle %dd", name, staleDays)
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}

// checkins folds asked and answered check-ins: every open one, plus those
// answered in the last 7 days.
func checkins(live []event.Event, today time.Time, loc *time.Location) []Checkin {
	idx := map[string]int{}
	out := []Checkin{}
	for _, e := range live {
		switch e.Kind {
		case "checkin.asked":
			if _, dup := idx[e.Subject]; dup {
				continue
			}
			q, _ := decode(e.Data)["question"].(string)
			idx[e.Subject] = len(out)
			out = append(out, Checkin{ID: e.Subject, Pillar: e.Pillar, Question: q, From: e.Source.Agent, Asked: e.LocalDate(loc), Open: true})
		case "checkin.answered":
			if i, ok := idx[e.Subject]; ok && out[i].Open {
				out[i].Open = false
				out[i].Answer, _ = decode(e.Data)["text"].(string)
				out[i].Answered = e.LocalDate(loc)
			}
		}
	}
	cutoff := today.AddDate(0, 0, -7).Format("2006-01-02")
	kept := []Checkin{}
	for _, c := range out {
		if c.Open || c.Answered >= cutoff {
			kept = append(kept, c)
		}
	}
	return kept
}
