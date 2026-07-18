# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`cellar` is a keyboard-driven terminal SQL client (a Bubble Tea TUI) for MySQL, PostgreSQL, and SQLite, with native SSH-bastion tunneling. Single static Go binary. See `README.md` for the user-facing feature set and full keymap.

## Commands

Go 1.25+. Everything runs through the Makefile or `go` directly.

- Build: `make build` (→ `./cellar`) or `make run` (build + run)
- Install: `make install` (→ `~/.local/bin`, tagged version) / `make install-dev` (dev-`<sha>` marker). `PREFIX=/somewhere make install` to relocate.
- Test all: `make test` (`go test ./...`) — race: `make race`
- Single package: `go test ./drivers` — single test: `go test ./drivers -run TestIsQueryMutation`
- Vet: `make vet`. Lint (CI gate): `golangci-lint run` — config in `.golangci.yml` (safe variant + gosec/sqlclosecheck/rowserrcheck). No lint make target; run the binary directly.
- CI (`.github/workflows/ci.yml`) runs golangci-lint, `go test ./...`, `go build ./...` on PRs. Releases fire on pushed `vX.Y.Z` tags via goreleaser.

Version is injected at build time via `-ldflags -X main.version=...`; don't hardcode it.

## Architecture

Standard Elm architecture (Bubble Tea). Three layers, each its own concern:

1. **UI / state** — `internal/tui/ui`. `Model` (`model.go`) holds all state; `Update` (`update.go`) is a message switch delegating to `handle*Msg` / `handleKeyPress`; per-screen views render from the model. Screens are a `types.Screen` enum (`internal/tui/types`).
2. **Commands** — `internal/tui/commands`. All async/side-effecting work (connect, load tables, run query, persist) is a `tea.Cmd` factory returning a `types.*Msg`. `Commands.DriverFor` is an injectable provider→driver picker so connect-flow tests substitute a stub driver — use it, don't construct drivers inline in command code.
3. **Drivers** — `drivers`. `Driver` interface (`driver.go`) with three implementations: `MySQL`, `Postgres`, `SQLite`. Long-running methods (`GetRecords`, `ExecuteQuery`, `ExecuteDMLStatement`) take a `context.Context` so `esc` can cancel an in-flight query; metadata getters are ctx-less. Provider strings live in `drivers/constants.go` — note `DriverSqlite = "sqlite3"` (not `"sqlite"`).

Message types are defined in `internal/tui/types/messages.go`. Data flow for anything async: keypress → `handleKeyPress` returns a `tea.Cmd` from `Commands` → command runs off the UI goroutine → emits a `types.*Msg` → `Update` routes it to a `handle*Msg` that mutates the model.

### Tab mirror invariant (easy to break)

Both the browse grid and the SQL editor use a "live mirror of the active slice element" pattern:

- `m.Browse` mirrors `m.Tabs[m.TabActive]`
- `m.EditorArea` / `m.EditorContent` / `m.SavedName` / `m.SavedBaseline` mirror `m.QueryTabs[m.QueryTabActive]`

When switching tabs you must sync the live mirror back into the slice **before** loading the new element, and out again after. Editing the mirror without writing it back loses state on the next tab switch. Grep existing tab-switch handlers before touching this.

### Persistence

Three independent managers, each owning one directory under `~/.config/cellar/`:

- `internal/state` → `state/<conn>.json` — query buffers (tabs), sidebar pref. Restored per connection.
- `internal/saved` → `saved_queries/<conn>.toml` — named saved queries. Naming a tab (`ctrl+s`) *is* saving a query; the name binds the tab title and the saved entry in one step.
- `internal/history` → `history/<conn>.json` — query history, capped by `history.MaxPerConnection` (set from config at startup).

Config resolution deliberately uses XDG (`$XDG_CONFIG_HOME`, else `~/.config` on **every** platform) — NOT `os.UserConfigDir`, which splits config into `~/Library/Application Support` on macOS. Keep this consistent if you add a new persisted artifact.

### Config

`internal/tui/config`. Global `~/.config/cellar/config.toml`; a per-repo `.cellar.toml` (walked up to git root) replaces the connection list wholesale. `[application]` maps to `models.AppConfig`.

`QueryRowLimit` has non-obvious semantics resolved in `commands.queryRowLimit()`: `0`/unset → default 5000, `-1` → unlimited (mapped to internal `0`). `ExecuteQuery` fetches `limit+1` rows as a truncation probe; `capQueryRows` trims the probe and flags whether the result was capped. Table browsing is paged server-side and ignores this cap.

### Read-only safety

`drivers/validation.go` — `IsQueryMutation` blocks non-SELECT statements for read-only connections **before they reach the wire**. It blanks strings/comments/quoted identifiers first, then word-boundary matches blocked keywords anywhere in the statement (catches `WITH ... (DELETE ...)`, `EXPLAIN ANALYZE DELETE`, etc. — a prefix check is not enough). If you add DML paths, route them through this check.

## Gotchas

- **Imports are `charm.land/...` vanity paths, not `github.com/charmbracelet/...`** for bubbletea/bubbles/lipgloss v2. Match existing imports; the github paths are a different (older) module.
- Comments referencing "the tview app" are legacy: `config.toml` is kept byte-compatible with a former tview-based writer (e.g. never persisting SSH port `"22"`). The current code is Bubble Tea only — treat those comments as config-compat notes, not live code.
- The kitty keyboard protocol drives the good keybindings (`ctrl+enter`, `ctrl+]`/`ctrl+[`); everything has a legacy fallback (`ctrl+r`, `ctrl+pgup/pgdn`). Don't add a chord without a fallback.
