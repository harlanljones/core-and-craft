package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harlanljones/core-and-craft/internal/snapshot"
	"github.com/harlanljones/core-and-craft/internal/store"
)

const key = "ape-SECRET-for-tests"

func testEnv(t *testing.T, vars map[string]string) (env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	la, _ := time.LoadLocation("America/Los_Angeles")
	var out, errb bytes.Buffer
	return env{
		data:   filepath.Join(dir, "data"),
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
		t.Fatalf("acta %s exited %d: %s", strings.Join(args, " "), code, e.stderr.(*bytes.Buffer).String())
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

// goodLine returns the first event line of the seed file.
func goodLine(t *testing.T) string {
	t.Helper()
	seed, _ := filepath.Glob("../../seed/*.jsonl")
	if len(seed) != 1 {
		t.Fatalf("want one seed file, got %v", seed)
	}
	b, err := os.ReadFile(seed[0])
	if err != nil {
		t.Fatal(err)
	}
	return strings.SplitN(string(b), "\n", 2)[0]
}

func TestIngestQuarantinesBadFile(t *testing.T) {
	e, out, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	body := goodLine(t) + "\n" + `{"v":1,"kind":` + "\n"
	bad := filepath.Join(e.inboxDir(), "bad.jsonl")
	os.WriteFile(bad, []byte(body), 0o644)
	if code := run(e, []string{"ingest"}); code == 0 {
		t.Fatal("want failure")
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Fatal("rejected file must leave the inbox")
	}
	moved := filepath.Join(e.rejectedDir(), "bad.jsonl")
	got, err := os.ReadFile(moved)
	if err != nil || string(got) != body {
		t.Fatalf("rejected copy %q, %v", got, err)
	}
	reason, err := os.ReadFile(moved + ".err")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reason), "bad.jsonl rejected") || !strings.Contains(string(reason), "line 2:") {
		t.Fatalf(".err %q", reason)
	}
	if strings.Contains(string(reason), "line 1:") {
		t.Fatalf(".err blames the good line: %q", reason)
	}
	if !strings.Contains(errb.String(), "bad.jsonl rejected") || !strings.Contains(errb.String(), "line 2:") {
		t.Fatalf("stderr %q", errb.String())
	}
	if !strings.Contains(out.String(), "ingest: 0 file(s), 0 new") {
		t.Fatalf("stdout %q", out.String())
	}
	// The next run must not see the quarantined file.
	out.Reset()
	errb.Reset()
	mustRun(t, e, "ingest")
	if !strings.Contains(out.String(), "ingest: 0 file(s), 0 new, 0 already logged") {
		t.Fatalf("second ingest %q", out.String())
	}
	if _, err := os.Stat(moved); err != nil {
		t.Fatal("quarantined file must stay in rejected/")
	}
}

func TestIngestQuarantineNameCollision(t *testing.T) {
	e, _, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	bodies := []string{`{"first":` + "\n", `{"second":` + "\n", `{"third":` + "\n"}
	for _, b := range bodies {
		os.WriteFile(filepath.Join(e.inboxDir(), "bad.jsonl"), []byte(b), 0o644)
		if code := run(e, []string{"ingest"}); code == 0 {
			t.Fatal("want failure")
		}
	}
	for i, name := range []string{"bad.jsonl", "bad-1.jsonl", "bad-2.jsonl"} {
		p := filepath.Join(e.rejectedDir(), name)
		got, err := os.ReadFile(p)
		if err != nil || string(got) != bodies[i] {
			t.Fatalf("%s = %q, %v; want %q", name, got, err, bodies[i])
		}
		if _, err := os.Stat(p + ".err"); err != nil {
			t.Fatalf("%s.err: %v", name, err)
		}
	}
}

