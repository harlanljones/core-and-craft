package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harlanljones/core-and-craft/internal/graph"
)

const luther = `---
type: person
title: Martin Luther
pillars: [faith]
authored: "[[ninety-five-theses]]"
teaches:
  - "[[sola-fide]]"
source: "[[religion-and-history]]"
---
Posted the [[ninety-five-theses]] in 1517; see also [[john-calvin]].
`

// noteLine is a note.added inbox line naming path and the sha256 of body.
func noteLine(t *testing.T, slug, path, body, supersedes string) string {
	t.Helper()
	return noteLineAs(t, "duet:competence", slug, path, body, supersedes)
}

func noteLineAs(t *testing.T, agent, slug, path, body, supersedes string) string {
	t.Helper()
	sup := ""
	if supersedes != "" {
		sup = fmt.Sprintf(`,"supersedes":%q`, supersedes)
	}
	return fmt.Sprintf(`{"v":1,"at":"2026-10-29","at_precision":"day","recorded_at":"2026-10-29T18:00:00Z","pillar":"faith","kind":"note.added","subject":"note:%s","source":{"via":"chat","agent":%q},"data":{"path":%q,"sha256":%q,"title":"x"}%s}`,
		slug, agent, path, sumHex([]byte(body)), sup)
}

func TestInboxPullPlacesNoteBodies(t *testing.T) {
	e, d, out, errb := pullEnv(t)
	sum := sumHex([]byte(luther))
	d.add("2026-10-29T180000Z-duet-competence.jsonl", "IDEV", noteLine(t, "martin-luther", "notes/nodes/martin-luther.md", luther, "")+"\n")
	// The event file can arrive first: it waits, unrejected.
	mustRun(t, e, "inbox", "pull")
	if !strings.Contains(out.String(), "1 waiting for note bodies") || !strings.Contains(errb.String(), "waits for 1 note bod") {
		t.Fatalf("out %q err %q", out, errb)
	}
	if n := logCount(t, e); n != 0 {
		t.Fatalf("logged %d before the body arrived", n)
	}
	d.add(sum+".md", "IDBODY", luther)
	// A .md not named by its sha256 is skipped, not failed: acta never
	// deletes it from Drive, so failing would fail every run.
	d.add("notes.md", "IDBAD", "not a body name")
	out.Reset()
	mustRun(t, e, "inbox", "pull")
	if !strings.Contains(errb.String(), `skipped "notes.md"`) || !strings.Contains(out.String(), "1 skipped") {
		t.Fatalf("out %q err %q", out, errb)
	}
	got, err := os.ReadFile(filepath.Join(e.data, "notes", "nodes", "martin-luther.md"))
	if err != nil || string(got) != luther {
		t.Fatalf("placed %q, %v", got, err)
	}
	if n := logCount(t, e); n != 1 {
		t.Fatalf("logged %d, want 1", n)
	}
	if left, _ := filepath.Glob(filepath.Join(e.inboxDir(), "*")); len(left) != 0 {
		t.Fatalf("inbox not empty: %v", left)
	}
}

// stageNote puts an envelope file and its body in the inbox.
func stageNote(t *testing.T, e env, name, line, body string) {
	t.Helper()
	os.WriteFile(filepath.Join(e.inboxDir(), name), []byte(line+"\n"), 0o644)
	os.WriteFile(filepath.Join(e.inboxDir(), sumHex([]byte(body))+".md"), []byte(body), 0o644)
}

func readNote(t *testing.T, e env, path string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(e.data, filepath.FromSlash(path)))
	return string(b)
}

// Two channels can both write a shared node: neither knows the other's
// event id, so the latest version wins the path and the one it replaces
// stays readable in note-versions/.
func TestIngestTakesAnotherChannelsVersion(t *testing.T) {
	e, _, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	path := "notes/nodes/martin-luther.md"
	v1, v2 := luther, luther+"Also translated the Bible.\n"
	stageNote(t, e, "a.jsonl", noteLineAs(t, "duet:competence", "martin-luther", path, v1, ""), v1)
	mustRun(t, e, "ingest")
	stageNote(t, e, "b.jsonl", noteLineAs(t, "duet:faith", "martin-luther", path, v2, ""), v2)
	mustRun(t, e, "ingest")
	if got := readNote(t, e, path); got != v2 {
		t.Fatalf("note %q", got)
	}
	if b, err := os.ReadFile(e.versionPath(sumHex([]byte(v1)))); err != nil || string(b) != v1 {
		t.Fatalf("replaced version not kept: %q, %v", b, err)
	}
	if !strings.Contains(errb.String(), "duet:faith's version replaces one it doesn't supersede") {
		t.Fatalf("stderr %q", errb)
	}
	if n := logCount(t, e); n != 2 {
		t.Fatalf("logged %d", n)
	}

	// The first file delivered again never rolls the note back.
	stageNote(t, e, "a2.jsonl", noteLineAs(t, "duet:competence", "martin-luther", path, v1, ""), v1)
	mustRun(t, e, "ingest")
	if got := readNote(t, e, path); got != v2 {
		t.Fatalf("a duplicate rolled the note back: %q", got)
	}
	if left, _ := filepath.Glob(filepath.Join(e.inboxDir(), "*.*")); len(left) != 0 {
		t.Fatalf("inbox not empty: %v", left)
	}
}

