// Package snapshot folds the event log into snapshot.json, the one file
// the CORE & CRAFT widgets read. Progress is arithmetic over effective events;
// nothing here is self-reported.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
	"github.com/harlanljones/core-and-craft/internal/graph"
	"github.com/harlanljones/core-and-craft/internal/store"
)

const (
	FileName      = "snapshot.json"
	defaultMinAcc = 97.0
	defaultRuns   = 3
	defaultDays   = 7
	dailyWindow   = 30
)

type Source struct {
	Lines  int    `json:"lines"`
	SHA256 string `json:"sha256"`
}

type Day struct {
	Date string   `json:"date"`
	Best *float64 `json:"best"`
	Runs int      `json:"runs"`
}

type Skill struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Pillar         string         `json:"pillar"`
	Metric         string         `json:"metric"`
	MinAcc         float64        `json:"min_acc"`
	Protocol       map[string]any `json:"protocol,omitempty"`
	Best           *float64       `json:"best"`
	BestEvent      string         `json:"best_event,omitempty"`
	BestAt         string         `json:"best_at,omitempty"`
	Target         *float64       `json:"target"`
	SustainRuns    int            `json:"sustain_runs"`
	SustainDays    int            `json:"sustain_days"`
	Passed         bool           `json:"passed"`
	PassedAt       string         `json:"passed_at,omitempty"`
	RunsQualifying int            `json:"runs_qualifying"`
	RunsTotal      int            `json:"runs_total"`
	LastAt         string         `json:"last_at,omitempty"`
	TodayBest      *float64       `json:"today_best"`
	Daily          []Day          `json:"daily"`
}

type Pillar struct {
	Pillar   string `json:"pillar"`
	Events7d int    `json:"events_7d"`
	Today    int    `json:"today"`
	XP       int    `json:"xp"`
	Level    int    `json:"level"`
	Into     int    `json:"into"`
	Need     int    `json:"need"`
}

type Today struct {
	Date        string `json:"date"`
	TypingTests int    `json:"typing_tests"`
	Events      int    `json:"events"`
}

type Snapshot struct {
	V       int               `json:"v"`
	BuiltAt string            `json:"built_at"`
	TZ      string            `json:"tz"`
	Source  Source            `json:"source"`
	Skills  []Skill           `json:"skills"`
	Today   Today             `json:"today"`
	Pillars []Pillar          `json:"pillars"`
	Errors  []store.LineError `json:"errors"`

	Quests   []QuestView `json:"quests"`
	Proposed []Proposal  `json:"proposed"`
	Checkins []Checkin   `json:"checkins"`
	Asks     []Ask       `json:"asks"`
	XP       Level       `json:"xp"`
	Streak   Streak      `json:"streak"`
	History  []int       `json:"history"`
	// LeveledToday lists the pillars that passed a level boundary today, in
	// framework order, for the town's ✦.
	LeveledToday []string `json:"leveled_today"`

	// Graph counts graph.json, which acta snapshot writes beside this file
	// (docs/knowledge-graph.md). Build leaves it nil; cmd/acta fills it in.
	Graph *graph.Summary `json:"graph,omitempty"`
}

// Effective drops retracted events and events superseded by a live one.
func Effective(evs []event.Event) []event.Event {
	retracted := map[string]bool{}
	for _, e := range evs {
		if e.Kind == "event.retracted" {
			retracted[e.Subject] = true
		}
	}
	superseded := map[string]bool{}
	for _, e := range evs {
		if e.Supersedes != "" && !retracted[e.ID] {
			superseded[e.Supersedes] = true
		}
	}
	var out []event.Event
	for _, e := range evs {
		if e.Kind == "event.retracted" || retracted[e.ID] || superseded[e.ID] {
			continue
		}
		out = append(out, e)
	}
	return out
}

// imported reports whether a typing.test came from the Monkeytype archiver.
func imported(e event.Event) bool {
	return strings.HasPrefix(e.ID, "mt:")
}

