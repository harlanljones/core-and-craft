// Package monkeytype pulls test results with an ApeKey and maps them to
// typing.test events. The key only ever travels in the Authorization
// header: never in a URL, a log line or an error.
package monkeytype

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
)

const (
	DefaultBaseURL = "https://api.monkeytype.com"
	// PageSize matches Monkeytype's documented batch cap of 1,000.
	PageSize = 1000
	// sinceParam is the lower-bound filter (GetResultsQuerySchema in
	// Monkeytype's packages/contracts/src/results.ts). The query schema is
	// strict, so an unknown name fails the request rather than being ignored.
	sinceParam = "onOrAfterTimestamp"
)

type Client struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
}

// Error carries only the status and Monkeytype's message field.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("monkeytype: HTTP %d", e.Status)
	}
	return fmt.Sprintf("monkeytype: HTTP %d: %s", e.Status, e.Message)
}

var ErrNoKey = errors.New("MONKEYTYPE_APE_KEY is not set")

type envelope struct {
	Message string            `json:"message"`
	Data    []json.RawMessage `json:"data"`
}

// Results returns every result at or after since (zero means all the
// account exposes), oldest first, paging by offset until a short page or
// the account's cap (HTTP 422 past the cap ends paging cleanly).
func (c *Client) Results(ctx context.Context, since time.Time) ([]map[string]any, error) {
	if c.Key == "" {
		return nil, ErrNoKey
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	var out []map[string]any
	for offset := 0; ; offset += PageSize {
		q := url.Values{}
		q.Set("limit", strconv.Itoa(PageSize))
		q.Set("offset", strconv.Itoa(offset))
		if !since.IsZero() {
			q.Set(sinceParam, strconv.FormatInt(since.UnixMilli(), 10))
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/results?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "ApeKey "+c.Key)
		req.Header.Set("Accept", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			// url.Error repeats the URL, which holds no key; drop it anyway
			// to keep errors short.
			var ue *url.Error
			if errors.As(err, &ue) {
				return nil, fmt.Errorf("monkeytype: request failed: %v", ue.Err)
			}
			return nil, fmt.Errorf("monkeytype: request failed")
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("monkeytype: read body: %v", err)
		}
		var env envelope
		_ = json.Unmarshal(body, &env)
		if resp.StatusCode == http.StatusUnprocessableEntity && offset > 0 {
			break
		}
		if resp.StatusCode != http.StatusOK {
			// Redact in case the server ever echoes the key back.
			return nil, &Error{Status: resp.StatusCode, Message: strings.ReplaceAll(env.Message, c.Key, "[redacted]")}
		}
		for _, raw := range env.Data {
			var m map[string]any
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.UseNumber()
			if err := dec.Decode(&m); err != nil {
				return nil, fmt.Errorf("monkeytype: decode result: %v", err)
			}
			out = append(out, m)
		}
		if len(env.Data) < PageSize {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return millis(out[i]) < millis(out[j]) })
	return out, nil
}

// keep lists the result fields copied into typing.test data.
var keep = []string{
	"wpm", "rawWpm", "acc", "consistency", "mode", "mode2", "language",
	"punctuation", "numbers", "difficulty", "lazyMode", "funbox",
	"testDuration", "charStats", "isPb", "bailedOut", "restartCount",
	"incompleteTestSeconds",
}

// defaults are the values the API leaves out of a stored result (see
// buildDbResult in Monkeytype's backend/src/utils/result.ts). ToEvent puts
// them back so the event says what was run and protocol matching works.
func defaults() map[string]any {
	return map[string]any{
		"language": "english", "punctuation": false, "numbers": false,
		"lazyMode": false, "bailedOut": false, "difficulty": "normal",
		"funbox": []any{}, "isPb": false, "restartCount": 0,
		"incompleteTestSeconds": 0,
	}
}

func millis(m map[string]any) int64 {
	switch v := m["timestamp"].(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
		if f, err := v.Float64(); err == nil {
			return int64(f)
		}
	case float64:
		return int64(v)
	}
	return 0
}

// ToEvent maps one result to a typing.test event with id mt:<_id>.
func ToEvent(m map[string]any, loc *time.Location, now time.Time) (event.Event, error) {
	id, _ := m["_id"].(string)
	if id == "" {
		return event.Event{}, errors.New("monkeytype: result without _id")
	}
	ms := millis(m)
	if ms == 0 {
		return event.Event{}, fmt.Errorf("monkeytype: result %s without timestamp", id)
	}
	data := defaults()
	for _, k := range keep {
		if v, ok := m[k]; ok {
			data[k] = v
		}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return event.Event{}, err
	}
	return event.Event{
		ID:          "mt:" + id,
		V:           event.Version,
		At:          time.UnixMilli(ms).In(loc).Format(time.RFC3339),
		AtPrecision: event.PrecisionSecond,
		RecordedAt:  now.UTC().Format(time.RFC3339),
		Pillar:      "competence",
		Kind:        "typing.test",
		Subject:     "skill:typing",
		Source:      event.Source{Via: "monkeytype-api", Agent: "acta-mt", Ref: id},
		Data:        raw,
	}, nil
}