// Edits made in Obsidian and not yet logged are never overwritten: the
// arriving version waits in note-versions/ and doctor says to merge.
func TestIngestKeepsUnloggedEdits(t *testing.T) {
	e, out, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	path := "notes/nodes/martin-luther.md"
	v1, local, v2 := luther, luther+"My own note.\n", luther+"Also translated the Bible.\n"
	stageNote(t, e, "a.jsonl", noteLine(t, "martin-luther", path, v1, ""), v1)
	mustRun(t, e, "ingest")
	os.WriteFile(filepath.Join(e.data, path), []byte(local), 0o644)
	stageNote(t, e, "b.jsonl", noteLine(t, "martin-luther", path, v2, lineIDs(t, e)[0]), v2)
	mustRun(t, e, "ingest")
	if got := readNote(t, e, path); got != local {
		t.Fatalf("local edits overwritten: %q", got)
	}
	if b, _ := os.ReadFile(e.versionPath(sumHex([]byte(v2)))); string(b) != v2 {
		t.Fatalf("arriving version not kept: %q", b)
	}
	out.Reset()
	mustRun(t, e, "doctor")
	if !strings.Contains(out.String(), "hold edits the log doesn't name") || !strings.Contains(out.String(), "run acta note add "+path) {
		t.Fatalf("doctor %q", out)
	}
	mustRun(t, e, "snapshot")
	b, _ := os.ReadFile(e.graphPath())
	if !strings.Contains(string(b), "the logged version is in note-versions/"+sumHex([]byte(v2))+".md") {
		t.Fatalf("graph %s", b)
	}
}

// A retracted note's file stays on disk; filing the note again replaces it.
func TestIngestRefilesARetractedNote(t *testing.T) {
	e, _, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	path := "notes/nodes/martin-luther.md"
	v1, v2 := luther, luther+"Also translated the Bible.\n"
	stageNote(t, e, "a.jsonl", noteLine(t, "martin-luther", path, v1, ""), v1)
	mustRun(t, e, "ingest")
	mustRun(t, e, "retract", lineIDs(t, e)[0], "--agent", "manual")
	stageNote(t, e, "b.jsonl", noteLine(t, "martin-luther", path, v2, ""), v2)
	mustRun(t, e, "ingest")
	if got := readNote(t, e, path); got != v2 {
		t.Fatalf("note %q", got)
	}
}

func TestIngestRejectsABodyUnderTheWrongName(t *testing.T) {
	e, _, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	line := noteLine(t, "martin-luther", "notes/nodes/martin-luther.md", luther, "")
	os.WriteFile(filepath.Join(e.inboxDir(), "a.jsonl"), []byte(line+"\n"), 0o644)
	os.WriteFile(filepath.Join(e.inboxDir(), sumHex([]byte(luther))+".md"), []byte("tampered"), 0o644)
	if code := run(e, []string{"ingest"}); code != 1 {
		t.Fatal("want rejection")
	}
	if !strings.Contains(errb.String(), "does not hash to its name") {
		t.Fatalf("stderr %q", errb)
	}
}

func lineIDs(t *testing.T, e env) []string {
	t.Helper()
	b, _ := os.ReadFile(e.logPath())
	var ids []string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var ev struct {
			ID string `json:"id"`
		}
		json.Unmarshal([]byte(l), &ev)
		ids = append(ids, ev.ID)
	}
	return ids
}

func TestNoteAddLogsLocalEdits(t *testing.T) {
	e, out, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	p := filepath.Join(e.data, "notes", "nodes", "martin-luther.md")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(luther), 0o644)

	mustRun(t, e, "note", "add", p)
	mustRun(t, e, "note", "add", "notes/nodes/martin-luther.md")
	if !strings.Contains(out.String(), "already logged") {
		t.Fatalf("second add %q", out)
	}
	os.WriteFile(p, []byte(luther+"More.\n"), 0o644)
	out.Reset()
	mustRun(t, e, "doctor")
	if !strings.Contains(out.String(), "changed since logged; run acta note add notes/nodes/martin-luther.md") {
		t.Fatalf("doctor %q", out)
	}
	mustRun(t, e, "note", "add", p)
	b, _ := os.ReadFile(e.logPath())
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[1], `"supersedes":"`+lineIDs(t, e)[0]+`"`) || !strings.Contains(lines[0], `"pillar":"faith"`) {
		t.Fatalf("log %q", b)
	}

	// Outside notes/, or a bad slug, is refused.
	bad := filepath.Join(e.data, "notes", "nodes", "Bad Name.md")
	os.WriteFile(bad, []byte("x"), 0o644)
	if code := run(e, []string{"note", "add", bad}); code != 1 {
		t.Fatal("want refusal")
	}
}

