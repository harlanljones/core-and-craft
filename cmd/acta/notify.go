package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/harlanljones/core-and-craft/internal/snapshot"
)

// notifyState is $ACTA_STATE/notified.json: what was already announced.
type notifyState struct {
	Checkins  []string `json:"checkins"`
	Proposals []string `json:"proposals"`
	Nudged    string   `json:"nudged,omitempty"` // local day of the last evening nudge
}

const (
	defaultNotifier = "omarchy-notification-send"
	nudgeHour       = 20
)

func (e env) notifiedPath() string { return filepath.Join(e.state, "notified.json") }

// cmdNotify sends a desktop notification once per open check-in, once per
// batch of new proposals, and once per evening when the streak is unkept.
// The first run on an existing snapshot only records a baseline, unless
// --all is given.
func cmdNotify(e env, args []string) error {
	all := false
	for _, a := range args {
		if a != "--all" {
			return errors.New("usage: acta notify [--all]")
		}
		all = true
	}
	b, err := os.ReadFile(e.snapshotPath())
	if err != nil {
		return fmt.Errorf("no snapshot yet: run acta snapshot (%w)", err)
	}
	var s snapshot.Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	var st notifyState
	first := false
	if nb, err := os.ReadFile(e.notifiedPath()); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		first = true
	} else if err := json.Unmarshal(nb, &st); err != nil {
		return fmt.Errorf("%s: %w", e.notifiedPath(), err)
	}

	seenC := toSet(st.Checkins)
	seenP := toSet(st.Proposals)

	if first && !all {
		for _, c := range s.Checkins {
			if c.Open {
				seenC[c.ID] = true
			}
		}
		for _, p := range s.Proposed {
			seenP[p.ID] = true
		}
		st.Checkins, st.Proposals = keys(seenC), keys(seenP)
		if err := saveNotified(e, st); err != nil {
			return err
		}
		fmt.Fprintln(e.stdout, "notify: first run, recorded the current items as a baseline and sent nothing")
		return nil
	}

	sent := 0
	var failed error
	send := func(title, body string) bool {
		if err := e.sendNotification(title, body); err != nil {
			failed = err
			return false
		}
		sent++
		return true
	}

	for _, c := range s.Checkins {
		if !c.Open || seenC[c.ID] {
			continue
		}
		if send("Check-in · "+pillarName(c.Pillar), c.Question) {
			seenC[c.ID] = true
		}
	}

	var newP []string
	for _, p := range s.Proposed {
		if !seenP[p.ID] {
			newP = append(newP, p.ID)
		}
	}
	if len(newP) > 0 {
		title := fmt.Sprintf("%d proposals moored", len(newP))
		if len(newP) == 1 {
			title = "1 proposal moored"
		}
		if send(title, "Accept or decline them in the quest log.") {
			for _, id := range newP {
				seenP[id] = true
			}
		}
	}

	loc := e.loc
	if s.TZ != "" {
		if l, err := time.LoadLocation(s.TZ); err == nil {
			loc = l
		}
	}
	now := e.now().In(loc)
	day := now.Format("2006-01-02")
	if now.Hour() >= nudgeHour && !s.Streak.Today && st.Nudged != day {
		if send("Pax · Streak", "Not yet kept today") {
			st.Nudged = day
		}
	}

	st.Checkins, st.Proposals = keys(seenC), keys(seenP)
	if err := saveNotified(e, st); err != nil {
		return err
	}
	if failed != nil {
		return failed
	}
	fmt.Fprintf(e.stdout, "notify: sent %d notification(s)\n", sent)
	return nil
}

func (e env) sendNotification(title, body string) error {
	cmd := strings.Fields(e.getenv("ACTA_NOTIFY_CMD"))
	if len(cmd) == 0 {
		cmd = []string{defaultNotifier}
	}
	c := exec.Command(cmd[0], append(cmd[1:], title, body)...)
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("notifier %s: %w: %s", cmd[0], err, strings.TrimSpace(string(out)))
	}
	return nil
}

func saveNotified(e env, st notifyState) error {
	if err := os.MkdirAll(e.state, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := e.notifiedPath() + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, e.notifiedPath())
}

func pillarName(p string) string {
	if p == "" {
		return "General"
	}
	return strings.ToUpper(p[:1]) + p[1:]
}

func toSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
