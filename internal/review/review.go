// Package review schedules recall cards with FSRS. A note with a recall:
// line is the card card:<slug>, and the review.logged events under that
// subject are its history. The fold runs go-fsrs's long-term scheduler with
// fuzz off, so the same log and clock always give the same schedule
// (docs/knowledge-graph.md).
package review

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v3"

	"github.com/harlanljones/core-and-craft/internal/event"
)

const (
	Kind = "review.logged"
	// MatureDays is the stability at which a card counts as mature.
	MatureDays = 21.0
	// Prefix starts every card subject; the rest is the note's slug.
	Prefix = "card:"
)

// Ratings are FSRS's four answers, 1 to 4.
var Ratings = []string{"again", "hard", "good", "easy"}

// Card is one card's schedule as the snapshot and graph.json show it.
type Card struct {
	ID             string  `json:"id"`
	State          string  `json:"state"` // new, learning, review, relearning
	Due            string  `json:"due"`   // local date
	Stability      float64 `json:"stability"`
	Difficulty     float64 `json:"difficulty"`
	Retrievability float64 `json:"retrievability"`
	Reps           int     `json:"reps"`
	Lapses         int     `json:"lapses"`
	LastReview     string  `json:"last_review,omitempty"`
	MatureOn       string  `json:"mature_on,omitempty"`
}

// Review is one folded review.logged and what it left the card at.
type Review struct {
	ID, Card, Pillar, Date string
	Rating                 int
	Stability, Difficulty  float64
	Due                    string
	// FirstMature is true for the review that first took the card to
	// MatureDays of stability.
	FirstMature bool
}

// Deck holds every reviewed card's FSRS state.
type Deck struct {
	Reviews []Review
	cards   map[string]*state
	loc     *time.Location
}

type state struct {
	c        fsrs.Card
	matureOn string
}

func scheduler() *fsrs.FSRS {
	p := fsrs.DefaultParam()
	// Reviews come a day at a time, from the board or a quiz in chat, so
	// there are no same-day learning steps; and no fuzz, so the fold is
	// deterministic.
	p.EnableShortTerm = false
	p.EnableFuzz = false
	return fsrs.NewFSRS(p)
}

// At is the moment a review counts at: noon on its local date. A review
// logged in chat carries only a day, one from the board a second; both
// schedule the same way, and a due date is a whole day.
func At(date string, loc *time.Location) time.Time {
	d, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return time.Time{}
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, loc)
}

// RatingOf reads data.rating, a whole number 1..4.
func RatingOf(data json.RawMessage) (int, bool) {
	var d struct {
		Rating *float64 `json:"rating"`
	}
	if json.Unmarshal(data, &d) != nil || d.Rating == nil {
		return 0, false
	}
	r := *d.Rating
	if r != math.Trunc(r) || r < 1 || r > 4 {
		return 0, false
	}
	return int(r), true
}

// ParseRating reads 1..4 or again, hard, good, easy.
func ParseRating(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	for i, name := range Ratings {
		if s == name || s == string(rune('1'+i)) {
			return i + 1, true
		}
	}
	return 0, false
}

// Slug is the note slug a card subject names.
func Slug(card string) string { return strings.TrimPrefix(card, Prefix) }

// Fold folds the live review.logged events (retractions and supersedes
// already applied) in date order, then seq, so a quiz filed late still
// lands on its own day.
func Fold(live []event.Event, loc *time.Location) *Deck {
	type rv struct {
		ev     event.Event
		date   string
		rating int
	}
	var rs []rv
	for _, e := range live {
		if e.Kind != Kind || !strings.HasPrefix(e.Subject, Prefix) {
			continue
		}
		r, ok := RatingOf(e.Data)
		if !ok {
			continue
		}
		rs = append(rs, rv{e, e.LocalDate(loc), r})
	}
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].date != rs[j].date {
			return rs[i].date < rs[j].date
		}
		return rs[i].ev.Seq < rs[j].ev.Seq
	})
	d := &Deck{cards: map[string]*state{}, loc: loc}
	f := scheduler()
	for _, r := range rs {
		st, ok := d.cards[r.ev.Subject]
		if !ok {
			st = &state{c: fsrs.NewCard()}
			d.cards[r.ev.Subject] = st
		}
		st.c = f.Next(st.c, At(r.date, loc), fsrs.Rating(r.rating)).Card
		out := Review{
			ID: r.ev.ID, Card: r.ev.Subject, Pillar: r.ev.Pillar, Date: r.date, Rating: r.rating,
			Stability: round4(st.c.Stability), Difficulty: round4(st.c.Difficulty), Due: st.c.Due.In(loc).Format("2006-01-02"),
		}
		if st.matureOn == "" && st.c.Stability >= MatureDays {
			st.matureOn, out.FirstMature = r.date, true
		}
		d.Reviews = append(d.Reviews, out)
	}
	return d
}