func TestSnapshotWritesTheGraph(t *testing.T) {
	e, out, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	stageDaySeed(t, e)
	mustRun(t, e, "ingest")
	p := filepath.Join(e.data, "notes", "nodes", "martin-luther.md")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(luther), 0o644)
	mustRun(t, e, "note", "add", p)
	mustRun(t, e, "quest", "add", "--pillar", "competence", "--title", "Typing drill", "--every", "daily", "--skill", "skill:typing", "--rule", "runs:1")
	out.Reset()
	mustRun(t, e, "snapshot")
	if !strings.Contains(out.String(), "graph: ") {
		t.Fatalf("output %q", out)
	}
	s := readSnapshot(t, e)
	if s.Graph == nil || s.Graph.Notes != 2 || s.Graph.New7d != 2 {
		t.Fatalf("summary %+v", s.Graph)
	}
	b, err := os.ReadFile(e.graphPath())
	if err != nil {
		t.Fatal(err)
	}
	var g graph.Graph
	json.Unmarshal(b, &g)
	has := func(from, to, typ string) bool {
		for _, ed := range g.Edges {
			if ed.From == from && ed.To == to && ed.Type == typ {
				return true
			}
		}
		return false
	}
	var quest string
	for _, n := range g.Nodes {
		if n.Kind == "quest" {
			quest = n.ID
		}
	}
	for _, c := range [][3]string{
		{"note:martin-luther", "note:ninety-five-theses", "authored"},
		{"note:martin-luther", "note:religion-and-history", "source"},
		{"note:martin-luther", "note:john-calvin", "mentions"},
		{"note:martin-luther", "pillar:faith", "in_pillar"},
		{"note:religion-and-history", "pillar:competence", "in_pillar"},
		{"skill:typing", "pillar:competence", "in_pillar"},
		{quest, "skill:typing", "trains"},
	} {
		if !has(c[0], c[1], c[2]) {
			t.Errorf("missing edge %v", c)
		}
	}
	if has("note:martin-luther", "note:ninety-five-theses", "mentions") {
		t.Error("a body link that repeats a typed edge should not add mentions")
	}
}

func TestNoteAddReadsPillarsLikeTheGraph(t *testing.T) {
	e, _, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	p := putNote(t, e, "notes/nodes/sola-fide.md", "---\ntype: concept\npillars: [ Faith ]\n---\nBy faith alone.\n")
	mustRun(t, e, "note", "add", p)
	b, _ := os.ReadFile(e.logPath())
	if !strings.Contains(string(b), `"pillar":"faith"`) {
		t.Fatalf("log %q", b)
	}
}

// putNote writes a note under the data directory and returns its path.
func putNote(t *testing.T, e env, rel, body string) string {
	t.Helper()
	p := filepath.Join(e.data, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNoteAddRefusesOneSlugInTwoFolders(t *testing.T) {
	e, out, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	a := putNote(t, e, "notes/faith/luther.md", luther)
	mustRun(t, e, "note", "add", a)
	b := putNote(t, e, "notes/nodes/luther.md", luther)

	// The other folder's file is refused, and the log doesn't change.
	errb.Reset()
	if code := run(e, []string{"note", "add", b}); code != 1 {
		t.Fatalf("exit %d, want refusal", code)
	}
	if s := errb.String(); !strings.Contains(s, "notes/nodes/luther.md") || !strings.Contains(s, "notes/faith/luther.md") || !strings.Contains(s, "one slug names one node") {
		t.Fatalf("error %q", s)
	}
	if n := len(lineIDs(t, e)); n != 1 {
		t.Fatalf("logged %d, want 1", n)
	}

	// Doctor names the pair once, and doesn't also call either unlogged.
	out.Reset()
	mustRun(t, e, "doctor")
	s := out.String()
	if !strings.Contains(s, "notes: slug luther is in 2 folders: notes/faith/luther.md, notes/nodes/luther.md; one slug names one node") || strings.Contains(s, "doesn't name; run") {
		t.Fatalf("doctor %q", s)
	}
}

func TestNoteAddMoveReplacesTheOldFile(t *testing.T) {
	e, out, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	old := putNote(t, e, "notes/faith/luther.md", luther)
	mustRun(t, e, "note", "add", old)
	first := lineIDs(t, e)[0]
	nw := putNote(t, e, "notes/nodes/luther.md", luther+"A shared node now.\n")

	// Without --move it is refused, and the error points at --move.
	errb.Reset()
	if code := run(e, []string{"note", "add", nw}); code != 1 || !strings.Contains(errb.String(), "--move") {
		t.Fatalf("want refusal naming --move, got %d %q", code, errb)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("refusal removed the old file")
	}

	mustRun(t, e, "note", "add", "--move", nw)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old file still there: %v", err)
	}
	if !fileExists(e.versionPath(sumHex([]byte(luther)))) {
		t.Fatal("old body not kept in note-versions")
	}
	b, _ := os.ReadFile(e.logPath())
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[1], `"path":"notes/nodes/luther.md"`) || !strings.Contains(lines[1], `"supersedes":"`+first+`"`) {
		t.Fatalf("log %q", b)
	}
	out.Reset()
	mustRun(t, e, "doctor")
	if !strings.Contains(out.String(), "notes: ok, 1 logged") {
		t.Fatalf("doctor %q", out)
	}
}