func TestIngestKeepLeavesRejectedFile(t *testing.T) {
	e, _, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	bad := filepath.Join(e.inboxDir(), "bad.jsonl")
	os.WriteFile(bad, []byte(`{"v":1,"kind":"quest.tapped"}`+"\n"), 0o644)
	if code := run(e, []string{"ingest", "--keep"}); code == 0 {
		t.Fatal("want failure")
	}
	if _, err := os.Stat(bad); err != nil {
		t.Fatal("--keep must leave the rejected file in the inbox")
	}
	if _, err := os.Stat(e.rejectedDir()); !os.IsNotExist(err) {
		t.Fatal("--keep must not create or fill rejected/")
	}
	if !strings.Contains(errb.String(), "bad.jsonl rejected") || !strings.Contains(errb.String(), "left in place") {
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

// Real results come back with default settings left out. After the seed
// sets the protocol, pulled runs must still qualify.
func TestMtPullStrippedResultsQualify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := []map[string]any{{
			"_id": "res0", "timestamp": 1_761_750_000_000, "wpm": 95, "acc": 98,
			"mode": "time", "mode2": "60",
		}}
		json.NewEncoder(w).Encode(map[string]any{"message": "ok", "data": data})
	}))
	defer srv.Close()

	e, _, _ := testEnv(t, map[string]string{"MONKEYTYPE_APE_KEY": key})
	e.mtBaseURL = srv.URL
	mustRun(t, e, "init")
	seed, _ := filepath.Glob("../../seed/*.jsonl")
	b, _ := os.ReadFile(seed[0])
	os.WriteFile(filepath.Join(e.inboxDir(), filepath.Base(seed[0])), b, 0o644)
	mustRun(t, e, "ingest")
	mustRun(t, e, "mt", "pull")
	mustRun(t, e, "snapshot")
	sk := readSnapshot(t, e).Skills[0]
	if sk.RunsQualifying != 3 || *sk.Best != 95 {
		t.Fatalf("pulled run should qualify: qualifying %d, best %v", sk.RunsQualifying, *sk.Best)
	}
}

// Pulling a run already logged from a screenshot must not count it twice.
func TestMtPullOfScreenshotRunCountsOnce(t *testing.T) {
	la, _ := time.LoadLocation("America/Los_Angeles")
	ts := time.Date(2026, 10, 29, 10, 0, 0, 0, la).UnixMilli()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Seed run 5 (88 wpm, 97% acc) as Monkeytype has it.
		data := []map[string]any{{"_id": "res5", "timestamp": ts, "wpm": 88.21, "acc": 97.14, "mode": "time", "mode2": "60"}}
		json.NewEncoder(w).Encode(map[string]any{"message": "ok", "data": data})
	}))
	defer srv.Close()

	e, _, _ := testEnv(t, map[string]string{"MONKEYTYPE_APE_KEY": key})
	e.mtBaseURL = srv.URL
	mustRun(t, e, "init")
	seed, _ := filepath.Glob("../../seed/*.jsonl")
	b, _ := os.ReadFile(seed[0])
	os.WriteFile(filepath.Join(e.inboxDir(), filepath.Base(seed[0])), b, 0o644)
	mustRun(t, e, "ingest")
	mustRun(t, e, "mt", "pull")
	mustRun(t, e, "snapshot")
	s := readSnapshot(t, e)
	sk := s.Skills[0]
	if sk.RunsTotal != 5 || sk.RunsQualifying != 2 || s.Today.TypingTests != 5 || *sk.Best != 88.2 {
		t.Fatalf("total %d qualifying %d today %d best %v", sk.RunsTotal, sk.RunsQualifying, s.Today.TypingTests, *sk.Best)
	}
}

func runOut(t *testing.T, e env, args ...string) string {
	t.Helper()
	out := e.stdout.(*bytes.Buffer)
	out.Reset()
	mustRun(t, e, args...)
	return strings.TrimSpace(out.String())
}

func listQuests(t *testing.T, e env) map[string]snapshot.Quest {
	t.Helper()
	var qs []snapshot.Quest
	if err := json.Unmarshal([]byte(runOut(t, e, "quest", "list", "--json", "--all")), &qs); err != nil {
		t.Fatal(err)
	}
	m := map[string]snapshot.Quest{}
	for _, q := range qs {
		m[q.ID] = q
	}
	return m
}

