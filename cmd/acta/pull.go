package main

// acta inbox pull: copy new Duet envelope files, and the note bodies they
// name (<sha256>.md), from the Drive folder into inbox/ and ingest them. The remote is read only. The Google service
// account that rclone uses can read the shared folder but cannot delete
// files it does not own, so `rclone move` copied each file and then failed
// with 403 insufficientFilePermissions. Instead, acta remembers the Drive
// file IDs it has fetched in $ACTA_STATE/inbox-seen.json and fetches each
// ID once. Files stay in Drive; the user deletes old ones by hand.

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

const defaultInboxRemote = inboxRemote + ":"

// driveFile is one entry of `rclone lsjson`.
type driveFile struct {
	Path    string
	Name    string
	ID      string
	Size    int64
	ModTime string
	IsDir   bool
}

// seenEntry records one fetched Drive file.
type seenEntry struct {
	Name string `json:"name"`
	At   string `json:"at"`
}

func (e env) inboxSeenPath() string { return filepath.Join(e.state, "inbox-seen.json") }

func (e env) inboxRemote() string {
	if r := strings.TrimSpace(e.getenv("ACTA_INBOX_REMOTE")); r != "" {
		return r
	}
	return defaultInboxRemote
}

// remotePath joins the remote and a listed path: "ccinbox:" + "a.jsonl",
// or "ccinbox:sub" + "/" + "a.jsonl".
func remotePath(remote, p string) string {
	if strings.HasSuffix(remote, ":") || strings.HasSuffix(remote, "/") {
		return remote + p
	}
	return remote + "/" + p
}

