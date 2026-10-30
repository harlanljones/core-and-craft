# Knowledge graph

Every pillar's notes become one typed graph. A note is a markdown file under `notes/`. Its frontmatter names its type and its typed links. `acta snapshot` folds the live notes, the skills, the quests and the nine pillars into `$ACTA_STATE/graph.json`. The board reads that file; nothing else writes it.

The rule stays the same as for the rest of CORE & CRAFT: the log and the notes are the record, and acta derives the rest. There are no new event kinds. `note.added` names a version of a note by `path` and `sha256`, and acta only places a note's body when an accepted event names it.

Why: `docs/decisions/0003-knowledge-graph.md`.

## Where notes live

```
~/core-and-craft/notes/
  competence/religion-and-history.md   a pillar's own notes: a study session, a reading, a plan
  faith/...
  nodes/martin-luther.md               nodes every pillar shares: people, events, concepts
  nodes/ninety-five-theses.md
```

- `notes/<pillar>/<slug>.md` is a pillar's own note. Its type defaults to `subject`.
- `notes/nodes/<slug>.md` is a shared node. One slug names one node across all nine pillars, so two channels that both write about Luther write to the same node.
- The slug is the file name: lowercase letters, digits and hyphens. The note's subject is `note:<slug>`, so a slug is unique across `notes/`.
- The folder opens as an Obsidian vault as it is. Obsidian draws the typed links too, because each one is a `[[link]]`.

## The note file

```markdown
---
type: person
title: "Martin Luther"
aliases: ["Luther"]
pillars: [faith]
authored: "[[ninety-five-theses]]"
teaches:
  - "[[sola-fide]]"
  - "[[sola-scriptura]]"
source: "[[religion-and-history]]"
recall: "When did Luther post the 95 Theses?"
---
# Martin Luther

Posted the [[ninety-five-theses]] on Oct 31 1517, against indulgences.
```

Fields:

| key | means |
|---|---|
| `type` | one of the node types below. Required in `notes/nodes/`; a pillar note defaults to `subject`. |
| `title` | the name the board shows. Default: the first `# heading`, then the slug. |
| `aliases` | other names, for search and for Obsidian. |
| `pillars` | the pillars it belongs to, full lowercase words. The first is its colour on the board. Default: the folder's pillar, or the event's. |
| `date` | when, for an event: `1517-10-31`, `1648`, `451`. |
| `recall` | one question for spaced repetition. Reviews come in a later change; the field is read now so notes can carry it. |
| `private` | `true` keeps it off anything published. |
| an edge type | one link or a list of links: `"[[slug]]"`, `"[[folder/slug\|label]]"`, or an id such as `skill:typing` or `pillar:faith`. |

