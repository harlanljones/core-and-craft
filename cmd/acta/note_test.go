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
	sup := ""
	if supersedes != "" {
		sup = fmt.Sprintf(`,"supersedes":%q`, supersedes)
	}
	return fmt.Sprintf(`{"v":1,"at":"2026-10-29","at_precision":"day","recorded_at":"2026-10-29T18:00:00Z","pillar":"faith","kind":"note.added","subject":"note:%s","source":{"via":"chat","agent":"duet:competence"},"data":{"path":%q,"sha256":%q,"title":"x"}%s}`,
		slug, path, sumHex([]byte(body)), sup)
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
	d.add("notes.md", "IDBAD", "not a body name")
	out.Reset()
	if code := run(e, []string{"inbox", "pull"}); code != 1 {
		t.Fatalf("a refused name should fail the run, got %d", code)
	}
	if !strings.Contains(errb.String(), `refused "notes.md"`) {
		t.Fatalf("stderr %q", errb)
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

func TestIngestRefusesAnUnsupersededVersion(t *testing.T) {
	e, _, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	path := "notes/nodes/martin-luther.md"
	v1, v2 := luther, luther+"Also translated the Bible.\n"
	stage := func(name, line, body string) {
		os.WriteFile(filepath.Join(e.inboxDir(), name), []byte(line+"\n"), 0o644)
		os.WriteFile(filepath.Join(e.inboxDir(), sumHex([]byte(body))+".md"), []byte(body), 0o644)
	}
	stage("a.jsonl", noteLine(t, "martin-luther", path, v1, ""), v1)
	mustRun(t, e, "ingest")

	// A second version that doesn't supersede the first is rejected.
	stage("b.jsonl", noteLine(t, "martin-luther", path, v2, ""), v2)
	if code := run(e, []string{"ingest"}); code != 1 {
		t.Fatal("want rejection")
	}
	if !strings.Contains(errb.String(), "holds another version") {
		t.Fatalf("stderr %q", errb)
	}

	// The same version superseding the first replaces the file.
	first := lineIDs(t, e)[0]
	stage("c.jsonl", noteLine(t, "martin-luther", path, v2, first), v2)
	mustRun(t, e, "ingest")
	got, _ := os.ReadFile(filepath.Join(e.data, path))
	if string(got) != v2 {
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
