package graph

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseFront(t *testing.T) {
	src := "---\n" +
		"type: tradition\n" +
		"title: \"Lutheranism\" # comment\n" +
		"aliases: [Lutheran, 'Evangelical, Lutheran']\n" +
		"branch_of: \"[[protestantism]]\"\n" +
		"founded_by:\n" +
		"  - \"[[martin-luther]]\"\n" +
		"  - '[[philip-melanchthon|Melanchthon]]'\n" +
		"meta:\n" +
		"  nested: true\n" +
		"empty:\n" +
		"---\n# Body\n"
	f, body, warns, err := ParseFront([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"type":       {"tradition"},
		"title":      {"Lutheranism"},
		"aliases":    {"Lutheran", "Evangelical, Lutheran"},
		"branch_of":  {"[[protestantism]]"},
		"founded_by": {"[[martin-luther]]", "[[philip-melanchthon|Melanchthon]]"},
		"meta":       nil,
		"empty":      nil,
	}
	if !reflect.DeepEqual(f.Vals, want) {
		t.Fatalf("vals %#v", f.Vals)
	}
	if strings.Join(f.Keys, ",") != "type,title,aliases,branch_of,founded_by,meta,empty" {
		t.Fatalf("keys %v", f.Keys)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "meta: nested") {
		t.Fatalf("warns %v", warns)
	}
	if string(body) != "# Body\n" {
		t.Fatalf("body %q", body)
	}
}

func TestParseFrontErrors(t *testing.T) {
	for _, src := range []string{
		"---\ntype: x\n",             // never closes
		"---\n- orphan\n---\n",       // list item with no key
		"---\ntitle: \"open\n---\n",  // unclosed quote
		"---\na: 1\na: 2\n---\n",     // duplicate key
		"---\nnot a key line\n---\n", // no colon
		"---\nlist: [a, [b]\n---\n",  // unbalanced
	} {
		if _, _, _, err := ParseFront([]byte(src)); err == nil {
			t.Errorf("want error for %q", src)
		}
	}
	f, body, _, err := ParseFront([]byte("# Just a note\n"))
	if err != nil || len(f.Keys) != 0 || string(body) != "# Just a note\n" {
		t.Fatalf("no frontmatter: %v %q %v", f, body, err)
	}
}

func TestTarget(t *testing.T) {
	for in, want := range map[string]string{
		"[[martin-luther]]":              "note:martin-luther",
		"[[nodes/martin-luther|Luther]]": "note:martin-luther",
		"Martin Luther":                  "note:martin-luther",
		"skill:typing":                   "skill:typing",
		"[[pillar:faith]]":               "pillar:faith",
		"[[sola-fide#Origins]]":          "note:sola-fide",
	} {
		if got, ok := Target(in); !ok || got != want {
			t.Errorf("Target(%q) = %q, want %q", in, got, want)
		}
	}
	if _, ok := Target("[[ ]]"); ok {
		t.Error("empty link should not resolve")
	}
}

func TestBuild(t *testing.T) {
	in := Input{
		Notes: []Note{
			{ID: "note:lutheranism", Path: "notes/nodes/lutheranism.md", Pillar: "faith", Added: "2026-10-30", Content: []byte(
				"---\ntype: tradition\npillars: [faith, competence]\nbranch_of: \"[[protestantism]]\"\nfounded_by: \"[[martin-luther]]\"\nsee_also: \"[[calvinism]]\"\nrecall: Who founded it?\n---\nLinks to [[martin-luther]] and [[weber]].\n")},
			{ID: "note:martin-luther", Path: "notes/nodes/martin-luther.md", Pillar: "faith", Added: "2026-10-20", Content: []byte("# Martin Luther\n")},
			{ID: "note:study", Path: "notes/competence/study.md", Pillar: "competence", Added: "2026-10-30", Mismatch: true, Content: []byte("---\nprivate: true\n---\n# Study\n")},
			{ID: "note:gone", Path: "notes/competence/gone.md", Missing: true},
		},
		Skills: []Ref{{ID: "skill:typing", Title: "Typing", Pillar: "competence"}},
		Quests: []Ref{{ID: "quest:drill", Title: "Drill", Pillar: "competence", Skill: "skill:typing"}},
	}
	g := Build(in, "2026-10-30T00:00:00Z")
	again := Build(in, "2026-10-30T00:00:00Z")
	if !reflect.DeepEqual(g, again) {
		t.Fatal("Build is not deterministic")
	}
	node := map[string]Node{}
	for _, n := range g.Nodes {
		node[n.ID] = n
	}
	l := node["note:lutheranism"]
	if l.Type != "tradition" || l.Pillar != "faith" || l.Recall != "Who founded it?" || l.Degree != 4 {
		t.Fatalf("lutheranism %+v", l)
	}
	if n := node["note:martin-luther"]; n.Type != "concept" || n.Title != "Martin Luther" || n.Pillar != "faith" || n.Stub {
		t.Fatalf("luther %+v", n)
	}
	if n := node["note:protestantism"]; !n.Stub {
		t.Fatalf("protestantism should be a stub: %+v", n)
	}
	if n := node["note:study"]; !n.Private || n.Type != "subject" || n.Pillar != "competence" || n.Title != "Study" {
		t.Fatalf("study %+v", n)
	}
	edges := map[string]bool{}
	for _, e := range g.Edges {
		edges[e.From+" "+e.Type+" "+e.To] = true
	}
	for _, want := range []string{
		"note:lutheranism branch_of note:protestantism",
		"note:lutheranism founded_by note:martin-luther",
		"note:lutheranism related note:calvinism",
		"note:lutheranism mentions note:weber",
		"note:lutheranism in_pillar pillar:competence",
		"quest:drill trains skill:typing",
		"skill:typing in_pillar pillar:competence",
	} {
		if !edges[want] {
			t.Errorf("missing %s", want)
		}
	}
	if edges["note:lutheranism mentions note:martin-luther"] {
		t.Error("typed edge repeated as mentions")
	}
	var msgs []string
	for _, p := range g.Errors {
		msgs = append(msgs, p.ID+": "+p.Msg)
	}
	all := strings.Join(msgs, "\n")
	for _, want := range []string{"see_also is not an edge type", "martin-luther: no type", "study.md changed since it was logged", "gone.md is missing"} {
		if !strings.Contains(all, want) {
			t.Errorf("errors lack %q:\n%s", want, all)
		}
	}
	s := Summarize(g, "2026-10-24")
	// A note whose file is missing keeps its node, from the event.
	if s.Notes != 4 || s.New7d != 2 || s.Stubs != 3 || s.Nodes != 9+1+1+4+3 {
		t.Fatalf("summary %+v", s)
	}
}

func TestParseFrontLinkLists(t *testing.T) {
	for _, c := range []struct {
		name, line string
		want       []string
		warn       bool
	}{
		{"single bare link", "authored: [[ninety-five-theses]]", []string{"[[ninety-five-theses]]"}, false},
		{"bare link with label", "authored: [[ninety-five-theses|95 Theses]]", []string{"[[ninety-five-theses|95 Theses]]"}, false},
		{"bare link with comma in label", "authored: [[a|b, c]]", []string{"[[a|b, c]]"}, false},
		{"bare link with apostrophe", "authored: [[Luther's Theses]]", []string{"[[Luther's Theses]]"}, false},
		{"bare link, comment", "authored: [[a]] # first, maybe", []string{"[[a]]"}, false},
		{"unquoted flow list", "teaches: [[[sola-fide]], [[sola-scriptura]]]", []string{"[[sola-fide]]", "[[sola-scriptura]]"}, false},
		{"unquoted flow list of one", "teaches: [[[sola-fide]]]", []string{"[[sola-fide]]"}, false},
		{"flow list with spaces", "teaches: [ [[a]], [[b]] ]", []string{"[[a]]", "[[b]]"}, false},
		{"flow list with a quoted link", `teaches: [[[a]], "[[b]]"]`, []string{"[[a]]", "[[b]]"}, false},
		{"two bare links", "teaches: [[a]], [[b]]", []string{"[[a]]", "[[b]]"}, false},
		{"quoted flow list", `teaches: ["[[a]]", '[[b|B]]']`, []string{"[[a]]", "[[b|B]]"}, false},
		{"quoted link", `authored: "[[a]]"`, []string{"[[a]]"}, false},
		{"text between links", "teaches: [[a]] and [[b]]", nil, true},
		{"text after link", "teaches: [[a]] (see also)", nil, true},
		{"stray bracket", "teaches: [[[a]], [[b]]", nil, true},
		{"unclosed link", "teaches: [[a]", nil, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, _, warns, err := ParseFront([]byte("---\n" + c.line + "\n---\n"))
			if err != nil {
				t.Fatal(err)
			}
			key, _, _ := strings.Cut(c.line, ":")
			if got := f.Vals[key]; !reflect.DeepEqual(got, c.want) {
				t.Errorf("vals %#v, want %#v", got, c.want)
			}
			if c.warn != (len(warns) > 0) {
				t.Errorf("warns %v, want warning: %v", warns, c.warn)
			}
			if c.warn && !strings.Contains(warns[0], "skipped") {
				t.Errorf("warn %q", warns[0])
			}
		})
	}
	// Block lists keep working, with or without quotes.
	f, _, warns, err := ParseFront([]byte("---\nteaches:\n  - \"[[a]]\"\n  - [[b]]\n---\n"))
	if err != nil || len(warns) != 0 || !reflect.DeepEqual(f.Vals["teaches"], []string{"[[a]]", "[[b]]"}) {
		t.Fatalf("block list: %#v %v %v", f.Vals, warns, err)
	}
}

