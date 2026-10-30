package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fakeDrive stands in for rclone and a Drive folder. It answers lsjson and
// copyto only; any other rclone verb (move, delete, ...) fails the test.
type fakeDrive struct {
	t        *testing.T
	remote   string
	files    []driveFile       // listing order
	body     map[string]string // by Path
	listErr  error
	copyFail map[string]bool // by Path
	calls    [][]string
}

func newFakeDrive(t *testing.T) *fakeDrive {
	return &fakeDrive{t: t, remote: "ccinbox:", body: map[string]string{}, copyFail: map[string]bool{}}
}

func (d *fakeDrive) add(name, id, body string) {
	d.files = append(d.files, driveFile{Path: name, Name: name, ID: id, Size: int64(len(body)), ModTime: "2026-10-29T10:00:00Z"})
	d.body[name] = body
}

func (d *fakeDrive) remove(id string) {
	for i, f := range d.files {
		if f.ID == id {
			d.files = append(d.files[:i], d.files[i+1:]...)
			return
		}
	}
}

func (d *fakeDrive) copies() []string {
	var out []string
	for _, c := range d.calls {
		if c[0] == "copyto" {
			out = append(out, c[1])
		}
	}
	return out
}

func (d *fakeDrive) command(name string, args ...string) (string, error) {
	if name != "rclone" {
		return "", exec.ErrNotFound
	}
	d.calls = append(d.calls, args)
	switch args[0] {
	case "lsjson":
		want := []string{"lsjson", d.remote, "--files-only", "--include", "*.jsonl", "--drive-skip-gdocs"}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			d.t.Errorf("lsjson args %q, want %q", args, want)
		}
		if d.listErr != nil {
			return "", d.listErr
		}
		b, _ := json.Marshal(d.files)
		return string(b), nil
	case "copyto":
		if len(args) != 4 || args[3] != "--drive-skip-gdocs" || !strings.HasPrefix(args[1], d.remote) {
			d.t.Errorf("copyto args %q", args)
			return "", errors.New("bad args")
		}
		p := strings.TrimPrefix(args[1], d.remote)
		if d.copyFail[p] {
			return "", errors.New("exit status 1: 403 rate limited")
		}
		body, ok := d.body[p]
		if !ok {
			return "", errors.New("exit status 3: not found")
		}
		return "", os.WriteFile(args[2], []byte(body), 0o644)
	default:
		d.t.Errorf("rclone %q must never run", args)
		return "", errors.New("forbidden")
	}
}

func pullEnv(t *testing.T) (env, *fakeDrive, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	e, out, errb := testEnv(t, nil)
	mustRun(t, e, "init")
	out.Reset()
	d := newFakeDrive(t)
	e.command = d.command
	return e, d, out, errb
}

