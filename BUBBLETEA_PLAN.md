# Bubble Tea Migration — Phase 1: Connection Screen

Pivot from tview to Bubble Tea, starting with the connection-management screen (the worst part of the current UI). This document scopes **Phase 1 only**: a standalone Bubble Tea binary that rebuilds the connection picker + add/edit form + SSH config, reusing this repo's non-UI layers. Ship it, live with it, then decide whether to migrate the rest (tree, results grid, SQL editor).

Reference implementation: `~/dev/redis-tui` (`github.com/bearded-giant/redis-tui`) — same author, same stack, the UX bar we're matching.

## Open Questions for User

1. [non-blocking] **Binary name.** New binary alongside `lazysql` during Phase 1 (can't co-drive one terminal — see Constraints). Proposed: `lazytea`. Alternatives: `lazysql-next`, `ltui`. Default: `lazytea`.
2. [non-blocking] **Config sharing.** Recommended: the new app reads/writes the *same* `~/.config/lazysql/config.toml` (same `[[database]]` format incl. the new `ssh_*` fields), so connections show up in both binaries during the transition. Alternative: separate config file. Default: share `config.toml`.
3. [non-blocking] **Repo location.** New app as a subdir of this repo (`cmd/lazytea/` or `tea/`) sharing the module, or a fresh repo. Default: subdir of this repo, same Go module — lets it import `drivers`/`helpers`/`models` directly with no replace directives.
4. [non-blocking] **`ConnectionPages` sever.** Recommended: skip for Phase 1 (accept the harmless dead tview dependency that `models`/`drivers` drag in); sever later when migrating the home screen. Default: defer.

## Orientation (read before starting — current state)

**Repos:**
- This repo (lazysql fork): `~/dev/golang/lazysql`, module `github.com/jorgerojas26/lazysql`, Go 1.23. Currently a tview/tcell app. Binary symlinked at `~/.local/bin/lazysql`. `go` is installed via Homebrew (`$(brew --prefix)/bin`).
- Template (redis-tui): `~/dev/redis-tui`, module `github.com/bearded-giant/redis-tui`, Go 1.26. The Bubble Tea app this plan copies.

**What already exists (Phase 1 builds on it):**
- Native SSH tunnel landed on branch **`feat/native-ssh-tunnel`** (pushed to origin, no PR). It added `helpers/ssh_tunnel.go`, the `models.Connection` SSH fields, and tunnel-aware connect in the *tview* app. The new app reuses `helpers/ssh_tunnel.go` + `models.Connection` directly — see **Reusable APIs** at the bottom (verified signatures + line refs).
- `Makefile` (`build`/`install`/`test`/`race`/`vet`). `Makefile` install target symlinks to `~/.local/bin`.

**Build / run:**
```bash
make build && ./lazysql                              # existing tview app
go build -o lazytea ./cmd/lazytea && ./lazytea       # the new app (once scaffolded)
```

**Read first, in order:**
1. This doc.
2. redis-tui `internal/ui/{model,update,screens_connection,view_connections}.go` — the exact template to copy.
3. redis-tui `internal/cmd/commands_connection.go`, `internal/db/config.go`, `main.go` — async + persistence + entrypoint patterns.
4. This repo `helpers/ssh_tunnel.go`, `drivers/driver.go`, `models/models.go`, `app/config.go` — what you reuse / lift (see Reusable APIs).

## Goal & Success Criteria

Rebuild *only* the connection screen on Bubble Tea, good enough to use daily:

- List saved connections (cards, keyboard nav, the redis-tui look)
- Add / edit / delete / duplicate a connection
- Native SSH tunnel fields (reuse `helpers.OpenTunnelForURL` — already built)
- Test connection (F2-equivalent) and connect, both tunnel-aware
- Reads/writes the existing `config.toml` so it's a drop-in for managing connections

Done when: connecting to a real DB (incl. through an SSH bastion) works end-to-end and the connection-management UX is visibly better than the tview version. The new app can hand off to the existing tview app for the actual data browsing in Phase 1 (see "Phase 1 boundary"), or just prove the screen and stop.

## Constraint That Shapes Everything

**Bubble Tea and tview cannot share one terminal.** Both own stdin and the render loop. There is no "half-migrated" running app. So Phase 1 is a *separate binary*, not an in-place swap. The two coexist on disk; you run whichever you want. Full migration (later phases) ends with one Bubble Tea binary replacing the tview one.

## Reuse Boundary (proven with `go list -deps`)

| Package | Safe to import? | Notes |
|---|---|---|
| `helpers/` | **Yes, clean** | `ssh_tunnel.go`, `ParseConnectionString`, `GetFreePort` — pure, no tview |
| `commands/` | **Yes, clean** | no imports at all |
| `drivers/` | Yes, builds | pulls tview transitively (via `models`); dead-linked, never initialized. Use the `Driver` interface + `Connect`/`TestConnection` as-is |
| `models/` | Yes, builds | reuse `models.Connection` directly. tview leak is `ConnectionPages` only (5 refs, all in `components/`) — sever later to make clean |
| `app/` | **No** | `app/app.go init()` runs `tview.NewApplication()` on import; `config.go` mutates the global `App`. Never import |
| `keymap/`, `components/` | **No** | tcell / tview |

What this means concretely:

- **Reuse directly:** `helpers.OpenTunnelForURL(ctx, *SSHConfig, dbURL, defaultPort) (string, *Tunnel, error)`, `helpers.Tunnel.Close()`, `drivers.{MySQL,Postgres,SQLite,MSSQL}` + the `Driver` interface, `models.Connection` (incl. the `ssh_*` fields).
- **Reimplement (~30 lines, tview-free):** a `LoadConfig` that reads/merges `config.toml` and returns a *local* `*Config` instead of mutating the `app.App` global. Lift these pure functions from `app/config.go` verbatim: `DefaultConfigFile`, `FindLocalConfig`, `mergeMaps`/`mergeValues`, `expandEnvVars`, `parseConfigURL`, and the body of `SaveConnections` (keep its `0o600` + secret-blanking). Skip `ApplyKeymapConfig` (Bubble Tea doesn't use tcell keymaps).

Net: the new app shares lazysql's exact on-disk connection format with zero changes to the existing app.

## Architecture (mirror redis-tui's Elm model)

Single `Model`, one `Update`, one `View`, screens multiplexed by a `Screen` enum. No nested `tea.Model` sub-components — handlers are value-receiver methods (`func (m Model) handleX(msg) (Model, tea.Cmd)`). redis-tui uses **no** `bubbles/list` or `bubbles/table` — the list is hand-rolled lipgloss cards with a manual cursor + scroll window; forms are `[]textinput.Model` + a focus index. Copy that.

Screens for Phase 1:

```
ScreenConnections     // the list
ScreenAddConnection   // add form (shared render with edit)
ScreenEditConnection  // edit form
ScreenSSHTunnel       // SSH sub-screen (staged into PendingSSH)
ScreenTestConnection  // test result
ScreenConfirmDelete   // generic confirm dialog
```

Patterns to copy verbatim from redis-tui (`internal/ui/`):

- **Routing:** `Update` type-switches on `Msg`; `tea.KeyMsg` → `handleKeyPress` → `switch m.Screen`; typed msgs → `handle<Msg>`. `View` parallels with `getScreenView()` switching `m.Screen`.
- **Form:** `[]textinput.Model` + `FocusIdx`, with a **focus-index→input-index mapping** when toggles/hidden fields interleave (redis-tui's `connInputIndex`/`sshInputIndex`). `Tab`/`Shift+Tab` cycle `% fieldCount`; forward the keystroke only to the focused input.
- **Add vs Edit:** disambiguate with `EditingConnection *Connection` (nil = add). Duplicate via `DuplicatingFrom`. Helper trio per form: `reset*` / `populate*` / `convert*Inputs`.
- **SSH sub-screen:** edits local textinputs; on Enter materializes into `PendingSSH *SSHConfig`; Esc discards; infers return screen from `EditingConnection`. Last focus index is the enable toggle.
- **Async ops:** `tea.Cmd` closures on a `*Commands` DI struct, each returning a typed `Msg{... Err error}`. Set `Loading`+`StatusMsg` before, clear in the result handler. Errors ride `Msg.Err` → status bar / error box.
- **Confirm dialog:** generic `ConfirmType string` + `ConfirmData any` → one `ScreenConfirmDelete`.
- **Styling:** package-level lipgloss style vars; `lipgloss.Place(w,h,Center,Center,...)` to center modals; `JoinHorizontal` for rows; raw `switch msg.String()` for keys with hand-written footer help.

### Connection → connect flow (with SSH)

```
Enter on a connection
  -> build helpers.SSHConfig from conn.SSH* (if conn.UseSSH)
  -> url, tun, err := helpers.OpenTunnelForURL(ctx, cfg, conn.URL, defaultPort(conn.Provider))
  -> driver := pick by conn.Provider
  -> driver.Connect(url)        // tunnel-aware, same as the tview app now does
  -> on success: keep tun for teardown; hand off / browse
  -> on failure: tun.Close()
```

Test (don't connect): same but `driver.TestConnection(url)` then `tun.Close()` immediately.

## Directory Layout (subdir, shared module)

```
cmd/lazytea/main.go        entrypoint: flags, setup(), tea.NewProgram(m, WithAltScreen(), WithMouseCellMotion())
internal/tui/              NOTE: dir is "tui", not "tea" — bubbletea is imported as `tea`, avoid the package-name clash
  types/        Screen enum, Msg types  (reuse models.Connection — do NOT redefine the connection type)
  config/       tview-free LoadConfig/SaveConnections returning a local *Config (lifts app/config.go pure parts)
  commands/     *Commands DI struct; tea.Cmd factories: LoadConnections, SaveConnection, DeleteConnection, Connect, TestConnection, TestSSH
  ui/           model.go (Model + helpers), update.go (routing), screens_connection.go, view_connections.go, styles.go
```

Watch the package names: nothing should be `package tea` (collides with `github.com/charmbracelet/bubbletea`). Use `package tui`, `package config`, `package commands`, `package ui`.

Keep the dependency direction one-way: `ui -> commands -> config`, all reusing `drivers`/`helpers`/`models` from the parent module. A `service`-style interface seam around config + driver access (redis-tui's `service.ConfigService`/`RedisService`) makes the UI testable without a live DB — worth adding even at Phase 1 scale.

## Dependencies to Add

```bash
go get github.com/charmbracelet/bubbletea@v1.3.10
go get github.com/charmbracelet/bubbles@v1.0.0      # textinput only
go get github.com/charmbracelet/lipgloss@v1.1.0
```

`pelletier/go-toml/v2` is already present (keep TOML for config compat). `golang.org/x/crypto` already present (SSH). Skip `vimtea` — no editor in the connection screen.

## Implementation Order

1. **Scaffold (half day).** `cmd/lazytea/main.go` + the `internal/tea/` dirs. Empty `Model` with a `ScreenConnections` that renders "hello". Wire `tea.NewProgram` with alt-screen + mouse. Confirm it builds and runs alongside `lazysql`.
2. **Config layer (half day).** tview-free `LoadConfig` returning `*Config`; lift the pure helpers from `app/config.go`. Round-trip test against a real `config.toml`. Confirm it reads existing connections incl. `ssh_*` fields.
3. **Connection list (1 day).** `Model.Connections` + `SelectedIdx`, hand-rolled cards + scroll window + nav (`j/k`, arrows), footer help, empty state. `a` add / `e` edit / `d` delete / `D` duplicate / `Enter` connect / `r` reload key handling (no-ops first).
4. **Add/Edit form (1 day).** `[]textinput.Model` (Name, URL, Read-Only toggle), focus cycling, save → `commands.SaveConnection` → `config.SaveConnections`. Reuse `helpers.ParseConnectionString` for provider/validation.
5. **SSH sub-screen (half day).** 6 fields (host/port/user/key-file/passphrase/password) + enable toggle, staged into `PendingSSH`, `ctrl+t` test via `commands.TestSSH` (dial bastion only). Fold into `models.Connection` SSH fields on save.
6. **Connect + Test (1 day).** Wire the connect flow above through `helpers.OpenTunnelForURL` + `drivers`. Tunnel teardown on failure/exit. Status bar + error box.
7. **Confirm dialog + polish (half day).** Generic delete confirm; lipgloss styling pass to match redis-tui.

Phase 1 boundary: stop after step 7. On successful connect, either (a) print the resolved (tunneled) URL and exec the existing `lazysql --url ...` for browsing, or (b) just declare the connection valid and exit. Decide once the screen feels right.

## Testing

Copy redis-tui's approach — **no `teatest`, no driving a live `tea.Program`.** Build the `Model`, call handler methods directly, assert on the returned `(Model, tea.Cmd)` and mutated fields. The `service`/`*Commands` interface seam lets command tests run with a fake config + a stub driver, no live DB. Reuse the `helpers/ssh_tunnel_test.go` in-process sshd harness style for any tunnel-path test.

Minimum tests: config round-trip (incl. SSH fields + secret blanking), form focus cycling + save→Connection conversion, SSH staging (PendingSSH), connect-flow URL/tunnel wiring (mock driver).

## Build

```bash
go build -o lazytea ./cmd/lazytea
./lazytea
```

Add a `lazytea` target to the `Makefile` (mirror the existing `build`/`install`).

## Risks

- **Hand-rolled list/form is more code than expected.** redis-tui proves it's tractable, but budget for the focus-index mapping fiddliness (toggles interleaved with inputs).
- **Config divergence.** If the new app writes `config.toml` slightly differently (key casing, omitempty), it could churn the file when both binaries edit it. Mitigate: lift the `SaveConnections` body verbatim, round-trip test.
- **Scope creep into a full rewrite.** Phase 1 is the connection screen only. Resist pulling in the tree/grid until you've lived with it.

## Reusable APIs (verified against the code on branch `feat/native-ssh-tunnel`)

Line numbers are real at time of writing — confirm with grep, don't trust blindly.

**SSH tunnel — `helpers/ssh_tunnel.go` (tview-clean; import directly):**
```go
type SSHConfig struct {              // :28
    Host           string
    Port           int               // 0 => 22
    User           string
    Password       string
    PrivateKeyPath string
    Passphrase     string
    LocalPort      int               // 0 => kernel-assigned free port
}
// call this: opens tunnel to the DB host:port in dbURL, returns URL rewritten to 127.0.0.1:<local>
func OpenTunnelForURL(ctx context.Context, cfg *SSHConfig, dbURL, defaultPort string) (string, *Tunnel, error)  // :371
func OpenSSHTunnel(ctx context.Context, cfg *SSHConfig, remoteAddr string) (*Tunnel, error)                     // :321
func (t *Tunnel) Close() error    // :234  idempotent — caller MUST Close (on disconnect, connect-fail, exit)
func (t *Tunnel) LocalAddr() string  // :219
func (t *Tunnel) LocalPort() int     // :223
```
Auth precedence: private key (+passphrase) > password > ssh-agent. Strict `~/.ssh/known_hosts`; a missing file is a hard error carrying a `ssh-keyscan -H <host>` hint — surface it in the UI.

**Connection type — `models/models.go` (reuse as the shared type; SSH fields `:38-44`):**
```go
UseSSH        bool   `toml:"use_ssh,omitempty"`
SSHHost       string `toml:"ssh_host,omitempty"`
SSHPort       string `toml:"ssh_port,omitempty"`   // STRING here; helpers.SSHConfig.Port is INT
SSHUser       string `toml:"ssh_user,omitempty"`
SSHKeyFile    string `toml:"ssh_key_file,omitempty"`
SSHPassphrase string `toml:"ssh_passphrase,omitempty"`   // blanked on save (never persisted)
SSHPassword   string `toml:"ssh_password,omitempty"`      // blanked on save (never persisted)
```
Boundary: `strconv.Atoi(SSHPort)` at the edge; empty => 0 => defaults to 22 in the tunnel.

**Connection→tunnel mapping (copy from `components/connection_selection.go`):**
- `sshConfigFromConnection(conn) (*helpers.SSHConfig, error)` — `:332`, string→int port.
- `defaultDBPort(provider) string` — `:351`: postgres `5432`, sqlserver `1433`, else `3306`.
- SQLite has no host — never tunnel (`provider == drivers.DriverSqlite`).

**Drivers — `drivers/` (`Driver` interface in `driver.go`; import directly):**
- `Connect(urlstr string) error`, `TestConnection(urlstr string) error` — pure, URL-only, no tview at runtime.
- Pick by `conn.Provider`: `DriverMySQL="mysql"`, `DriverPostgres="postgres"`, `DriverSqlite="sqlite3"`, `DriverMSSQL="sqlserver"` (`drivers/constants.go:9-12`). Concrete: `&drivers.MySQL{}` / `&drivers.Postgres{}` / `&drivers.SQLite{}` / `&drivers.MSSQL{}`.

**Config — lift these pure funcs from `app/config.go` (do NOT import package `app`):**
- `type Config struct` `:16` (pure: `*models.AppConfig`, `[]models.Connection`, `models.KeymapConfig`; toml tags `application`/`database`/`keymap`)
- `DefaultConfigFile()` `:49` · `FindLocalConfig()` `:63` · `mergeMaps`/`mergeValues` `:95`/`:113` · `expandEnvVars` `:190` · `parseConfigURL` `:241`
- `(*Config).SaveConnections` body `:200` — keeps `0o600` + blanks SSHPassphrase/SSHPassword; reuse verbatim against a local `*Config`.
- Reimplement `LoadConfig` (`:125`) to read+merge+unmarshal into a **returned** local `*Config` instead of the `app.App` global; skip `ApplyKeymapConfig`.

## References

- redis-tui (template): `~/dev/redis-tui` — `internal/ui/{model,update,view_connections,screens_connection}.go`, `internal/cmd/commands_connection.go`, `internal/db/config.go`, `main.go`
- This repo's reusable layers: `helpers/ssh_tunnel.go`, `drivers/driver.go`, `models/models.go`, `app/config.go` (lift pure parts)
- Supersedes the tview-feature items in `FORK_PLAN.md` (those become moot if the migration proceeds)
- Bubble Tea: https://github.com/charmbracelet/bubbletea · Bubbles: https://github.com/charmbracelet/bubbles · Lip Gloss: https://github.com/charmbracelet/lipgloss
