// Command acta is the only writer of the CORE & CRAFT event log and the
// builder of the snapshot the widgets read.
//
//	acta init                   create the data and state directories
//	acta mt pull [--all]        archive new Monkeytype results
//	acta log <kind> [flags]     append one event
//	acta retract <id> [flags]   append event.retracted for an event
//	acta ingest [file...]       append inbox files written by remote agents
//	acta note add <file>...     log notes written or edited on this machine
//	acta review [--next]        list the due recall cards, or show the next one
//	acta review <card> --rating R  log one review (again, hard, good, easy)
//	acta snapshot               rebuild snapshot.json and graph.json from the log
//	acta status [--json]        one line for a bar widget
//	acta doctor                 check the log's invariants and the inbox route
//	acta quest <command>        add, finish, undo, accept, decline, drop, snooze, edit, list
//	acta checkin ask|answer     a pillar question and its answer
//	acta rest [--at D]          a rest day: keeps the streak
//	acta ask --pillar P --text T  a question for a pillar's Duet agent
//	acta relay [--dry-run]      send the board's answers and asks to the Duet agents
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
	"github.com/harlanljones/core-and-craft/internal/relay"
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
	// command runs an external program and returns its stdout (doctor's
	// rclone and systemctl checks). nil means no external programs.
	command func(name string, args ...string) (string, error)
	// relayBaseURL stands in for https://api.anthropic.com in tests.
	relayBaseURL string
}

func (e env) logPath() string      { return filepath.Join(e.data, store.FileName) }
func (e env) inboxDir() string     { return filepath.Join(e.data, "inbox") }
func (e env) rejectedDir() string  { return filepath.Join(e.inboxDir(), "rejected") }
func (e env) snapshotPath() string { return filepath.Join(e.state, snapshot.FileName) }
func (e env) relayPath() string    { return filepath.Join(e.state, relay.FileName) }

func defaultEnv() (env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return env{}, err
	}
	data := os.Getenv("ACTA_DATA")
	if data == "" {
		data = filepath.Join(home, "core-and-craft")
	}
	state := os.Getenv("ACTA_STATE")
	if state == "" {
		base := os.Getenv("XDG_STATE_HOME")
		if base == "" {
			base = filepath.Join(home, ".local", "state")
		}
		state = filepath.Join(base, "core-and-craft")
	}
	loc := time.Local
	if tz := os.Getenv("ACTA_TZ"); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	return env{data: data, state: state, loc: loc, now: time.Now, stdout: os.Stdout, stderr: os.Stderr, getenv: os.Getenv, command: runCommand}, nil
}

func main() {
	e, err := defaultEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "acta:", err)
		os.Exit(1)
	}
	os.Exit(run(e, os.Args[1:]))
}

const usage = `usage: acta <command>
  init | mt pull [--all] | log <kind> | retract <id> | ingest [file...] | inbox pull [--dry-run] | note add <file>... | review [--next] | review <card> --rating R | snapshot | status [--json] | notify [--all] | doctor | version
  quest add|done|undo|accept|decline|drop|snooze|edit|list | checkin ask|answer | rest
  ask --pillar P --text T | relay [--dry-run]`

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
			fmt.Fprintln(e.stderr, "usage: acta mt pull [--all]")
			return 2
		}
		err = cmdMtPull(e, args[2:])
	case "log":
		err = cmdLog(e, args[1:])
	case "retract":
		err = cmdRetract(e, args[1:])
	case "quest":
		err = cmdQuest(e, args[1:])
	case "checkin":
		err = cmdCheckin(e, args[1:])
	case "rest":
		err = cmdRest(e, args[1:])
	case "ask":
		err = cmdAsk(e, args[1:])
	case "relay":
		err = cmdRelay(e, args[1:])
	case "ingest":
		err = cmdIngest(e, args[1:])
	case "inbox":
		if len(args) < 2 || args[1] != "pull" {
			fmt.Fprintln(e.stderr, "usage: acta inbox pull [--dry-run]")
			return 2
		}
		err = cmdInboxPull(e, args[2:])
	case "note":
		err = cmdNote(e, args[1:])
	case "review":
		err = cmdReview(e, args[1:])
	case "snapshot":
		err = cmdSnapshot(e)
	case "status":
		err = cmdStatus(e, args[1:])
	case "notify":
		err = cmdNotify(e, args[1:])
	case "doctor":
		err = cmdDoctor(e)
	case "version":
		fmt.Fprintln(e.stdout, "acta", version)
	case "-h", "--help", "help":
		fmt.Fprintln(e.stdout, usage)
	default:
		fmt.Fprintf(e.stderr, "acta: unknown command %q\n%s\n", args[0], usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(e.stderr, "acta:", err)
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
		return errors.New("usage: acta log <kind> --subject S --agent A [--pillar P] [--data JSON] [--at T] [--via V] [--note N] [--ref R] [--supersedes ID]")
	}
	kind := args[0]
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	subject := fs.String("subject", "", "skill:<slug>, note:<slug> or card:<slug>")
	pillar := fs.String("pillar", "competence", "CORE & CRAFT pillar, full word")
	data := fs.String("data", "{}", "kind-specific JSON object")
	at := fs.String("at", "", "RFC 3339 time or YYYY-MM-DD (default now)")
	via := fs.String("via", "manual", "chat, manual, keybr, ...")
	agent := fs.String("agent", "", "writer: acta, claude-code, hermes, manual or duet:<channel>")
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
		return errors.New("usage: acta retract <id> --agent A [--reason R] [--pillar P]")
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

