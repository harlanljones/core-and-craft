// Package relay carries what Harlan does on the board back to the Claude Duet
// pillar agents. The inbox brings their events in; relay takes Harlan's
// answers out: a question to a pillar (pillar.asked), an answer to a
// pillar's check-in, a yes or no on a pillar's proposed quest.
//
// Each pillar channel keeps a routine with an API trigger. acta POSTs the
// items to that routine's fire URL with its bearer token, and the routine
// wakes the channel's Claude with them. Nothing here writes events.jsonl:
// what was sent lives in $ACTA_STATE/relay.json.
package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
)

// FileName is the sent record in the state directory.
const FileName = "relay.json"

// Host is where every fire goes. A route that names any other host is
// refused, so a token is never sent anywhere else.
const Host = "https://api.anthropic.com"

// MaxItems bounds one fire. A routine takes 30 fires an hour; the rest wait
// for the next run.
const MaxItems = 20

// MaxText is the fire endpoint's limit on text, in characters.
const MaxText = 65536

// Header opens every payload, so a routine can tell the board's message
// from anything else.
const Header = "acta relay v1"

var fireRe = regexp.MustCompile(`^https://api\.anthropic\.com/v1/claude_code/routines/trig_[A-Za-z0-9]+/fire$`)

// ValidURL reports whether u is a routine fire URL on api.anthropic.com.
func ValidURL(u string) bool { return fireRe.MatchString(u) }

// Re is what an item answers: the check-in's question or the quest's title.
type Re struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Question string `json:"question,omitempty"`
	Title    string `json:"title,omitempty"`
}

// Item is one event bound for one pillar's agent.
type Item struct {
	Event event.Event `json:"event"`
	Re    *Re         `json:"re,omitempty"`
	// To is the channel, a pillar word: duet:<To> is the agent.
	To string `json:"-"`
}

func dataString(raw json.RawMessage, key string) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// channelOf returns the pillar a duet:<channel> agent speaks for, or "".
func channelOf(agent string) string {
	c, ok := strings.CutPrefix(agent, "duet:")
	if !ok {
		return ""
	}
	for _, p := range event.Pillars {
		if p == c {
			return c
		}
	}
	return ""
}

// Select picks, in log order, Harlan's own events that a pillar agent is
// waiting on. live must already be effective (retractions applied), so a
// retracted ask is never sent.
func Select(live []event.Event) []Item {
	asked := map[string]event.Event{}
	added := map[string]event.Event{}
	var out []Item
	for _, e := range live {
		switch e.Kind {
		case "checkin.asked":
			if _, ok := asked[e.Subject]; !ok {
				asked[e.Subject] = e
			}
		case "quest.added":
			if _, ok := added[e.Subject]; !ok {
				added[e.Subject] = e
			}
		}
		if e.Source.Agent != "manual" {
			continue
		}
		switch e.Kind {
		case "pillar.asked":
			out = append(out, Item{Event: e, To: e.Pillar})
		case "checkin.answered":
			q, ok := asked[e.Subject]
			if to := channelOf(q.Source.Agent); ok && to != "" {
				out = append(out, Item{Event: e, To: to, Re: &Re{Kind: q.Kind, ID: q.ID, Question: dataString(q.Data, "question")}})
			}
		case "quest.accepted", "quest.declined":
			q, ok := added[e.Subject]
			if to := channelOf(q.Source.Agent); ok && to != "" {
				out = append(out, Item{Event: e, To: to, Re: &Re{Kind: q.Kind, ID: q.ID, Title: dataString(q.Data, "title")}})
			}
		}
	}
	return out
}

// Payload is the fire text: the header line, then one JSON item per line.
func Payload(to string, items []Item) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s · to duet:%s · %d item(s) from Harlan's CORE & CRAFT board.\n", Header, to, len(items))
	b.WriteString(`Each line below is one JSON item: {"event": <envelope v1 event acta logged>, "re": <the check-in or quest it answers, if any>}.` + "\n")
	for _, it := range items {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(it); err != nil {
			return "", err
		}
		b.Write(buf.Bytes())
	}
	return b.String(), nil
}

// Batch takes the first items that fit in one fire: at most MaxItems and
// MaxText characters. It always takes at least one, which fits because an
// event's text is capped at 2,000 bytes.
func Batch(to string, items []Item) ([]Item, string, error) {
	n := min(len(items), MaxItems)
	for ; n > 0; n-- {
		text, err := Payload(to, items[:n])
		if err != nil {
			return nil, "", err
		}
		if len([]rune(text)) <= MaxText || n == 1 {
			return items[:n], text, nil
		}
	}
	return nil, "", nil
}

// Sent records one delivered event.
type Sent struct {
	At      string `json:"at"`
	To      string `json:"to"`
	Session string `json:"session"`
}

// State is relay.json.
type State struct {
	V    int             `json:"v"`
	Sent map[string]Sent `json:"sent"`
}

// Load reads the state. found is false when there is none yet.
func Load(path string) (s State, found bool, err error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{V: 1, Sent: map[string]Sent{}}, false, nil
	}
	if err != nil {
		return s, false, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, true, fmt.Errorf("%s: %w", path, err)
	}
	if s.Sent == nil {
		s.Sent = map[string]Sent{}
	}
	return s, true, nil
}

// Save replaces the state atomically (temp file, fsync, rename).
func Save(path string, s State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".relay-*.json")
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

// Lock takes an exclusive flock on <path>.lock, so a manual run and the unit
// never send the same item twice. Call the returned func to release it.
func Lock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// Route is one pillar routine's fire URL and bearer token.
type Route struct {
	URL   string
	Token string
}

// RateLimited is a 429: the routine or the account used its hourly fires.
type RateLimited struct{ RetryAfter string }

func (r RateLimited) Error() string {
	if r.RetryAfter == "" {
		return "rate limited"
	}
	return "rate limited, retry after " + r.RetryAfter + "s"
}

// Client fires routines.
type Client struct {
	HTTP *http.Client
	// BaseURL replaces Host in the request, for tests. Routes are still
	// checked against Host.
	BaseURL string
}

// Fire POSTs text to the route and returns the session the run started.
// Errors never include the token.
func (c Client) Fire(ctx context.Context, r Route, text string) (string, error) {
	if !ValidURL(r.URL) {
		return "", fmt.Errorf("route %q is not a routine fire URL on %s", r.URL, Host)
	}
	if r.Token == "" {
		return "", errors.New("no token")
	}
	url := r.URL
	if c.BaseURL != "" {
		url = c.BaseURL + strings.TrimPrefix(url, Host)
	}
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("fire: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", RateLimited{RetryAfter: resp.Header.Get("Retry-After")}
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Error.Type != "" {
			return "", fmt.Errorf("fire: %d %s: %s", resp.StatusCode, e.Error.Type, e.Error.Message)
		}
		return "", fmt.Errorf("fire: HTTP %d", resp.StatusCode)
	}
	var ok struct {
		Session string `json:"claude_code_session_id"`
	}
	if err := json.Unmarshal(raw, &ok); err != nil || ok.Session == "" {
		return "", fmt.Errorf("fire: unexpected reply %.200q", raw)
	}
	return ok.Session, nil
}
