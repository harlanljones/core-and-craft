package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
	"github.com/harlanljones/core-and-craft/internal/relay"
	"github.com/harlanljones/core-and-craft/internal/store"
)

// cmdAsk logs Harlan's question to a pillar's Duet agent. acta relay sends
// it; the agent's pillar.answered comes back through the inbox.
func cmdAsk(e env, args []string) error {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	pillar := fs.String("pillar", "", "the pillar whose agent you ask")
	text := fs.String("text", "", "the question")
	agent, via := writerFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pillar == "" || strings.TrimSpace(*text) == "" {
		return errors.New("ask needs --pillar and --text")
	}
	id := "ask:" + slug(*text, e.now())
	ev, err := newEvent(e, "pillar.asked", id, *pillar, toJSON(map[string]any{"text": strings.TrimSpace(*text)}), "", *via, *agent, "", "", "")
	if err != nil {
		return err
	}
	if _, err := store.Append(e.logPath(), []event.Event{ev}); err != nil {
		return err
	}
	fmt.Fprintf(e.stdout, "%s asked duet:%s\n", id, *pillar)
	return nil
}

// relayRoute reads one pillar's fire URL and token: ACTA_RELAY_<PILLAR> and
// ACTA_RELAY_<PILLAR>_TOKEN, or a systemd credential named relay-<pillar>.
// ok is false when the pillar has no route yet.
func relayRoute(e env, pillar string) (r relay.Route, ok bool, err error) {
	key := "ACTA_RELAY_" + strings.ToUpper(pillar)
	r.URL = strings.TrimSpace(e.getenv(key))
	r.Token = strings.TrimSpace(e.getenv(key + "_TOKEN"))
	if r.Token == "" {
		if dir := e.getenv("CREDENTIALS_DIRECTORY"); dir != "" {
			if b, err := os.ReadFile(filepath.Join(dir, "relay-"+pillar)); err == nil {
				r.Token = strings.TrimSpace(string(b))
			}
		}
	}
	switch {
	case r.URL == "" && r.Token == "":
		return r, false, nil
	case !relay.ValidURL(r.URL):
		return r, false, fmt.Errorf("%s is not a routine fire URL on %s", key, relay.Host)
	case r.Token == "":
		return r, false, fmt.Errorf("%s has a URL but no token (%s_TOKEN or credential relay-%s)", key, key, pillar)
	}
	return r, true, nil
}

// cmdRelay sends every item no pillar agent has had yet, one fire per pillar.
// The first run marks what is already in the log as sent, so installing it
// never replays history. A pillar without a route keeps its items waiting.
func cmdRelay(e env, args []string) error {
	fs := flag.NewFlagSet("relay", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	dry := fs.Bool("dry-run", false, "print what would be sent; send and record nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	unlock, err := relay.Lock(e.relayPath())
	if err != nil {
		return err
	}
	defer unlock()
	st, found, err := relay.Load(e.relayPath())
	if err != nil {
		return err
	}
	live, err := liveLog(e)
	if err != nil {
		return err
	}
	items := relay.Select(live)
	now := e.now().UTC().Format(time.RFC3339)
	if !found {
		if *dry {
			fmt.Fprintf(e.stdout, "relay: no state yet; the first run marks %d item(s) as sent\n", len(items))
			return nil
		}
		for _, it := range items {
			st.Sent[it.Event.ID] = relay.Sent{At: now, To: it.To, Session: "baseline"}
		}
		if err := relay.Save(e.relayPath(), st); err != nil {
			return err
		}
		fmt.Fprintf(e.stdout, "relay: first run, %d earlier item(s) marked as sent\n", len(items))
		return nil
	}

	byPillar := map[string][]relay.Item{}
	for _, it := range items {
		if _, done := st.Sent[it.Event.ID]; !done {
			byPillar[it.To] = append(byPillar[it.To], it)
		}
	}
	client := relay.Client{BaseURL: e.relayBaseURL}
	var errs []error
	for _, p := range event.Pillars {
		pending := byPillar[p]
		if len(pending) == 0 {
			continue
		}
		route, ok, err := relayRoute(e, p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		batch, text, err := relay.Batch(p, pending)
		if err != nil {
			return err
		}
		if *dry {
			fmt.Fprintf(e.stdout, "--- %s (%d waiting, route: %v)\n%s", p, len(pending), ok, text)
			continue
		}
		if !ok {
			fmt.Fprintf(e.stdout, "relay: %s: %d waiting, no route (ACTA_RELAY_%s)\n", p, len(pending), strings.ToUpper(p))
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		session, err := client.Fire(ctx, route, text)
		cancel()
		var rl relay.RateLimited
		if errors.As(err, &rl) {
			fmt.Fprintf(e.stdout, "relay: %s: %d waiting, %v\n", p, len(pending), rl)
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p, err))
			continue
		}
		for _, it := range batch {
			st.Sent[it.Event.ID] = relay.Sent{At: now, To: p, Session: session}
		}
		// Record each fire as it lands: there is no idempotency key, so a
		// crash after a fire must not send the same items again.
		if err := relay.Save(e.relayPath(), st); err != nil {
			return err
		}
		fmt.Fprintf(e.stdout, "relay: %s: sent %d, %s\n", p, len(batch), session)
		if left := len(pending) - len(batch); left > 0 {
			fmt.Fprintf(e.stdout, "relay: %s: %d left for the next run\n", p, left)
		}
	}
	return errors.Join(errs...)
}