// readInbox parses one inbox file. Lines may omit id (acta computes it) and
// must omit or ignore seq (acta assigns it). Any bad line rejects the file;
// cmdIngest then moves it to inbox/rejected/ (see quarantine). remote is true
// for files found in inbox/: those come from remote agents, so a line that
// claims a local writer (manual, acta, acta-mt) is rejected. A file named on
// the command line is Harlan's own and may hold any writer.
func readInbox(path string, remote bool) ([]event.Event, error) {
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
		if remote && event.LocalAgent(ev.Source.Agent) {
			errs = append(errs, fmt.Errorf("line %d: source.agent %q writes only on this machine, never through the inbox", n, ev.Source.Agent))
			continue
		}
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
	logged, _, err := store.ReadFile(e.logPath())
	if err != nil {
		return err
	}
	var failed, moved []string
	total, dups, waits := 0, 0, 0
	for _, p := range files {
		evs, err := readInbox(p, fromInbox)
		var plans []placement
		if err == nil {
			var waiting int
			var errs []error
			plans, waiting, errs = planNotes(e, filepath.Dir(p), evs, logged, fromInbox)
			if len(errs) > 0 {
				err = errors.Join(errs...)
			} else if fromInbox && waiting > 0 {
				// Drive may hand over the event file before its bodies; the
				// next run tries again (doctor warns after an hour).
				fmt.Fprintf(e.stderr, "acta: %s waits for %d note bod(ies)\n", filepath.Base(p), waiting)
				waits++
				continue
			}
		}
		if err != nil {
			msg := fmt.Sprintf("%s rejected:\n%v\n", filepath.Base(p), err)
			fmt.Fprintf(e.stderr, "acta: %s", msg)
			failed = append(failed, filepath.Base(p))
			if fromInbox && !*keep {
				dst, qerr := quarantine(e.rejectedDir(), p, msg)
				if qerr != nil {
					return qerr
				}
				moved = append(moved, filepath.Base(dst))
				fmt.Fprintf(e.stderr, "acta: moved %s to inbox/rejected/%s\n", filepath.Base(p), filepath.Base(dst))
			}
			continue
		}
		if err := placeNotes(plans); err != nil {
			return err
		}
		res, err := store.Append(e.logPath(), evs)
		if err != nil {
			return err
		}
		logged = append(logged, res.Appended...)
		total += len(res.Appended)
		dups += res.Duplicates
		if fromInbox && !*keep {
			if err := os.Remove(p); err != nil {
				return err
			}
			for _, pl := range plans {
				if pl.from != "" {
					os.Remove(pl.from)
				}
			}
		}
	}
	notes := ""
	if waits > 0 {
		notes = fmt.Sprintf(", %d waiting for note bodies", waits)
	}
	fmt.Fprintf(e.stdout, "ingest: %d file(s), %d new, %d already logged%s\n", len(files)-len(failed)-waits, total, dups, notes)
	if len(failed) > 0 {
		where := "left in place"
		if len(moved) > 0 {
			where = "moved to inbox/rejected"
		}
		return fmt.Errorf("%d file(s) rejected and %s: %s", len(failed), where, strings.Join(failed, ", "))
	}
	return nil
}

// quarantine moves a rejected inbox file into dir and writes the rejection
// text next to it as <name>.err. It never overwrites: if the name is taken it
// tries <base>-1<ext>, <base>-2<ext>, ... The file is hard-linked into place
// (link fails if the target exists) and only then removed from the inbox, so
// it is never lost.
func quarantine(dir, src, msg string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := filepath.Base(src)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for n := 0; ; n++ {
		cand := name
		if n > 0 {
			cand = fmt.Sprintf("%s-%d%s", base, n, ext)
		}
		dst := filepath.Join(dir, cand)
		if _, err := os.Lstat(dst + ".err"); err == nil {
			continue
		}
		if err := os.Link(src, dst); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", err
		}
		f, err := os.OpenFile(dst+".err", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return "", err
		}
		_, werr := io.WriteString(f, msg)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return "", werr
		}
		if err := os.Remove(src); err != nil {
			return "", err
		}
		return dst, nil
	}
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
	if s.Graph, s.Cards, err = buildGraph(e, evs, s); err != nil {
		return err
	}
	if err := snapshot.Write(e.snapshotPath(), s); err != nil {
		return err
	}
	fmt.Fprintf(e.stdout, "snapshot: %d events, %d skill(s), %d error(s) -> %s\n", len(evs), len(s.Skills), len(bad), e.snapshotPath())
	fmt.Fprintf(e.stdout, "graph: %d nodes, %d edges, %d note(s), %d stub(s), %d problem(s) -> %s\n", s.Graph.Nodes, s.Graph.Edges, s.Graph.Notes, s.Graph.Stubs, s.Graph.Errors, e.graphPath())
	fmt.Fprintf(e.stdout, "cards: %d, %d due, %d new, %d mature\n", s.Cards.Total, s.Cards.Due, s.Cards.New, s.Cards.Mature)
	return nil
}

func cmdStatus(e env, args []string) error {
	b, err := os.ReadFile(e.snapshotPath())
	if err != nil {
		return fmt.Errorf("no snapshot yet: run acta snapshot (%w)", err)
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

// cmdDoctor checks the log's invariants, then the Duet inbox route. Only log
// problems fail the command; route findings are printed as warn: lines.
func cmdDoctor(e env) error {
	err := doctorLog(e)
	r := checkRoute(e)
	for _, l := range r.lines {
		fmt.Fprintln(e.stdout, l)
	}
	if r.warns > 0 {
		fmt.Fprintf(e.stdout, "doctor: %d warning(s)\n", r.warns)
	}
	return err
}

func doctorLog(e env) error {
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
