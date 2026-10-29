// Package store reads and appends events.jsonl. Only lq writes the file,
// one writer at a time under flock, one line per event.
package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/harlanljones/core-and-craft/internal/event"
)

const FileName = "events.jsonl"

// maxLine bounds one event line; charStats and notes keep lines small.
const maxLine = 1 << 20

type LineError struct {
	Line int    `json:"line"`
	Err  string `json:"error"`
}

// Scan reads every line of r. Bad lines come back as LineErrors, never
// silently dropped.
func Scan(r io.Reader) ([]event.Event, []LineError, error) {
	var evs []event.Event
	var bad []LineError
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), maxLine)
	n := 0
	for sc.Scan() {
		n++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e event.Event
		if err := json.Unmarshal(line, &e); err != nil {
			bad = append(bad, LineError{Line: n, Err: err.Error()})
			continue
		}
		if err := event.Validate(e, true); err != nil {
			bad = append(bad, LineError{Line: n, Err: err.Error()})
			continue
		}
		evs = append(evs, e)
	}
	return evs, bad, sc.Err()
}

// ReadFile scans path. A missing file is an empty log.
func ReadFile(path string) ([]event.Event, []LineError, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	return Scan(f)
}

// Result reports what Append did.
type Result struct {
	Appended   []event.Event
	Duplicates int
}

// Append adds evs to the log at path. It takes an exclusive flock, skips
// any event whose id is already in the file (retracted ones included, so a
// retracted import is never re-added), assigns seq after the current last
// seq, writes the lines and fsyncs before releasing the lock. Inputs must
// already carry an id.
func Append(path string, evs []event.Event) (Result, error) {
	var res Result
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return res, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return res, err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return res, fmt.Errorf("lock %s: %w", path, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return res, err
	}
	existing, bad, err := Scan(f)
	if err != nil {
		return res, err
	}
	if len(bad) > 0 {
		return res, fmt.Errorf("%s has %d invalid line(s), first at line %d: %s; run lq doctor", path, len(bad), bad[0].Line, bad[0].Err)
	}
	seen := make(map[string]bool, len(existing))
	var last int64
	for _, e := range existing {
		seen[e.ID] = true
		if e.Seq > last {
			last = e.Seq
		}
	}

	var buf bytes.Buffer
	for _, e := range evs {
		if seen[e.ID] {
			res.Duplicates++
			continue
		}
		last++
		e.Seq = last
		if err := event.Validate(e, true); err != nil {
			return Result{}, fmt.Errorf("event %s: %w", e.ID, err)
		}
		line, err := event.Marshal(e)
		if err != nil {
			return Result{}, err
		}
		buf.Write(line)
		buf.WriteByte('\n')
		seen[e.ID] = true
		res.Appended = append(res.Appended, e)
	}
	if buf.Len() == 0 {
		return res, nil
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		return Result{}, err
	}
	return res, f.Sync()
}