func readSeen(t *testing.T, e env) map[string]seenEntry {
	t.Helper()
	b, err := os.ReadFile(e.inboxSeenPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]seenEntry
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func seenIDs(m map[string]seenEntry) string {
	var ids []string
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

// eventLine returns a valid inbox line with a distinct subject.
func eventLine(t *testing.T, n int) string {
	t.Helper()
	return fmt.Sprintf(`{"v":1,"at":"2026-10-29T09:%02d:00-07:00","at_precision":"second","recorded_at":"2026-10-29T17:00:00Z","pillar":"competence","kind":"note.added","subject":"note:pull-%d","source":{"via":"chat","agent":"duet:study"},"data":{}}`, n, n)
}

func logCount(t *testing.T, e env) int {
	t.Helper()
	b, err := os.ReadFile(e.logPath())
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "\n")
}

func TestInboxPullFetchesOnceAndIngests(t *testing.T) {
	e, d, out, errb := pullEnv(t)
	d.add("a.jsonl", "IDAAAAAAAAAA", eventLine(t, 1)+"\n")
	d.add("b.jsonl", "IDBBBBBBBBBB", eventLine(t, 2)+"\n")
	if code := run(e, []string{"inbox", "pull"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errb)
	}
	if !strings.Contains(out.String(), "inbox pull: 2 new, 0 already seen, 0 failed") || !strings.Contains(out.String(), "ingest: 2 file(s), 2 new") {
		t.Fatalf("output %q", out)
	}
	if got := seenIDs(readSeen(t, e)); got != "IDAAAAAAAAAA,IDBBBBBBBBBB" {
		t.Fatalf("seen %s", got)
	}
	if s := readSeen(t, e)["IDAAAAAAAAAA"]; s.Name != "a.jsonl" || s.At == "" {
		t.Fatalf("entry %+v", s)
	}
	if n := logCount(t, e); n != 2 {
		t.Fatalf("log has %d lines", n)
	}
	// Ingest removed the fetched files; no temp file is left.
	if ents, _ := os.ReadDir(e.inboxDir()); len(ents) != 0 {
		t.Fatalf("inbox left %v", ents)
	}

	// Second run: nothing new on Drive, nothing fetched.
	out.Reset()
	before := len(d.copies())
	if code := run(e, []string{"inbox", "pull"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errb)
	}
	if len(d.copies()) != before {
		t.Fatalf("second run copied %v", d.copies()[before:])
	}
	if !strings.Contains(out.String(), "inbox pull: 0 new, 2 already seen, 0 failed") {
		t.Fatalf("second output %q", out)
	}
}

func TestInboxPullFailedCopyNotRecorded(t *testing.T) {
	e, d, out, errb := pullEnv(t)
	d.add("a.jsonl", "IDA", eventLine(t, 1)+"\n")
	d.add("b.jsonl", "IDB", eventLine(t, 2)+"\n")
	d.add("c.jsonl", "IDC", eventLine(t, 3)+"\n")
	d.copyFail["b.jsonl"] = true
	if code := run(e, []string{"inbox", "pull"}); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(out.String(), "inbox pull: 2 new, 0 already seen, 1 failed") {
		t.Fatalf("output %q", out)
	}
	if !strings.Contains(errb.String(), "b.jsonl") || !strings.Contains(errb.String(), "403") {
		t.Fatalf("stderr %q", errb)
	}
	if got := seenIDs(readSeen(t, e)); got != "IDA,IDC" {
		t.Fatalf("seen %s", got)
	}
	if n := logCount(t, e); n != 2 {
		t.Fatalf("log has %d lines", n)
	}
	// Next run retries only the failed one.
	delete(d.copyFail, "b.jsonl")
	before := len(d.copies())
	out.Reset()
	if code := run(e, []string{"inbox", "pull"}); code != 0 {
		t.Fatalf("retry exit %d: %s", code, errb)
	}
	if got := d.copies()[before:]; len(got) != 1 || got[0] != "ccinbox:b.jsonl" {
		t.Fatalf("retry copied %v", got)
	}
	if got := seenIDs(readSeen(t, e)); got != "IDA,IDB,IDC" {
		t.Fatalf("seen %s", got)
	}
}

func TestInboxPullListingFailure(t *testing.T) {
	e, d, out, errb := pullEnv(t)
	d.add("a.jsonl", "IDA", eventLine(t, 1)+"\n")
	mustRun(t, e, "inbox", "pull")
	before, _ := os.ReadFile(e.inboxSeenPath())
	d.listErr = errors.New("exit status 1: couldn't list directory")
	d.add("b.jsonl", "IDB", eventLine(t, 2)+"\n")
	out.Reset()
	if code := run(e, []string{"inbox", "pull"}); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errb.String(), `list "ccinbox:"`) || !strings.Contains(errb.String(), "couldn't list") {
		t.Fatalf("stderr %q", errb)
	}
	after, _ := os.ReadFile(e.inboxSeenPath())
	if !bytes.Equal(before, after) {
		t.Fatalf("state changed:\n%s\n%s", before, after)
	}
	if strings.Contains(out.String(), "ingest:") || len(d.copies()) != 1 {
		t.Fatalf("listing failure still did work: %q %v", out, d.copies())
	}
}

func TestInboxPullPrunesGoneIDs(t *testing.T) {
	e, d, _, _ := pullEnv(t)
	d.add("a.jsonl", "IDA", eventLine(t, 1)+"\n")
	d.add("b.jsonl", "IDB", eventLine(t, 2)+"\n")
	mustRun(t, e, "inbox", "pull")
	d.remove("IDA") // the user deleted it on Drive
	mustRun(t, e, "inbox", "pull")
	if got := seenIDs(readSeen(t, e)); got != "IDB" {
		t.Fatalf("seen %s, want IDB", got)
	}
	d.remove("IDB")
	mustRun(t, e, "inbox", "pull")
	if got := readSeen(t, e); len(got) != 0 {
		t.Fatalf("seen %v, want empty", got)
	}
}