// questID pulls the quest:<slug> id from "quest:<slug> added (...)".
func questID(t *testing.T, out string) string {
	t.Helper()
	id, _, _ := strings.Cut(out, " ")
	if !strings.HasPrefix(id, "quest:") {
		t.Fatalf("no quest id in %q", out)
	}
	return id
}

func TestQuestLifecycle(t *testing.T) {
	e, _, _ := testEnv(t, nil)
	mustRun(t, e, "init")

	mine := questID(t, runOut(t, e, "quest", "add", "--pillar", "relationships", "--title", "Call Mom!", "--priority", "1"))
	if !strings.HasPrefix(mine, "quest:call-mom-") {
		t.Fatalf("slug %s", mine)
	}
	agent := questID(t, runOut(t, e, "quest", "add", "--pillar", "economic", "--title", "Review October spend", "--agent", "duet:economic", "--via", "chat"))
	daily := questID(t, runOut(t, e, "quest", "add", "--pillar", "competence", "--title", "Typing drill", "--every", "daily", "--skill", "skill:typing", "--rule", "runs:1"))

	qs := listQuests(t, e)
	if qs[mine].Status != "active" || qs[mine].Priority != 1 || qs[agent].Status != "proposed" || qs[daily].Every != "daily" {
		t.Fatalf("after add: %+v", qs)
	}

	// Short ids work; done on a proposal is refused.
	short := strings.TrimPrefix(agent, "quest:")
	if code := run(e, []string{"quest", "done", short}); code == 0 {
		t.Fatal("done on a proposed quest should fail")
	}
	if code := run(e, []string{"quest", "accept", short, "--agent", "duet:economic"}); code == 0 {
		t.Fatal("an agent must not accept a proposal")
	}
	mustRun(t, e, "quest", "accept", short)
	mustRun(t, e, "quest", "done", mine)
	mustRun(t, e, "quest", "done", daily)
	qs = listQuests(t, e)
	if qs[agent].Status != "active" || qs[mine].Status != "done" || qs[daily].LastDone != "2026-10-29" {
		t.Fatalf("after done: %+v", qs)
	}

	mustRun(t, e, "quest", "undo", mine)
	if q := listQuests(t, e)[mine]; q.Status != "active" || q.LastDone != "" {
		t.Fatalf("after undo: %+v", q)
	}
	if code := run(e, []string{"quest", "undo", mine}); code == 0 {
		t.Fatal("undo with nothing done should fail")
	}

	mustRun(t, e, "quest", "edit", daily, "--priority", "3", "--every", "weekdays")
	mustRun(t, e, "quest", "snooze", agent, "--until", "2026-11-02")
	mustRun(t, e, "quest", "drop", mine)
	qs = listQuests(t, e)
	if qs[daily].Priority != 3 || qs[daily].Every != "weekdays" || qs[agent].SnoozedUntil != "2026-11-02" || qs[mine].Status != "dropped" {
		t.Fatalf("after edit/snooze/drop: %+v", qs)
	}
	if code := run(e, []string{"quest", "snooze", daily, "--until", "soon"}); code == 0 {
		t.Fatal("bad until should fail")
	}
	if code := run(e, []string{"quest", "done", "quest:nope"}); code == 0 {
		t.Fatal("unknown quest should fail")
	}

	nope := questID(t, runOut(t, e, "quest", "add", "--pillar", "economic", "--title", "Call Mom", "--agent", "duet:economic"))
	mustRun(t, e, "quest", "decline", nope)
	if q := listQuests(t, e)[nope]; q.Status != "declined" {
		t.Fatalf("after decline: %+v", q)
	}

	// The default list hides dropped and declined quests.
	out := runOut(t, e, "quest", "list")
	if strings.Contains(out, "Call Mom") || !strings.Contains(out, "Typing drill") {
		t.Fatalf("list %q", out)
	}
	out = runOut(t, e, "doctor")
	if !strings.Contains(out, "ok") {
		t.Fatalf("doctor %q", out)
	}
}

