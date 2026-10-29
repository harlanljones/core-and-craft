package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
	"github.com/harlanljones/core-and-craft/internal/snapshot"
	"github.com/harlanljones/core-and-craft/internal/store"
)

const questUsage = `usage: acta quest <command>
  add --pillar P --title T [--priority 1..3] [--every R] [--skill S --rule R] [--agent A] [--via V]
  done | undo | accept | decline | drop <id> [--agent A] [--via V]
  snooze <id> --until YYYY-MM-DD
  edit <id> [--title T] [--priority N] [--every R]
  list [--json] [--all]`

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug makes <words>-<4 hex> from text: readable, and unique enough that
// two quests with the same title added at different moments differ.
func slug(text string, now time.Time) string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(text), "-"), "-")
	if len(s) > 32 {
		s = strings.TrimRight(s[:32], "-")
	}
	if s == "" {
		s = "x"
	}
	sum := sha256.Sum256([]byte(text + "|" + now.UTC().Format(time.RFC3339Nano)))
	return s + "-" + hex.EncodeToString(sum[:])[:4]
}

// liveLog reads the log and returns its effective events in seq order.
func liveLog(e env) ([]event.Event, error) {
	evs, _, err := store.ReadFile(e.logPath())
	if err != nil {
		return nil, err
	}
	return snapshot.Effective(evs), nil
}

// findQuest resolves "quest:<slug>" or a bare "<slug>".
func findQuest(e env, arg string) (snapshot.Quest, []event.Event, error) {
	id := arg
	if !strings.HasPrefix(id, "quest:") {
		id = "quest:" + id
	}
	live, err := liveLog(e)
	if err != nil {
		return snapshot.Quest{}, nil, err
	}
	for _, q := range snapshot.Quests(live, e.loc) {
		if q.ID == id {
			return q, live, nil
		}
	}
	return snapshot.Quest{}, nil, fmt.Errorf("no quest %s", id)
}

func toJSON(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// writerFlags adds --agent and --via, the two flags every write takes.
func writerFlags(fs *flag.FlagSet) (*string, *string) {
	return fs.String("agent", "manual", "writer: manual, claude-code, hermes or duet:<channel>"),
		fs.String("via", "manual", "chat, manual, board, ...")
}

func cmdQuest(e env, args []string) error {
	if len(args) == 0 {
		return errors.New(questUsage)
	}
	switch args[0] {
	case "add":
		return questAdd(e, args[1:])
	case "list":
		return questList(e, args[1:])
	case "done", "undo", "accept", "decline", "drop", "snooze", "edit":
		if len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return fmt.Errorf("usage: acta quest %s <id>", args[0])
		}
		return questChange(e, args[0], args[1], args[2:])
	}
	return errors.New(questUsage)
}

