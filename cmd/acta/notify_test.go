package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harlanljones/core-and-craft/internal/snapshot"
)

// notifyEnv returns an env whose notifier appends "title|body" lines to a file.
func notifyEnv(t *testing.T, hour int) (env, func() []string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.txt")
	script := filepath.Join(dir, "fake-notify.sh")
	body := "#!/bin/sh\nprintf '%s|%s\\n' \"$1\" \"$2\" >> " + log + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	e, _, _ := testEnv(t, map[string]string{"ACTA_NOTIFY_CMD": script})
	la := e.loc
	e.now = func() time.Time { return time.Date(2026, 10, 29, hour, 0, 0, 0, la) }
	if err := os.MkdirAll(e.state, 0o755); err != nil {
		t.Fatal(err)
	}
	calls := func() []string {
		b, _ := os.ReadFile(log)
		s := strings.TrimSpace(string(b))
		if s == "" {
			return nil
		}
		return strings.Split(s, "\n")
	}
	return e, calls
}

func putSnapshot(t *testing.T, e env, s snapshot.Snapshot) {
	t.Helper()
	s.TZ = "America/Los_Angeles"
	b, _ := json.Marshal(s)
	if err := os.WriteFile(e.snapshotPath(), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func checkin(id, pillar, q string) snapshot.Checkin {
	return snapshot.Checkin{ID: id, Pillar: pillar, Question: q, Open: true}
}

func TestNotifyBaselineSilent(t *testing.T) {
	e, calls := notifyEnv(t, 12)
	putSnapshot(t, e, snapshot.Snapshot{
		Checkins: []snapshot.Checkin{checkin("c1", "body", "Slept well?")},
		Proposed: []snapshot.Proposal{{ID: "q1"}},
	})
	mustRun(t, e, "notify")
	if len(calls()) != 0 {
		t.Fatalf("baseline sent %v", calls())
	}
	if !strings.Contains(e.stdout.(interface{ String() string }).String(), "baseline") {
		t.Fatal("baseline not reported")
	}
	mustRun(t, e, "notify")
	if len(calls()) != 0 {
		t.Fatalf("existing items announced after baseline: %v", calls())
	}
}

func TestNotifyNewCheckinOnce(t *testing.T) {
	e, calls := notifyEnv(t, 12)
	putSnapshot(t, e, snapshot.Snapshot{})
	mustRun(t, e, "notify")
	putSnapshot(t, e, snapshot.Snapshot{Checkins: []snapshot.Checkin{
		checkin("c1", "body", "Did you move today?"),
		{ID: "c2", Pillar: "mind", Question: "answered", Open: false},
	}})
	mustRun(t, e, "notify")
	mustRun(t, e, "notify")
	got := calls()
	if len(got) != 1 || got[0] != "Check-in · Body|Did you move today?" {
		t.Fatalf("calls = %v", got)
	}
}

func TestNotifyProposalsBatch(t *testing.T) {
	e, calls := notifyEnv(t, 12)
	putSnapshot(t, e, snapshot.Snapshot{Proposed: []snapshot.Proposal{{ID: "q0"}}})
	mustRun(t, e, "notify") // baseline
	putSnapshot(t, e, snapshot.Snapshot{Proposed: []snapshot.Proposal{{ID: "q0"}, {ID: "q1"}, {ID: "q2"}}})
	mustRun(t, e, "notify")
	mustRun(t, e, "notify")
	got := calls()
	if len(got) != 1 || !strings.HasPrefix(got[0], "2 proposals moored|") {
		t.Fatalf("calls = %v", got)
	}
}

func TestNotifyEveningNudgeOncePerDay(t *testing.T) {
	e, calls := notifyEnv(t, 21)
	putSnapshot(t, e, snapshot.Snapshot{})
	mustRun(t, e, "notify") // baseline, silent
	if len(calls()) != 0 {
		t.Fatalf("baseline nudged: %v", calls())
	}
	mustRun(t, e, "notify")
	mustRun(t, e, "notify")
	if len(calls()) != 1 || !strings.HasPrefix(calls()[0], "Pax") {
		t.Fatalf("calls = %v", calls())
	}
	// Next evening nudges again; a kept streak or an early hour does not.
	la := e.loc
	e.now = func() time.Time { return time.Date(2026, 10, 30, 21, 0, 0, 0, la) }
	mustRun(t, e, "notify")
	if len(calls()) != 2 {
		t.Fatalf("next day: %v", calls())
	}
	e.now = func() time.Time { return time.Date(2026, 10, 31, 19, 59, 0, 0, la) }
	mustRun(t, e, "notify")
	e.now = func() time.Time { return time.Date(2026, 11, 1, 21, 0, 0, 0, la) }
	putSnapshot(t, e, snapshot.Snapshot{Streak: snapshot.Streak{Days: 3, Today: true}})
	mustRun(t, e, "notify")
	if len(calls()) != 2 {
		t.Fatalf("unexpected nudge: %v", calls())
	}
}

func TestNotifyAllFirstRun(t *testing.T) {
	e, calls := notifyEnv(t, 12)
	putSnapshot(t, e, snapshot.Snapshot{
		Checkins: []snapshot.Checkin{checkin("c1", "craft", "Shipped?")},
		Proposed: []snapshot.Proposal{{ID: "q1"}},
	})
	mustRun(t, e, "notify", "--all")
	if len(calls()) != 2 {
		t.Fatalf("calls = %v", calls())
	}
	mustRun(t, e, "notify", "--all")
	if len(calls()) != 2 {
		t.Fatalf("repeat: %v", calls())
	}
}

func TestNotifyFailureKeepsItemPending(t *testing.T) {
	e, calls := notifyEnv(t, 12)
	putSnapshot(t, e, snapshot.Snapshot{})
	mustRun(t, e, "notify")
	putSnapshot(t, e, snapshot.Snapshot{Checkins: []snapshot.Checkin{checkin("c1", "mind", "How is focus?")}})
	good := e.getenv
	e.getenv = func(k string) string {
		if k == "ACTA_NOTIFY_CMD" {
			return "/nonexistent/notifier"
		}
		return good(k)
	}
	if code := run(e, []string{"notify"}); code == 0 {
		t.Fatal("want failure")
	}
	e.getenv = good
	mustRun(t, e, "notify")
	if got := calls(); len(got) != 1 {
		t.Fatal(fmt.Sprint(got))
	}
}
