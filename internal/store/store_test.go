package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/harlanljones/core-and-craft/internal/event"
)

func ev(id string) event.Event {
	return event.Event{
		ID: id, V: 1, At: "2026-10-29", AtPrecision: event.PrecisionDay,
		RecordedAt: "2026-10-29T17:45:00Z", Pillar: "competence", Kind: "typing.test",
		Subject: "skill:typing", Source: event.Source{Via: "chat", Agent: "lq"},
		Data: json.RawMessage(`{"wpm":80,"acc":97}`),
	}
}

func TestAppendAssignsSeqAndDedupes(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	res, err := Append(path, []event.Event{ev("mt:a"), ev("mt:b")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Appended) != 2 || res.Appended[1].Seq != 2 {
		t.Fatalf("got %+v", res)
	}
	res, err = Append(path, []event.Event{ev("mt:b"), ev("mt:c"), ev("mt:c")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Appended) != 1 || res.Duplicates != 2 || res.Appended[0].Seq != 3 {
		t.Fatalf("got appended=%d dups=%d", len(res.Appended), res.Duplicates)
	}
	evs, bad, err := ReadFile(path)
	if err != nil || len(bad) > 0 || len(evs) != 3 {
		t.Fatalf("read back %d events, %v, %v", len(evs), bad, err)
	}
}

func TestAppendConcurrentSeqIsUniqueAndGapFree(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	const n = 40
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := Append(path, []event.Event{ev(fmt.Sprintf("mt:r%d", i))}); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	evs, bad, err := ReadFile(path)
	if err != nil || len(bad) > 0 {
		t.Fatal(err, bad)
	}
	if len(evs) != n {
		t.Fatalf("want %d events, got %d", n, len(evs))
	}
	for i, e := range evs {
		if e.Seq != int64(i+1) {
			t.Fatalf("line %d has seq %d", i+1, e.Seq)
		}
	}
}

func TestAppendRefusesCorruptLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(path, []event.Event{ev("mt:a")}); err == nil {
		t.Fatal("want error on corrupt log")
	}
}

func TestScanReportsBadLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if _, err := Append(path, []event.Event{ev("mt:a")}); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"id":"mt:b","v":9}` + "\n")
	f.Close()
	evs, bad, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || len(bad) != 1 || bad[0].Line != 2 {
		t.Fatalf("evs=%d bad=%+v", len(evs), bad)
	}
}
