package snapshot

import (
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
	"github.com/harlanljones/core-and-craft/internal/quest"
)

// Quest is a quest's current state, folded from its events.
type Quest struct {
	ID           string   `json:"id"`
	Pillar       string   `json:"pillar"`
	Title        string   `json:"title"`
	Priority     int      `json:"priority"`
	Every        string   `json:"every"`
	Skill        string   `json:"skill,omitempty"`
	Rule         string   `json:"rule,omitempty"`
	Status       string   `json:"status"` // proposed, active, declined, dropped, done (once quests)
	From         string   `json:"from"`
	Added        string   `json:"added"`
	SnoozedUntil string   `json:"snoozed_until,omitempty"`
	LastDone     string   `json:"last_done,omitempty"`
	DoneDates    []string `json:"-"`
}

// DoneOn reports whether the quest was marked done on date (YYYY-MM-DD).
func (q Quest) DoneOn(date string) bool {
	for _, d := range q.DoneDates {
		if d == date {
			return true
		}
	}
	return false
}

// Quests folds live (effective, seq-ordered) events into quests, in the
// order they were added. The first quest.added for a subject wins; later
// ones are ignored. A quest from any agent but manual starts proposed.
func Quests(live []event.Event, loc *time.Location) []Quest {
	idx := map[string]int{}
	var out []Quest
	for _, e := range live {
		if e.Kind == "quest.added" {
			if _, dup := idx[e.Subject]; dup {
				continue
			}
			m := decode(e.Data)
			q := Quest{ID: e.Subject, Pillar: e.Pillar, Priority: 2, Every: "once", From: e.Source.Agent, Added: e.LocalDate(loc), Status: "proposed"}
			applyEdit(&q, m)
			if s, _ := m["skill"].(string); s != "" {
				q.Skill = s
			}
			if s, _ := m["rule"].(string); s != "" {
				q.Rule = s
			}
			if e.Source.Agent == "manual" {
				q.Status = "active"
			}
			if s, _ := m["status"].(string); s == "proposed" {
				q.Status = "proposed"
			}
			idx[e.Subject] = len(out)
			out = append(out, q)
			continue
		}
		i, ok := idx[e.Subject]
		if !ok {
			continue
		}
		q := &out[i]
		switch e.Kind {
		case "quest.accepted":
			if q.Status == "proposed" {
				q.Status = "active"
			}
		case "quest.declined":
			if q.Status == "proposed" {
				q.Status = "declined"
			}
		case "quest.dropped":
			q.Status = "dropped"
		case "quest.edited":
			applyEdit(q, decode(e.Data))
		case "quest.snoozed":
			q.SnoozedUntil, _ = decode(e.Data)["until"].(string)
		case "quest.done":
			d := e.LocalDate(loc)
			if !q.DoneOn(d) {
				q.DoneDates = append(q.DoneDates, d)
			}
			if d > q.LastDone {
				q.LastDone = d
			}
			if q.Every == "once" && q.Status == "active" {
				q.Status = "done"
			}
		}
	}
	return out
}

// applyEdit copies title, priority and every from quest.added or
// quest.edited data. Validation already checked them.
func applyEdit(q *Quest, m map[string]any) {
	if s, ok := m["title"].(string); ok {
		q.Title = s
	}
	if f, ok := num(m["priority"]); ok {
		q.Priority = int(f)
	}
	if s, ok := m["every"].(string); ok {
		if ev, err := quest.Parse(s); err == nil {
			q.Every = ev.String()
		}
	}
}