func TestTargetFiles(t *testing.T) {
	for _, in := range []string{
		"[[diagram.png]]", "[[assets/Diagram.PNG|big]]", "[[paper.pdf#page=3]]", "photo.jpg", "[[clip.mp4]]",
	} {
		if got, ok := Target(in); ok {
			t.Errorf("Target(%q) = %q, want no note", in, got)
		}
		if !isFileLink(in) {
			t.Errorf("isFileLink(%q) = false", in)
		}
	}
	for in, want := range map[string]string{
		"[[sola-fide.md]]":  "note:sola-fide",
		"[[Node.js]]":       "note:node-js",
		"[[Genesis 1.1]]":   "note:genesis-1-1",
		"[[St. Augustine]]": "note:st-augustine",
		"[[nodes/x.md]]":    "note:x",
	} {
		if got, ok := Target(in); !ok || got != want {
			t.Errorf("Target(%q) = %q, %v, want %q", in, got, ok, want)
		}
		if isFileLink(in) {
			t.Errorf("isFileLink(%q) = true", in)
		}
	}
}

func TestStripCode(t *testing.T) {
	for _, c := range []struct{ name, in, gone, kept string }{
		{"inline", "see `[[x]]` and [[y]]", "[[x]]", "[[y]]"},
		{"double backticks", "see ``a ` [[x]]`` and [[y]]", "[[x]]", "[[y]]"},
		{"backtick fence", "a [[y]]\n```go\n[[x]]\n```\nb", "[[x]]", "[[y]]"},
		{"tilde fence", "a [[y]]\n~~~\n[[x]]\n~~~\nb", "[[x]]", "[[y]]"},
		{"longer fence", "````\n```\n[[x]]\n```\n````\n[[y]]", "[[x]]", "[[y]]"},
		{"indented fence", "- item\n  ```\n  [[x]]\n  ```\n  [[y]]", "[[x]]", "[[y]]"},
		{"unclosed fence", "[[y]]\n```\n[[x]]\n", "[[x]]", "[[y]]"},
		{"lone backtick", "it`s [[y]] fine\n\nand `[[z]]`", "[[z]]", "[[y]]"},
		{"span on a fence-like line", "```[[x]]``` [[y]]", "[[x]]", "[[y]]"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := stripCode(c.in)
			if strings.Contains(got, c.gone) || !strings.Contains(got, c.kept) {
				t.Errorf("stripCode(%q) = %q", c.in, got)
			}
		})
	}
}

