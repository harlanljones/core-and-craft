package main

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/harlanljones/core-and-craft/internal/graph"
)

const graphSeed = "../../seed/2026-10-30T235000Z-claude-code-graph.jsonl"

// due lists the cards acta review shows as due, in its order.
func due(t *testing.T, e env) []string {
	t.Helper()
	var ids []string
	for _, l := range strings.Split(runOut(t, e, "review"), "\n") {
		if strings.HasPrefix(l, "card:") {
			ids = append(ids, strings.Fields(l)[0])
		}
	}
	return ids
}

// TestReviewFiveCardsFiveDays is the FSRS proof on the seed: five of its
// recall cards, reviewed from the command line on five days running, each
// with its own run of answers.
func TestReviewFiveCardsFiveDays(t *testing.T) {
	e, out, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	mustRun(t, e, "ingest", graphSeed)

	// Each card's answers, in order, every time it comes due.
	answers := map[string][]string{
		"card:great-schism":       {"good", "good"},
		"card:ninety-five-theses": {"again", "good", "good"},
		"card:hillel":             {"easy"},
		"card:letter-of-james":    {"hard", "hard", "good"},
		"card:abrahamic":          {"good", "again", "good"},
	}
	five := map[string]bool{}
	for id := range answers {
		five[id] = true
	}
	var log []string
	want := [][]string{
		{"card:abrahamic", "card:great-schism", "card:hillel", "card:letter-of-james", "card:ninety-five-theses"}, // Oct 31: all new
		{"card:ninety-five-theses"},                                           // Nov 1: again comes back the next day
		{"card:letter-of-james"},                                              // Nov 2: hard, two days
		{"card:abrahamic", "card:great-schism"},                               // Nov 3: good on a new card, three days
		{"card:abrahamic", "card:letter-of-james", "card:ninety-five-theses"}, // Nov 4: the lapse comes back
	}
	reviews := 0
	for day := 0; day < 5; day++ {
		at := time.Date(2026, 10, 31+day, 21, 0, 0, 0, e.loc)
		e.now = func() time.Time { return at }
		var today []string
		for _, id := range due(t, e) {
			if five[id] {
				today = append(today, id)
			}
		}
		sort.Strings(today)
		if !reflect.DeepEqual(today, want[day]) {
			t.Errorf("%s: due %v, want %v", at.Format("Jan 2"), today, want[day])
		}
		for _, id := range today {
			r := "good"
			if len(answers[id]) > 0 {
				r, answers[id] = answers[id][0], answers[id][1:]
			}
			out.Reset()
			mustRun(t, e, "review", id, "--rating", r)
			log = append(log, at.Format("Jan 2")+"  "+strings.TrimSpace(out.String()))
			reviews++
		}
	}
	t.Logf("five cards, five days:\n%s", strings.Join(log, "\n"))

	mustRun(t, e, "snapshot")
	s := readSnapshot(t, e)
	if s.Cards == nil || s.Cards.Total != 13 || s.Cards.New != 8 || s.Cards.ReviewedToday != 3 || s.Cards.NextDue != "2026-11-07" {
		t.Fatalf("cards %+v", s.Cards)
	}
	// Nodes earn no xp; each review earns 2, none matured yet.
	if s.XP.Total != 2*reviews {
		t.Errorf("xp %d for %d reviews", s.XP.Total, reviews)
	}
	b, _ := os.ReadFile(e.graphPath())
	var g graph.Graph
	json.Unmarshal(b, &g)
	for _, n := range g.Nodes {
		if n.ID == "note:hillel" {
			if n.Card == nil || n.Card.Due != "2026-11-16" || n.Card.State != "review" || n.Card.Retrievability <= 0.9 {
				t.Errorf("hillel's card %+v", n.Card)
			}
		}
		if n.ID == "note:martin-luther" && n.Card != nil {
			t.Error("a note with no recall: line has no card")
		}
	}
}

func TestReviewRefusesUnknownCardsAndBadRatings(t *testing.T) {
	e, _, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	mustRun(t, e, "ingest", graphSeed)
	for _, args := range [][]string{
		{"review", "card:martin-luther", "--rating", "good"}, // a note with no recall:
		{"review", "card:nobody", "--rating", "good"},
		{"review", "card:great-schism", "--rating", "fine"},
	} {
		errb.Reset()
		if code := run(e, args); code == 0 {
			t.Errorf("acta %s: want an error", strings.Join(args, " "))
		}
	}
	mustRun(t, e, "review", "great-schism", "--rating", "3", "--duration-ms", "5200")
	if out := runOut(t, e, "review", "--next"); !strings.Contains(out, "card:613-commandments · faith · due 2026-10-29\nHow many commandments does Judaism count?\nanswer: notes/nodes/613-commandments.md") {
		t.Errorf("--next:\n%s", out)
	}
}

func TestSnapshotFlagsReviewsWithNoCard(t *testing.T) {
	e, _, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	mustRun(t, e, "ingest", graphSeed)
	// A quiz from chat for a card no note names is kept in the log, and listed.
	mustRun(t, e, "log", "review.logged", "--subject", "card:sabbath", "--pillar", "faith", "--agent", "duet:faith", "--via", "chat", "--data", `{"rating":3}`)
	mustRun(t, e, "snapshot")
	b, _ := os.ReadFile(e.graphPath())
	var g graph.Graph
	json.Unmarshal(b, &g)
	found := false
	for _, p := range g.Errors {
		found = found || (p.ID == "card:sabbath" && strings.Contains(p.Msg, "no note with a recall: line"))
	}
	if !found {
		t.Errorf("errors %+v", g.Errors)
	}
	if s := readSnapshot(t, e); s.XP.Total != 0 || s.Graph.Errors != len(g.Errors) {
		t.Errorf("xp %d, graph errors %d of %d", s.XP.Total, s.Graph.Errors, len(g.Errors))
	}
}
