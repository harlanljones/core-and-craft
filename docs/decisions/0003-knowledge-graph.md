# 0003: One knowledge graph for all nine pillars, built by acta from notes

Status: proposed, 2026-10-30

## Context

The competence channel proposed a knowledge graph on Oct 29: markdown notes with typed frontmatter links (`branch_of`, `founded_by`, `teaches`, ...), about 60 seed nodes from the religion and history notes, Obsidian to view them and Quartz to publish. Nothing was built. Harlan asked to extend it to all of CORE & CRAFT.

What was already here:

- `note.added` `{path, title, sha256, from_pillar, sessions}` and `notes/<pillar>/` folders, with one seed note.
- The Drive inbox moves only `.jsonl`, so a Duet channel had no way to send a note's text. The seed note was copied by hand.
- Roadmap step 5: FSRS reviews over knowledge notes, then the graph. `review.logged` is already in the envelope.

## Decision

- **Shape.** A note is a markdown file. A pillar's own notes live in `notes/<pillar>/`; nodes every pillar shares (people, events, concepts) live in `notes/nodes/`, one slug per node. Frontmatter holds `type`, `title`, `aliases`, `pillars`, `date`, `recall`, `private` and typed edges from a fixed vocabulary. Edges are `[[links]]`, so Obsidian draws them as well. Body links are `mentions`. See `docs/knowledge-graph.md`.
- **No new event kinds.** `note.added` names one version of a note by `path` and `sha256`; a new version supersedes the old event. acta validates the path and that its slug is the subject's.
- **Bodies through the inbox.** A channel uploads the body as `<sha256>.md` beside its envelope file. Ingest places the body only when an accepted `note.added` names it and the hash matches. The latest version filed takes the path, since two channels writing one shared node can't know each other's event ids; the version it replaces moves to `note-versions/<sha256>.md`. Edits the log doesn't name are never overwritten: the arriving version waits in `note-versions/` for a merge. An envelope file whose body hasn't arrived waits instead of failing. acta stays the only writer of `notes/` as far as the log goes; a note Harlan edits in Obsidian is logged with `acta note add`.
- **The graph is derived.** `acta snapshot` writes `graph.json`: the live notes, the skills, the active and proposed quests and the nine pillars as nodes, with typed, body, pillar and `trains` edges. Missing targets are stubs. Problems (a note edited since it was logged, an unknown edge) are listed, not fatal. `snapshot.json` carries a small summary.
- **Xp.** A note earns its 15 xp once, dated by its first version, so a new version doesn't move it. Nodes in `notes/nodes/` earn none, so the 63-node seed doesn't flood the levels.
- **Seed.** The competence proposal's nodes are written as `seed/notes/nodes/*.md`, filed by `seed/2026-10-30T235000Z-claude-code-graph.jsonl`, with `source:` pointing at the religion and history note.

## Alternatives rejected

- **The note's text inside the event.** It would put whole documents into an append-only log and a 1 MB line limit. The hash binds the file just as firmly.
- **A graph database.** Under a hundred nodes; files and one derived JSON file cover it until there are thousands.
- **Separate skill notes and a typing CSV (the competence proposal's `skills/typing.md` and `data/typing.csv`).** The log already holds the skill, its protocol, its target and every run; a second copy would drift.
- **Free-form edge names.** The board needs a known set to draw and filter. Unknown names are kept as `related` with a warning, so nothing is lost.
- **Requiring approval for every note.** Notes are records of what was studied. Only nodes a language model extracts would be proposals (`docs/knowledge-graph.md`, last section).

## Consequences

- Duet channels that file notes must upload the body as `<sha256>.md` beside the envelope file. Older note lines without `path` still ingest.
- Inbox files that only hold events behave as before. One that names a body waits until the body arrives.
- `board/test/check.sh` and the tests ingest `seed/` from the command line, as Harlan would.
- Reviews and the board's Library map are separate changes; the map waits for a mockup.