func TestBuildLinks(t *testing.T) {
	body := "---\ntype: concept\nteaches: [[[a]], [[b]]]\nsee_also: \"[[diagram.png]]\"\nauthored: [[c]]\n---\n" +
		"![[diagram.png]] ![[paper.pdf]] ![[other-note]] [[real]]\n" +
		"`[[inline]]`\n\n```\n[[fenced]]\n```\n\n~~~\n[[tilde]]\n~~~\n"
	g := Build(Input{Notes: []Note{{ID: "note:n", Path: "notes/nodes/n.md", Content: []byte(body)}}}, "")
	edges := map[string]bool{}
	for _, e := range g.Edges {
		edges[e.Via+" "+e.Type+" "+e.To] = true
	}
	for _, want := range []string{
		"frontmatter teaches note:a", "frontmatter teaches note:b", "frontmatter authored note:c",
		"body mentions note:other-note", "body mentions note:real",
	} {
		if !edges[want] {
			t.Errorf("missing %s", want)
		}
	}
	for _, bad := range []string{"note:diagram-png", "note:paper-pdf", "note:inline", "note:fenced", "note:tilde", "note:a-b", "note:a-b-c"} {
		for k := range edges {
			if strings.HasSuffix(k, " "+bad) {
				t.Errorf("unexpected edge %s", k)
			}
		}
		for _, n := range g.Nodes {
			if n.ID == bad {
				t.Errorf("unexpected node %s", n.ID)
			}
		}
	}
	var msgs []string
	for _, p := range g.Errors {
		msgs = append(msgs, p.Msg)
	}
	all := strings.Join(msgs, "\n")
	if !strings.Contains(all, "see_also: [[diagram.png]] is a file, not a note") {
		t.Errorf("errors lack the file problem:\n%s", all)
	}
}
