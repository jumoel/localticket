# lt

A local issue tracker for LLM coding sessions. Like GitHub issues, but stored in a single SQLite file under your home directory and addressable from a single binary.

## Why this exists

When a model is working through a task and notices something else that ought to happen later, the natural reflex is to drop the note in chat. Chat is a terrible place for it. The note disappears with the session, gets buried under the next prompt, or ends up as a TODO comment that nobody finds again.

`lt` gives the model a project-scoped, queryable, structured place to put those notes. Tickets survive the session. Multiple sessions on the same project see each other's work. Multiple projects stay isolated.

## Install

```sh
go install github.com/jumoel/localticket/cmd/lt@latest
```

The binary lands at `$GOBIN/lt` (or `$HOME/go/bin/lt` if `GOBIN` is unset). Storage lives at `~/.localticket/db.sqlite` and is created on first use.

The binary itself has one external dependency, `modernc.org/sqlite`, which is pure Go - no CGO, no system SQLite needed.

## Quick start

```sh
lt project create scratch
lt new -p scratch "Refactor the rate limiter" --body "Token bucket leaks under contention" --label refactor
lt list -p scratch
lt show -p scratch 1
lt close -p scratch 1
```

Output is JSON when stdout isn't a TTY, a small table when it is. Force either with `--json` or `--pretty`.

## Commands

```
lt project create <name>
lt project list
lt project delete <name> [--force]

lt new    -p <project> <title>  [--body T|--body-file P|--body -] [--label L]... [--link TYPE:ID]...
lt list   -p <project>          [--status open|in-progress|closed|all] [--label L]...
lt show   -p <project> <id>
lt edit   -p <project> <id>     [--title T] [--body T|--body-file P|--body -]
lt status -p <project> <id> open|in-progress|closed
lt close  -p <project> <id>
lt reopen -p <project> <id>
lt label  add|rm -p <project> <id> <label>...
lt link   add    -p <project> <id> <type> <other-id>
lt link   rm     -p <project> <id> <other-id>
lt search -p <project> <query>...
```

Project names and labels match `[a-z0-9_-]{1,64}`. Ticket IDs are sequential per project (`#1`, `#2`, …).

Link types accepted on input: `blocks`, `blocked-by`, `parent`, `child`, `duplicate-of`, `related`. The schema stores only the canonical four (`blocks`, `parent`, `duplicate-of`, `related`); `blocked-by` and `child` are normalized by swapping the endpoints, so a ticket viewed from either side sees the relationship from its own perspective.

## Output

Pretty mode is for humans. JSON mode is for tools. The single-ticket JSON shape is:

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

`list` and `search` wrap it as `{"tickets": [...]}`. Errors come back as `{"error": "...", "code": "..."}` on stderr when the resolved output mode is JSON.

Exit codes:

```
0  success
1  user error (bad args, validation)
2  not found
3  conflict (e.g. duplicate label, link already exists)
4  internal (DB failure, etc.)
```

## CLAUDE.md snippet

Drop this into your `~/.claude/CLAUDE.md` (or a project-level CLAUDE.md) so the model knows the tool is on the machine and when to use it:

```md
## lt - local issue tracker

This machine has `lt`, a project-scoped local issue tracker (`lt --help` for the full command list). Use it when:

- You finish a task and notice unrelated work that should happen later. Don't drop it in chat. File it: `lt new -p <project> "<title>" --body "<context>"`.
- The user changes direction mid-task and you have abandoned work worth keeping. File it as a closed ticket so it's recoverable: `lt new -p <project> "<title>" --body "<what you did>"` followed by `lt close -p <project> <id>`.
- You need to look up earlier follow-up notes for the same project. Run `lt -p <project> list` or `lt -p <project> search <query>`.
- You're doing something that depends on or blocks an existing ticket. Link it: `lt new ... --link blocks:3` or `lt link add -p <project> <id> blocks <other>`.

Always pass `--project/-p`. Pick a project name that matches the work surface, not the session - for a feature in repo `acme-api`, use `--project acme-api`, never `--project session-2026-05-04`. Project names match `[a-z0-9_-]`.

Output is JSON when stdout isn't a TTY, so piping through `jq` works without extra flags. Pass `--json` to force it. The `id` field in JSON is the per-project ticket number you reference in subsequent commands.

Exit codes: 0 ok, 1 user error, 2 not found, 3 conflict, 4 internal. Branch on these instead of parsing error prose.

If a project doesn't exist yet (any command will return exit code 2 with `code: "not_found"` and a message naming the project), create it explicitly first: `lt project create <name>`. Auto-create is intentionally off so a typo doesn't spawn a project.
```

## Storage and concurrency

The DB file is opened with `journal_mode=WAL`, `busy_timeout=5000`, and `foreign_keys=ON`. Multiple `lt` processes against the same DB serialize on writes through SQLite's lock; reads are non-blocking under WAL. Two LLM sessions hammering the same project will not corrupt anything, though they may briefly wait on each other.

To wipe the store completely, delete `~/.localticket/db.sqlite` (and the `-shm`/`-wal` siblings). To back it up, copy those three files together while no `lt` process is mid-write.

## Development

```sh
make check    # gofmt, go vet, go test, go build
make test
make build    # produces ./lt
```

Tests are hermetic - they run against a fresh temp `HOME` so your real `~/.localticket/` is untouched.
