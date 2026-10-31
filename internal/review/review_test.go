package review

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
)

func rev(seq int64, card, at string, rating int) event.Event {
	prec := event.PrecisionSecond
	if len(at) == len("2006-01-02") {
		prec = event.PrecisionDay
	}
	return event.Event{
		ID: fmt.Sprintf("evt_%016d", seq), Seq: seq, At: at, AtPrecision: prec, Pillar: "faith",
		Kind: Kind, Subject: card, Data: json.RawMessage(fmt.Sprintf(`{"rating":%d}`, rating)),
	}
}

// TestFoldMatchesGoFSRSReference folds go-fsrs's own long-term reference
// sequence (TestLongTermScheduler in go-fsrs v3.3.1: thirteen reviews, each
// on the day the last one made it due) through review.logged events, and
// checks the same intervals, stability and difficulty come out.
func TestFoldMatchesGoFSRSReference(t *testing.T) {
	ratings := []int{3, 3, 3, 3, 3, 3, 1, 1, 3, 3, 3, 3, 3}
	wantIvl := []int{3, 11, 35, 101, 269, 669, 12, 2, 5, 12, 26, 55, 112}
	wantS := []float64{3.173, 10.7389, 34.5776, 100.7483, 269.2838, 669.3093, 11.8987, 2.236, 5.2001, 11.8993, 26.4917, 55.4949, 111.9726}
	wantD := []float64{5.2824, 5.273, 5.2635, 5.2542, 5.2448, 5.2355, 6.7654, 7.794, 7.773, 7.7521, 7.7312, 7.7105, 7.6899}

	day := time.Date(2022, 11, 29, 0, 0, 0, 0, time.UTC)
	var evs []event.Event
	for i, r := range ratings {
		evs = append(evs, rev(int64(i+1), "card:x", day.Format("2006-01-02"), r))
		day = day.AddDate(0, 0, wantIvl[i])
	}
	d := Fold(evs, time.UTC)
	var ivl []int
	var s, df []float64
	for _, r := range d.Reviews {
		from, _ := time.Parse("2006-01-02", r.Date)
		to, _ := time.Parse("2006-01-02", r.Due)
		ivl = append(ivl, int(to.Sub(from).Hours()/24))
		s = append(s, r.Stability)
		df = append(df, r.Difficulty)
	}
	if !reflect.DeepEqual(ivl, wantIvl) {
		t.Errorf("intervals\n got %v\nwant %v", ivl, wantIvl)
	}
	if !reflect.DeepEqual(s, wantS) {
		t.Errorf("stability\n got %v\nwant %v", s, wantS)
	}
	if !reflect.DeepEqual(df, wantD) {
		t.Errorf("difficulty\n got %v\nwant %v", df, wantD)
	}

	// First mature at the third review (stability 34.6 >= 21), and only once.
	var mature []int
	for i, r := range d.Reviews {
		if r.FirstMature {
			mature = append(mature, i)
		}
	}
	if !reflect.DeepEqual(mature, []int{2}) {
		t.Errorf("first mature at %v, want [2]", mature)
	}

	// On its due day a card sits at about the 90% retention FSRS aims for.
	last := d.Reviews[len(d.Reviews)-1]
	c := d.Card("card:x", At(last.Due, time.UTC))
	if math.Abs(c.Retrievability-0.9) > 0.01 || c.State != "review" || c.Reps != 13 || c.Lapses != 2 {
		t.Errorf("card at due: %+v", c)
	}
}

func TestFoldOrdersByDateAndAgreesAcrossPrecision(t *testing.T) {
	la, _ := time.LoadLocation("America/Los_Angeles")
	// A quiz filed late (seq 3) for Oct 30 still folds before the Nov 2 tap.
	late := []event.Event{
		rev(1, "card:a", "2026-10-29", 3),
		rev(2, "card:a", "2026-11-02T21:40:00-08:00", 3),
		rev(3, "card:a", "2026-10-30", 2),
	}
	inOrder := []event.Event{
		rev(1, "card:a", "2026-10-29T08:05:00-07:00", 3),
		rev(2, "card:a", "2026-10-30T23:59:00-07:00", 2),
		rev(3, "card:a", "2026-11-02", 3),
	}
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, la)
	a, b := Fold(late, la).Card("card:a", now), Fold(inOrder, la).Card("card:a", now)
	if a != b {
		t.Errorf("late quiz or day precision changed the schedule:\n%+v\n%+v", a, b)
	}
	if a.LastReview != "2026-11-02" {
		t.Errorf("last review %s", a.LastReview)
	}
}

func TestNewCardsSummaryAndQueue(t *testing.T) {
	loc := time.UTC
	evs := []event.Event{
		rev(1, "card:a", "2026-10-20", 3), // due Oct 23, overdue on Oct 31
		rev(2, "card:b", "2026-10-30", 3), // due Nov 2
		rev(3, "card:c", "2026-10-31", 1), // due Nov 1
		{Kind: Kind, Subject: "card:d", Data: json.RawMessage(`{"rating":9}`), At: "2026-10-31", AtPrecision: event.PrecisionDay},
	}
	d := Fold(evs, loc)
	if got := d.Reviewed(); !reflect.DeepEqual(got, []string{"card:a", "card:b", "card:c"}) {
		t.Errorf("reviewed %v", got)
	}
	now := time.Date(2026, 10, 31, 18, 0, 0, 0, loc)
	var cards []Card
	for _, id := range []string{"card:a", "card:b", "card:c", "card:e"} {
		cards = append(cards, d.Card(id, now))
	}
	if e := cards[3]; e.State != "new" || e.Due != "2026-10-31" || e.Retrievability != 0 {
		t.Errorf("new card %+v", e)
	}
	got := Summarize(cards, d.Reviews, "2026-10-31")
	want := Summary{Total: 4, New: 1, Due: 2, Overdue: 1, ReviewedToday: 1, NextDue: "2026-11-01"}
	if got != want {
		t.Errorf("summary\n got %+v\nwant %+v", got, want)
	}
	var q []string
	for _, c := range Queue(cards, "2026-10-31") {
		q = append(q, c.ID)
	}
	if !reflect.DeepEqual(q, []string{"card:a", "card:e"}) {
		t.Errorf("queue %v", q)
	}
}

func TestParseRating(t *testing.T) {
	for in, want := range map[string]int{"1": 1, "again": 1, "Hard": 2, "3": 3, "good": 3, " easy ": 4} {
		if got, ok := ParseRating(in); !ok || got != want {
			t.Errorf("%q: %d %v", in, got, ok)
		}
	}
	for _, in := range []string{"0", "5", "ok", ""} {
		if _, ok := ParseRating(in); ok {
			t.Errorf("%q: want no rating", in)
		}
	}
}
