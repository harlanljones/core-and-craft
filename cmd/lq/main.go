// Command lq is the only writer of the LifeQuest event log and the
// builder of the snapshot the widgets read.
//
//	lq init                     create the data and state directories
//	lq mt pull [--all]          archive new Monkeytype results
//	lq log <kind> [flags]       append one event
//	lq retract <id> [flags]     append event.retracted for an event
//	lq ingest [file...]         append inbox files written by remote agents
//	lq snapshot                 rebuild snapshot.json from the log
//	lq status [--json]          one line for a bar widget
//	lq doctor                   check the log's invariants
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
	"github.com/harlanljones/core-and-craft/internal/monkeytype"
	"github.com/harlanljones/core-and-craft/internal/snapshot"
	"github.com/harlanljones/core-and-craft/internal/store"
)

var version = "dev"

type env struct {
	data, state string
	loc         *time.Location
	now         func() time.Time
	stdout      io.Writer
	stderr      io.Writer
	getenv      func(string) string
	mtBaseURL   string
}

func (e env) logPath() string      { return filepath.Join(e.data, store.FileName) }
func (e env) inboxDir() string     { return filepath.Join(e.data, "inbox") }
func (e env) snapshotPath() string { return filepath.Join(e.state, snapshot.FileName) }

func defaultEnv() (env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return env{}, err
	}
	data := os.Getenv("LQ_DATA")
	if data == "" {
		data = filepath.Join(home, "life")
	}
	state := os.Getenv("LQ_STATE")
	if state == "" {
		base := os.Getenv("XDG_STATE_HOME")
		if base == "" {
			base = filepath.Join(home, ".local", "state")
		}
		state = filepath.Join(base, "lifequest")
	}
	loc := time.Local
	if tz := os.Getenv("LQ_TZ"); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	return env{data: data, state: state, loc: loc, now: time.Now, stdout: os.Stdout, stderr: os.Stderr, getenv: os.Getenv}, nil
}

func main() {
	e, err := defaultEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "lq:", err)
		os.Exit(1)
	}
	os.Exit(run(e, os.Args[1:]))
}

const usage = `usage: lq <command>
  init | mt pull [--all] | log <kind> | retract <id> | ingest [file...] | snapshot | status [--json] | doctor | version`