func TestInboxPullRejectedNotRefetched(t *testing.T) {
	e, d, out, errb := pullEnv(t)
	d.add("bad.jsonl", "IDBAD", eventLine(t, 1)+"\n"+`{"v":1,"kind":`+"\n")
	d.add("good.jsonl", "IDGOOD", eventLine(t, 2)+"\n")
	if code := run(e, []string{"inbox", "pull"}); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(out.String(), "inbox pull: 2 new, 0 already seen, 0 failed") {
		t.Fatalf("output %q", out)
	}
	if !strings.Contains(errb.String(), "moved bad.jsonl to inbox/rejected/bad.jsonl") {
		t.Fatalf("stderr %q", errb)
	}
	if _, err := os.Stat(filepath.Join(e.rejectedDir(), "bad.jsonl.err")); err != nil {
		t.Fatal(err)
	}
	if got := seenIDs(readSeen(t, e)); got != "IDBAD,IDGOOD" {
		t.Fatalf("seen %s", got)
	}
	before := len(d.copies())
	out.Reset()
	errb.Reset()
	if code := run(e, []string{"inbox", "pull"}); code != 0 {
		t.Fatalf("second exit %d: %s", code, errb)
	}
	if len(d.copies()) != before {
		t.Fatalf("rejected file refetched: %v", d.copies()[before:])
	}
	if rej, _ := filepath.Glob(filepath.Join(e.rejectedDir(), "*.jsonl")); len(rej) != 1 {
		t.Fatalf("rejected %v", rej)
	}
}