// Reviewed lists the cards with at least one review, sorted.
func (d *Deck) Reviewed() []string {
	out := make([]string, 0, len(d.cards))
	for id := range d.cards {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Card is id's schedule at now. A card nobody has reviewed is new and due
// today.
func (d *Deck) Card(id string, now time.Time) Card {
	today := now.In(d.loc).Format("2006-01-02")
	st, ok := d.cards[id]
	if !ok {
		return Card{ID: id, State: "new", Due: today}
	}
	c := Card{
		ID: id, State: stateName(st.c.State),
		Due:        st.c.Due.In(d.loc).Format("2006-01-02"),
		Stability:  round4(st.c.Stability),
		Difficulty: round4(st.c.Difficulty),
		Reps:       int(st.c.Reps), Lapses: int(st.c.Lapses),
		LastReview: st.c.LastReview.In(d.loc).Format("2006-01-02"),
		MatureOn:   st.matureOn,
	}
	// Retrievability on FSRS's forgetting curve; a review later today than
	// now (at noon) counts as just done.
	last := st.c.LastReview
	if now.Before(last) {
		now = last
	}
	c.Retrievability = round4(scheduler().GetRetrievability(st.c, now))
	return c
}

func stateName(s fsrs.State) string {
	switch s {
	case fsrs.Learning:
		return "learning"
	case fsrs.Review:
		return "review"
	case fsrs.Relearning:
		return "relearning"
	}
	return "new"
}

func round4(f float64) float64 { return math.Round(f*1e4) / 1e4 }

// Summary is the cards line in snapshot.json.
type Summary struct {
	Total         int    `json:"total"`
	New           int    `json:"new"`
	Due           int    `json:"due"`     // due today or earlier, new cards included
	Overdue       int    `json:"overdue"` // due before today
	Mature        int    `json:"mature"`  // stability of MatureDays or more
	ReviewedToday int    `json:"reviewed_today"`
	NextDue       string `json:"next_due,omitempty"` // the earliest due date after today
}

// Summarize counts cards on today and the reviews logged on it.
func Summarize(cards []Card, reviews []Review, today string) Summary {
	s := Summary{Total: len(cards)}
	for _, c := range cards {
		switch {
		case c.State == "new":
			s.New++
		case c.Stability >= MatureDays:
			s.Mature++
		}
		switch {
		case c.Due <= today:
			s.Due++
			if c.Due < today {
				s.Overdue++
			}
		case s.NextDue == "" || c.Due < s.NextDue:
			s.NextDue = c.Due
		}
	}
	for _, r := range reviews {
		if r.Date == today {
			s.ReviewedToday++
		}
	}
	return s
}

// Queue orders the due cards: reviewed cards first, the longest overdue
// and then the faintest first, then new cards by id.
func Queue(cards []Card, today string) []Card {
	var due []Card
	for _, c := range cards {
		if c.Due <= today {
			due = append(due, c)
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		a, b := due[i], due[j]
		if (a.State == "new") != (b.State == "new") {
			return b.State == "new"
		}
		if a.Due != b.Due {
			return a.Due < b.Due
		}
		if a.Retrievability != b.Retrievability {
			return a.Retrievability < b.Retrievability
		}
		return a.ID < b.ID
	})
	return due
}
