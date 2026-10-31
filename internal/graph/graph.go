package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/harlanljones/core-and-craft/internal/event"
)

// FileName is graph.json, next to snapshot.json in $ACTA_STATE.
const FileName = "graph.json"

// NodeTypes is the fixed list of note types. A note with another type keeps
// it, with a warning in errors.
var NodeTypes = []string{
	"subject",      // a pillar's own note: a study session, a reading, a plan
	"person",       //
	"tradition",    // a school, movement, religion or branch of one
	"family",       // a group of traditions
	"event",        // something that happened, with a date
	"concept",      // an idea, principle or term
	"work",         // a book, letter, paper or talk
	"place",        //
	"organization", //
	"dataset",      // a survey or source of numbers
}

// EdgeTypes is the fixed vocabulary of typed links a note's frontmatter may
// hold, each read from the note to its targets. An unknown key that holds
// [[links]] becomes related, with a warning.
var EdgeTypes = []string{
	"member_of",    // Judaism -> Abrahamic
	"branch_of",    // Lutheran -> Protestantism
	"split_from",   // Eastern Orthodoxy -> Catholicism
	"part_of",      // a battle -> a war
	"founded_by",   // Reformed -> Calvin
	"authored",     // Luther -> 95 Theses
	"teaches",      // Judaism -> covenant
	"argues_for",   // Paul -> faith
	"opposed",      // 95 Theses -> indulgences
	"led_to",       // 95 Theses -> Reformation
	"responded_to", // Council of Trent -> Reformation
	"resolved_by",  // faith vs works -> Joint Declaration
	"coined_by",    // Protestant work ethic -> Weber
	"measured_by",  // Protestantism -> Pew 2023-24
	"practices",    // a note -> skill:typing
	"source",       // a node -> the note it came from
	"related",      // anything else
}

// Edge types acta adds itself.
const (
	edgeMentions = "mentions"  // a [[link]] in a note's body
	edgeInPillar = "in_pillar" // a note, skill or quest -> its pillar
	edgeTrains   = "trains"    // a quest -> its skill (quest.added skill)
)

// reserved keys are node fields, never edges.
var reserved = map[string]bool{
	"type": true, "title": true, "aliases": true, "pillars": true, "recall": true,
	"private": true, "date": true, "year": true, "tags": true, "cssclasses": true,
}

// Note is one live note.added and the file it names.
type Note struct {
	ID       string // note:<slug>
	Path     string // notes/<folder>/<slug>.md, "" for an event without a file
	Pillar   string // the event's pillar
	Added    string // local date of the first live note.added
	Title    string // the event's title, used when the file has none
	Content  []byte // nil when the file is missing
	Mismatch bool   // the file is not the version the log names
	Missing  bool
	Saved    string // where the logged version waits when the file holds other edits
}

// Ref is a skill or quest from the snapshot.
type Ref struct {
	ID, Title, Pillar, Status, Skill string
}

type Input struct {
	Notes  []Note
	Skills []Ref
	Quests []Ref
}

type Node struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"` // note, skill, quest, pillar
	Type    string   `json:"type,omitempty"`
	Title   string   `json:"title"`
	Pillar  string   `json:"pillar,omitempty"`
	Pillars []string `json:"pillars,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
	Path    string   `json:"path,omitempty"`
	Added   string   `json:"added,omitempty"`
	Date    string   `json:"date,omitempty"`
	Status  string   `json:"status,omitempty"`
	Recall  string   `json:"recall,omitempty"`
	Private bool     `json:"private,omitempty"`
	Stub    bool     `json:"stub,omitempty"`
	Degree  int      `json:"degree"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
	Via  string `json:"via"` // frontmatter, body or log
}

type Problem struct {
	ID  string `json:"id"`
	Msg string `json:"error"`
}

type Graph struct {
	V       int       `json:"v"`
	BuiltAt string    `json:"built_at"`
	Nodes   []Node    `json:"nodes"`
	Edges   []Edge    `json:"edges"`
	Errors  []Problem `json:"errors"`
}

// Summary is the graph's line in snapshot.json.
type Summary struct {
	Nodes  int `json:"nodes"`
	Edges  int `json:"edges"`
	Notes  int `json:"notes"`
	Stubs  int `json:"stubs"`
	New7d  int `json:"new_7d"`
	Errors int `json:"errors"`
}