func TestInboxPullNameCollision(t *testing.T) {
	e, d, _, _ := pullEnv(t)
	local := filepath.Join(e.inboxDir(), "a.jsonl")
	mine := eventLine(t, 9) + "\n"
	if err := os.WriteFile(local, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	d.add("a.jsonl", "1234567890abcdef", eventLine(t, 1)+"\n")
	mustRun(t, e, "inbox", "pull")
	s := readSeen(t, e)["1234567890abcdef"]
	if s.Name != "a.12345678.jsonl" {
		t.Fatalf("local name %q, want a.12345678.jsonl", s.Name)
	}
	// Both files were ingested: the local one was never overwritten.
	b, _ := os.ReadFile(e.logPath())
	if !strings.Contains(string(b), "note:pull-9") || !strings.Contains(string(b), "note:pull-1") {
		t.Fatalf("log:\n%s", b)
	}
}

func TestInboxPullCollisionKeepsLocalBytes(t *testing.T) {
	e, d, _, _ := pullEnv(t)
	local := filepath.Join(e.inboxDir(), "a.jsonl")
	os.WriteFile(local, []byte("local\n"), 0o644)
	d.add("a.jsonl", "ABCDEFGHIJ", "remote\n")
	got, err := fetchOne(e, d.remote, d.files[0])
	if err != nil {
		t.Fatal(err)
	}
	if got != "a.ABCDEFGH.jsonl" {
		t.Fatalf("name %q", got)
	}
	if b, _ := os.ReadFile(local); string(b) != "local\n" {
		t.Fatalf("local file overwritten: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(e.inboxDir(), got)); string(b) != "remote\n" {
		t.Fatalf("fetched %q", b)
	}
	// Same bytes already in place: reused, not duplicated.
	again, err := fetchOne(e, d.remote, d.files[0])
	if err != nil || again != got {
		t.Fatalf("again %q %v", again, err)
	}
}

func TestInboxPullRefusesUnsafeNames(t *testing.T) {
	e, d, out, errb := pullEnv(t)
	for i, n := range []string{"../x.jsonl", "a/b.jsonl", `a\b.jsonl`, "x.txt", ".jsonl", ".hidden.jsonl"} {
		d.add(n, fmt.Sprintf("ID%d", i), eventLine(t, i)+"\n")
	}
	d.add("ok.jsonl", "IDOK", eventLine(t, 7)+"\n")
	if code := run(e, []string{"inbox", "pull"}); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if got := d.copies(); len(got) != 1 || got[0] != "ccinbox:ok.jsonl" {
		t.Fatalf("copied %v", got)
	}
	if !strings.Contains(out.String(), "1 new, 0 already seen, 6 failed") {
		t.Fatalf("output %q", out)
	}
	for _, n := range []string{"../x.jsonl", "a/b.jsonl", "x.txt"} {
		if !strings.Contains(errb.String(), fmt.Sprintf("refused %q", n)) {
			t.Fatalf("stderr lacks %s: %q", n, errb)
		}
	}
	if got := seenIDs(readSeen(t, e)); got != "IDOK" {
		t.Fatalf("seen %s", got)
	}
	if _, err := os.Stat(filepath.Join(e.data, "x.jsonl")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("file written outside the inbox")
	}
}

func TestInboxPullDryRun(t *testing.T) {
	e, d, out, errb := pullEnv(t)
	d.add("a.jsonl", "IDA", eventLine(t, 1)+"\n")
	mustRun(t, e, "inbox", "pull")
	d.remove("IDA")
	d.add("b.jsonl", "IDB", eventLine(t, 2)+"\n")
	before, _ := os.ReadFile(e.inboxSeenPath())
	logBefore, _ := os.ReadFile(e.logPath())
	copies := len(d.copies())
	out.Reset()
	if code := run(e, []string{"inbox", "pull", "--dry-run"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errb)
	}
	if !strings.Contains(out.String(), "would fetch b.jsonl (IDB") || !strings.Contains(out.String(), "dry run, 1 to fetch, 0 already seen") {
		t.Fatalf("output %q", out)
	}
	after, _ := os.ReadFile(e.inboxSeenPath())
	logAfter, _ := os.ReadFile(e.logPath())
	if !bytes.Equal(before, after) || !bytes.Equal(logBefore, logAfter) || len(d.copies()) != copies {
		t.Fatal("dry run changed something")
	}
	if left, _ := filepath.Glob(filepath.Join(e.inboxDir(), "*.jsonl")); len(left) != 0 {
		t.Fatalf("dry run wrote %v", left)
	}

	// A fresh state dir: dry run creates nothing there.
	e2, d2, _, _ := pullEnv(t)
	d2.add("a.jsonl", "IDA", eventLine(t, 1)+"\n")
	mustRun(t, e2, "inbox", "pull", "--dry-run")
	if ents, _ := os.ReadDir(e2.state); len(ents) != 0 {
		t.Fatalf("dry run created %v", ents)
	}
}

func TestInboxPullStateFileAtomic0600(t *testing.T) {
	e, d, _, _ := pullEnv(t)
	d.add("a.jsonl", "IDA", eventLine(t, 1)+"\n")
	mustRun(t, e, "inbox", "pull")
	fi, err := os.Stat(e.inboxSeenPath())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o, want 600", fi.Mode().Perm())
	}
	// No temp files left beside it.
	ents, _ := os.ReadDir(e.state)
	for _, en := range ents {
		if strings.HasPrefix(en.Name(), ".inbox-seen-") {
			t.Fatalf("temp file left: %s", en.Name())
		}
	}
	// A failed save leaves the old file whole: make the dir read-only.
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	old, _ := os.ReadFile(e.inboxSeenPath())
	os.Chmod(e.state, 0o500)
	defer os.Chmod(e.state, 0o755)
	if err := saveSeen(e.inboxSeenPath(), map[string]seenEntry{"X": {Name: "x.jsonl"}}); err == nil {
		t.Fatal("save into a read-only dir should fail")
	}
	if now, _ := os.ReadFile(e.inboxSeenPath()); !bytes.Equal(old, now) {
		t.Fatal("failed save changed the state file")
	}
}

func TestInboxPullRemoteOverride(t *testing.T) {
	e, d, _, _ := pullEnv(t)
	e.getenv = func(k string) string {
		if k == "ACTA_INBOX_REMOTE" {
			return "other:"
		}
		return ""
	}
	d.remote = "other:"
	d.add("a.jsonl", "IDA", eventLine(t, 1)+"\n")
	mustRun(t, e, "inbox", "pull")
	if got := d.copies(); len(got) != 1 || got[0] != "other:a.jsonl" {
		t.Fatalf("copied %v", got)
	}
	if remotePath("x:sub", "a.jsonl") != "x:sub/a.jsonl" {
		t.Fatal("remotePath")
	}
}

func TestInboxUsage(t *testing.T) {
	e, _, _ := testEnv(t, nil)
	if code := run(e, []string{"inbox"}); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if code := run(e, []string{"inbox", "pull", "extra"}); code != 1 {
		t.Fatalf("exit %d", code)
	}
}
