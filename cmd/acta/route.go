package main

// The Duet-to-acta inbox route: Drive folder -> rclone remote ccinbox ->
// acta-inbox.service -> inbox/ -> acta ingest. doctor checks each hop it can
// see without the network, so a failure shows before the route goes quiet.

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	inboxRemote   = "ccinbox"
	inboxUnit     = "acta-inbox.service"
	inboxMaxAge   = 30 * time.Minute
	clientIDDocs  = "https://rclone.org/drive/#making-your-own-client-id"
	systemdLayout = "Mon 2006-01-02 15:04:05 MST"
)

// runCommand runs name with args and returns its stdout. If name is not on
// PATH it tries ~/.local/bin/name, where the systemd units expect rclone.
func runCommand(name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", err
		}
		alt := filepath.Join(home, ".local", "bin", name)
		if _, serr := os.Stat(alt); serr != nil {
			return "", err
		}
		path = alt
	}
	out, err := exec.Command(path, args...).Output()
	return string(out), err
}

// runExt runs an external program through e.command. A nil e.command acts
// as if the program is not installed, so tests never reach the real host.
func (e env) runExt(name string, args ...string) (string, error) {
	if e.command == nil {
		return "", exec.ErrNotFound
	}
	return e.command(name, args...)
}

// routeReport collects one line per check. Only warnings count; doctor's exit
// code stays tied to the log's invariants.
type routeReport struct {
	lines []string
	warns int
}

func (r *routeReport) ok(format string, a ...any) {
	r.lines = append(r.lines, fmt.Sprintf(format, a...))
}

func (r *routeReport) warn(format string, a ...any) {
	r.warns++
	r.lines = append(r.lines, "warn: "+fmt.Sprintf(format, a...))
}

func checkRoute(e env) *routeReport {
	r := &routeReport{}
	checkRejected(e, r)
	checkClientID(e, r)
	checkInboxPull(e, r)
	return r
}

// checkRejected counts quarantined inbox files and shows the newest reason.
func checkRejected(e env, r *routeReport) {
	files, err := filepath.Glob(filepath.Join(e.rejectedDir(), "*.jsonl"))
	if err != nil {
		r.warn("inbox: %v", err)
		return
	}
	if len(files) == 0 {
		r.ok("inbox: ok, no rejected files")
		return
	}
	type rej struct {
		name string
		at   time.Time
	}
	var rs []rej
	for _, f := range files {
		// The .err file is written when the file is quarantined; the .jsonl
		// keeps the mtime it had in the inbox (it is hard-linked).
		var at time.Time
		if fi, err := os.Stat(f + ".err"); err == nil {
			at = fi.ModTime()
		} else if fi, err := os.Stat(f); err == nil {
			at = fi.ModTime()
		}
		rs = append(rs, rej{filepath.Base(f), at})
	}
	sort.Slice(rs, func(i, j int) bool {
		if !rs[i].at.Equal(rs[j].at) {
			return rs[i].at.After(rs[j].at)
		}
		return rs[i].name > rs[j].name
	})
	newest := rs[0].name
	reason := errReason(filepath.Join(e.rejectedDir(), newest+".err"))
	r.warn("inbox: %d rejected file(s) in %s; newest %s: %s", len(rs), e.rejectedDir(), newest, reason)
}

// errReason returns the first line of a .err file's reason. quarantine writes
// "<name> rejected:" as a header line, so that header is skipped.
func errReason(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "no .err file"
	}
	var first string
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if first == "" {
			first = l
			if strings.HasSuffix(l, " rejected:") {
				continue
			}
		}
		return l
	}
	if first == "" {
		return "empty .err file"
	}
	return first
}