func run(e env, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(e.stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "init":
		err = cmdInit(e)
	case "mt":
		if len(args) < 2 || args[1] != "pull" {
			fmt.Fprintln(e.stderr, "usage: lq mt pull [--all]")
			return 2
		}
		err = cmdMtPull(e, args[2:])
	case "log":
		err = cmdLog(e, args[1:])
	case "retract":
		err = cmdRetract(e, args[1:])
	case "ingest":
		err = cmdIngest(e, args[1:])
	case "snapshot":
		err = cmdSnapshot(e)
	case "status":
		err = cmdStatus(e, args[1:])
	case "doctor":
		err = cmdDoctor(e)
	case "version":
		fmt.Fprintln(e.stdout, "lq", version)
	case "-h", "--help", "help":
		fmt.Fprintln(e.stdout, usage)
	default:
		fmt.Fprintf(e.stderr, "lq: unknown command %q\n%s\n", args[0], usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(e.stderr, "lq:", err)
		return 1
	}
	return 0
}

func cmdInit(e env) error {
	for _, d := range []string{e.data, e.inboxDir(), e.state} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(e.logPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	f.Close()
	fmt.Fprintf(e.stdout, "data  %s\nstate %s\n", e.data, e.state)
	return nil
}

// apeKey reads the env var first, then a systemd credential.
func apeKey(e env) string {
	if k := strings.TrimSpace(e.getenv("MONKEYTYPE_APE_KEY")); k != "" {
		return k
	}
	if dir := e.getenv("CREDENTIALS_DIRECTORY"); dir != "" {
		if b, err := os.ReadFile(filepath.Join(dir, "monkeytype")); err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return ""
}

func cmdMtPull(e env, args []string) error {
	fs := flag.NewFlagSet("mt pull", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	all := fs.Bool("all", false, "ignore the last archived result and fetch everything the account exposes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	key := apeKey(e)
	if key == "" {
		return errors.New("no Monkeytype key: set MONKEYTYPE_APE_KEY or a systemd credential named monkeytype")
	}
	evs, _, err := store.ReadFile(e.logPath())
	if err != nil {
		return err
	}
	var since time.Time
	if !*all {
		for _, ev := range evs {
			if !strings.HasPrefix(ev.ID, "mt:") {
				continue
			}
			if t, err := ev.Time(e.loc); err == nil && t.After(since) {
				since = t
			}
		}
		if !since.IsZero() {
			// One day of overlap; dedupe by id makes it free.
			since = since.Add(-24 * time.Hour)
		}
	}
	c := monkeytype.Client{BaseURL: e.mtBaseURL, Key: key}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	results, err := c.Results(ctx, since)
	if err != nil {
		return err
	}
	now := e.now()
	var out []event.Event
	for _, r := range results {
		ev, err := monkeytype.ToEvent(r, e.loc, now)
		if err != nil {
			return err
		}
		out = append(out, ev)
	}
	res, err := store.Append(e.logPath(), out)
	if err != nil {
		return err
	}
	fmt.Fprintf(e.stdout, "mt pull: %d new, %d already logged\n", len(res.Appended), res.Duplicates)
	return nil
}

func cmdLog(e env, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: lq log <kind> --subject S --agent A [--pillar P] [--data JSON] [--at T] [--via V] [--note N] [--ref R] [--supersedes ID]")
	}
	kind := args[0]
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	subject := fs.String("subject", "", "skill:<slug>, note:<slug> or card:<slug>")
	pillar := fs.String("pillar", "competence", "CORE & CRAFT pillar, full word")
	data := fs.String("data", "{}", "kind-specific JSON object")
	at := fs.String("at", "", "RFC 3339 time or YYYY-MM-DD (default now)")
	via := fs.String("via", "manual", "chat, manual, keybr, ...")
	agent := fs.String("agent", "", "writer: lq, claude-code, hermes, manual or duet:<channel>")
	note := fs.String("note", "", "free text")
	ref := fs.String("ref", "", "source reference")
	supersedes := fs.String("supersedes", "", "id of the event this corrects")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	ev, err := newEvent(e, kind, *subject, *pillar, *data, *at, *via, *agent, *note, *ref, *supersedes)
	if err != nil {
		return err
	}
	return appendOne(e, ev)
}

func cmdRetract(e env, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: lq retract <id> --agent A [--reason R] [--pillar P]")
	}
	target := args[0]
	fs := flag.NewFlagSet("retract", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	reason := fs.String("reason", "", "why")
	agent := fs.String("agent", "", "writer")
	pillar := fs.String("pillar", "", "pillar (default: the target's)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	evs, _, err := store.ReadFile(e.logPath())
	if err != nil {
		return err
	}
	var found *event.Event
	for i := range evs {
		if evs[i].ID == target {
			found = &evs[i]
		}
	}
	if found == nil {
		return fmt.Errorf("no event %s in the log", target)
	}
	p := *pillar
	if p == "" {
		p = found.Pillar
	}
	data, _ := json.Marshal(map[string]string{"reason": *reason})
	ev, err := newEvent(e, "event.retracted", target, p, string(data), "", "manual", *agent, "", "", "")
	if err != nil {
		return err
	}
	return appendOne(e, ev)
}

func newEvent(e env, kind, subject, pillar, data, at, via, agent, note, ref, supersedes string) (event.Event, error) {
	now := e.now()
	prec := event.PrecisionSecond
	switch {
	case at == "":
		at = now.In(e.loc).Format(time.RFC3339)
	case len(at) == len("2006-01-02"):
		prec = event.PrecisionDay
	}
	ev := event.Event{
		V: event.Version, At: at, AtPrecision: prec,
		RecordedAt: now.UTC().Format(time.RFC3339Nano),
		Pillar:     pillar, Kind: kind, Subject: subject,
		Source: event.Source{Via: via, Agent: agent, Note: note, Ref: ref},
		Data:   json.RawMessage(data), Supersedes: supersedes,
	}
	id, err := event.NativeID(ev)
	if err != nil {
		return ev, fmt.Errorf("data: %w", err)
	}
	ev.ID = id
	if err := event.Validate(ev, false); err != nil {
		return ev, err
	}
	return ev, nil
}

func appendOne(e env, ev event.Event) error {
	res, err := store.Append(e.logPath(), []event.Event{ev})
	if err != nil {
		return err
	}
	if len(res.Appended) == 0 {
		fmt.Fprintf(e.stdout, "%s already logged\n", ev.ID)
		return nil
	}
	fmt.Fprintf(e.stdout, "%s seq %d\n", res.Appended[0].ID, res.Appended[0].Seq)
	return nil
}

// readInbox parses one inbox file. Lines may omit id (lq computes it) and
// must omit or ignore seq (lq assigns it). Any bad line rejects the file.
func readInbox(path string) ([]event.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	var evs []event.Event
	var errs []error
	n := 0
	for sc.Scan() {
		n++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev event.Event
		if err := json.Unmarshal(line, &ev); err != nil {
			errs = append(errs, fmt.Errorf("line %d: %w", n, err))
			continue
		}
		ev.Seq = 0
		if ev.ID == "" {
			id, err := event.NativeID(ev)
			if err != nil {
				errs = append(errs, fmt.Errorf("line %d: %w", n, err))
				continue
			}
			ev.ID = id
		}
		if err := event.Validate(ev, false); err != nil {
			errs = append(errs, fmt.Errorf("line %d: %w", n, err))
			continue
		}
		evs = append(evs, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return evs, nil
}

func cmdIngest(e env, args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	keep := fs.Bool("keep", false, "leave inbox files in place after ingesting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	files := fs.Args()
	fromInbox := len(files) == 0
	if fromInbox {
		matches, err := filepath.Glob(filepath.Join(e.inboxDir(), "*.jsonl"))
		if err != nil {
			return err
		}
		sort.Strings(matches)
		files = matches
	}
	var failed []string
	total, dups := 0, 0
	for _, p := range files {
		evs, err := readInbox(p)
		if err != nil {
			fmt.Fprintf(e.stderr, "lq: %s rejected:\n%v\n", filepath.Base(p), err)
			failed = append(failed, filepath.Base(p))
			continue
		}
		res, err := store.Append(e.logPath(), evs)
		if err != nil {
			return err
		}
		total += len(res.Appended)
		dups += res.Duplicates
		if fromInbox && !*keep {
			if err := os.Remove(p); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(e.stdout, "ingest: %d file(s), %d new, %d already logged\n", len(files)-len(failed), total, dups)
	if len(failed) > 0 {
		return fmt.Errorf("%d file(s) rejected and left in place: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

func cmdSnapshot(e env) error {
	evs, bad, err := store.ReadFile(e.logPath())
	if err != nil {
		return err
	}
	src, err := snapshot.SourceOf(e.logPath())
	if err != nil {
		return err
	}
	s := snapshot.Build(evs, bad, src, e.now(), e.loc)
	if err := snapshot.Write(e.snapshotPath(), s); err != nil {
		return err
	}
	fmt.Fprintf(e.stdout, "snapshot: %d events, %d skill(s), %d error(s) -> %s\n", len(evs), len(s.Skills), len(bad), e.snapshotPath())
	return nil
}

func cmdStatus(e env, args []string) error {
	b, err := os.ReadFile(e.snapshotPath())
	if err != nil {
		return fmt.Errorf("no snapshot yet: run lq snapshot (%w)", err)
	}
	if len(args) > 0 && args[0] == "--json" {
		_, err := e.stdout.Write(b)
		return err
	}
	var s snapshot.Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	fmt.Fprintln(e.stdout, statusLine(s))
	return nil
}

func fmtNum(p *float64) string {
	if p == nil {
		return "-"
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", *p), ".0")
}

func statusLine(s snapshot.Snapshot) string {
	for _, sk := range s.Skills {
		if sk.ID != "skill:typing" {
			continue
		}
		line := "typing " + fmtNum(sk.Best)
		if sk.Target != nil {
			line += "/" + fmtNum(sk.Target)
		}
		line += " wpm"
		if sk.Passed {
			line += " passed"
		}
		return fmt.Sprintf("%s · %d today", line, s.Today.TypingTests)
	}
	return fmt.Sprintf("%d events today", s.Today.Events)
}

func cmdDoctor(e env) error {
	f, err := os.Open(e.logPath())
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(e.stdout, "doctor: no log yet")
		return nil
	}
	if err != nil {
		return err
	}
	evs, bad, err := store.Scan(f)
	f.Close()
	if err != nil {
		return err
	}
	var problems []string
	for _, b := range bad {
		problems = append(problems, fmt.Sprintf("line %d: %s", b.Line, b.Err))
	}
	ids := map[string]int64{}
	var prev int64
	limit := e.now().Add(5 * time.Minute)
	for _, ev := range evs {
		if s, ok := ids[ev.ID]; ok {
			problems = append(problems, fmt.Sprintf("duplicate id %s (seq %d and %d)", ev.ID, s, ev.Seq))
		}
		ids[ev.ID] = ev.Seq
		if ev.Seq != prev+1 {
			problems = append(problems, fmt.Sprintf("seq %d follows %d", ev.Seq, prev))
		}
		prev = ev.Seq
		if t, err := ev.Time(e.loc); err == nil && t.After(limit) {
			problems = append(problems, fmt.Sprintf("%s is in the future (%s)", ev.ID, ev.At))
		}
	}
	for _, ev := range evs {
		if ev.Supersedes != "" {
			if _, ok := ids[ev.Supersedes]; !ok {
				problems = append(problems, fmt.Sprintf("%s supersedes missing %s", ev.ID, ev.Supersedes))
			}
		}
		if ev.Kind == "event.retracted" {
			if _, ok := ids[ev.Subject]; !ok {
				problems = append(problems, fmt.Sprintf("%s retracts missing %s", ev.ID, ev.Subject))
			}
		}
	}
	if len(problems) == 0 {
		fmt.Fprintf(e.stdout, "doctor: %d events, ok\n", len(evs))
		return nil
	}
	for _, p := range problems {
		fmt.Fprintln(e.stdout, p)
	}
	return fmt.Errorf("doctor: %d problem(s)", len(problems))
}
