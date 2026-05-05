# Release Notes

## v1.0.0

Highlights since v0.1.0:

- `lt edit --section Effort --content-file new.md` rewrites a single section without touching the rest of the body.
- `lt close 1 3 5 7` (and `reopen`, `status`, `label add|rm`) act on many tickets at once. The exit code is the first failure's; partial successes still happen.
- New link types: `supersedes`, `references`, `derived-from`, with their inverse aliases. `lt link list` prints the whole project graph as flat from/type/to rows.
- Templates: drop a markdown skeleton in `~/.localticket/templates/` and use `lt new --template <name>`.
- `lt list --columns id,title,labels,updated_at` (also on `lt search`). Time columns render relative.
- `lt search --help` and the README now list the FTS5 operators that actually work.

The DB schema migrated from v1 to v2 to widen the link `CHECK` constraint. Existing data is preserved.

See [CHANGELOG.md](CHANGELOG.md) for the full list.

## v0.1.0

`lt` is a local issue tracker for LLM coding sessions. Tickets live in a SQLite file under your home directory and are managed by a single `lt` binary.

First tagged release. Same code that's been on `main`; CI now runs on every PR and the release workflow attaches platform binaries to each tag.

See the [README](README.md) for usage.
