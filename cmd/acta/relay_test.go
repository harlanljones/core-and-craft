package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/harlanljones/core-and-craft/internal/relay"
)

const relayToken = "sk-ant-oat01-RELAY-SECRET"

// fireServer stands in for the routine fire endpoint and keeps each text.
type fireServer struct {
	mu     sync.Mutex
	texts  map[string][]string // routine id -> texts
	status int
}

func newFireServer(t *testing.T) (*fireServer, *httptest.Server) {
	f := &fireServer{texts: map[string][]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+relayToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if f.status != 0 {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(f.status)
			return
		}
		var body struct{ Text string }
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		trig := strings.Split(r.URL.Path, "/")[4]
		f.texts[trig] = append(f.texts[trig], body.Text)
		io.WriteString(w, `{"type":"routine_fire","claude_code_session_id":"session_`+trig+`"}`)
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func fireURL(trig string) string {
	return "https://api.anthropic.com/v1/claude_code/routines/" + trig + "/fire"
}

func TestAskAndRelay(t *testing.T) {
	f, srv := newFireServer(t)
	vars := map[string]string{
		"ACTA_RELAY_RECOVERY":       fireURL("trig_rec"),
		"ACTA_RELAY_RECOVERY_TOKEN": relayToken,
	}
	e, out, _ := testEnv(t, vars)
	e.relayBaseURL = srv.URL
	mustRun(t, e, "init")

	// Before relay ever ran: an old answer is history, not news.
	old := runOut(t, e, "checkin", "ask", "--pillar", "recovery", "--question", "Old?", "--agent", "duet:recovery")
	oldID, _, _ := strings.Cut(old, " ")
	mustRun(t, e, "checkin", "answer", oldID, "--text", "long ago", "--via", "board")
	if got := runOut(t, e, "relay"); !strings.Contains(got, "first run, 1 earlier item(s) marked as sent") {
		t.Fatalf("first run %q", got)
	}

	ask := runOut(t, e, "ask", "--pillar", "recovery", "--text", "When should screens go off?", "--via", "board")
	askID, _, _ := strings.Cut(ask, " ")
	if !strings.HasPrefix(askID, "ask:") || !strings.HasSuffix(ask, "asked duet:recovery") {
		t.Fatalf("ask %q", ask)
	}
	q := runOut(t, e, "quest", "add", "--pillar", "faith", "--title", "Read one lesson", "--agent", "duet:faith")
	qid := questID(t, q)
	mustRun(t, e, "quest", "accept", qid, "--via", "board")

	dry := runOut(t, e, "relay", "--dry-run")
	if !strings.Contains(dry, "--- recovery (1 waiting, route: true)") || !strings.Contains(dry, "--- faith (1 waiting, route: false)") {
		t.Fatalf("dry run %q", dry)
	}
	if len(f.texts) != 0 {
		t.Fatal("dry run fired")
	}

	got := runOut(t, e, "relay")
	if !strings.Contains(got, "relay: recovery: sent 1, session_trig_rec") || !strings.Contains(got, "relay: faith: 1 waiting, no route (ACTA_RELAY_FAITH)") {
		t.Fatalf("relay %q", got)
	}
	texts := f.texts["trig_rec"]
	if len(texts) != 1 || !strings.HasPrefix(texts[0], relay.Header+" · to duet:recovery · 1 item(s)") || !strings.Contains(texts[0], `"subject":"`+askID+`"`) || strings.Contains(texts[0], "long ago") {
		t.Fatalf("fired %q", texts)
	}
	if strings.Contains(out.String(), relayToken) {
		t.Fatal("token printed")
	}

	// Nothing new: nothing fires. The faith item still waits for its route.
	if got := runOut(t, e, "relay"); strings.Contains(got, "sent") || !strings.Contains(got, "faith: 1 waiting") {
		t.Fatalf("second relay %q", got)
	}
	if len(f.texts["trig_rec"]) != 1 {
		t.Fatal("sent twice")
	}

	// The faith route arrives; its waiting item goes, with the quest's title.
	vars["ACTA_RELAY_FAITH"] = fireURL("trig_fai")
	vars["ACTA_RELAY_FAITH_TOKEN"] = relayToken
	if got := runOut(t, e, "relay"); !strings.Contains(got, "faith: sent 1") {
		t.Fatalf("faith relay %q", got)
	}
	if ft := f.texts["trig_fai"]; len(ft) != 1 || !strings.Contains(ft[0], `"kind":"quest.accepted"`) || !strings.Contains(ft[0], `"title":"Read one lesson"`) {
		t.Fatalf("faith fired %q", ft)
	}

	// The agent's answer comes back through the inbox and shows on the board.
	ans := `{"v":1,"at":"2026-10-29T12:30:00-07:00","at_precision":"second","recorded_at":"2026-10-29T19:30:00Z","pillar":"recovery","kind":"pillar.answered","subject":"` + askID + `","source":{"via":"chat","agent":"duet:recovery"},"data":{"text":"21:30, phone in the kitchen."}}`
	os.WriteFile(filepath.Join(e.inboxDir(), "2026-10-29T193000Z-duet:recovery.jsonl"), []byte(ans+"\n"), 0o644)
	mustRun(t, e, "ingest")
	mustRun(t, e, "snapshot")
	as := readSnapshot(t, e).Asks
	if len(as) != 1 || as[0].ID != askID || as[0].Open || as[0].Answer != "21:30, phone in the kitchen." {
		t.Fatalf("asks %+v", as)
	}
	// An agent's answer is not Harlan's, so it never relays back.
	if got := runOut(t, e, "relay"); strings.Contains(got, "sent") {
		t.Fatalf("relayed the agent's own answer: %q", got)
	}
}

func TestRelayKeepsItemsOnFailure(t *testing.T) {
	f, srv := newFireServer(t)
	vars := map[string]string{"ACTA_RELAY_TIME": fireURL("trig_tim"), "ACTA_RELAY_TIME_TOKEN": relayToken}
	e, _, errb := testEnv(t, vars)
	e.relayBaseURL = srv.URL
	mustRun(t, e, "init")
	mustRun(t, e, "relay")
	mustRun(t, e, "ask", "--pillar", "time", "--text", "Which meeting can go?")

	f.status = http.StatusTooManyRequests
	if got := runOut(t, e, "relay"); !strings.Contains(got, "time: 1 waiting, rate limited, retry after 60s") {
		t.Fatalf("429 %q", got)
	}
	f.status = http.StatusNotFound
	if code := run(e, []string{"relay"}); code == 0 || !strings.Contains(errb.String(), "time: fire: HTTP 404") || strings.Contains(errb.String(), relayToken) {
		t.Fatalf("404: code %d, %q", code, errb.String())
	}
	f.status = 0
	if got := runOut(t, e, "relay"); !strings.Contains(got, "time: sent 1") {
		t.Fatalf("retry %q", got)
	}
}

func TestRelayRefusesForeignRoutes(t *testing.T) {
	_, srv := newFireServer(t)
	vars := map[string]string{"ACTA_RELAY_CAREER": "https://example.com/v1/claude_code/routines/trig_x/fire", "ACTA_RELAY_CAREER_TOKEN": relayToken}
	e, _, errb := testEnv(t, vars)
	e.relayBaseURL = srv.URL
	mustRun(t, e, "init")
	mustRun(t, e, "relay")
	mustRun(t, e, "ask", "--pillar", "career", "--text", "What shipped?")
	if code := run(e, []string{"relay"}); code == 0 || !strings.Contains(errb.String(), "ACTA_RELAY_CAREER is not a routine fire URL") {
		t.Fatalf("foreign route: code %d, %q", code, errb.String())
	}
	if code := run(e, []string{"ask", "--pillar", "career", "--text", "x", "--agent", "duet:career"}); code == 0 {
		t.Fatal("an agent asked a pillar")
	}
}

func TestRelayTokenFromCredential(t *testing.T) {
	f, srv := newFireServer(t)
	creds := t.TempDir()
	os.WriteFile(filepath.Join(creds, "relay-outlook"), []byte(relayToken+"\n"), 0o600)
	vars := map[string]string{"ACTA_RELAY_OUTLOOK": fireURL("trig_out"), "CREDENTIALS_DIRECTORY": creds}
	e, _, _ := testEnv(t, vars)
	e.relayBaseURL = srv.URL
	mustRun(t, e, "init")
	mustRun(t, e, "relay")
	mustRun(t, e, "ask", "--pillar", "outlook", "--text", "How was the week?")
	if got := runOut(t, e, "relay"); !strings.Contains(got, "outlook: sent 1") || len(f.texts["trig_out"]) != 1 {
		t.Fatalf("credential relay %q", got)
	}
}

// A 202 with no body still started the run: the item is recorded, not resent.
func TestRelayRecordsA2xxWithoutSession(t *testing.T) {
	f, srv := newFireServer(t)
	vars := map[string]string{"ACTA_RELAY_TIME": fireURL("trig_tim"), "ACTA_RELAY_TIME_TOKEN": relayToken}
	e, _, _ := testEnv(t, vars)
	e.relayBaseURL = srv.URL
	mustRun(t, e, "init")
	mustRun(t, e, "relay")
	mustRun(t, e, "ask", "--pillar", "time", "--text", "Which meeting can go?")
	f.status = http.StatusAccepted
	if got := runOut(t, e, "relay"); !strings.Contains(got, "time: sent 1, unknown") {
		t.Fatalf("202 %q", got)
	}
	f.status = 0
	if got := runOut(t, e, "relay"); strings.Contains(got, "sent") {
		t.Fatalf("resent after a 202: %q", got)
	}
	st, _, _ := relay.Load(e.relayPath())
	if len(st.Sent) != 1 {
		t.Fatalf("relay.json %+v", st)
	}
	for _, s := range st.Sent {
		if s.Session != relay.SessionUnknown || s.To != "time" {
			t.Fatalf("recorded %+v", s)
		}
	}
}