// Unshadowed drops each hand-logged typing.test (a run read off a
// screenshot in chat, say) that an imported Monkeytype run repeats: same
// subject, same local date, same wpm and acc rounded to whole numbers, as
// Monkeytype's result screen shows them. Each import stands in for at most
// one hand-logged run, and the import wins, since it carries the exact
// numbers. The log keeps both; only the fold counts one.
func Unshadowed(live []event.Event, loc *time.Location) []event.Event {
	key := func(e event.Event) (string, bool) {
		m := decode(e.Data)
		wpm, ok1 := num(m["wpm"])
		acc, ok2 := num(m["acc"])
		if !ok1 || !ok2 {
			return "", false
		}
		return fmt.Sprintf("%s|%s|%.0f|%.0f", e.Subject, e.LocalDate(loc), math.Round(wpm), math.Round(acc)), true
	}
	imports := map[string]int{}
	for _, e := range live {
		if e.Kind == "typing.test" && imported(e) {
			if k, ok := key(e); ok {
				imports[k]++
			}
		}
	}
	if len(imports) == 0 {
		return live
	}
	out := make([]event.Event, 0, len(live))
	for _, e := range live {
		if e.Kind == "typing.test" && !imported(e) {
			if k, ok := key(e); ok && imports[k] > 0 {
				imports[k]--
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

func decode(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	return m
}

func num(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	}
	return 0, false
}

func truthy(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

// noFunbox treats a missing, empty, "none" or empty-list funbox as none.
func noFunbox(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == "" || x == "none"
	case []any:
		return len(x) == 0 || (len(x) == 1 && fmt.Sprint(x[0]) == "none")
	}
	return false
}

// matches reports whether a run's settings meet the skill's protocol.
func matches(protocol, run map[string]any) bool {
	for _, k := range []string{"mode", "mode2", "language", "punctuation", "numbers"} {
		want, ok := protocol[k]
		if !ok {
			continue
		}
		if fmt.Sprint(run[k]) != fmt.Sprint(want) {
			return false
		}
	}
	if f, ok := protocol["funbox"]; ok && noFunbox(f) && !noFunbox(run["funbox"]) {
		return false
	}
	if lz, ok := protocol["lazyMode"]; ok && !truthy(lz) && truthy(run["lazyMode"]) {
		return false
	}
	return true
}

type run struct {
	ev  event.Event
	t   time.Time
	wpm float64
}

func round1(f float64) *float64 {
	r := math.Round(f*10) / 10
	return &r
}

// Build folds evs (already validated) into a snapshot at now in loc.
func Build(evs []event.Event, bad []store.LineError, src Source, now time.Time, loc *time.Location) Snapshot {
	live := Effective(evs)
	sort.SliceStable(live, func(i, j int) bool { return live[i].Seq < live[j].Seq })
	live = Unshadowed(live, loc)
	today := now.In(loc).Format("2006-01-02")

	snap := Snapshot{
		V:       1,
		BuiltAt: now.UTC().Format(time.RFC3339),
		TZ:      loc.String(),
		Source:  src,
		Skills:  []Skill{},
		Today:   Today{Date: today},
		Errors:  bad,
	}
	if snap.Errors == nil {
		snap.Errors = []store.LineError{}
	}

	// Last definition, protocol and target per skill, in log order.
	type skillDef struct {
		def, protocol, target map[string]any
		pillar                string
	}
	defs := map[string]*skillDef{}
	var order []string
	for _, e := range live {
		switch e.Kind {
		case "skill.defined", "skill.protocol_set", "skill.target_set":
			d, ok := defs[e.Subject]
			if !ok {
				d = &skillDef{}
				defs[e.Subject] = d
				order = append(order, e.Subject)
			}
			switch e.Kind {
			case "skill.defined":
				d.def, d.pillar = decode(e.Data), e.Pillar
			case "skill.protocol_set":
				d.protocol = decode(e.Data)
			case "skill.target_set":
				d.target = decode(e.Data)
			}
		}
	}

	cutoff7 := now.Add(-7 * 24 * time.Hour)
	pillarCount := map[string]int{}
	todayCount := map[string]int{}
	for _, e := range live {
		if e.LocalDate(loc) == today {
			snap.Today.Events++
			todayCount[e.Pillar]++
			if e.Kind == "typing.test" {
				snap.Today.TypingTests++
			}
		}
		if t, err := e.Time(loc); err == nil && !t.Before(cutoff7) && !t.After(now) {
			pillarCount[e.Pillar]++
		}
	}
	for _, p := range event.Pillars {
		snap.Pillars = append(snap.Pillars, Pillar{Pillar: p, Events7d: pillarCount[p], Today: todayCount[p]})
	}

	runsByDay := map[string]map[string]int{}
	for _, id := range order {
		d := defs[id]
		if d.def == nil {
			continue
		}
		sk := Skill{
			ID:          id,
			Pillar:      d.pillar,
			Metric:      "best_wpm_at_acc",
			MinAcc:      defaultMinAcc,
			Protocol:    d.protocol,
			SustainRuns: defaultRuns,
			SustainDays: defaultDays,
			Daily:       []Day{},
		}
		if s, ok := d.def["name"].(string); ok {
			sk.Name = s
		}
		if s, ok := d.def["metric"].(string); ok {
			sk.Metric = s
		}
		if f, ok := num(d.protocol["min_acc"]); ok {
			sk.MinAcc = f
		}
		if d.target != nil {
			if f, ok := num(d.target["min_acc"]); ok {
				sk.MinAcc = f
			}
			if f, ok := num(d.target["target"]); ok {
				sk.Target = &f
			}
			if f, ok := num(d.target["sustain_runs"]); ok && f >= 1 {
				sk.SustainRuns = int(f)
			}
			if f, ok := num(d.target["sustain_days"]); ok && f >= 1 {
				sk.SustainDays = int(f)
			}
		}

		var qual []run
		for _, e := range live {
			if e.Kind != "typing.test" || e.Subject != id {
				continue
			}
			sk.RunsTotal++
			if sk.LastAt == "" || later(e, sk.LastAt, loc) {
				sk.LastAt = e.At
			}
			m := decode(e.Data)
			wpm, ok1 := num(m["wpm"])
			acc, ok2 := num(m["acc"])
			if !ok1 || !ok2 || acc < sk.MinAcc || truthy(m["bailedOut"]) {
				continue
			}
			if d.protocol != nil && !matches(d.protocol, m) {
				continue
			}
			t, err := e.Time(loc)
			if err != nil {
				continue
			}
			qual = append(qual, run{ev: e, t: t, wpm: wpm})
		}
		sk.RunsQualifying = len(qual)
		runsByDay[id] = map[string]int{}

		byDay := map[string]*Day{}
		for _, r := range qual {
			if sk.Best == nil || r.wpm > *sk.Best {
				sk.Best, sk.BestEvent, sk.BestAt = round1(r.wpm), r.ev.ID, r.ev.At
			}
			date := r.ev.LocalDate(loc)
			dd, ok := byDay[date]
			if !ok {
				dd = &Day{Date: date}
				byDay[date] = dd
			}
			dd.Runs++
			runsByDay[id][date]++
			if dd.Best == nil || r.wpm > *dd.Best {
				dd.Best = round1(r.wpm)
			}
		}
		if dd, ok := byDay[today]; ok {
			sk.TodayBest = dd.Best
		}
		start := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
		for i := dailyWindow - 1; i >= 0; i-- {
			date := start.AddDate(0, 0, -i).Format("2006-01-02")
			if dd, ok := byDay[date]; ok {
				sk.Daily = append(sk.Daily, *dd)
			} else {
				sk.Daily = append(sk.Daily, Day{Date: date})
			}
		}

		if sk.Target != nil {
			var hits []run
			for _, r := range qual {
				if r.wpm >= *sk.Target {
					hits = append(hits, r)
				}
			}
			sort.SliceStable(hits, func(i, j int) bool { return hits[i].t.Before(hits[j].t) })
			window := time.Duration(sk.SustainDays) * 24 * time.Hour
			for i := 0; i+sk.SustainRuns-1 < len(hits); i++ {
				end := hits[i+sk.SustainRuns-1]
				if end.t.Sub(hits[i].t) <= window {
					sk.Passed, sk.PassedAt = true, end.ev.At
					break
				}
			}
		}
		snap.Skills = append(snap.Skills, sk)
	}

	// The game: quests ranked by the engine, check-ins, xp, levels, streak.
	todayT := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	qs := Quests(live, loc)
	skillByID := map[string]Skill{}
	for _, sk := range snap.Skills {
		skillByID[sk.ID] = sk
	}
	minutes := map[string]map[string]float64{}
	for _, e := range live {
		if e.Kind != studySessionKind {
			continue
		}
		if f, ok := num(decode(e.Data)["minutes"]); ok {
			if minutes[e.Subject] == nil {
				minutes[e.Subject] = map[string]float64{}
			}
			minutes[e.Subject][e.LocalDate(loc)] += f
		}
	}
	snap.Quests, snap.Proposed = rank(qs, engineInputs{
		today: todayT, quiet: pillarCount, skills: skillByID, runs: runsByDay, minutes: minutes, timeZone: loc,
	})
	snap.Checkins = checkins(live, todayT, loc)
	snap.Asks = asks(live, todayT, loc)
	questByID := map[string]Quest{}
	for _, q := range qs {
		questByID[q.ID] = q
	}
	var perPillar map[string]Level
	gs := gains(live, xpInputs{runsByDay: runsByDay, skills: snap.Skills, quests: questByID, notes: firstNotes(evs)}, loc)
	snap.XP, perPillar, snap.History = rollUp(gs, todayT)
	snap.LeveledToday = leveledToday(gs, todayT)
	for i := range snap.Pillars {
		l := perPillar[snap.Pillars[i].Pillar]
		snap.Pillars[i].XP, snap.Pillars[i].Level, snap.Pillars[i].Into, snap.Pillars[i].Need = l.Total, l.Level, l.Into, l.Need
	}
	snap.Streak = streakOf(live, todayT, loc)
	return snap
}

func later(e event.Event, at string, loc *time.Location) bool {
	t1, err1 := e.Time(loc)
	other := event.Event{At: at, AtPrecision: event.PrecisionSecond}
	if len(at) == len("2006-01-02") {
		other.AtPrecision = event.PrecisionDay
	}
	t2, err2 := other.Time(loc)
	if err1 != nil || err2 != nil {
		return false
	}
	return !t1.Before(t2)
}

// SourceOf hashes the raw log so the widget can tell which log it shows.
func SourceOf(path string) (Source, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Source{SHA256: hex.EncodeToString(sha256.New().Sum(nil))}, nil
	}
	if err != nil {
		return Source{}, err
	}
	lines := 0
	for _, c := range b {
		if c == '\n' {
			lines++
		}
	}
	sum := sha256.Sum256(b)
	return Source{Lines: lines, SHA256: hex.EncodeToString(sum[:])}, nil
}

// Write stores s at path via a temp file and rename, so widgets watching
// the file never read half a snapshot.
func Write(path string, s Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
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
	return os.Rename(tmp.Name(), path)
}