func TestCheckinAndRest(t *testing.T) {
	e, _, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	out := runOut(t, e, "checkin", "ask", "--pillar", "recovery", "--question", "What's your target sleep window?", "--agent", "duet:recovery")
	id, _, _ := strings.Cut(out, " ")
	if !strings.HasPrefix(id, "checkin:") {
		t.Fatalf("ask %q", out)
	}
	mustRun(t, e, "checkin", "answer", id, "--text", "22:30 to 06:30")
	if code := run(e, []string{"checkin", "answer", "checkin:nope", "--text", "x"}); code == 0 {
		t.Fatal("answer to an unknown check-in should fail")
	}
	if code := run(e, []string{"checkin", "answer", id}); code == 0 {
		t.Fatal("answer needs text")
	}
	mustRun(t, e, "rest")
	if out := runOut(t, e, "rest"); !strings.Contains(out, "already") {
		t.Fatalf("second rest %q", out)
	}
	evs, bad, _ := store.ReadFile(e.logPath())
	if len(bad) != 0 || len(evs) != 3 {
		t.Fatalf("want 3 events, got %d (bad %v)", len(evs), bad)
	}
	if evs[1].Pillar != "recovery" || evs[1].Source.Agent != "manual" || evs[2].Subject != "day:2026-10-29" {
		t.Fatalf("events %+v", evs)
	}
}

// fakeCommands answers external programs from a table keyed by name; a
// missing name acts as not installed.
func fakeCommands(t *testing.T, outs map[string]string, errs map[string]error) func(string, ...string) (string, error) {
	t.Helper()
	return func(name string, args ...string) (string, error) {
		if err, ok := errs[name]; ok {
			return outs[name], err
		}
		out, ok := outs[name]
		if !ok {
			return "", exec.ErrNotFound
		}
		return out, nil
	}
}

func rcloneConf(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "rclone.conf")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return "Configuration file is stored at:\n" + p + "\n"
}

func TestDoctorRejectedInbox(t *testing.T) {
	e, out, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	mustRun(t, e, "doctor")
	if !strings.Contains(out.String(), "inbox: ok, no rejected files") {
		t.Fatalf("doctor %q", out.String())
	}
	os.MkdirAll(e.rejectedDir(), 0o755)
	old := time.Now().Add(-time.Hour)
	for _, f := range []struct{ name, msg string }{
		{"a.jsonl", "a.jsonl rejected:\nline 1: invalid character 'x'\n"},
		{"b.jsonl", "b.jsonl rejected:\nline 3: unknown agent \"stranger\"\nline 4: more\n"},
	} {
		p := filepath.Join(e.rejectedDir(), f.name)
		os.WriteFile(p, []byte("{}\n"), 0o644)
		os.WriteFile(p+".err", []byte(f.msg), 0o644)
		if f.name == "a.jsonl" {
			os.Chtimes(p, old, old)
			os.Chtimes(p+".err", old, old)
		}
	}
	out.Reset()
	mustRun(t, e, "doctor") // warnings do not fail doctor
	got := out.String()
	if !strings.Contains(got, "warn: inbox: 2 rejected file(s)") ||
		!strings.Contains(got, `newest b.jsonl: line 3: unknown agent "stranger"`) ||
		!strings.Contains(got, "doctor: 1 warning(s)") {
		t.Fatalf("doctor %q", got)
	}
}

