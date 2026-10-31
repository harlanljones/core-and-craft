package main

// Notes and the knowledge graph (docs/knowledge-graph.md). A note.added
// event names one version of a note file by path and sha256. acta is the
// only writer of notes/ as of the log: ingest places a note's body only when
// an accepted event names it, and acta note add logs a note Harlan wrote or
// edited on this machine. acta snapshot folds the live notes, skills and
// quests into graph.json.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
	"github.com/harlanljones/core-and-craft/internal/graph"
	"github.com/harlanljones/core-and-craft/internal/snapshot"
	"github.com/harlanljones/core-and-craft/internal/store"
)

func (e env) graphPath() string { return filepath.Join(e.state, graph.FileName) }

// bodyWait is how long a file may wait in inbox/ for its note bodies before
// doctor warns.
const bodyWait = time.Hour

func sumHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// noteRef is the path and sha256 a note.added names, if any.
func noteRef(ev event.Event) (path, sum string) {
	if ev.Kind != "note.added" {
		return "", ""
	}
	var d struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	json.Unmarshal(ev.Data, &d)
	return d.Path, d.SHA256
}

// placement is one note body ingest writes under notes/.
type placement struct {
	dst, sum string
	body     []byte
	from     string // inbox/<sha256>.md to remove afterwards, or ""
}

// planNotes finds the body of every note.added in evs that names a file.
// A body is looked for, in order, as <dir>/<sha256>.md, inbox/<sha256>.md,
// <dir>/<path> (only for a file named on the command line, such as the seed)
// and the note already in place. It must hash to the event's sha256.
//
// waiting counts bodies not found yet. For an inbox file that defers the
// whole file; a file named on the command line is logged anyway, with a
// warning. errs reject the file: a body under the wrong name, or a path that
// already holds another version the event does not supersede.
func planNotes(e env, dir string, evs []event.Event, logged []event.Event, remote bool) (plans []placement, waiting int, errs []error) {
	known := map[string]string{} // event id -> sha256 it names
	for _, ev := range logged {
		if _, sum := noteRef(ev); sum != "" {
			known[ev.ID] = sum
		}
	}
	for _, ev := range evs {
		if _, sum := noteRef(ev); sum != "" {
			known[ev.ID] = sum
		}
	}
	current := map[string]string{} // dst -> sha256 of what will be there
	for i, ev := range evs {
		path, sum := noteRef(ev)
		if path == "" {
			continue
		}
		dst := filepath.Join(e.data, filepath.FromSlash(path))
		var body []byte
		from := ""
		cands := []string{filepath.Join(dir, sum+".md")}
		if inbox := filepath.Join(e.inboxDir(), sum+".md"); filepath.Clean(dir) != filepath.Clean(e.inboxDir()) {
			cands = append(cands, inbox)
		}
		if !remote {
			cands = append(cands, filepath.Join(dir, filepath.FromSlash(path)))
		}
		cands = append(cands, dst)
		for j, c := range cands {
			b, err := os.ReadFile(c)
			if err != nil {
				continue
			}
			if sumHex(b) == sum {
				body = b
				if strings.HasSuffix(c, sum+".md") && filepath.Dir(c) == filepath.Clean(e.inboxDir()) {
					from = c
				}
				break
			}
			if j == 0 || strings.HasSuffix(c, sum+".md") {
				errs = append(errs, fmt.Errorf("event %d (%s): %s does not hash to its name", i+1, ev.Subject, filepath.Base(c)))
			}
		}
		if body == nil {
			waiting++
			if !remote {
				fmt.Fprintf(e.stderr, "acta: %s: no body for %s (%s); logged without it\n", filepath.Base(dir), ev.Subject, path)
			}
			continue
		}
		have, ok := current[dst]
		if !ok {
			if b, err := os.ReadFile(dst); err == nil {
				have = sumHex(b)
			}
		}
		if have != "" && have != sum && (ev.Supersedes == "" || known[ev.Supersedes] != have) {
			errs = append(errs, fmt.Errorf("event %d (%s): %s holds another version; a new version must supersede the note.added for it", i+1, ev.Subject, path))
			continue
		}
		current[dst] = sum
		plans = append(plans, placement{dst: dst, sum: sum, body: body, from: from})
	}
	return plans, waiting, errs
}

// placeNotes writes each body by temp file and rename, so a reader never
// sees half a note.
func placeNotes(plans []placement) error {
	for _, p := range plans {
		if b, err := os.ReadFile(p.dst); err == nil && sumHex(b) == p.sum {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p.dst), 0o755); err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(p.dst), ".note-*.part")
		if err != nil {
			return err
		}
		_, werr := tmp.Write(p.body)
		if serr := tmp.Sync(); werr == nil {
			werr = serr
		}
		if cerr := tmp.Close(); werr == nil {
			werr = cerr
		}
		if werr == nil {
			werr = os.Rename(tmp.Name(), p.dst)
		}
		if werr != nil {
			os.Remove(tmp.Name())
			return werr
		}
	}
	return nil
}

