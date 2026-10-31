# 0004: Channels read the notes they write to, and propose the nodes they extract

Status: proposed, 2026-10-30

## Context

- Since PR #34, the latest version filed takes a note's path, and the version it replaces moves to `note-versions/<sha256>.md`. Two channels can now both write a shared node.
- The channels can't see `notes/`. A channel writing Luther from memory replaces a richer version (the old one is kept, but it leaves the graph), and it can't set `supersedes` because it never sees event ids. Nothing tells a channel the note format either: the `<sha256>.md` name, the paths, the frontmatter, the two vocabularies.
- The last section of `docs/knowledge-graph.md` specifies a separate extraction engine (OpenRouter, the Anthropic API or a local model) and doesn't build it. The review found gaps: it never says private notes stay home, a "proposed note" has nowhere to live, node and edge ids take different forms, the `known` list grows without bound, a rerun proposes the same nodes again, and the `evidence` quotes aren't checked.
- The Duet pillar channels are already Claude. They hold the study context, and the competence channel proposed the 63-node seed by hand.
- The inbox remote `ccinbox:` is a Google service account on a folder in Harlan's personal My Drive. A service account has no Drive storage of its own, so it can read the folder but is not expected to create files there (not yet tried). Shared drives need Google Workspace.

## Decision

### The outbox: what the channels read

- `acta snapshot` writes `$ACTA_STATE/outbox/`: `notes-index.json`, and the body of each live, non-private note as `<sha256>.md`.
- `notes-index.json` holds `v`, `built_at`, the node and edge type vocabularies, and per live note `{id, path, type, title, aliases, pillars, sha256, event}`. `event` is the id of the `note.added` a new version should supersede. A private note lists only `id`, `path`, `sha256` and `event`: its slug is taken, and nothing else of it leaves diocletian. It also lists the pending proposals and the declined ones (below).
- `acta outbox push`, run after each snapshot, does `rclone sync $ACTA_STATE/outbox ccoutbox:`. `ccoutbox:` is a second remote, authorized once as Harlan with `scope=drive.file`, rooted at a folder "core-and-craft outbox" that rclone creates. With `drive.file`, rclone sees only the files it made. The inbox remote stays read only.
- Before filing, a channel reads the outbox through its Drive connector. It updates an existing node from its current body, sets `supersedes` to that node's `event`, and picks a slug nobody uses.
- `docs/duet-notes.md` is the channel prompt for notes, written like `docs/duet-relay.md`: paths, body naming, frontmatter, vocabularies, the outbox, and proposals.

### Extraction: the channels propose

- Extraction runs in a pillar channel, when Harlan asks it to split a note into nodes. acta builds no engine: there is no API key or spend on diocletian, and the model doing the work is the one that knows the study. A routine may run it later.
- A proposal is a `note.added` with `data.status: "proposed"`, `data.from: {note, sha256}` (the note version it came from) and `data.evidence` (the text it quotes from that version). Its `path` is where the node would live. Ingest keeps the body in `note-versions/<sha256>.md` and never places it at `path`.
- From a `duet:` agent, a `note.added` that would create a new node in `notes/nodes/` counts as proposed, with or without the field, in the same way a quest from an agent is proposed. The fold checks this in `seq` order, so existing events are unaffected. Updating an existing node, or filing a note in a pillar folder, goes straight in as today, because those are records of what was studied.
- **Accept.** `acta note accept <id>` places the body by the usual rules and logs a `manual` `note.added` for the same path and sha256 that supersedes the proposal.
- **Decline.** `acta note decline <id>` retracts it with reason `declined`. Declined proposals stay in the index, by slug and source sha256, so a channel doesn't propose them again from the same note version.
- **Board and relay.** The town shows pending proposals beside proposed quests, with accept and decline taps. `acta relay` sends the accept or decline to the channel that proposed it, as it does for quests.
- **Checks.** Each check lists a problem and never rejects anything, since Harlan decides anyway.
  - The evidence must appear in the source version, compared with whitespace collapsed.
  - Repeats of a slug that is already pending from the same source version are not shown; the first one is.
- **What stays out.** `graph.json` leaves proposals out. `snapshot.json` lists them as `proposed_notes` for the board. A proposal earns no xp, and an accepted node, being in `notes/nodes/`, earns none either.
- **Private notes.** They are never in the outbox, and `docs/duet-notes.md` tells channels not to extract from a note marked `private: true`.

### What the old specification becomes

The request and answer JSON in `docs/knowledge-graph.md` is replaced. The outbox is the request (what is known and the vocabularies), and proposals are the answer, with one id form (`note:<slug>` subjects and `[[slug]]` links). A local engine, an `acta extract` that calls an API, could write the same proposals later without changing anything else.

## Alternatives rejected

- **acta calls the Anthropic API, OpenRouter or a local model.** That puts a key and spend on diocletian, and a second Claude without the channel's context. The proposal format leaves this open.
- **Sending the index through `acta relay`.** Every fire wakes a channel and uses one of its 30 fires an hour. The index changes with every note, and a channel only needs it when it files one.
- **Uploading with the inbox's service account.** It has no storage of its own in a personal My Drive. Keeping the inbox remote read only also leaves one job per credential.
- **New kinds (`note.proposed`, `note.accepted`, `note.declined`).** `status` on `note.added`, a superseding version and a retraction already say this, and the decision record for the knowledge graph keeps notes to one kind.
- **Every channel node straight in.** A shared node is vocabulary for all nine pillars, so a new one waits for Harlan. Changes to existing nodes don't.

## Consequences

- Harlan sets up `ccoutbox:` once: `rclone config create ccoutbox drive scope=drive.file`, with a browser sign-in, then the outbox folder.
- Each pillar channel gets the prompt in `docs/duet-notes.md`.
- Order of work:
  1. The outbox and `docs/duet-notes.md`.
  2. Proposals: the ingest and fold rule, `acta note accept` and `acta note decline`.
  3. The town's proposal popup and the relay items.
