# lt

A local issue tracker for LLM coding sessions. Tickets live in a SQLite file under your home directory and are managed by a single `lt` binary.

## Why this exists

When a model notices something to do later, it usually drops a note in chat. That note disappears with the session or becomes a TODO comment nobody finds.

`lt` gives the model a project-scoped store for those notes. Tickets persist across sessions, and projects stay isolated.

## Install

### Pre-built binaries

Grab the archive for your platform from the [latest release](https://github.com/jumoel/localticket/releases/latest):

- macOS (Apple Silicon): `lt-<version>-darwin-arm64.tar.gz`
- macOS (Intel): `lt-<version>-darwin-amd64.tar.gz`
- Linux (x86_64): `lt-<version>-linux-amd64.tar.gz`
- Linux (ARM64): `lt-<version>-linux-arm64.tar.gz`
- Windows: `lt-<version>-windows-amd64.zip`

Extract and place `lt` (or `lt.exe`) on your `$PATH`.

### From source

```sh
go install github.com/jumoel/localticket/cmd/lt@latest
```

Binary at `$GOBIN/lt` (default `$HOME/go/bin/lt`).

Storage lives at `~/.localticket/db.sqlite`, created on first use. SQLite is `modernc.org/sqlite` - pure Go, no CGO.

## Quick start

```sh
lt project create scratch
lt new -p scratch "Refactor the rate limiter" --body "Token bucket leaks under contention" --label refactor
lt list -p scratch
lt show -p scratch 1
lt close -p scratch 1
```

Each command prints JSON when piped and a table on a TTY. Override with `--json` or `--pretty`.

Pipe a body in from anywhere:

```sh
echo "Token bucket leaks under contention" | lt new -p scratch "Refactor the rate limiter" --body -
```

## Commands

```
lt project create <name>
lt project list                         (alias: ls)
lt project delete <name> [--force]      (alias: rm)

lt new    -p <project> <title>...  [--body T|--body-file P|--body -] [--label L]... [--link TYPE:ID]...
lt list   -p <project>             [--status open|in-progress|closed|all] [--label L]... [--columns C1,C2,...]
lt show   -p <project> <id>
lt edit   -p <project> <id>        [--title T] [--body T|--body-file P|--body -]
lt status -p <project> <id> open|in-progress|closed
lt close  -p <project> <id>
lt reopen -p <project> <id>
lt label  add|rm -p <project> <id> <label>...
lt link   add    -p <project> <id> <type> <other-id>
lt link   rm     -p <project> <id> <other-id>
lt search -p <project> <query>... [--columns C1,C2,...]

lt summary [--swiftbar]
lt watch   [-p <project>] [--since RFC3339] [--interval 2s]

lt --help        Show usage
lt --version     Show version
```

Project names and labels match `[a-z0-9_-]{1,64}`. Ticket IDs are sequential per project (`#1`, `#2`, …). Multi-word titles can be passed without quotes.

For the body, `lt new` checks `--body-file`, then `--body -` (stdin), then `--body TEXT`, then piped stdin. With no flag and a TTY, it opens `$VISUAL`/`$EDITOR`/`vi` on a temp file. An empty editor buffer aborts.

`lt list` defaults to open and in-progress. Pass `--status closed` or `--status all` to include closed.

`--columns` picks which TTY columns to show. Available: `id`, `title`, `status`, `labels`, `links`, `updated_at`, `created_at`, `closed_at`. Default is `id,status,title,labels,updated_at`. Time columns render as relative ("2h ago"). `--columns` works on `lt search` too. JSON output is unaffected.

`lt search` runs an FTS5 query against title and body:

| Form              | Meaning                                                     |
|-------------------|-------------------------------------------------------------|
| `word1 word2`     | both terms must appear (AND)                                |
| `word1 OR word2`  | either term                                                 |
| `"word1 word2"`   | exact phrase                                                |
| `prefix*`         | prefix match                                                |
| `word1 NOT word2` | exclude word2                                               |
| `title:term`      | match only the title column                                 |
| `body:term`       | match only the body column                                  |
| `NEAR(w1 w2, n)`  | w1 and w2 within n tokens of each other, in the same column |

```sh
lt search -p api 'rate*'                       # rate, rate-limiter, rates, ...
lt search -p api 'title:auth'                  # only when "auth" is in the title
lt search -p api '"token bucket" OR refactor'  # phrase OR keyword
```

`lt watch` polls the DB every `--interval` (default 2s, min 500ms) and emits an event for each change since the last tick. It watches all projects unless you pass `-p`. Output is JSONL when piped, otherwise a `time  project#id  action  details` table.

Link types accepted on input: `blocks`, `blocked-by`, `parent`, `child`, `duplicate-of`, `related`. The schema stores only the canonical four (`blocks`, `parent`, `duplicate-of`, `related`). Inputs of `blocked-by` and `child` are flipped to their inverse, but each ticket sees the relationship from its own side, so one sees `blocks #7` and the other sees `blocked-by #3`.

## Output

Single ticket:

```json
{
  "project": "scratch",
  "id": 3,
  "title": "...",
  "body": "...",
  "status": "open",
  "labels": ["refactor"],
  "links": [{"type": "blocks", "target": 7}],
  "created_at": "2026-05-04T12:00:00Z",
  "updated_at": "2026-05-04T12:00:00Z",
  "closed_at": null
}
```

`list` and `search` wrap an array of these objects as `{"tickets": [...]}`.

`project list` returns `{"projects": [...]}` with `name`, `created_at`, and a `tickets` count map (`open`, `in_progress`, `closed`). `project create` returns `{"name": "...", "created_at": "..."}`. `project delete` returns `{"deleted": "<name>"}`.

`summary`:

```json
{
  "projects": [
    {"name": "scratch", "open": 2, "in_progress": 0, "closed": 1, "last_updated": "2026-05-04T12:00:00Z"}
  ],
  "top": [
    {
      "project": "scratch", "id": 3, "title": "...", "body": "...", "status": "open",
      "labels": ["refactor"],
      "links": [{"type": "blocks", "target": 7}],
      "updated_at": "2026-05-04T12:00:00Z"
    }
  ],
  "totals": {"open": 2, "in_progress": 0, "closed": 1, "projects": 1}
}
```

`top` lists up to 5 of the most recently touched non-closed tickets across all projects.

`watch` line:

```json
{"observed_at": "2026-05-04T12:00:00Z", "project": "scratch", "id": 3, "action": "label_added", "label": "refactor"}
```

`action` values: `created`, `updated`, `closed`, `reopened`, `status_changed`, `title_changed`, `body_changed`, `label_added`, `label_removed`, `link_added`, `link_removed`. Status and title changes carry `from` and `to`. Body changes carry `body`. Label and link events carry `label`, `link_type`, and `link_target`. Fields irrelevant to the action are omitted.

Errors come back on stderr as `{"error": "...", "code": "..."}` when the output mode is JSON.

Exit codes: `0` ok, `1` user error, `2` not found, `3` conflict, `4` internal.

## CLAUDE.md snippet

Drop into `~/.claude/CLAUDE.md` (or a project CLAUDE.md):

```md
## lt - local issue tracker

This machine has `lt`, a project-scoped local issue tracker (`lt --help` for the full list). Use it when:

- You finish a task and notice unrelated work that should happen later. Don't drop it in chat. File it: `lt new -p <project> "<title>" --body "<context>"`.
- The user changes direction mid-task and you have abandoned work worth keeping. File and close it: `lt new -p <project> "<title>" --body "<what you did>"` then `lt close -p <project> <id>`.
- You need earlier follow-up notes. Run `lt -p <project> list` or `lt -p <project> search <query>`.
- A task depends on or blocks an existing ticket. Link it: `lt new ... --link blocks:3` or `lt link add -p <project> <id> blocks <other>`.

Pick a project name that matches the work surface, not the session - for a feature in `acme-api`, use `--project acme-api`, never `--project session-2026-05-04`. Project names match `[a-z0-9_-]`. Every ticket command needs `-p`. `summary` doesn't accept it; `watch` makes it optional.

Output is JSON when stdout isn't a TTY, so `jq` works directly. The `id` field is the per-project number to reference in later commands.

Exit codes: 0 ok, 1 user error, 2 not found, 3 conflict, 4 internal. Branch on these, don't parse error prose.

If a project doesn't exist, any command returns exit code 2 with `code: "not_found"`. Create it explicitly: `lt project create <name>`. Auto-create is off so a typo doesn't spawn a project.
```

## Menu bar (SwiftBar)

`lt` ships a [SwiftBar](https://swiftbar.app) plugin that puts an open-ticket count in the macOS menu bar. The dropdown shows per-project counts (most recently updated first, capped at ten) and the five most recently touched non-closed tickets.

```sh
brew install --cask swiftbar
ln -s "$PWD/swiftbar/localticket.5m.sh" "$HOME/Library/Application Support/SwiftBar/Plugins/"
```

Run from the repo root, or substitute an absolute path. The `.5m.` is the refresh interval - rename to `.10m.` or `.1h.` to change it. The plugin is read-only; clicking a ticket does nothing, since SwiftBar can't return you to the model session that filed it.

The plugin is a shim around `lt summary --swiftbar`. Run that to see what SwiftBar will render.

## Storage and concurrency

The DB uses WAL mode, `busy_timeout=5000`, and foreign keys. Concurrent writers serialize on SQLite's lock, but readers never block. Two sessions writing to the same project won't corrupt anything, though one may wait up to 5 seconds for the other before giving up.

To wipe everything, delete `~/.localticket/db.sqlite` and its `-shm`/`-wal` siblings. To back up, copy all three together when no `lt` process is mid-write.

## Development

```sh
make check    # gofmt, go vet, go test, go build
make test
make build    # produces ./lt
```

Tests are hermetic - they run against a fresh temp `HOME`, so your real `~/.localticket/` is untouched.
