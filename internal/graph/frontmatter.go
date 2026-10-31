// Package graph reads note frontmatter and folds notes, skills, quests and
// the nine pillars into graph.json. acta builds it next to the snapshot; the
// notes and the log stay the source of truth.
package graph

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// Front is a note's frontmatter. acta reads the part of YAML that Obsidian's
// properties write: `key: value`, quoted scalars, flow lists `[a, b]` and
// block lists of `- item`. Nested maps are skipped with a warning.
type Front struct {
	Keys []string            // in file order
	Vals map[string][]string // a scalar is a one-item list
}

// Get returns the first value of key, or "".
func (f Front) Get(key string) string {
	if v := f.Vals[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

var keyRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*):(.*)$`)

// ParseFront splits a note into frontmatter and body. A note with no
// frontmatter has an empty Front. warns lists lines acta skipped; err means
// the block never closes or a line can't be read.
func ParseFront(b []byte) (f Front, body []byte, warns []string, err error) {
	f = Front{Vals: map[string][]string{}}
	b = bytes.TrimPrefix(b, []byte("\ufeff"))
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return f, []byte(text), nil, nil
	}
	lines := strings.Split(text[len("---\n"):], "\n")
	end := -1
	for i, l := range lines {
		if t := strings.TrimRight(l, " \t"); t == "---" || t == "..." {
			end = i
			break
		}
	}
	if end < 0 {
		return f, nil, nil, fmt.Errorf("frontmatter: no closing ---")
	}
	body = []byte(strings.Join(lines[end+1:], "\n"))
	cur := "" // the key whose block list is open
	for n, l := range lines[:end] {
		line := n + 2 // 1-based, after the opening ---
		trim := strings.TrimSpace(l)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasPrefix(trim, "- ") || trim == "-" {
			if cur == "" {
				return f, nil, warns, fmt.Errorf("frontmatter line %d: list item with no key", line)
			}
			v, err := scalar(strings.TrimSpace(strings.TrimPrefix(trim, "-")))
			if err != nil {
				return f, nil, warns, fmt.Errorf("frontmatter line %d: %v", line, err)
			}
			if v != "" {
				f.Vals[cur] = append(f.Vals[cur], v)
			}
			continue
		}
		if l != strings.TrimLeft(l, " \t") {
			if cur != "" {
				warns = append(warns, fmt.Sprintf("frontmatter line %d: %s: nested values aren't read", line, cur))
			}
			continue
		}
		m := keyRe.FindStringSubmatch(l)
		if m == nil {
			return f, nil, warns, fmt.Errorf("frontmatter line %d: want key: value", line)
		}
		k, raw := m[1], strings.TrimSpace(m[2])
		if _, dup := f.Vals[k]; dup {
			return f, nil, warns, fmt.Errorf("frontmatter line %d: %s appears twice", line, k)
		}
		f.Keys = append(f.Keys, k)
		f.Vals[k] = nil
		cur = ""
		switch {
		case raw == "":
			cur = k
		case strings.HasPrefix(raw, "[") && !strings.HasPrefix(raw, "[["):
			items, err := flowList(raw)
			if err != nil {
				return f, nil, warns, fmt.Errorf("frontmatter line %d: %s: %v", line, k, err)
			}
			f.Vals[k] = items
		default:
			v, err := scalar(raw)
			if err != nil {
				return f, nil, warns, fmt.Errorf("frontmatter line %d: %s: %v", line, k, err)
			}
			if v != "" {
				f.Vals[k] = []string{v}
			}
		}
	}
	return f, body, warns, nil
}

// scalar reads one value: "double", 'single' or plain (a trailing # comment
// after a space is dropped).
func scalar(s string) (string, error) {
	switch {
	case strings.HasPrefix(s, `"`):
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			c := s[i]
			switch {
			case c == '\\' && i+1 < len(s):
				i++
				switch s[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					b.WriteByte(s[i])
				}
			case c == '"':
				if rest := strings.TrimSpace(s[i+1:]); rest != "" && !strings.HasPrefix(rest, "#") {
					return "", fmt.Errorf("text after closing quote")
				}
				return b.String(), nil
			default:
				b.WriteByte(c)
			}
		}
		return "", fmt.Errorf("unclosed quote")
	case strings.HasPrefix(s, "'"):
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			if s[i] == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				if rest := strings.TrimSpace(s[i+1:]); rest != "" && !strings.HasPrefix(rest, "#") {
					return "", fmt.Errorf("text after closing quote")
				}
				return b.String(), nil
			}
			b.WriteByte(s[i])
		}
		return "", fmt.Errorf("unclosed quote")
	}
	if i := strings.Index(s, " #"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s), nil
}

// flowList reads [a, "b", [[c]]]. Commas inside quotes or [[links]] don't split.
func flowList(s string) ([]string, error) {
	if !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("unclosed [")
	}
	inner := s[1 : len(s)-1]
	var items []string
	var cur strings.Builder
	depth := 0
	var quote byte
	flush := func() error {
		t := strings.TrimSpace(cur.String())
		cur.Reset()
		if t == "" {
			return nil
		}
		v, err := scalar(t)
		if err != nil {
			return err
		}
		items = append(items, v)
		return nil
	}
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[':
			depth++
		case c == ']':
			depth--
		case c == ',' && depth == 0:
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		cur.WriteByte(c)
	}
	if quote != 0 || depth != 0 {
		return nil, fmt.Errorf("unbalanced list")
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return items, nil
}