// liveNotes is the latest live note.added per subject, in subject order,
// with the date of the first one.
func liveNotes(evs []event.Event, loc *time.Location) ([]event.Event, map[string]string) {
	live := snapshot.Effective(evs)
	sort.SliceStable(live, func(i, j int) bool { return live[i].Seq < live[j].Seq })
	latest := map[string]event.Event{}
	first := map[string]string{}
	for _, ev := range live {
		if ev.Kind != "note.added" {
			continue
		}
		latest[ev.Subject] = ev
		if _, ok := first[ev.Subject]; !ok {
			first[ev.Subject] = ev.LocalDate(loc)
		}
	}
	// A superseded version drops out of live; its date still counts as added.
	for _, ev := range evs {
		if _, ok := latest[ev.Subject]; ok && ev.Kind == "note.added" {
			if d := ev.LocalDate(loc); d < first[ev.Subject] {
				first[ev.Subject] = d
			}
		}
	}
	out := make([]event.Event, 0, len(latest))
	for _, ev := range latest {
		out = append(out, ev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Subject < out[j].Subject })
	return out, first
}

// graphInput reads the files the live notes name and takes skills and quests
// from the snapshot.
func graphInput(e env, evs []event.Event, s snapshot.Snapshot) graph.Input {
	var in graph.Input
	notes, first := liveNotes(evs, e.loc)
	for _, ev := range notes {
		path, sum := noteRef(ev)
		var d struct {
			Title string `json:"title"`
		}
		json.Unmarshal(ev.Data, &d)
		n := graph.Note{ID: ev.Subject, Path: path, Pillar: ev.Pillar, Added: first[ev.Subject], Title: d.Title}
		if path != "" {
			b, err := os.ReadFile(filepath.Join(e.data, filepath.FromSlash(path)))
			if err != nil {
				n.Missing = true
			} else {
				n.Content = b
				n.Mismatch = sumHex(b) != sum
			}
		}
		in.Notes = append(in.Notes, n)
	}
	for _, sk := range s.Skills {
		st := "active"
		if sk.Passed {
			st = "passed"
		}
		in.Skills = append(in.Skills, graph.Ref{ID: sk.ID, Title: sk.Name, Pillar: sk.Pillar, Status: st})
	}
	for _, q := range s.Quests {
		in.Quests = append(in.Quests, graph.Ref{ID: q.ID, Title: q.Title, Pillar: q.Pillar, Status: "active", Skill: q.Skill})
	}
	for _, q := range s.Proposed {
		in.Quests = append(in.Quests, graph.Ref{ID: q.ID, Title: q.Title, Pillar: q.Pillar, Status: "proposed"})
	}
	return in
}

// buildGraph writes graph.json and returns its summary for the snapshot.
func buildGraph(e env, evs []event.Event, s snapshot.Snapshot) (*graph.Summary, error) {
	now := e.now()
	g := graph.Build(graphInput(e, evs, s), now.UTC().Format(time.RFC3339))
	if err := graph.Write(e.graphPath(), g); err != nil {
		return nil, err
	}
	sum := graph.Summarize(g, now.In(e.loc).AddDate(0, 0, -6).Format("2006-01-02"))
	return &sum, nil
}