// checkClientID reads rclone.conf (no network) and warns when the ccinbox
// remote has no client_id of its own, so it uses rclone's shared one. A
// remote that uses a service account needs no client_id: it passes, and its
// key file must exist and not be readable by others.
func checkClientID(e env, r *routeReport) {
	out, err := e.runExt("rclone", "config", "file")
	if errors.Is(err, exec.ErrNotFound) {
		r.ok("rclone: not configured (rclone not found)")
		return
	}
	if err != nil {
		r.warn("rclone: config file: %v", err)
		return
	}
	path := lastLine(out)
	if path == "" {
		r.warn("rclone: config file: no path in %q", strings.TrimSpace(out))
		return
	}
	section, found, err := iniSection(path, inboxRemote)
	if errors.Is(err, os.ErrNotExist) {
		r.ok("rclone: not configured (no %s)", path)
		return
	}
	if err != nil {
		r.warn("rclone: %v", err)
		return
	}
	if !found {
		r.ok("rclone: not configured (no [%s] remote in %s)", inboxRemote, path)
		return
	}
	// A service account needs no OAuth client: the folder is shared with it.
	if creds := strings.TrimSpace(section["service_account_credentials"]); creds != "" {
		r.ok("rclone: ok, remote %s uses a service account", inboxRemote)
		return
	}
	if sa := strings.TrimSpace(section["service_account_file"]); sa != "" {
		fi, err := os.Stat(sa)
		switch {
		case err != nil:
			r.warn("rclone: remote %s service account file %s: %v", inboxRemote, sa, err)
		case fi.Mode().Perm()&0o077 != 0:
			r.warn("rclone: service account key %s is readable by others (mode %o); chmod 600 it", sa, fi.Mode().Perm())
		default:
			r.ok("rclone: ok, remote %s uses a service account", inboxRemote)
		}
		return
	}
	if strings.TrimSpace(section["client_id"]) == "" {
		r.warn("rclone: remote %s uses rclone's shared Google Drive client_id, which is being retired during 2026; make your own: %s", inboxRemote, clientIDDocs)
		return
	}
	r.ok("rclone: ok, remote %s has its own client_id", inboxRemote)
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// iniSection returns the keys of [name] in an rclone.conf-style INI file.
// Values are never printed: the section holds the OAuth token.
func iniSection(path, name string) (map[string]string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	keys := map[string]string{}
	found, in := false, false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || l[0] == '#' || l[0] == ';' {
			continue
		}
		if strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			in = strings.TrimSpace(l[1:len(l)-1]) == name
			found = found || in
			continue
		}
		if !in {
			continue
		}
		k, v, ok := strings.Cut(l, "=")
		if ok {
			keys[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return keys, found, sc.Err()
}

// checkInboxPull reads the last result of acta-inbox.service. No systemctl
// (or no user manager) means no line at all.
func checkInboxPull(e env, r *routeReport) {
	out, err := e.runExt("systemctl", "--user", "show", inboxUnit, "--timestamp=unix", "-p", "ExecMainExitTimestamp,Result,LoadState,ActiveState")
	if err != nil {
		return
	}
	props := map[string]string{}
	for _, l := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(l), "="); ok {
			props[k] = v
		}
	}
	if s := props["LoadState"]; s != "" && s != "loaded" {
		r.ok("inbox pull: not configured (%s is %s)", inboxUnit, s)
		return
	}
	// A running oneshot has no exit timestamp yet; that is not "never ran".
	if st := props["ActiveState"]; st == "active" || st == "activating" {
		r.ok("inbox pull: running now")
		return
	}
	if res := props["Result"]; res != "" && res != "success" {
		r.warn("inbox pull: %s last run failed (Result=%s); see journalctl --user -u %s", inboxUnit, res, inboxUnit)
		return
	}
	ts := strings.TrimSpace(props["ExecMainExitTimestamp"])
	if ts == "" || ts == "n/a" {
		r.warn("inbox pull: %s has not run yet", inboxUnit)
		return
	}
	at, err := parseSystemdTime(ts, e.loc)
	if err != nil {
		r.warn("inbox pull: ExecMainExitTimestamp %q: %v", ts, err)
		return
	}
	age := e.now().Sub(at)
	if age > inboxMaxAge {
		r.warn("inbox pull: last run %s ago (at %s), over %s; check acta-inbox.timer", age.Round(time.Minute), at.In(e.loc).Format(time.RFC3339), inboxMaxAge)
		return
	}
	r.ok("inbox pull: ok, last run %s ago", age.Round(time.Second))
}

// parseSystemdTime reads "@<unix seconds>" (--timestamp=unix) or systemd's
// default "Thu 2026-10-29 11:55:00 PDT".
func parseSystemdTime(s string, loc *time.Location) (time.Time, error) {
	if strings.HasPrefix(s, "@") {
		n, err := strconv.ParseInt(s[1:], 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(n, 0), nil
	}
	return time.ParseInLocation(systemdLayout, s, loc)
}
