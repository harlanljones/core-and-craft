// Package event defines the v1 envelope for lines in events.jsonl.
package event

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/harlanljones/core-and-craft/internal/quest"
)

const Version = 1

const (
	PrecisionSecond = "second"
	PrecisionDay    = "day"
)

// Pillars are stored as full lowercase words; initials clash (C, R).
var Pillars = []string{
	"career", "outlook", "relationships", "economic",
	"competence", "recovery", "agents", "faith", "time",
}

// Kinds lists every event kind v1 accepts.
var Kinds = []string{
	"skill.defined",
	"skill.protocol_set",
	"skill.target_set",
	"typing.test",
	"typing.keystats",
	"study.session",
	"note.added",
	"review.logged",
	"event.retracted",
	"quest.added",
	"quest.accepted",
	"quest.declined",
	"quest.done",
	"quest.snoozed",
	"quest.dropped",
	"quest.edited",
	"checkin.asked",
	"checkin.answered",
	"pillar.asked",
	"pillar.answered",
	"day.rested",
}

// Agents that may write. duet:<channel> covers the Claude Duet channels.
var agentExact = []string{"acta", "acta-mt", "claude-code", "hermes", "manual"}

const agentDuetPrefix = "duet:"

// localAgents write only on this machine, through acta itself: Harlan's own
// commands and taps (manual), acta's bookkeeping (acta) and the Monkeytype
// pull (acta-mt). An inbox file comes from a remote agent, so it may not
// claim one of these.
var localAgents = []string{"acta", "acta-mt", "manual"}

// LocalAgent reports whether a is a writer that never arrives by the inbox.
func LocalAgent(a string) bool { return contains(localAgents, a) }

type Source struct {
	Via   string `json:"via"`
	Agent string `json:"agent"`
	Ref   string `json:"ref,omitempty"`
	Note  string `json:"note,omitempty"`
}

type Event struct {
	ID          string          `json:"id"`
	V           int             `json:"v"`
	At          string          `json:"at"`
	AtPrecision string          `json:"at_precision"`
	Seq         int64           `json:"seq,omitempty"`
	RecordedAt  string          `json:"recorded_at"`
	Pillar      string          `json:"pillar"`
	Kind        string          `json:"kind"`
	Subject     string          `json:"subject"`
	Source      Source          `json:"source"`
	Data        json.RawMessage `json:"data"`
	Supersedes  string          `json:"supersedes,omitempty"`
}