func questAdd(e env, args []string) error {
	fs := flag.NewFlagSet("quest add", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	pillar := fs.String("pillar", "", "CORE & CRAFT pillar, full word")
	title := fs.String("title", "", "one concrete next action")
	priority := fs.Int("priority", 2, "1 (high) to 3 (low)")
	every := fs.String("every", "once", "once, daily, weekdays, weekly:mon,thu, every:Nd, monthly:D")
	skill := fs.String("skill", "", "skill:<slug> whose evidence can finish it")
	rule := fs.String("rule", "", "runs:N or minutes:N, with --skill")
	agent, via := writerFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pillar == "" || *title == "" {
		return errors.New("quest add needs --pillar and --title")
	}
	data := map[string]any{"title": *title, "priority": *priority, "every": *every}
	if *skill != "" {
		data["skill"] = *skill
	}
	if *rule != "" {
		data["rule"] = *rule
	}
	status := "active"
	if *agent != "manual" {
		status = "proposed"
	}
	data["status"] = status
	id := "quest:" + slug(*title, e.now())
	ev, err := newEvent(e, "quest.added", id, *pillar, toJSON(data), "", *via, *agent, "", "", "")
	if err != nil {
		return err
	}
	if _, err := store.Append(e.logPath(), []event.Event{ev}); err != nil {
		return err
	}
	fmt.Fprintf(e.stdout, "%s added (%s)\n", id, status)
	return nil
}

func questChange(e env, verb, arg string, args []string) error {
	fs := flag.NewFlagSet("quest "+verb, flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	agent, via := writerFlags(fs)
	until := fs.String("until", "", "snooze: YYYY-MM-DD")
	title := fs.String("title", "", "edit: new title")
	priority := fs.Int("priority", 0, "edit: 1 to 3")
	every := fs.String("every", "", "edit: new recurrence")
	if err := fs.Parse(args); err != nil {
		return err
	}
	q, live, err := findQuest(e, arg)
	if err != nil {
		return err
	}
	open := q.Status == "active"
	kind, data, subject := "quest."+verb, map[string]any{}, q.ID
	switch verb {
	case "done":
		if !open {
			return fmt.Errorf("%s is %s, not active", q.ID, q.Status)
		}
	case "accept", "decline":
		if q.Status != "proposed" {
			return fmt.Errorf("%s is %s, not proposed", q.ID, q.Status)
		}
		// Approval is Harlan's: an agent can't accept or decline a proposal.
		if *agent != "manual" {
			return fmt.Errorf("only manual can %s a proposed quest", verb)
		}
		kind += "ed"
	case "drop":
		if q.Status == "dropped" {
			return fmt.Errorf("%s is already dropped", q.ID)
		}
		kind = "quest.dropped"
	case "snooze":
		kind = "quest.snoozed"
		data["until"] = *until
	case "edit":
		kind = "quest.edited"
		if *title != "" {
			data["title"] = *title
		}
		if *priority != 0 {
			data["priority"] = *priority
		}
		if *every != "" {
			data["every"] = *every
		}
	case "undo":
		// Retract the latest live done for this quest.
		var last *event.Event
		for i := range live {
			if live[i].Kind == "quest.done" && live[i].Subject == q.ID {
				last = &live[i]
			}
		}
		if last == nil {
			return fmt.Errorf("%s has nothing done to undo", q.ID)
		}
		kind, subject, data = "event.retracted", last.ID, map[string]any{"reason": "undo"}
	}
	ev, err := newEvent(e, kind, subject, q.Pillar, toJSON(data), "", *via, *agent, "", "", "")
	if err != nil {
		return err
	}
	return appendOne(e, ev)
}

func questList(e env, args []string) error {
	fs := flag.NewFlagSet("quest list", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	asJSON := fs.Bool("json", false, "print JSON")
	all := fs.Bool("all", false, "include done, declined and dropped quests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	live, err := liveLog(e)
	if err != nil {
		return err
	}
	qs := []snapshot.Quest{}
	for _, q := range snapshot.Quests(live, e.loc) {
		if *all || q.Status == "active" || q.Status == "proposed" {
			qs = append(qs, q)
		}
	}
	if *asJSON {
		return json.NewEncoder(e.stdout).Encode(qs)
	}
	for _, q := range qs {
		fmt.Fprintf(e.stdout, "%-40s %-13s p%d %-14s %-8s %s\n", q.ID, q.Pillar, q.Priority, q.Every, q.Status, q.Title)
	}
	return nil
}

func cmdCheckin(e env, args []string) error {
	const usage = "usage: acta checkin ask --pillar P --question Q [--agent A] | answer <id> --text T"
	if len(args) == 0 {
		return errors.New(usage)
	}
	fs := flag.NewFlagSet("checkin", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	agent, via := writerFlags(fs)
	switch args[0] {
	case "ask":
		pillar := fs.String("pillar", "", "pillar the question belongs to")
		question := fs.String("question", "", "one open question")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *pillar == "" {
			return errors.New("checkin ask needs --pillar")
		}
		id := "checkin:" + slug(*question, e.now())
		ev, err := newEvent(e, "checkin.asked", id, *pillar, toJSON(map[string]any{"question": *question}), "", *via, *agent, "", "", "")
		if err != nil {
			return err
		}
		if _, err := store.Append(e.logPath(), []event.Event{ev}); err != nil {
			return err
		}
		fmt.Fprintf(e.stdout, "%s asked\n", id)
		return nil
	case "answer":
		if len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return errors.New(usage)
		}
		id := args[1]
		if !strings.HasPrefix(id, "checkin:") {
			id = "checkin:" + id
		}
		text := fs.String("text", "", "the answer")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		live, err := liveLog(e)
		if err != nil {
			return err
		}
		pillar := ""
		for _, ev := range live {
			if ev.Kind == "checkin.asked" && ev.Subject == id {
				pillar = ev.Pillar
			}
		}
		if pillar == "" {
			return fmt.Errorf("no check-in %s", id)
		}
		ev, err := newEvent(e, "checkin.answered", id, pillar, toJSON(map[string]any{"text": *text}), "", *via, *agent, "", "", "")
		if err != nil {
			return err
		}
		return appendOne(e, ev)
	}
	return errors.New(usage)
}

// cmdRest logs a rest day. It's filed under recovery, and a day is rested
// at most once.
func cmdRest(e env, args []string) error {
	fs := flag.NewFlagSet("rest", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	at := fs.String("at", "", "YYYY-MM-DD (default today)")
	agent, via := writerFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	day := *at
	if day == "" {
		day = e.now().In(e.loc).Format("2006-01-02")
	}
	live, err := liveLog(e)
	if err != nil {
		return err
	}
	subject := "day:" + day
	for _, ev := range live {
		if ev.Kind == "day.rested" && ev.Subject == subject {
			fmt.Fprintf(e.stdout, "%s already rested\n", day)
			return nil
		}
	}
	ev, err := newEvent(e, "day.rested", subject, "recovery", "{}", day, *via, *agent, "", "", "")
	if err != nil {
		return err
	}
	return appendOne(e, ev)
}