Any `[[link]]` in the body becomes a `mentions` edge, unless a typed edge already joins the same two nodes. Other keys (Obsidian's own, `tags`, a created date) are ignored. An unknown key that holds `[[links]]` becomes `related` edges, with a problem listed in `graph.json`.

acta reads the part of YAML that Obsidian's properties write: `key: value`, quoted values, `[a, b]` lists and `- item` lists. A nested map is skipped with a problem listed. A note with no frontmatter is fine: it's a `subject` note with links from its body only.

### Node types

`subject`, `person`, `tradition`, `family`, `event`, `concept`, `work`, `place`, `organization`, `dataset`. Another type is kept, with a problem listed.

### Edge types

Each edge reads from the note to its targets.

| edge | example |
|---|---|
| `member_of` | Judaism → Abrahamic |
| `branch_of` | Lutheranism → Protestantism |
| `split_from` | Eastern Orthodoxy → Catholicism |
| `part_of` | a battle → a war |
| `founded_by` | Reformed tradition → John Calvin |
| `authored` | Martin Luther → 95 Theses |
| `teaches` | Judaism → covenant |
| `argues_for` | Paul → faith vs works |
| `opposed` | 95 Theses → indulgences |
| `led_to` | 95 Theses → Reformation |
| `responded_to` | Council of Trent → Reformation |
| `resolved_by` | faith vs works → Joint Declaration |
| `coined_by` | Protestant work ethic → Max Weber |
| `measured_by` | Protestantism → Pew 2023-24 |
| `practices` | a note → `skill:typing` |
| `source` | a node → the note it came from |
| `related` | anything else |

acta adds three kinds itself: `mentions` (body links), `in_pillar` (a note, skill or quest to its pillar) and `trains` (a quest to the skill in its `quest.added`).

## Filing a note

### From a Duet channel (through the Drive inbox)

A channel saves two kinds of file to the "core-and-craft inbox" Drive folder, raw and unconverted:

1. The note body, named by its sha256: `<sha256 of the exact bytes>.md`.
2. An envelope file, as for every event: `<recorded_at>-<agent>.jsonl`, with one `note.added` line per note.

```json
{"v":1,"at":"2026-10-30","at_precision":"day","recorded_at":"2026-10-30T18:00:00Z","pillar":"faith","kind":"note.added","subject":"note:martin-luther","source":{"via":"chat","agent":"duet:competence"},"data":{"path":"notes/nodes/martin-luther.md","sha256":"<64 hex>","title":"Martin Luther","type":"person","from_pillar":"faith"}}
```

```python
import hashlib
body = open("martin-luther.md", "rb").read()
name = hashlib.sha256(body).hexdigest() + ".md"   # upload the body under this name
```

`acta inbox pull` fetches both kinds. Ingest then:

- finds each body as `inbox/<sha256>.md` and checks that it hashes to its name;
- writes it to `notes/<folder>/<slug>.md` (temp file and rename);
- appends the events and removes both files from `inbox/`.

If the envelope file arrives before a body, it waits in `inbox/` and the next run tries again. It isn't rejected; `acta doctor` warns after an hour. A body under the wrong name, or a path that already holds another version (below), rejects the whole file as any bad line does.

### A new version

Write the whole note again and file a `note.added` with `supersedes` set to the id of the event for the version it replaces. acta replaces a note file only then: if the path holds a version that the new event doesn't supersede, the file is rejected. Retract a note with `acta retract <id>`; the file stays on disk and `acta doctor` lists it as not logged.

### On this machine (Obsidian)

```sh
acta note add ~/core-and-craft/notes/nodes/martin-luther.md
```

logs the file as it is now, superseding the version the log had (`--pillar`, `--agent`, `--via` as for `acta log`). A file edited but not re-added shows in `acta doctor` and in `graph.json`'s problems, with the command to run.

### From the command line

`acta ingest <file>...` (Harlan's own files, such as `seed/`) also looks for a body at `<the file's folder>/<path>`, so `seed/notes/nodes/x.md` sits beside `seed/*.jsonl`. A body it can't find is logged anyway, with a warning.

## Xp

A note earns 15 xp once, the first time its subject is logged under a pillar folder. A new version earns nothing more. A shared node in `notes/nodes/` earns nothing: it's split out of a note that already earned it.

## graph.json

```json
{
  "v": 1,
  "built_at": "2026-10-30T23:50:00Z",
  "nodes": [
    {"id": "note:martin-luther", "kind": "note", "type": "person", "title": "Martin Luther", "pillar": "faith", "pillars": ["faith"], "aliases": ["Luther"], "path": "notes/nodes/martin-luther.md", "added": "2026-10-30", "degree": 5},
    {"id": "note:protestantism", "kind": "note", "stub": true, "title": "protestantism", "degree": 3},
    {"id": "pillar:faith", "kind": "pillar", "title": "Faith", "pillar": "faith", "degree": 0},
    {"id": "skill:typing", "kind": "skill", "title": "Typing", "pillar": "competence", "status": "active", "degree": 1},
    {"id": "quest:typing-drill-1a2b", "kind": "quest", "title": "15 min typing drill", "pillar": "competence", "status": "active", "degree": 1}
  ],
  "edges": [{"from": "note:martin-luther", "to": "note:ninety-five-theses", "type": "authored", "via": "frontmatter"}],
  "errors": [{"id": "note:study", "error": "notes/competence/study.md changed since it was logged: run acta note add notes/competence/study.md"}]
}
```

- `kind` is `note`, `skill`, `quest` or `pillar`. Skills come from `skill.defined`; quests are the active and proposed ones in the snapshot.
- A link to a node nobody has written yet is a `stub`: a note to write.
- `degree` counts edges other than `in_pillar`, so it measures knowledge links.
- `via` is `frontmatter`, `body` or `log`.
- Nodes sort by id and edges by from, to and type, so the same log and notes always give the same file.

`snapshot.json` carries a summary for tiles:

```json
"graph": {"nodes": 74, "edges": 205, "notes": 64, "stubs": 0, "new_7d": 64, "errors": 0}
```

## Later

- **Reviews (FSRS).** A note with `recall:` becomes `card:<slug>`. `review.logged` (`{card, rating 1..4, duration_ms}`, already in the envelope) is folded with go-fsrs into due cards and a memory strength per node. A review earns a small fixed xp with a daily cap. Reviews come from board taps (`acta review`) or from a channel's quiz in chat, through the inbox.
- **The board.** A "Bibliotheca · Library" tile (cards due, new nodes this week) opens a full map on the two islands: pillar hubs on their own island, nodes coloured by pillar, sized by degree and dimmed as memory fades. A tapped node shows its links and the review buttons. acta will precompute positions into `graph.json` so the board only draws. A mockup comes first.
- **SQLite.** Roadmap step 4 can load `graph.json` as tables when queries need it.

## Extraction by a language model (interface only)

A separate engine (OpenRouter, the Anthropic API or a local model) may split study notes into nodes. Nothing here builds it. The request acta would make:

```json
{
  "note": {"id": "note:religion-and-history", "path": "notes/competence/religion-and-history.md", "body": "..."},
  "known": [{"id": "note:martin-luther", "type": "person", "title": "Martin Luther", "aliases": ["Luther"]}],
  "node_types": ["subject", "person", "..."],
  "edge_types": ["member_of", "branch_of", "..."]
}
```

and the answer it expects:

```json
{
  "nodes": [{"slug": "council-of-trent", "type": "event", "title": "Council of Trent", "aliases": [], "date": null, "evidence": "Calvin, Zwingli, Henry VIII; Council of Trent."}],
  "edges": [{"from": "note:council-of-trent", "to": "note:reformation", "type": "responded_to", "evidence": "..."}]
}
```

Each node and edge quotes the text it came from. The answer is a proposal: it lands as proposed notes for Harlan to accept, like an agent's quest, and is never filed straight into `notes/`. Notes a pillar channel files itself go straight in, because they're records of what was studied.