func TestDoctorServiceAccount(t *testing.T) {
	key := func(mode os.FileMode) string {
		p := filepath.Join(t.TempDir(), "sa.json")
		if err := os.WriteFile(p, []byte(`{"private_key":"SECRET"}`), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name, conf, want string
		warn             bool
	}{
		{"private key file", "[ccinbox]\ntype = drive\nservice_account_file = " + key(0o600) + "\n", "rclone: ok, remote ccinbox uses a service account", false},
		{"key readable by others", "[ccinbox]\ntype = drive\nservice_account_file = " + key(0o644) + "\n", "warn: rclone: service account key", true},
		{"key file missing", "[ccinbox]\ntype = drive\nservice_account_file = /nonexistent/sa.json\n", "warn: rclone: remote ccinbox service account file /nonexistent/sa.json", true},
		{"inline credentials", "[ccinbox]\ntype = drive\nservice_account_credentials = {\"private_key\":\"SECRET\"}\n", "rclone: ok, remote ccinbox uses a service account", false},
		{"service account beats a stale shared client", "[ccinbox]\ntype = drive\nservice_account_file = " + key(0o600) + "\ntoken = {\"access_token\":\"SECRET\"}\n", "rclone: ok, remote ccinbox uses a service account", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, out, _ := testEnv(t, nil)
			e.command = fakeCommands(t, map[string]string{"rclone": rcloneConf(t, c.conf)}, nil)
			mustRun(t, e, "doctor")
			got := out.String()
			if !strings.Contains(got, c.want) {
				t.Fatalf("doctor %q, want %q", got, c.want)
			}
			if strings.Contains(got, "SECRET") {
				t.Fatalf("doctor printed a secret: %q", got)
			}
			if strings.Contains(got, "shared Google Drive client_id") {
				t.Fatalf("doctor %q warns about client_id for a service account", got)
			}
			if !c.warn && strings.Contains(got, "rclone: warn") {
				t.Fatalf("doctor %q should not warn about rclone", got)
			}
		})
	}
}

func TestDoctorClientID(t *testing.T) {
	cases := []struct {
		name string
		outs map[string]string
		want string
	}{
		{"missing", nil, "warn: rclone: remote ccinbox uses rclone's shared Google Drive client_id"},
		{"empty", nil, "warn: rclone: remote ccinbox uses rclone's shared"},
		{"present", nil, "rclone: ok, remote ccinbox has its own client_id"},
		{"remote absent", nil, "rclone: not configured (no [ccinbox] remote"},
		{"no config file", nil, "rclone: not configured (no "},
		{"no rclone", map[string]string{}, "rclone: not configured (rclone not found)"},
	}
	confs := map[string]string{
		"missing":       "[other]\nclient_id = x\n\n[ccinbox]\ntype = drive\nscope = drive\ntoken = {\"access_token\":\"SECRET\"}\n",
		"empty":         "[ccinbox]\ntype = drive\nclient_id =\n",
		"present":       "[ccinbox]\ntype = drive\nclient_id = 123.apps.googleusercontent.com\nclient_secret = SECRET\n",
		"remote absent": "[gdrive]\ntype = drive\n",
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, out, _ := testEnv(t, nil)
			outs := c.outs
			switch {
			case confs[c.name] != "":
				outs = map[string]string{"rclone": rcloneConf(t, confs[c.name])}
			case c.name == "no config file":
				outs = map[string]string{"rclone": "Configuration file doesn't exist, but rclone will use this path:\n" + filepath.Join(t.TempDir(), "rclone.conf") + "\n"}
			}
			e.command = fakeCommands(t, outs, nil)
			mustRun(t, e, "doctor")
			got := out.String()
			if !strings.Contains(got, c.want) {
				t.Fatalf("doctor %q, want %q", got, c.want)
			}
			if strings.Contains(got, "SECRET") {
				t.Fatalf("doctor printed a secret: %q", got)
			}
			if strings.HasPrefix(c.want, "warn:") && !strings.Contains(got, "https://rclone.org/drive/#making-your-own-client-id") {
				t.Fatalf("doctor %q lacks the docs link", got)
			}
			if !strings.HasPrefix(c.want, "warn:") && strings.Contains(got, "warn:") {
				t.Fatalf("doctor %q should not warn", got)
			}
		})
	}
}

