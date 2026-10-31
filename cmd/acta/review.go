package main

import (
	"errors"
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
	"github.com/harlanljones/core-and-craft/internal/graph"
	"github.com/harlanljones/core-and-craft/internal/review"
	"github.com/harlanljones/core-and-craft/internal/snapshot"
	"github.com/harlanljones/core-and-craft/internal/store"
)

// deck schedules a card for every note node with a recall: line, fills in
// each node's card, and lists reviews of cards no such note names as
// problems. It returns the cards in id order and the folded deck.
func deck(g *graph.Graph, evs []event.Event, now time.Time, loc *time.Location) ([]review.Card, *review.Deck) {
	d := review.Fold(snapshot.Effective(evs), loc)
	named := map[string]bool{}
	var cards []review.Card
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind != "note" || n.Stub || n.Recall == "" {
			continue
		}
		id := review.Prefix + strings.TrimPrefix(n.ID, "note:")
		c := d.Card(id, now)
		n.Card = &c
		named[id] = true
		cards = append(cards, c)
	}
	for _, id := range d.Reviewed() {
		if !named[id] {
			g.Errors = append(g.Errors, graph.Problem{ID: id, Msg: fmt.Sprintf("%s has reviews, but no note with a recall: line names it", id)})
		}
	}
	sort.Slice(g.Errors, func(i, j int) bool { return g.Errors[i].ID < g.Errors[j].ID })
	sort.Slice(cards, func(i, j int) bool { return cards[i].ID < cards[j].ID })
	return cards, d
}

// recallNodes builds the graph from the log and the note files, without
// writing it, for acta review.
func recallNodes(e env, evs []event.Event) (graph.Graph, []review.Card, *review.Deck) {
	now := e.now()
	g := graph.Build(graphInput(e, evs, snapshot.Snapshot{}), now.UTC().Format(time.RFC3339))
	cards, d := deck(&g, evs, now, e.loc)
	return g, cards, d
}

const reviewUsage = `usage: acta review [--next] | acta review <card> --rating again|hard|good|easy [--duration-ms MS] [--at T] [--pillar P] [--agent A] [--via V]`

// cmdReview lists the due cards, shows the next one, or logs a review.
func cmdReview(e env, args []string) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return reviewLog(e, args[0], args[1:])
	}
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	next := fs.Bool("next", false, "show only the next due card and its question")
	if err := fs.Parse(args); err != nil {
		return errors.New(reviewUsage)
	}
	evs, _, err := store.ReadFile(e.logPath())
	if err != nil {
		return err
	}
	g, cards, d := recallNodes(e, evs)
	today := e.now().In(e.loc).Format("2006-01-02")
	sum := review.Summarize(cards, d.Reviews, today)
	queue := review.Queue(cards, today)
	nodes := map[string]graph.Node{}
	for _, n := range g.Nodes {
		nodes[n.ID] = n
	}
	question := func(c review.Card) graph.Node { return nodes["note:"+review.Slug(c.ID)] }
	if len(cards) == 0 {
		fmt.Fprintln(e.stdout, "no cards: add a recall: line to a note")
		return nil
	}
	if len(queue) == 0 {
		fmt.Fprintf(e.stdout, "nothing due · %d card(s) · next due %s\n", sum.Total, sum.NextDue)
		return nil
	}
	if *next {
		c := queue[0]
		n := question(c)
		fmt.Fprintf(e.stdout, "%s · %s · due %s\n%s\n", c.ID, n.Pillar, c.Due, n.Recall)
		if n.Path != "" {
			fmt.Fprintf(e.stdout, "answer: %s\n", n.Path)
		}
		fmt.Fprintf(e.stdout, "rate: acta review %s --rating again|hard|good|easy\n", c.ID)
		return nil
	}
	fmt.Fprintf(e.stdout, "%d due · %d overdue · %d new · %d card(s)\n", sum.Due, sum.Overdue, sum.New, sum.Total)
	for _, c := range queue {
		r := "    -"
		if c.State != "new" {
			r = fmt.Sprintf("%5.2f", c.Retrievability)
		}
		fmt.Fprintf(e.stdout, "%-36s %-10s due %s  R %s  %s\n", c.ID, c.State, c.Due, r, question(c).Recall)
	}
	return nil
}

// reviewLog appends one review.logged for a card a note's recall: names.
func reviewLog(e env, arg string, args []string) error {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	rating := fs.String("rating", "", "again, hard, good or easy (1 to 4)")
	duration := fs.Int64("duration-ms", -1, "time to answer, in milliseconds")
	at := fs.String("at", "", "RFC 3339 time or YYYY-MM-DD (default now)")
	pillar := fs.String("pillar", "", "pillar (default: the note's first pillar)")
	agent, via := writerFlags(fs)
	if err := fs.Parse(args); err != nil {
		return errors.New(reviewUsage)
	}
	r, ok := review.ParseRating(*rating)
	if !ok {
		return fmt.Errorf("--rating: want again, hard, good or easy (1 to 4), got %q", *rating)
	}
	slug := strings.TrimPrefix(strings.TrimPrefix(arg, "note:"), review.Prefix)
	id := review.Prefix + slug
	evs, _, err := store.ReadFile(e.logPath())
	if err != nil {
		return err
	}
	g, _, _ := recallNodes(e, evs)
	var node *graph.Node
	for i := range g.Nodes {
		if g.Nodes[i].ID == "note:"+slug && g.Nodes[i].Recall != "" {
			node = &g.Nodes[i]
		}
	}
	if node == nil {
		return fmt.Errorf("no card %s: no note in the log has a recall: line for note:%s", id, slug)
	}
	p := *pillar
	if p == "" {
		p = node.Pillar
	}
	data := map[string]any{"rating": r}
	if *duration >= 0 {
		data["duration_ms"] = *duration
	}
	ev, err := newEvent(e, review.Kind, id, p, toJSON(data), *at, *via, *agent, "", "", "")
	if err != nil {
		return err
	}
	res, err := store.Append(e.logPath(), []event.Event{ev})
	if err != nil {
		return err
	}
	if len(res.Appended) == 0 {
		fmt.Fprintf(e.stdout, "%s already logged\n", ev.ID)
		return nil
	}
	evs = append(evs, res.Appended[0])
	c := review.Fold(snapshot.Effective(evs), e.loc).Card(id, e.now())
	fmt.Fprintf(e.stdout, "%s %s · due %s · stability %.1fd\n", id, review.Ratings[r-1], c.Due, c.Stability)
	return nil
}
