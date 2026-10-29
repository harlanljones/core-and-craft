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
}

// Agents that may write. duet:<channel> covers the Claude Duet channels.
var agentExact = []string{"acta", "acta-mt", "claude-code", "hermes", "manual"}

const agentDuetPrefix = "duet:"

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
	dayRe     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	nativeRe  = regexp.MustCompile(`^evt_[0-9a-f]{16}$`)
	importRe  = regexp.MustCompile(`^[a-z][a-z0-9]*:[A-Za-z0-9_-]+$`)
	subjectRe = regexp.MustCompile(`^(skill|note|card):[a-z0-9][a-z0-9-]*$`)
)

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
	if e.Kind == "event.retracted" {
		if !ValidID(e.Subject) {
			add("subject: event.retracted needs the target event id, got %q", e.Subject)
		}
	} else if !subjectRe.MatchString(e.Subject) {
		add("subject: want skill:|note:|card:<slug>, got %q", e.Subject)
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
