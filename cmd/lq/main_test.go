package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harlanljones/core-and-craft/internal/snapshot"
)

const key = "ape-SECRET-for-tests"

func testEnv(t *testing.T, vars map[string]string) (env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	la, _ := time.LoadLocation("America/Los_Angeles")
	var out, errb bytes.Buffer
	return env{
		data:   filepath.Join(dir, "life"),
		state:  filepath.Join(dir, "state"),
		loc:    la,
		now:    func() time.Time { return time.Date(2026, 10, 29, 12, 0, 0, 0, la) },
		stdout: &out,
		stderr: &errb,
		getenv: func(k string) string { return vars[k] },
	}, &out, &errb
}

func mustRun(t *testing.T, e env, args ...string) {
	t.Helper()
	if code := run(e, args); code != 0 {
		t.Fatalf("lq %s exited %d: %s", strings.Join(args, " "), code, e.stderr.(*bytes.Buffer).String())
	}
}

func readSnapshot(t *testing.T, e env) snapshot.Snapshot {
	t.Helper()
	b, err := os.ReadFile(e.snapshotPath())
	if err != nil {
		t.Fatal(err)
	}
	var s snapshot.Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSeedIngestSnapshotStatus(t *testing.T) {
	e, out, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	seed, _ := filepath.Glob("../../seed/*.jsonl")
	if len(seed) != 1 {
		t.Fatalf("want one seed file, got %v", seed)
	}
	b, _ := os.ReadFile(seed[0])
	inbox := filepath.Join(e.inboxDir(), filepath.Base(seed[0]))
	os.WriteFile(inbox, b, 0o644)

	mustRun(t, e, "ingest")
	if !strings.Contains(out.String(), "8 new") {
		t.Fatalf("ingest output %q", out.String())
	}
	if _, err := os.Stat(inbox); !os.IsNotExist(err) {
		t.Fatal("inbox file should be removed after ingest")
	}
	// Re-ingesting the same content is a no-op.
	os.WriteFile(inbox, b, 0o644)
	out.Reset()
	mustRun(t, e, "ingest")
	if !strings.Contains(out.String(), "0 new, 8 already logged") {
		t.Fatalf("second ingest %q", out.String())
	}

	mustRun(t, e, "snapshot")
	s := readSnapshot(t, e)
	sk := s.Skills[0]
	if sk.ID != "skill:typing" || *sk.Best != 88 || sk.RunsQualifying != 2 || sk.RunsTotal != 5 {
		t.Fatalf("skill %+v", sk)
	}
	out.Reset()
	mustRun(t, e, "status")
	if got := strings.TrimSpace(out.String()); got != "typing 88 wpm · 5 today" {
		t.Fatalf("status %q", got)
	}
	out.Reset()
	mustRun(t, e, "doctor")
	if !strings.Contains(out.String(), "8 events, ok") {
		t.Fatalf("doctor %q", out.String())
	}
}

func TestIngestRejectsBadFileAndLeavesIt(t *testing.T) {
	e, _, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	bad := filepath.Join(e.inboxDir(), "bad.jsonl")
	os.WriteFile(bad, []byte(`{"v":1,"kind":"quest.tapped"}`+"\n"), 0o644)
	if code := run(e, []string{"ingest"}); code == 0 {
		t.Fatal("want failure")
	}
	if _, err := os.Stat(bad); err != nil {
		t.Fatal("rejected file must stay in the inbox")
	}
	if !strings.Contains(errb.String(), "bad.jsonl rejected") {
		t.Fatalf("stderr %q", errb.String())
	}
}

func TestLogTargetAndRetract(t *testing.T) {
	e, out, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	mustRun(t, e, "log", "skill.defined", "--subject", "skill:typing", "--agent", "claude-code", "--data", `{"name":"Typing"}`)
	mustRun(t, e, "log", "skill.target_set", "--subject", "skill:typing", "--agent", "claude-code", "--data", `{"target":100}`)
	out.Reset()
	mustRun(t, e, "log", "typing.test", "--subject", "skill:typing", "--agent", "claude-code", "--via", "chat", "--at", "2026-10-29T09:00:00-07:00", "--data", `{"wpm":101,"acc":98}`)
	id := strings.Fields(out.String())[0]
	mustRun(t, e, "snapshot")
	if s := readSnapshot(t, e); *s.Skills[0].Best != 101 || *s.Skills[0].Target != 100 {
		t.Fatalf("before retract %+v", s.Skills[0])
	}
	mustRun(t, e, "retract", id, "--agent", "claude-code", "--reason", "wrong mode")
	mustRun(t, e, "snapshot")
	if s := readSnapshot(t, e); s.Skills[0].Best != nil {
		t.Fatalf("after retract best should be empty, got %v", *s.Skills[0].Best)
	}
	out.Reset()
	mustRun(t, e, "doctor")
}

func TestLogRejectsUnknownAgent(t *testing.T) {
	e, _, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	if code := run(e, []string{"log", "study.session", "--subject", "skill:typing", "--agent", "stranger"}); code == 0 {
		t.Fatal("want rejection")
	}
}

func TestMtPullDedupesAndNeverPrintsKey(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "ApeKey "+key {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var data []map[string]any
		for i := 0; i < 3; i++ {
			data = append(data, map[string]any{
				"_id": fmt.Sprintf("res%d", i), "timestamp": 1_761_750_000_000 + int64(i)*60_000,
				"wpm": 90 + i, "acc": 97.5, "mode": "time", "mode2": "60", "language": "english",
				"punctuation": false, "numbers": false, "funbox": []string{}, "lazyMode": false,
			})
		}
		json.NewEncoder(w).Encode(map[string]any{"message": "ok", "data": data})
	}))
	defer srv.Close()

	e, out, errb := testEnv(t, map[string]string{"MONKEYTYPE_APE_KEY": key})
	e.mtBaseURL = srv.URL
	mustRun(t, e, "init")
	mustRun(t, e, "mt", "pull")
	if !strings.Contains(out.String(), "3 new, 0 already logged") {
		t.Fatalf("first pull %q", out.String())
	}
	out.Reset()
	mustRun(t, e, "mt", "pull")
	if !strings.Contains(out.String(), "0 new, 3 already logged") {
		t.Fatalf("second pull %q", out.String())
	}
	if strings.Contains(out.String()+errb.String(), key) {
		t.Fatal("key printed")
	}
	log, _ := os.ReadFile(e.logPath())
	if strings.Contains(string(log), key) {
		t.Fatal("key written to the log")
	}
}

func TestMtPullWithoutKey(t *testing.T) {
	e, _, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	if code := run(e, []string{"mt", "pull"}); code == 0 {
		t.Fatal("want failure without a key")
	}
	if !strings.Contains(errb.String(), "no Monkeytype key") {
		t.Fatalf("stderr %q", errb.String())
	}
}

func TestMtPullReadsSystemdCredential(t *testing.T) {
	creds := t.TempDir()
	os.WriteFile(filepath.Join(creds, "monkeytype"), []byte(key+"\n"), 0o600)
	e, _, _ := testEnv(t, map[string]string{"CREDENTIALS_DIRECTORY": creds})
	if got := apeKey(e); got != key {
		t.Fatal("credential not read")
	}
}
