package monkeytype

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

const secret = "ape-SECRET-do-not-print"

func result(i int, ts int64) map[string]any {
	return map[string]any{
		"_id": fmt.Sprintf("r%04d", i), "timestamp": ts, "wpm": 80.5, "rawWpm": 82.1,
		"acc": 97.2, "consistency": 78.4, "mode": "time", "mode2": "60",
		"language": "english", "punctuation": false, "numbers": false,
		"funbox": []string{}, "bailedOut": false, "charStats": []int{400, 3, 1, 0},
		"uid": "should-not-be-copied",
	}
}

func server(t *testing.T, total int, status int, msg string) (*httptest.Server, *[]string) {
	t.Helper()
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "ApeKey "+secret {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"Invalid ApeKey"}`)
			return
		}
		if strings.Contains(r.URL.String(), secret) {
			t.Error("key leaked into the URL")
		}
		queries = append(queries, r.URL.RawQuery)
		if status != http.StatusOK {
			w.WriteHeader(status)
			fmt.Fprintf(w, `{"message":%q}`, msg)
			return
		}
		off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		var data []map[string]any
		for i := off; i < total && i < off+lim; i++ {
			// Newest first, like the API.
			data = append(data, result(i, int64(1_761_700_000_000-i*60_000)))
		}
		json.NewEncoder(w).Encode(map[string]any{"message": "Results retrieved", "data": data})
	}))
	t.Cleanup(srv.Close)
	return srv, &queries
}

func TestResultsPagesAndSortsOldestFirst(t *testing.T) {
	srv, queries := server(t, 2*PageSize+5, http.StatusOK, "")
	c := Client{BaseURL: srv.URL, Key: secret}
	got, err := c.Results(context.Background(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2*PageSize+5 {
		t.Fatalf("got %d results", len(got))
	}
	if len(*queries) != 3 {
		t.Fatalf("want 3 pages, got %d", len(*queries))
	}
	if millis(got[0]) > millis(got[len(got)-1]) {
		t.Fatal("not oldest first")
	}
	if strings.Contains((*queries)[0], sinceParam) {
		t.Fatal("full pull should not send a lower bound")
	}
}

func TestResultsSendsLowerBound(t *testing.T) {
	srv, queries := server(t, 3, http.StatusOK, "")
	c := Client{BaseURL: srv.URL, Key: secret}
	since := time.UnixMilli(1_761_600_000_000)
	if _, err := c.Results(context.Background(), since); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains((*queries)[0], sinceParam+"=1761600000000") {
		t.Fatalf("query %q", (*queries)[0])
	}
}

func TestErrorsNeverContainKey(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError} {
		srv, _ := server(t, 0, status, "rate limited, key "+secret)
		c := Client{BaseURL: srv.URL, Key: secret}
		_, err := c.Results(context.Background(), time.Time{})
		if err == nil {
			t.Fatalf("status %d: want error", status)
		}
		// Even a server that echoes the key back must not get it printed.
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("key repeated in error: %v", err)
		}
	}
	bad := Client{BaseURL: "http://127.0.0.1:1", Key: secret}
	if _, err := bad.Results(context.Background(), time.Time{}); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("network error leaked key or passed: %v", err)
	}
}

func TestErrorHasStatusAndMessageOnly(t *testing.T) {
	srv, _ := server(t, 0, http.StatusTooManyRequests, "Too many requests")
	c := Client{BaseURL: srv.URL, Key: secret}
	_, err := c.Results(context.Background(), time.Time{})
	if err == nil || err.Error() != "monkeytype: HTTP 429: Too many requests" {
		t.Fatalf("got %v", err)
	}
}

func TestNoKey(t *testing.T) {
	if _, err := (&Client{}).Results(context.Background(), time.Time{}); err != ErrNoKey {
		t.Fatalf("got %v", err)
	}
}

func TestToEvent(t *testing.T) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	raw, _ := json.Marshal(result(7, 1_761_700_000_000))
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	dec.Decode(&m)
	ev, err := ToEvent(m, loc, time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID != "mt:r0007" || ev.Kind != "typing.test" || ev.Subject != "skill:typing" || ev.Source.Ref != "r0007" {
		t.Fatalf("got %+v", ev)
	}
	if !strings.HasSuffix(ev.At, "-07:00") {
		t.Fatalf("at should carry the local offset: %s", ev.At)
	}
	if strings.Contains(string(ev.Data), "uid") {
		t.Fatal("copied a field outside the keep list")
	}
	if !strings.Contains(string(ev.Data), `"rawWpm":82.1`) {
		t.Fatalf("data %s", ev.Data)
	}
}

// The API returns results as stored, with default settings left out
// (backend/src/utils/result.ts buildDbResult). ToEvent puts them back so
// the run can match a protocol.
func TestToEventFillsOmittedDefaults(t *testing.T) {
	m := map[string]any{
		"_id": "r0001", "timestamp": json.Number("1761700000000"),
		"wpm": json.Number("90"), "acc": json.Number("98"), "mode": "time", "mode2": "60",
	}
	ev, err := ToEvent(m, time.UTC, time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(ev.Data, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"language": "english", "punctuation": false, "numbers": false,
		"lazyMode": false, "bailedOut": false, "difficulty": "normal",
		"isPb": false, "restartCount": float64(0), "incompleteTestSeconds": float64(0),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %v, want %v", k, got[k], v)
		}
	}
	if fb, ok := got["funbox"].([]any); !ok || len(fb) != 0 {
		t.Errorf("funbox: got %v, want []", got["funbox"])
	}
}

func TestToEventKeepsNonDefaults(t *testing.T) {
	r := result(1, 1_761_700_000_000)
	r["language"], r["punctuation"], r["funbox"] = "german", true, []string{"mirror"}
	raw, _ := json.Marshal(r)
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	dec.Decode(&m)
	ev, err := ToEvent(m, time.UTC, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`"language":"german"`, `"punctuation":true`, `"funbox":["mirror"]`} {
		if !strings.Contains(string(ev.Data), s) {
			t.Errorf("data %s lacks %s", ev.Data, s)
		}
	}
}