var (
	dayRe      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	nativeRe   = regexp.MustCompile(`^evt_[0-9a-f]{16}$`)
	importRe   = regexp.MustCompile(`^[a-z][a-z0-9]*:[A-Za-z0-9_-]+$`)
	subjectRe  = regexp.MustCompile(`^(skill|note|card|quest|checkin|ask):[a-z0-9][a-z0-9-]*$`)
	daySubjRe  = regexp.MustCompile(`^day:\d{4}-\d{2}-\d{2}$`)
	skillRe    = regexp.MustCompile(`^skill:[a-z0-9][a-z0-9-]*$`)
	ruleRe     = regexp.MustCompile(`^(runs|minutes):([1-9][0-9]{0,3})$`)
	notePathRe = regexp.MustCompile(`^notes/([a-z]+)/([a-z0-9][a-z0-9-]*)\.md$`)
	sha256Re   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// NodesDir is the notes folder for nodes every pillar shares (people,
// events, concepts); notes/<pillar>/ holds a pillar's own notes.
const NodesDir = "nodes"

// NotePath splits a note path, notes/<pillar or nodes>/<slug>.md, into its
// folder and slug. ok is false for any other path.
func NotePath(p string) (folder, slug string, ok bool) {
	m := notePathRe.FindStringSubmatch(p)
	if m == nil || (m[1] != NodesDir && !contains(Pillars, m[1])) {
		return "", "", false
	}
	return m[1], m[2], true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ValidID reports whether id is a native evt_ id or an import id like mt:<_id>.
func ValidID(id string) bool {
	return nativeRe.MatchString(id) || (importRe.MatchString(id) && !strings.HasPrefix(id, "evt_"))
}

func validAgent(a string) bool {
	if contains(agentExact, a) {
		return true
	}
	return strings.HasPrefix(a, agentDuetPrefix) && len(a) > len(agentDuetPrefix)
}

// Time parses At in loc. Day-precision events resolve to local midnight.
func (e Event) Time(loc *time.Location) (time.Time, error) {
	if e.AtPrecision == PrecisionDay {
		return time.ParseInLocation("2006-01-02", e.At, loc)
	}
	return time.Parse(time.RFC3339, e.At)
}

// LocalDate returns the event's calendar date in loc as YYYY-MM-DD.
func (e Event) LocalDate(loc *time.Location) string {
	if e.AtPrecision == PrecisionDay {
		return e.At
	}
	t, err := e.Time(loc)
	if err != nil {
		return ""
	}
	return t.In(loc).Format("2006-01-02")
}

// Validate checks one event. requireSeq is false for inbox files, where
// acta assigns seq on ingest.
func Validate(e Event, requireSeq bool) error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if e.V != Version {
		add("v: want %d, got %d", Version, e.V)
	}
	if !ValidID(e.ID) {
		add("id: %q is neither evt_<16 hex> nor <source>:<id>", e.ID)
	}
	switch e.AtPrecision {
	case PrecisionDay:
		if !dayRe.MatchString(e.At) {
			add("at: day precision needs YYYY-MM-DD, got %q", e.At)
		}
	case PrecisionSecond:
		if _, err := time.Parse(time.RFC3339, e.At); err != nil {
			add("at: want RFC 3339 with offset, got %q", e.At)
		}
	default:
		add("at_precision: want second or day, got %q", e.AtPrecision)
	}
	if requireSeq && e.Seq < 1 {
		add("seq: must be >= 1")
	}
	if _, err := time.Parse(time.RFC3339, e.RecordedAt); err != nil {
		add("recorded_at: want RFC 3339, got %q", e.RecordedAt)
	}
	if !contains(Pillars, e.Pillar) {
		add("pillar: unknown %q", e.Pillar)
	}
	if !contains(Kinds, e.Kind) {
		add("kind: unknown %q", e.Kind)
	}
	switch {
	case e.Kind == "event.retracted":
		if !ValidID(e.Subject) {
			add("subject: event.retracted needs the target event id, got %q", e.Subject)
		}
	case e.Kind == "day.rested":
		if !daySubjRe.MatchString(e.Subject) {
			add("subject: day.rested needs day:YYYY-MM-DD, got %q", e.Subject)
		}
	case !subjectRe.MatchString(e.Subject):
		add("subject: want skill:|note:|card:|quest:|checkin:|ask:<slug>, got %q", e.Subject)
	case strings.HasPrefix(e.Kind, "quest.") && !strings.HasPrefix(e.Subject, "quest:"):
		add("subject: %s needs quest:<slug>, got %q", e.Kind, e.Subject)
	case strings.HasPrefix(e.Kind, "checkin.") && !strings.HasPrefix(e.Subject, "checkin:"):
		add("subject: %s needs checkin:<slug>, got %q", e.Kind, e.Subject)
	case strings.HasPrefix(e.Kind, "pillar.") && !strings.HasPrefix(e.Subject, "ask:"):
		add("subject: %s needs ask:<slug>, got %q", e.Kind, e.Subject)
	case e.Kind == "review.logged" && !strings.HasPrefix(e.Subject, "card:"):
		add("subject: review.logged needs card:<slug>, got %q", e.Subject)
	}
	if e.Source.Via == "" {
		add("source.via: required")
	}
	if !validAgent(e.Source.Agent) {
		add("source.agent: %q is not an allowed writer", e.Source.Agent)
	}
	d := bytes.TrimSpace(e.Data)
	if len(d) == 0 || d[0] != '{' || !json.Valid(d) {
		add("data: must be a JSON object")
	} else {
		errs = append(errs, validateData(e)...)
	}
	if e.Supersedes != "" && !ValidID(e.Supersedes) {
		add("supersedes: %q is not an event id", e.Supersedes)
	}
	return errors.Join(errs...)
}

// canonical re-encodes v with sorted object keys, compact separators and
// no HTML escaping. Numbers keep their literal form.
func canonical(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// NativeID is evt_ plus the first 16 hex chars of sha256 over the canonical
// JSON of {agent, at, data, kind, recorded_at, subject}. seq is left out so
// remote writers can name an event before acta assigns its seq.
func NativeID(e Event) (string, error) {
	data := e.Data
	if len(bytes.TrimSpace(data)) == 0 {
		data = json.RawMessage("{}")
	}
	body, err := json.Marshal(map[string]any{
		"agent":       e.Source.Agent,
		"at":          e.At,
		"data":        data,
		"kind":        e.Kind,
		"recorded_at": e.RecordedAt,
		"subject":     e.Subject,
	})
	if err != nil {
		return "", err
	}
	c, err := canonical(body)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(c)
	return "evt_" + hex.EncodeToString(sum[:])[:16], nil
}

// Marshal encodes e as one compact line without a trailing newline.
func Marshal(e Event) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// validateData checks the data of the kinds whose fields acta relies on.
// Other kinds only need a JSON object.
func validateData(e Event) []error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf("data."+format, a...)) }
	var m map[string]json.RawMessage
	if err := json.Unmarshal(e.Data, &m); err != nil {
		return []error{fmt.Errorf("data: %v", err)}
	}
	str := func(k string) (string, bool) {
		var v string
		raw, ok := m[k]
		if !ok {
			return "", false
		}
		if json.Unmarshal(raw, &v) != nil {
			add("%s: must be a string", k)
			return "", false
		}
		return v, true
	}
	text := func(k string, max int) {
		v, ok := str(k)
		if !ok || strings.TrimSpace(v) == "" {
			add("%s: required", k)
		} else if len(v) > max {
			add("%s: longer than %d bytes", k, max)
		}
	}
	priority := func() {
		raw, ok := m["priority"]
		if !ok {
			return
		}
		var p int
		if json.Unmarshal(raw, &p) != nil || p < 1 || p > 3 {
			add("priority: want 1, 2 or 3, got %s", raw)
		}
	}
	every := func() {
		if v, ok := str("every"); ok {
			if _, err := quest.Parse(v); err != nil {
				add("%v", err)
			}
		}
	}

	switch e.Kind {
	case "note.added":
		// path and sha256 bind the event to one version of a note file; the
		// inbox places that file (docs/knowledge-graph.md). Older lines may
		// carry neither.
		p, hasPath := str("path")
		sum, hasSum := str("sha256")
		if hasPath {
			_, slug, ok := NotePath(p)
			switch {
			case !ok:
				add("path: want notes/<pillar>/<slug>.md or notes/nodes/<slug>.md, got %q", p)
			case e.Subject != "note:"+slug:
				add("path: %s is not the file of %s", p, e.Subject)
			}
			if !hasSum {
				add("sha256: required with path")
			}
		}
		if hasSum && !sha256Re.MatchString(sum) {
			add("sha256: want 64 lowercase hex characters")
		}
		if hasSum && !hasPath {
			add("path: required with sha256")
		}
		if _, ok := m["title"]; ok {
			text("title", 200)
		}
	case "review.logged":
		// One FSRS review of card:<slug>, the recall: line of note:<slug>.
		raw, ok := m["rating"]
		var r float64
		switch {
		case !ok:
			add("rating: required")
		case json.Unmarshal(raw, &r) != nil || r != float64(int(r)) || r < 1 || r > 4:
			add("rating: want 1 (again), 2 (hard), 3 (good) or 4 (easy), got %s", raw)
		}
		if raw, ok := m["duration_ms"]; ok {
			var ms float64
			if json.Unmarshal(raw, &ms) != nil || ms != float64(int64(ms)) || ms < 0 || ms > 86400000 {
				add("duration_ms: want whole milliseconds, 0 to a day, got %s", raw)
			}
		}
		if c, ok := str("card"); ok && c != e.Subject && "card:"+c != e.Subject {
			add("card: %q is not %s", c, e.Subject)
		}
	case "quest.added":
		text("title", 200)
		priority()
		every()
		skill, hasSkill := str("skill")
		if hasSkill && !skillRe.MatchString(skill) {
			add("skill: want skill:<slug>, got %q", skill)
		}
		if rule, ok := str("rule"); ok {
			if !hasSkill {
				add("rule: needs a skill")
			}
			if !ruleRe.MatchString(rule) {
				add("rule: want runs:N or minutes:N, got %q", rule)
			}
		}
		if status, ok := str("status"); ok {
			switch {
			case status != "active" && status != "proposed":
				add("status: want active or proposed, got %q", status)
			case status == "active" && e.Source.Agent != "manual":
				// Only Harlan's own quests start active; agents propose.
				add("status: only manual quests start active; %s proposes", e.Source.Agent)
			}
		}
	case "quest.snoozed":
		if v, _ := str("until"); !dayRe.MatchString(v) {
			add("until: want YYYY-MM-DD, got %q", v)
		}
	case "quest.edited":
		_, t := m["title"]
		_, p := m["priority"]
		_, ev := m["every"]
		if !t && !p && !ev {
			add("edited: change at least one of title, priority, every")
		}
		if t {
			text("title", 200)
		}
		priority()
		every()
	case "checkin.asked":
		text("question", 300)
	case "checkin.answered":
		text("text", 2000)
	case "pillar.asked":
		// Harlan asks a pillar's Duet agent; the board relays it (acta relay).
		text("text", 2000)
		if e.Source.Agent != "manual" {
			add("asked: only manual asks a pillar; %s answers", e.Source.Agent)
		}
	case "pillar.answered":
		text("text", 2000)
		// A channel speaks for its own pillar: duet:time answers time's asks.
		if e.Source.Agent != agentDuetPrefix+e.Pillar {
			add("answered: only duet:%s answers a %s ask, got %s", e.Pillar, e.Pillar, e.Source.Agent)
		}
	}
	return errs
}
