# Changelog

All notable changes to `lt` are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-05-05

### Added

- Section-level body operations: `lt show --section <heading>` prints one section, `lt edit --section <heading> --content ...` rewrites it without touching the rest of the body.
- Bulk operations: `lt close`, `lt reopen`, `lt status`, and `lt label add|rm` accept multiple ids. Single-id calls keep the old bare-ticket return shape; multi-id calls wrap the result as `{"tickets": [...], "errors": [...]}`.
- New link types: `supersedes`, `references`, `derived-from`, with inverse aliases `superseded-by`, `referenced-by`, `derived-to`. `duplicate-of` gains `duplicates` as an inverse.
- `lt link list -p <project> [<id>] [--type T] [--include-closed]` lists project relationships as flat from/type/to rows.
- `lt new --template <name>` reads a markdown skeleton from `~/.localticket/templates/<project>/<name>.md` (project-scoped) or `~/.localticket/templates/<name>.md` (global). `lt template list` reports what's available.
- `lt list --columns C1,C2,...` and `lt search --columns C1,C2,...` pick which TTY columns to render. Time columns render as relative ("2h ago").
- FTS5 query syntax block in `lt --help` and the README, including verified support for AND, OR, phrase, prefix, NOT, column scoping, and NEAR (same-column).
- Stdin example for `--body -` in the README Quick start.
- Schema migration framework (`runMigrations`, version-keyed slice) so future schema changes can layer onto an old DB without manual SQL.

### Changed

- Database schema bumped from v1 to v2. `ticket_links.type` CHECK constraint widened to include the new link types. Existing rows are preserved by the v1→v2 migration.

## [0.1.0] - 2026-05-05

Initial tagged release. Existing CLI functionality. CI/CD added via GitHub Actions; release artifacts attached for darwin/{amd64,arm64}, linux/{amd64,arm64}, and windows/amd64.
