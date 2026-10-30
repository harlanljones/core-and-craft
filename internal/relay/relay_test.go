package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harlanljones/core-and-craft/internal/event"
)

type log struct{ evs []event.Event }

func (l *log) add(kind, subject, agent, data string) string {
	n := len(l.evs) + 1
	id := fmt.Sprintf("evt_%016x", n)
	l.evs = append(l.evs, event.Event{
		ID: id, V: 1, At: "2026-10-29", AtPrecision: event.PrecisionDay, Seq: int64(n),
		RecordedAt: "2026-10-29T17:00:00Z", Pillar: "recovery", Kind: kind, Subject: subject,
		Source: event.Source{Via: "board", Agent: agent}, Data: json.RawMessage(data),
	})
	return id
}

func TestSelect(t *testing.T) {
	l := &log{}
	l.add("checkin.asked", "checkin:sleep", "duet:recovery", `{"question":"Sleep window?"}`)
	l.add("checkin.asked", "checkin:mine", "manual", `{"question":"Mine?"}`)
	l.add("quest.added", "quest:walk", "duet:recovery", `{"title":"Walk","status":"proposed"}`)
	l.add("quest.added", "quest:odd", "duet:somewhere", `{"title":"Odd","status":"proposed"}`)
	ask := l.add("pillar.asked", "ask:bed", "manual", `{"text":"When should I stop screens?"}`)
	ans := l.add("checkin.answered", "checkin:sleep", "manual", `{"text":"22:30 to 06:30"}`)
	l.add("checkin.answered", "checkin:mine", "manual", `{"text":"not for an agent"}`)
	acc := l.add("quest.accepted", "quest:walk", "manual", `{}`)
	l.add("quest.declined", "quest:odd", "manual", `{}`)
	l.add("pillar.answered", "ask:bed", "duet:recovery", `{"text":"21:30"}`)
	l.add("quest.done", "quest:walk", "manual", `{}`)

	items := Select(l.evs)
	var got []string
	for _, it := range items {
		got = append(got, it.Event.ID+">"+it.To)
	}
	want := []string{ask + ">recovery", ans + ">recovery", acc + ">recovery"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got %v, want %v", got, want)
	}
	if items[1].Re == nil || items[1].Re.Question != "Sleep window?" || items[1].Re.ID != "evt_0000000000000001" {
		t.Fatalf("check-in answer re: %+v", items[1].Re)
	}
	if items[2].Re == nil || items[2].Re.Title != "Walk" || items[2].Re.Kind != "quest.added" {
		t.Fatalf("quest re: %+v", items[2].Re)
	}
}