var (
	bodyLinkRe = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)
	slugRunRe  = regexp.MustCompile(`[^a-z0-9]+`)
	refRe      = regexp.MustCompile(`^(note|skill|quest|pillar):[a-z0-9][a-z0-9-]*$`)
)

// Slug turns a link or title into a slug: "Martin Luther" -> "martin-luther".
func Slug(s string) string {
	return strings.Trim(slugRunRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// Target reads one link value: [[slug]], [[folder/slug|label]], a bare
// slug, or an id such as skill:typing or pillar:faith. ok is false for an
// empty value.
func Target(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "[[") && strings.HasSuffix(v, "]]") {
		v = v[2 : len(v)-2]
	}
	if i := strings.IndexAny(v, "|#"); i >= 0 {
		v = v[:i]
	}
	v = strings.TrimSuffix(strings.TrimSpace(v), ".md")
	if refRe.MatchString(v) {
		return v, true
	}
	if i := strings.LastIndex(v, "/"); i >= 0 {
		v = v[i+1:]
	}
	if s := Slug(v); s != "" {
		return "note:" + s, true
	}
	return "", false
}

func isLink(v string) bool { return strings.Contains(v, "[[") }

// Pillars reads a note's pillars: line the way the graph does: trimmed,
// lowercased, a "pillar:" prefix dropped, repeats dropped. bad are the
// values that aren't a pillar.
func Pillars(f Front) (pillars, bad []string) {
	for _, p := range f.Vals["pillars"] {
		p = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(p, "pillar:")))
		switch {
		case !contains(event.Pillars, p):
			bad = append(bad, p)
		case !contains(pillars, p):
			pillars = append(pillars, p)
		}
	}
	return pillars, bad
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Build folds the input into a graph. The same input always gives the same
// graph: nodes sort by id, edges by from, to and type.
func Build(in Input, builtAt string) Graph {
	g := Graph{V: 1, BuiltAt: builtAt, Nodes: []Node{}, Edges: []Edge{}, Errors: []Problem{}}
	nodes := map[string]*Node{}
	edges := map[Edge]bool{}
	typed := map[[2]string]bool{}
	problem := func(id, format string, a ...any) {
		g.Errors = append(g.Errors, Problem{id, fmt.Sprintf(format, a...)})
	}
	add := func(n Node) {
		if _, ok := nodes[n.ID]; !ok {
			nodes[n.ID] = &n
		}
	}
	link := func(from, to, typ, via string) {
		if from == to {
			return
		}
		edges[Edge{from, to, typ, via}] = true
		if via != "body" {
			typed[[2]string{from, to}] = true
		}
	}

	for _, p := range event.Pillars {
		add(Node{ID: "pillar:" + p, Kind: "pillar", Title: strings.ToUpper(p[:1]) + p[1:], Pillar: p})
	}
	for _, s := range in.Skills {
		add(Node{ID: s.ID, Kind: "skill", Title: s.Title, Pillar: s.Pillar, Status: s.Status})
		link(s.ID, "pillar:"+s.Pillar, edgeInPillar, "log")
	}
	for _, q := range in.Quests {
		add(Node{ID: q.ID, Kind: "quest", Title: q.Title, Pillar: q.Pillar, Status: q.Status})
		link(q.ID, "pillar:"+q.Pillar, edgeInPillar, "log")
		if q.Skill != "" {
			link(q.ID, q.Skill, edgeTrains, "log")
		}
	}

	var bodies []struct {
		id   string
		body string
	}
	for _, nt := range in.Notes {
		n := Node{ID: nt.ID, Kind: "note", Path: nt.Path, Added: nt.Added, Title: nt.Title}
		folder := ""
		if nt.Path != "" {
			folder, _, _ = event.NotePath(nt.Path)
		}
		switch {
		case nt.Missing:
			problem(nt.ID, "%s is missing; the log names it", nt.Path)
		case nt.Mismatch && nt.Saved != "":
			problem(nt.ID, "%s has edits the log doesn't name; the logged version is in %s: merge it in, then run acta note add %s", nt.Path, nt.Saved, nt.Path)
		case nt.Mismatch:
			problem(nt.ID, "%s changed since it was logged: run acta note add %s", nt.Path, nt.Path)
		}
		var front Front
		if nt.Content != nil {
			f, body, warns, err := ParseFront(nt.Content)
			for _, w := range warns {
				problem(nt.ID, "%s", w)
			}
			if err != nil {
				problem(nt.ID, "%v", err)
			} else {
				front = f
				bodies = append(bodies, struct{ id, body string }{nt.ID, string(body)})
				if n.Title == "" || front.Get("title") != "" {
					n.Title = front.Get("title")
				}
				if n.Title == "" {
					n.Title = firstHeading(string(body))
				}
			}
		}
		if n.Title == "" {
			n.Title = strings.TrimPrefix(nt.ID, "note:")
		}

		n.Type = front.Get("type")
		switch {
		case n.Type == "" && folder == event.NodesDir:
			n.Type = "concept"
			problem(nt.ID, "no type; read as concept")
		case n.Type == "":
			n.Type = "subject"
		case !contains(NodeTypes, n.Type):
			problem(nt.ID, "type %q is not in the vocabulary", n.Type)
		}
		var bad []string
		n.Pillars, bad = Pillars(front)
		for _, p := range bad {
			problem(nt.ID, "pillars: %q is not a pillar", p)
		}
		switch {
		case len(n.Pillars) == 0 && folder != "" && folder != event.NodesDir:
			n.Pillars = []string{folder}
		case len(n.Pillars) == 0 && nt.Pillar != "":
			n.Pillars = []string{nt.Pillar}
		}
		if len(n.Pillars) > 0 {
			n.Pillar = n.Pillars[0]
		}
		n.Aliases = front.Vals["aliases"]
		n.Recall = front.Get("recall")
		n.Date = front.Get("date")
		if n.Date == "" {
			n.Date = front.Get("year")
		}
		switch strings.ToLower(front.Get("private")) {
		case "true", "yes":
			n.Private = true
		}
		add(n)
		for _, p := range n.Pillars {
			link(n.ID, "pillar:"+p, edgeInPillar, "log")
		}
		for _, k := range front.Keys {
			if reserved[k] {
				continue
			}
			vals := front.Vals[k]
			typ := k
			if !contains(EdgeTypes, k) {
				hasLink := false
				for _, v := range vals {
					hasLink = hasLink || isLink(v)
				}
				if !hasLink {
					continue // Obsidian's own properties, a created date, ...
				}
				problem(nt.ID, "%s is not an edge type; read as related", k)
				typ = "related"
			}
			for _, v := range vals {
				if to, ok := Target(v); ok {
					link(n.ID, to, typ, "frontmatter")
				}
			}
		}
	}
	for _, b := range bodies {
		for _, m := range bodyLinkRe.FindAllStringSubmatch(b.body, -1) {
			to, ok := Target(m[1])
			if !ok || typed[[2]string{b.id, to}] {
				continue
			}
			link(b.id, to, edgeMentions, "body")
		}
	}

	for e := range edges {
		if _, ok := nodes[e.To]; !ok {
			kind, slug, _ := strings.Cut(e.To, ":")
			nodes[e.To] = &Node{ID: e.To, Kind: kind, Title: slug, Stub: true}
		}
		if e.Type != edgeInPillar {
			nodes[e.From].Degree++
			nodes[e.To].Degree++
		}
		g.Edges = append(g.Edges, e)
	}
	for _, n := range nodes {
		g.Nodes = append(g.Nodes, *n)
	}
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	sort.Slice(g.Edges, func(i, j int) bool {
		a, b := g.Edges[i], g.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Via < b.Via
	})
	sort.SliceStable(g.Errors, func(i, j int) bool { return g.Errors[i].ID < g.Errors[j].ID })
	return g
}

func firstHeading(body string) string {
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "# ") {
			return strings.TrimSpace(l[2:])
		}
	}
	return ""
}

// Summarize counts the graph for snapshot.json. since is the first local
// date (YYYY-MM-DD) that counts as new.
func Summarize(g Graph, since string) Summary {
	s := Summary{Edges: len(g.Edges), Errors: len(g.Errors)}
	for _, n := range g.Nodes {
		s.Nodes++
		switch {
		case n.Stub:
			s.Stubs++
		case n.Kind == "note":
			s.Notes++
			if n.Added >= since {
				s.New7d++
			}
		}
	}
	return s
}

// Write stores g at path via a temp file and rename, as the snapshot is.
func Write(path string, g Graph) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".graph-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