func TestDoctorInboxPull(t *testing.T) {
	e0, _, _ := testEnv(t, nil)
	now := e0.now()
	unix := func(d time.Duration) string { return fmt.Sprintf("@%d", now.Add(-d).Unix()) }
	cases := []struct {
		name, show string
		err        error
		want       string
	}{
		{"ok", "LoadState=loaded\nResult=success\nExecMainExitTimestamp=" + unix(4*time.Minute) + "\n", nil, "inbox pull: ok, last run 4m0s ago"},
		{"ok default format", "Result=success\nExecMainExitTimestamp=" + now.Add(-2*time.Minute).Format("Mon 2006-01-02 15:04:05 MST") + "\nLoadState=loaded\n", nil, "inbox pull: ok, last run 2m0s ago"},
		{"failed", "LoadState=loaded\nResult=exit-code\nExecMainExitTimestamp=" + unix(time.Minute) + "\n", nil, "warn: inbox pull: acta-inbox.service last run failed (Result=exit-code)"},
		{"old", "LoadState=loaded\nResult=success\nExecMainExitTimestamp=" + unix(2*time.Hour) + "\n", nil, "warn: inbox pull: last run 2h0m0s ago"},
		{"never ran", "LoadState=loaded\nResult=success\nExecMainExitTimestamp=\n", nil, "warn: inbox pull: acta-inbox.service has not run yet"},
		{"running now", "LoadState=loaded\nActiveState=activating\nResult=success\nExecMainExitTimestamp=\n", nil, "inbox pull: running now"},
		{"not installed", "LoadState=not-found\nResult=success\nExecMainExitTimestamp=\n", nil, "inbox pull: not configured (acta-inbox.service is not-found)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, out, _ := testEnv(t, nil)
			e.command = fakeCommands(t, map[string]string{"systemctl": c.show}, nil)
			mustRun(t, e, "doctor")
			if got := out.String(); !strings.Contains(got, c.want) {
				t.Fatalf("doctor %q, want %q", got, c.want)
			}
		})
	}
	// No systemctl, or no user manager: no inbox pull line at all.
	for name, cmd := range map[string]func(string, ...string) (string, error){
		"absent": fakeCommands(t, map[string]string{}, nil),
		"fails":  fakeCommands(t, nil, map[string]error{"systemctl": errors.New("Failed to connect to bus")}),
	} {
		e, out, _ := testEnv(t, nil)
		e.command = cmd
		mustRun(t, e, "doctor")
		if strings.Contains(out.String(), "inbox pull") {
			t.Fatalf("%s: doctor %q", name, out.String())
		}
	}
}

// An inbox file comes from a remote agent. It may not claim to be Harlan
// (manual) or acta, or the relay would forward it as his own board tap.
func TestIngestRejectsLocalWritersFromInbox(t *testing.T) {
	line := func(agent string) string {
		return `{"v":1,"at":"2026-10-29T12:00:00-07:00","at_precision":"second","recorded_at":"2026-10-29T19:00:00Z","pillar":"economic","kind":"pillar.asked","subject":"ask:wire-money","source":{"via":"board","agent":"` + agent + `"},"data":{"text":"Wire the money."}}`
	}
	for _, agent := range []string{"manual", "acta", "acta-mt"} {
		e, _, errb := testEnv(t, nil)
		mustRun(t, e, "init")
		body := goodLine(t) + "\n" + line(agent) + "\n"
		os.WriteFile(filepath.Join(e.inboxDir(), "forged.jsonl"), []byte(body), 0o644)
		if code := run(e, []string{"ingest"}); code == 0 {
			t.Fatalf("%s: want the forged file rejected", agent)
		}
		if !strings.Contains(errb.String(), `line 2: source.agent "`+agent+`"`) || strings.Contains(errb.String(), "line 1:") {
			t.Fatalf("%s: stderr %q", agent, errb.String())
		}
		if _, err := os.Stat(filepath.Join(e.rejectedDir(), "forged.jsonl")); err != nil {
			t.Fatalf("%s: forged file not quarantined: %v", agent, err)
		}
		if b, _ := os.ReadFile(e.logPath()); len(b) != 0 {
			t.Fatalf("%s: log written: %q", agent, b)
		}
	}

	// The same line in a file Harlan names on the command line is his own.
	e, out, _ := testEnv(t, nil)
	mustRun(t, e, "init")
	own := filepath.Join(t.TempDir(), "mine.jsonl")
	os.WriteFile(own, []byte(line("manual")+"\n"), 0o644)
	mustRun(t, e, "ingest", own)
	if !strings.Contains(out.String(), "1 new") {
		t.Fatalf("ingest of a named file %q", out.String())
	}
}