func TestPayloadAndBatch(t *testing.T) {
	l := &log{}
	for i := 0; i < MaxItems+5; i++ {
		l.add("pillar.asked", fmt.Sprintf("ask:q%d", i), "manual", `{"text":"<why> & how?"}`)
	}
	items := Select(l.evs)
	got, text, err := Batch("recovery", items)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxItems {
		t.Fatalf("batch of %d, want %d", len(got), MaxItems)
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if !strings.HasPrefix(lines[0], Header+" · to duet:recovery · 20 item(s)") || len(lines) != 2+MaxItems {
		t.Fatalf("payload head %q, %d lines", lines[0], len(lines))
	}
	var it Item
	if err := json.Unmarshal([]byte(lines[2]), &it); err != nil || it.Event.Kind != "pillar.asked" {
		t.Fatalf("item line %q: %v", lines[2], err)
	}
	if !strings.Contains(lines[2], `<why> & how?`) {
		t.Fatalf("html escaped: %s", lines[2])
	}
}

func TestBatchStaysUnderTheTextLimit(t *testing.T) {
	l := &log{}
	long := strings.Repeat("é", 1000) // 2,000 bytes, 1,000 characters
	for i := 0; i < 30; i++ {
		l.add("pillar.asked", fmt.Sprintf("ask:q%d", i), "manual", `{"text":"`+long+strings.Repeat(" x", 1)+`"}`)
	}
	big := Select(l.evs)
	for i := range big {
		big[i].Re = &Re{Kind: "quest.added", ID: "evt_0", Title: strings.Repeat("y", 2400)}
	}
	got, text, _ := Batch("recovery", big)
	if n := len([]rune(text)); n > MaxText || len(got) == 0 || len(got) == MaxItems {
		t.Fatalf("%d items, %d chars", len(got), n)
	}
}

func TestValidURL(t *testing.T) {
	good := "https://api.anthropic.com/v1/claude_code/routines/trig_01ABCdef/fire"
	if !ValidURL(good) {
		t.Fatal("good url refused")
	}
	for _, u := range []string{
		"http://api.anthropic.com/v1/claude_code/routines/trig_01ABC/fire",
		"https://api.anthropic.com.evil.dev/v1/claude_code/routines/trig_01ABC/fire",
		"https://api.anthropic.com/v1/claude_code/routines/trig_01ABC/fire?x=1",
		"https://evil.dev/v1/claude_code/routines/trig_01ABC/fire",
		"https://api.anthropic.com/v1/claude_code/routines/routine_01/fire",
	} {
		if ValidURL(u) {
			t.Errorf("accepted %s", u)
		}
	}
}

func TestFire(t *testing.T) {
	var gotAuth, gotVersion, gotText, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotVersion, gotPath = r.Header.Get("Authorization"), r.Header.Get("anthropic-version"), r.URL.Path
		var body struct{ Text string }
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		gotText = body.Text
		switch {
		case strings.Contains(body.Text, "busy"):
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusTooManyRequests)
		case strings.Contains(body.Text, "paused"):
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"routine is paused"}}`)
		default:
			io.WriteString(w, `{"type":"routine_fire","claude_code_session_id":"session_01X","claude_code_session_url":"https://claude.ai/code/session_01X"}`)
		}
	}))
	defer srv.Close()
	c := Client{HTTP: srv.Client(), BaseURL: srv.URL}
	r := Route{URL: "https://api.anthropic.com/v1/claude_code/routines/trig_01ABC/fire", Token: "sk-ant-oat01-secret"}
	ctx := context.Background()

	sess, err := c.Fire(ctx, r, "hello")
	if err != nil || sess != "session_01X" {
		t.Fatalf("fire: %q %v", sess, err)
	}
	if gotAuth != "Bearer sk-ant-oat01-secret" || gotVersion == "" || gotText != "hello" || gotPath != "/v1/claude_code/routines/trig_01ABC/fire" {
		t.Fatalf("request: auth %q version %q text %q path %q", gotAuth, gotVersion, gotText, gotPath)
	}
	_, err = c.Fire(ctx, r, "busy")
	if rl, ok := err.(RateLimited); !ok || rl.RetryAfter != "120" {
		t.Fatalf("429: %v", err)
	}
	_, err = c.Fire(ctx, r, "paused")
	if err == nil || !strings.Contains(err.Error(), "routine is paused") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("400: %v", err)
	}
	gotAuth = ""
	if _, err := c.Fire(ctx, Route{URL: "https://evil.dev/v1/claude_code/routines/trig_01ABC/fire", Token: "t"}, "x"); err == nil || gotAuth != "" {
		t.Fatal("fired at a foreign host")
	}
}

func TestStateRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state", FileName)
	s, found, err := Load(p)
	if err != nil || found || s.Sent == nil {
		t.Fatalf("empty load: %+v %v %v", s, found, err)
	}
	s.Sent["evt_1"] = Sent{At: "2026-10-29T17:00:00Z", To: "time", Session: "session_01X"}
	unlock, err := Lock(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(p, s); err != nil {
		t.Fatal(err)
	}
	unlock()
	back, found, err := Load(p)
	if err != nil || !found || back.Sent["evt_1"].Session != "session_01X" {
		t.Fatalf("reload: %+v %v %v", back, found, err)
	}
}

// Any 2xx means the run started. A body without a session id still counts,
// or the items would go again on every run.
func TestFireAny2xxIsDelivered(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   string
	}{
		{http.StatusCreated, `{"claude_code_session_id":"session_201"}`, "session_201"},
		{http.StatusAccepted, `{"claude_code_session_id":"session_202"}`, "session_202"},
		{http.StatusAccepted, ``, SessionUnknown},
		{http.StatusOK, ``, SessionUnknown},
		{http.StatusOK, `{}`, SessionUnknown},
		{http.StatusOK, `not json`, SessionUnknown},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			io.WriteString(w, tc.body)
		}))
		c := Client{HTTP: srv.Client(), BaseURL: srv.URL}
		r := Route{URL: "https://api.anthropic.com/v1/claude_code/routines/trig_01ABC/fire", Token: "t"}
		sess, err := c.Fire(context.Background(), r, "hello")
		srv.Close()
		if err != nil || sess != tc.want {
			t.Errorf("%d %q: got %q, %v; want %q", tc.status, tc.body, sess, err, tc.want)
		}
	}
}

// A redirect must not carry the private body (or the token) anywhere else:
// a 307 or 308 is a failure, and the target is never asked.
func TestFireDoesNotFollowRedirects(t *testing.T) {
	for _, code := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		var followed bool
		mux := http.NewServeMux()
		mux.HandleFunc("/elsewhere", func(w http.ResponseWriter, r *http.Request) {
			followed = true
			io.WriteString(w, `{"claude_code_session_id":"session_moved"}`)
		})
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/elsewhere", code)
		})
		srv := httptest.NewServer(mux)
		for _, hc := range []*http.Client{nil, srv.Client()} {
			c := Client{HTTP: hc, BaseURL: srv.URL}
			r := Route{URL: "https://api.anthropic.com/v1/claude_code/routines/trig_01ABC/fire", Token: "t"}
			sess, err := c.Fire(context.Background(), r, "private")
			if err == nil || followed || !strings.Contains(err.Error(), fmt.Sprint(code)) {
				t.Errorf("%d (client %v): session %q, err %v, followed %v", code, hc != nil, sess, err, followed)
			}
		}
		srv.Close()
	}
}

// The trig_ id in the fire URL is half the secret: errors show the host only.
func TestFireErrorsHideTheRoutine(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	base := srv.URL
	srv.Close() // nothing listens: the POST fails in the transport
	c := Client{BaseURL: base}
	r := Route{URL: "https://api.anthropic.com/v1/claude_code/routines/trig_01SECRET/fire", Token: "t"}
	_, err := c.Fire(context.Background(), r, "x")
	if err == nil || strings.Contains(err.Error(), "trig_") || !strings.Contains(err.Error(), strings.TrimPrefix(base, "http://")) {
		t.Fatalf("transport error %v", err)
	}
	_, err = c.Fire(context.Background(), Route{URL: "https://evil.dev/v1/claude_code/routines/trig_01SECRET/fire", Token: "t"}, "x")
	if err == nil || strings.Contains(err.Error(), "trig_") || !strings.Contains(err.Error(), "evil.dev") {
		t.Fatalf("foreign route error %v", err)
	}
}