// cmdNote: acta note add <file>... logs notes written on this machine (in
// Obsidian, say) as note.added, superseding the version the log had.
func cmdNote(e env, args []string) error {
	const use = "usage: acta note add <file>... [--pillar P] [--agent A] [--via V]"
	if len(args) == 0 || args[0] != "add" {
		return errors.New(use)
	}
	fs := flag.NewFlagSet("note add", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	pillar := fs.String("pillar", "", "pillar (default: the note's pillars, its folder, or the version before)")
	agent := fs.String("agent", "manual", "writer")
	via := fs.String("via", "manual", "chat, manual, ...")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New(use)
	}
	evs, _, err := store.ReadFile(e.logPath())
	if err != nil {
		return err
	}
	notes, _ := liveNotes(evs, e.loc)
	prev := map[string]event.Event{}
	for _, ev := range notes {
		prev[ev.Subject] = ev
	}
	root, err := filepath.Abs(e.data)
	if err != nil {
		return err
	}
	var out []event.Event
	for _, f := range fs.Args() {
		abs, err := filepath.Abs(f)
		if err != nil {
			return err
		}
		if _, err := os.Stat(abs); err != nil {
			// A path as the log writes it, relative to the data directory.
			abs = filepath.Join(root, filepath.FromSlash(f))
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("%s: not under %s", f, filepath.Join(root, "notes"))
		}
		rel = filepath.ToSlash(rel)
		folder, slug, ok := event.NotePath(rel)
		if !ok {
			return fmt.Errorf("%s: want notes/<pillar>/<slug>.md or notes/nodes/<slug>.md (lowercase slug)", rel)
		}
		b, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		front, body, _, err := graph.ParseFront(b)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		subject := "note:" + slug
		sum := sumHex(b)
		old, had := prev[subject]
		if p, s := noteRef(old); had && p == rel && s == sum {
			fmt.Fprintf(e.stdout, "%s already logged\n", rel)
			continue
		}
		p := *pillar
		for _, fp := range front.Vals["pillars"] {
			if p == "" && contains(event.Pillars, strings.TrimPrefix(fp, "pillar:")) {
				p = strings.TrimPrefix(fp, "pillar:")
			}
		}
		if p == "" && folder != event.NodesDir {
			p = folder
		}
		if p == "" && had {
			p = old.Pillar
		}
		if p == "" {
			return fmt.Errorf("%s: name a pillar with --pillar or a pillars: line", rel)
		}
		title := front.Get("title")
		if title == "" {
			title = firstLine(string(body))
		}
		if title == "" {
			title = slug
		}
		data := map[string]any{"path": rel, "sha256": sum, "title": title}
		if t := front.Get("type"); t != "" {
			data["type"] = t
		}
		raw, _ := json.Marshal(data)
		sup := ""
		if had {
			sup = old.ID
		}
		ev, err := newEvent(e, "note.added", subject, p, string(raw), "", *via, *agent, "", "", sup)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		out = append(out, ev)
	}
	if len(out) == 0 {
		return nil
	}
	res, err := store.Append(e.logPath(), out)
	if err != nil {
		return err
	}
	for _, ev := range res.Appended {
		fmt.Fprintf(e.stdout, "%s %s seq %d\n", ev.ID, ev.Subject, ev.Seq)
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// firstLine is a note's "# Title" heading, if it has one.
func firstLine(body string) string {
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "# ") {
			return strings.TrimSpace(l[2:])
		}
	}
	return ""
}

// checkNotes adds doctor's warn: lines for notes: inbox files still waiting
// for bodies, bodies nothing names, and notes that differ from the log or
// that the log doesn't name.
func checkNotes(e env, r *routeReport) {
	now := e.now()
	var waitingFiles, orphans []string
	if js, err := filepath.Glob(filepath.Join(e.inboxDir(), "*.jsonl")); err == nil {
		for _, f := range js {
			if fi, err := os.Stat(f); err == nil && now.Sub(fi.ModTime()) > bodyWait {
				waitingFiles = append(waitingFiles, filepath.Base(f))
			}
		}
	}
	if mds, err := filepath.Glob(filepath.Join(e.inboxDir(), "*.md")); err == nil {
		for _, f := range mds {
			if fi, err := os.Stat(f); err == nil && now.Sub(fi.ModTime()) > 24*time.Hour {
				orphans = append(orphans, filepath.Base(f))
			}
		}
	}
	if len(waitingFiles) > 0 {
		r.warn("notes: %d inbox file(s) waiting over an hour for note bodies: %s", len(waitingFiles), strings.Join(waitingFiles, ", "))
	}
	if len(orphans) > 0 {
		r.warn("notes: %d note bod(ies) in the inbox for over a day with no event: %s", len(orphans), strings.Join(orphans, ", "))
	}

	evs, _, err := store.ReadFile(e.logPath())
	if err != nil {
		r.warn("notes: %v", err)
		return
	}
	notes, _ := liveNotes(evs, e.loc)
	named := map[string]bool{}
	var missing, changed []string
	for _, ev := range notes {
		path, sum := noteRef(ev)
		if path == "" {
			continue
		}
		named[path] = true
		b, err := os.ReadFile(filepath.Join(e.data, filepath.FromSlash(path)))
		switch {
		case err != nil:
			missing = append(missing, path)
		case sumHex(b) != sum:
			changed = append(changed, path)
		}
	}
	var unlogged []string
	filepath.WalkDir(filepath.Join(e.data, "notes"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		rel, rerr := filepath.Rel(e.data, p)
		if rerr == nil && !named[filepath.ToSlash(rel)] {
			unlogged = append(unlogged, filepath.ToSlash(rel))
		}
		return nil
	})
	if len(missing) > 0 {
		r.warn("notes: %d note(s) the log names are missing: %s", len(missing), strings.Join(missing, ", "))
	}
	if len(changed) > 0 {
		r.warn("notes: %d note(s) changed since logged; run acta note add %s", len(changed), strings.Join(changed, " "))
	}
	if len(unlogged) > 0 {
		r.warn("notes: %d note(s) the log doesn't name; run acta note add %s", len(unlogged), strings.Join(unlogged, " "))
	}
	if len(waitingFiles)+len(orphans)+len(missing)+len(changed)+len(unlogged) == 0 {
		r.ok("notes: ok, %d logged", len(named))
	}
}