func cmdInboxPull(e env, args []string) error {
	fs := flag.NewFlagSet("inbox pull", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	dry := fs.Bool("dry-run", false, "list the files it would fetch and change nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("usage: acta inbox pull [--dry-run]")
	}
	remote := e.inboxRemote()

	if !*dry {
		if err := os.MkdirAll(e.state, 0o755); err != nil {
			return err
		}
		unlock, err := lockFile(e.inboxSeenPath() + ".lock")
		if err != nil {
			return err
		}
		defer unlock()
	}

	seen, err := loadSeen(e.inboxSeenPath())
	if err != nil {
		return err
	}
	files, err := listRemote(e, remote)
	if err != nil {
		return fmt.Errorf("inbox pull: list %q: %w", remote, err)
	}

	// Prune: keep only IDs that are still on Drive, so the file stays bounded.
	onDrive := make(map[string]bool, len(files))
	for _, f := range files {
		if f.ID != "" {
			onDrive[f.ID] = true
		}
	}
	dirty := false
	for id := range seen {
		if !onDrive[id] {
			if !*dry {
				delete(seen, id)
			}
			dirty = true
		}
	}

	fetched, already, failed := 0, 0, 0
	for _, f := range files {
		if f.IsDir {
			continue
		}
		if _, ok := seen[f.ID]; ok && f.ID != "" {
			already++
			continue
		}
		if err := checkRemoteName(f); err != nil {
			fmt.Fprintf(e.stderr, "acta: inbox pull: refused %q: %v\n", f.Path, err)
			failed++
			continue
		}
		if *dry {
			fmt.Fprintf(e.stdout, "would fetch %s (%s, %d bytes)\n", f.Path, f.ID, f.Size)
			fetched++
			continue
		}
		local, err := fetchOne(e, remote, f)
		if err != nil {
			fmt.Fprintf(e.stderr, "acta: inbox pull: %s: %v\n", f.Path, err)
			failed++
			continue
		}
		seen[f.ID] = seenEntry{Name: local, At: e.now().UTC().Format(time.RFC3339)}
		if err := saveSeen(e.inboxSeenPath(), seen); err != nil {
			return err
		}
		dirty = false
		fetched++
	}
	if dirty && !*dry {
		if err := saveSeen(e.inboxSeenPath(), seen); err != nil {
			return err
		}
	}

	if *dry {
		fmt.Fprintf(e.stdout, "inbox pull: dry run, %d to fetch, %d already seen, %d refused\n", fetched, already, failed)
		return nil
	}
	fmt.Fprintf(e.stdout, "inbox pull: %d new, %d already seen, %d failed\n", fetched, already, failed)

	var errs []error
	if failed > 0 {
		errs = append(errs, fmt.Errorf("inbox pull: %d file(s) failed", failed))
	}
	if err := cmdIngest(e, nil); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// listRemote runs rclone lsjson on the folder (no recursion).
func listRemote(e env, remote string) ([]driveFile, error) {
	out, err := e.runExt("rclone", "lsjson", remote, "--files-only", "--include", "*.jsonl", "--include", "*.md", "--drive-skip-gdocs")
	if err != nil {
		return nil, extErr(err)
	}
	var files []driveFile
	if err := json.Unmarshal([]byte(out), &files); err != nil {
		return nil, fmt.Errorf("parse lsjson output: %w", err)
	}
	for i := range files {
		if files[i].Path == "" {
			files[i].Path = files[i].Name
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// checkRemoteName refuses a listed file whose path could leave inbox/ or
// that ingest would not read.
func checkRemoteName(f driveFile) error {
	switch {
	case f.ID == "":
		return errors.New("no Drive file ID")
	case f.Path == "" || f.Path == "." || f.Path == "..":
		return errors.New("empty name")
	case strings.ContainsAny(f.Path, `/\`) || strings.ContainsRune(f.Path, 0):
		return errors.New("name has a path separator")
	case strings.HasPrefix(f.Path, "."):
		return errors.New("name starts with a dot")
	case strings.HasSuffix(f.Path, ".md"):
		// A note body is named by its sha256 (docs/knowledge-graph.md).
		if !bodyNameRe.MatchString(f.Path) {
			return errors.New("a note body must be named <sha256>.md")
		}
	case !strings.HasSuffix(f.Path, ".jsonl") || f.Path == ".jsonl":
		return errors.New("name does not end in .jsonl or .md")
	}
	return nil
}

var bodyNameRe = regexp.MustCompile(`^[0-9a-f]{64}\.md$`)

// fetchOne copies one Drive file into the inbox and returns its local name.
// rclone writes to a hidden temp name that the *.jsonl glob ignores; acta
// then hard-links it into place, which fails if the name exists, so an
// existing file is never overwritten. If the name is taken by a different
// file, it uses <base>.<id8>.jsonl (then <base>.<id8>-1.jsonl, ...); a .md
// body keeps its extension the same way.
func fetchOne(e env, remote string, f driveFile) (string, error) {
	inbox := e.inboxDir()
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		return "", err
	}
	id8 := f.ID
	if len(id8) > 8 {
		id8 = id8[:8]
	}
	tmp := filepath.Join(inbox, ".pull-"+safeID(id8)+".part")
	os.Remove(tmp) // a leftover from a crashed run
	defer os.Remove(tmp)
	if _, err := e.runExt("rclone", "copyto", remotePath(remote, f.Path), tmp, "--drive-skip-gdocs"); err != nil {
		return "", fmt.Errorf("copy: %w", extErr(err))
	}
	body, err := os.ReadFile(tmp)
	if err != nil {
		return "", fmt.Errorf("copy: %w", err)
	}
	ext := filepath.Ext(f.Path)
	base := strings.TrimSuffix(f.Path, ext)
	for n := 0; n < 100; n++ {
		cand := f.Path
		switch {
		case n == 1:
			cand = base + "." + safeID(id8) + ext
		case n > 1:
			cand = fmt.Sprintf("%s.%s-%d%s", base, safeID(id8), n-1, ext)
		}
		dst := filepath.Join(inbox, cand)
		err := os.Link(tmp, dst)
		if err == nil {
			return cand, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
		// The same bytes are already there (an earlier run copied the file
		// but could not record it): nothing to do.
		if old, rerr := os.ReadFile(dst); rerr == nil && bytes.Equal(old, body) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no free local name for %s", f.Path)
}

// safeID keeps a Drive ID usable in a file name.
func safeID(id string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return r
		}
		return '_'
	}, id)
}

// extErr adds an external program's stderr to its exit error.
func extErr(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if msg := strings.TrimSpace(string(ee.Stderr)); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
	}
	return err
}

func loadSeen(path string) (map[string]seenEntry, error) {
	seen := map[string]seenEntry{}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return seen, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &seen); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if seen == nil {
		seen = map[string]seenEntry{}
	}
	return seen, nil
}

// saveSeen writes the state atomically: temp file in the same directory,
// fsync, rename, then fsync the directory.
func saveSeen(path string, seen map[string]seenEntry) error {
	dir := filepath.Dir(path)
	b, err := json.MarshalIndent(seen, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".inbox-seen-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
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
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}

// lockFile takes an exclusive flock so the timer and a manual run cannot
// overlap. It waits for the other run to finish.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
