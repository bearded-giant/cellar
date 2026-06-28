# Bubble Tea Migration — Phase 2: Data Browsing (Full Parity)

Phase 1 rebuilt the connection screen on Bubble Tea (`lazytea`) and now hands off to the tview `lazysql` binary for the actual data browsing (connect → suspend → `lazysql <tunneled-url>` → quit → back). Phase 2 replaces that hand-off: rebuild the browsing experience — schema tree, results grid, SQL editor, row editing — on Bubble Tea, reaching feature parity with the tview app. When Phase 2 lands, `lazytea` is a standalone DB client and `lazysql` (tview) can be retired.

This doc scopes the work. It is a map + a sequence + an honest risk register, not a line-by-line plan.

## Status

**2026-06-27 — Phase 2.1 (SQL editor) landed.** From browse, `e` opens a vimtea editor (full vim modes + free SQL syntax highlighting via chroma — `query.sql` filename). `ctrl+r` executes: SELECT-ish queries (`select/with/explain/show/describe/desc` prefix) → `ExecuteQuery` → results fill the 2.0 grid (single page, paging disabled); other queries → `ExecuteDMLStatement`, info to the status bar; read-only connections reject DML via `drivers.ValidateQueryForReadOnly`. `ctrl+q` returns to browse (query text persists across opens). All green (build + vet + test).

- **New dep:** `github.com/kujtimiihoxha/vimtea v0.0.2` (+ chroma transitive). Toolchain is go 1.26.4; vimtea needs only 1.23.5 — no bump. `Editor.Update`/`SetSize` return `tea.Model` → always type-assert back to `vimtea.Editor`. vimtea's cursor-blink tick is forwarded via the Update catch-all.
- **Built:** `types.{ScreenEditor,QueryExecutedMsg}`, `commands.RunQuery` (SELECT/DML decision + RO gate), `ui/editor.go` (vimtea wiring, execute, result routing), `Model.{Editor,EditorContent}`.
- **DEFERRED — autocomplete:** `sql_completer` reuse is blocked — vimtea exposes no public cursor getter (`Cursor` is unexported), so the completion prefix can't be extracted. Needs a vimtea fork/PR exposing the cursor, or a hand-rolled editor. Completer/lexer left un-ported (YAGNI) until then.
- **DEFERRED — query history:** `internal/history` imports `app` (whose `init()` runs `tview.NewApplication()`) — can't be called from the tea binary without dragging tview in. Sever it first (extract `app.GetConfigPath` → app-free + make `MaxQueryHistoryPerConnection` a package var/param). `commands.RunQuery` has the one-line seam ready.

**2026-06-27 — Phase 2.0 (Browse MVP) landed.** Read-only in-app browse works: connect → schema tree (lazy-loaded, db→schema→table for Postgres, db→table for flat drivers) → pick a table → paginated, virtualized results grid with vertical + horizontal scroll. All green: `go build ./...`, `go vet`, `go test ./...` (incl. a real-sqlite end-to-end test).

- **Decision (Q1):** big-bang full parity is the goal — hand-off stays the **default** (`Enter`) safety net until parity. In-app browse is opt-in behind **`b`** on the connections list. Flip `Enter`→browse and retire hand-off when 2.1–2.3 land.
- **Built:** `types/browse.go` (Focus enum + 3 Msgs), `ConnectedMsg.{Driver,Browse}`, `commands/commands_browse.go` (LoadDatabases/LoadTables/LoadRecords), `ui/{browse,tree,grid}.go`, `Model.{ActiveDriver,Focus,Browse}`. `Connect(conn, browse bool)` now returns the live driver instead of discarding it; tunnel+driver stay alive while browsing, closed on `q`/`esc` disconnect.
- **Known limits / next:** Driver has no `Close()` — disconnect closes the tunnel (severs the pool transport); driver pool object lingers until GC (add `Close()` if reconnect-leak shows). Rune-counted width (not display-width) — fine for ASCII, swap `ansi.StringWidth` for CJK. No edit/SQL/meta-views yet → those still need the `Enter` hand-off. Next: 2.1 SQL editor (vimtea), then 2.2 DML.

## Open Questions for User

> Q1 resolved 2026-06-27 — see Status above (big-bang, hand-off stays default; browse behind `b`).


1. [BLOCKING] **MVP vs big-bang.** Recommended: ship an incremental MVP (tree → read-only records grid → pagination) that replaces the hand-off for *browsing*, keep the `lazysql` hand-off as the fallback for editing/SQL until those land. Alternative: build the whole thing before switching. Default: incremental (see Implementation Order).
2. [non-blocking] **SQL editor: `vimtea` or `bubbles/textarea`?** redis-tui uses `github.com/kujtimiihoxha/vimtea` (full vim mode) and it's proven in that codebase. The lazysql SQL editor is already vim-flavored, and its lexer + autocompleter are 100% reusable. Recommended: adopt `vimtea` (saves ~200 lines of vim state machine), wire the existing lexer for highlighting + completer for autocomplete. Alternative: hand-roll on `bubbles/textarea`. Default: `vimtea`.
3. [non-blocking] **Mouse support.** tview supports click-to-select cells / FK click-through. Bubble Tea has mouse via `WithMouseCellMotion` (already enabled), but hit-testing a hand-rolled grid is extra work. Default: keyboard-first parity; add mouse cell-selection as polish, defer FK-click to keyboard FK-jump.
4. [non-blocking] **Multi-tab now or later.** tview supports multiple open tables/queries as tabs. Default: single active table/result in MVP; add the tab manager in Phase 2.3.
5. [non-blocking] **Keymap reuse.** `app/keymap.go` + `keymap/` are tcell-free (they only import `commands`/`models`) — reusable if we adapt the input side (`tea.KeyMsg` → `keymap.Key`). Default: reuse the keymap config so user bindings carry over; wrap the resolver.

## Start Here (fresh session)

You are picking up a migration mid-stream. Phase 1 is done and committed; you are building Phase 2. Read this whole doc, then read the files in **Read order** below before writing any code.

**Repo:** `~/dev/golang/lazysql`, module `github.com/jorgerojas26/lazysql`, **go 1.24.2**, branch `feat/native-ssh-tunnel`. Toolchain: Homebrew go (`$(brew --prefix)/bin`). Deps already present: `bubbletea v1.3.10`, `bubbles v1.0.0`, `lipgloss v1.1.0`, `go-sqlmock v1.5.2`. The vim editor lib `vimtea v0.0.2` is in **redis-tui** (add it here if you adopt it — Open Question 2).

**Build / test / run (the gate you must keep green):**
```bash
go build ./...                       # MUST stay green — includes the tview lazysql app, not just lazytea
go vet ./cmd/lazytea/... ./internal/tui/...
go test ./internal/tui/...           # call handlers directly; no teatest, no live tea.Program
make install-dev                     # build lazytea (dev-<sha> version) + install to ~/.local/bin
make version                         # show tagged vs dev version strings
```
Do **not** launch the TUI from an automated/agent context (`./lazytea` with no args grabs the tty and hangs). Verify by building the binary + unit tests; do a real run only interactively.

**Read order (before coding):**
1. This doc + `BUBBLETEA_PLAN.md` (Phase 1 plan + the proven reuse boundary).
2. The Phase 1 tea code (the patterns to extend — see tree below). Especially `internal/tui/ui/{model,update,screens_connection,view_connections}.go` and `internal/tui/commands/commands_connection.go`.
3. `drivers/driver.go` — the data contract (verified list below; grep to re-confirm signatures, they drift).
4. The parity targets in `components/` for the subsystem you start with.
5. `~/dev/redis-tui/internal/ui/` for grid/list/editor idioms — closest real examples: `view_keys_list.go`, `view_keys_detail.go`, `screens_keys.go`, `model.go`, `update.go`, and `vimtea` usage in its editor screens.

## Orientation (current state)

**`lazytea` (Phase 1, built — extend this):**
```
cmd/lazytea/main.go                       entrypoint: flags, version, config load, ui.New, tea.NewProgram(alt-screen+mouse)
internal/tui/types/screens.go             Screen enum (iota; ScreenConnections=0)
internal/tui/types/messages.go            typed Msgs (ConnectionsLoadedMsg, ConnectedMsg{...Tunnel}, TestResultMsg, LazysqlExitedMsg, ...)
internal/tui/config/config.go             tview-free LoadConfig/SaveConnections (lifted from app/config.go; must match it byte-for-byte)
internal/tui/commands/commands.go         *Commands DI struct (holds *config.Config + injectable DriverFor func)
internal/tui/commands/commands_connection.go  tea.Cmd factories: LoadConnections/SaveConnection/DeleteConnection/Connect/TestConnection/TestSSH; sshConfigFromConnection, defaultDBPort, openDial
internal/tui/ui/model.go                  Model struct + form helpers (reset/populate/convert, focus-index mapping)
internal/tui/ui/update.go                 Update routing: KeyMsg→handleKeyPress→switch Screen; typed Msg→handle<Msg>
internal/tui/ui/screens_connection.go     connection-list + add/edit/SSH form + confirm handlers
internal/tui/ui/view_connections.go       list cards, form views, status bar, footer help
internal/tui/ui/styles.go                 lipgloss style vars
internal/tui/ui/handoff.go                tea.ExecProcess hand-off to lazysql (Phase 2 replaces this with in-app browse)
```
- `lazysql` (tview): the full browser — `components/` (~35 files, all tcell/tview-coupled). This is the parity target.
- Template: `~/dev/redis-tui` — same Bubble Tea idioms (hand-rolled lipgloss, `[]textinput`, `Screen` enum, `*Commands` DI).

## Goal & Success Criteria

Browse and edit a database entirely within `lazytea`, no hand-off:

- Schema tree (databases → schemas → tables/views/functions/procedures), lazy-loaded, searchable.
- Results grid: paginated records, vertical + horizontal scroll, cell selection, NULL/blob/JSON display.
- Row editing (DML): edit cells, insert/delete rows, pending-change tracking, commit/rollback through the driver.
- SQL editor: multiline, syntax highlighting, autocomplete, execute → results.
- Sidebar inspector: column metadata, constraints, foreign keys, indexes; FK jump-to-referenced-row.
- Pagination, sorting, filtering, CSV export, query history, clipboard.

Done when: the frost-dev workflow (connect through the SSM bastion, browse `frost-hasura-dev`, run a query, edit a row) works end-to-end in `lazytea` with no `lazysql` hand-off, and the UX matches or beats the tview version.

## Reuse Boundary (the leverage)

The data layer is **already import-clean and needs zero rewrite** — Phase 1 proved it. Phase 2 only rebuilds UI.

| Layer | Status | Notes |
|---|---|---|
| `drivers/` (Driver interface + 4 impls) | **Reuse 100%** | The 29-method contract below. Pure SQL, no tcell. |
| `models/` | **Reuse 100%** | `DBDMLChange`, `CellValue`, `CellValueType`, `DatabaseTableColumn`, `Query`, etc. (`ConnectionPages` is the only tview leak — never imported by the tea app). |
| `helpers/` | **Reuse 100%** | `csv.go` (export), `ssh_tunnel.go`, `ParseConnectionString`, clipboard via `lib/`, query history (`internal/history`). |
| `app/keymap.go`, `keymap/` | **Reuse w/ adapter** | tcell-free (import only `commands`/`models`). Adapt input: `tea.KeyMsg` → `keymap.Key` → `commands.Command`. |
| `components/sql_lexer.go` | **Reuse ~100%** | `tokenizeSQL` + helpers; only swap `tcell.Style` → `lipgloss.Style` (~20 lines). |
| `components/sql_completer.go` | **Reuse 100%** | `Autocompleter`, fuzzy scoring — no UI coupling. |
| SQL editor cursor/undo/text-buffer ops | **Reuse logic** | `moveLeft/Right/Up/Down`, `wordForward/Backward`, `pushUndo/undo/redo`, `insertRune/backspace/splitLine` — data-driven, port the state. |
| `app/app.go` | **Ignore** | tview init; lazytea has its own entrypoint. |
| `components/*` (tree, results_table, sidebar, sql_editor, home, pagination, menus, json_viewer) | **Rebuild** | tcell/tview. The work. |

### The Driver contract (what the UI must call — already built)

```
Connect(url) error · GetProvider() string · UseSchemas() bool · SupportsProgramming() bool
GetDatabases() ([]string, error)
GetTables(db) (map[string][]string, error) · GetViews/GetFunctions/GetProcedures(db) (map[string][]string, error)
GetTableColumns(db, table) ([][]string, error)
GetConstraints / GetForeignKeys / GetIndexes(db, table) ([][]string, error)
GetPrimaryKeyColumnNames(db, table) ([]string, error)
GetRecords(db, table, where, sort, offset, limit) ([][]string, int, string, error)   // rows, total, query, err
ExecuteQuery(query) ([][]string, int, error)
ExecuteDMLStatement(query) (string, error)
ExecutePendingChanges(changes []models.DBDMLChange) error
UpdateRecord(db, table, col, val, pkCol, pkVal) error · DeleteRecord(db, table, pkCol, pkVal) error
Get{View,Function,Procedure}Definition(db, name) (string, error)
FormatArg / FormatArgForQueryString / FormatReference / FormatPlaceholder / DMLChangeToQueryString   // SQL building
TestConnection(url) error · SetProvider(provider)   // 29 methods total — grep drivers/driver.go to re-confirm exact signatures
```

Pagination is offset/limit (`GetRecords` returns rows + total count). DML is batched via `ExecutePendingChanges([]DBDMLChange)`. This is the spine — the UI is just a driver of these calls.

## Architecture (extend the Phase 1 Elm model)

Same single-`Model` / one-`Update` / `Screen`-enum pattern as Phase 1, scaled up. The new hard problem is **multi-pane focus** — tview gave us `Pages`/`Flex`/`SetFocus` for free; Bubble Tea has none, so we hand-roll a focus manager.

- **Focus model:** a `Focus` enum (`FocusTree`, `FocusGrid`, `FocusSidebar`, `FocusEditor`, `FocusFilter`) + a `Screen` enum for full-screen modes (`ScreenBrowse`, `ScreenEditor`, `ScreenCellEdit`, `ScreenConfirm`, `ScreenJSON`). `Update` dispatches `tea.KeyMsg` to the focused pane's handler; global keys (tab-between-panes, quit) handled first.
- **Layout:** lipgloss `JoinHorizontal(tree | (grid over editor)) ` with a sidebar overlay; widths from `m.Width`. No `tview.Flex` — compute splits manually.
- **Async data:** every driver call is a `tea.Cmd` closure on `*commands.Commands` returning a typed `Msg{... Err error}` (extend the Phase 1 commands package). Loading flags + cancellation via `context` captured in the closure.
- **Modals:** lipgloss `Place`-centered overlays gated by `Screen`; reuse the Phase 1 confirm-dialog pattern.

New `internal/tui/` packages/files: extend `commands/` with DB-query factories; add `ui/{tree,grid,sidebar,editor,browse,modals}.go`; extend `types/` with the new `Msg`s + `Screen`/`Focus` enums.

## Phase 1 conventions & gotchas (follow these — they are load-bearing)

The Phase 1 code established conventions and fixed real bugs. New Phase 2 code MUST match, or it will be inconsistent and reintroduce solved problems.

- **Package naming:** nothing is `package tea` (collides with `bubbletea` imported as `tea`). Dir is `internal/tui` (not `internal/tea`). Packages: `tui`/`types`/`config`/`commands`/`ui`. Dependency direction one-way: `ui → commands → config`, all reusing `drivers`/`helpers`/`models`.
- **Never import `app`, `keymap` package as tcell, or `components`.** `app/app.go init()` runs `tview.NewApplication()`. Reuse only `drivers`/`helpers`/`models` and the *pure* parts of `app/config.go` + `app/keymap.go` (lift/adapt, don't import the tview-coupled bits).
- **Elm handlers are value-receiver:** `func (m Model) handleX(msg) (Model, tea.Cmd)`. Mutate the copy, return it. Don't mutate through a pointer and return a stale `m`.
- **No `App.Draw()` / no UI mutation from goroutines.** tview's async code calls `App.Draw()` from goroutines — that is illegal in Bubble Tea. ALL async work is a `tea.Cmd` closure that returns a typed `Msg`; state changes happen only in `Update`. Driver calls (`GetRecords`, `ExecuteQuery`, ...) go in `*commands.Commands` `tea.Cmd` factories returning `Msg{... Err error}`. Set `Loading`+`StatusMsg` before, clear in the result handler.
- **Test seam = the `*Commands` DI struct.** It carries an injectable `DriverFor func(provider string) drivers.Driver` so tests inject a stub driver (see `commands_connection_test.go`). Do NOT add a separate service package / one-impl interfaces.
- **Config churn is real.** `internal/tui/config` and `app/config.go` write the SAME `~/.config/lazysql/config.toml` (both binaries share it). `SaveConnections` must stay byte-identical to the tview version (key casing, omitempty, `0o600`, blank SSH passphrase/password on disk, keep them in memory). If you touch config, round-trip test it.
- **SSH port is a string↔int boundary.** `models.Connection.SSHPort` is `string`; `helpers.SSHConfig.Port` is `int`. Convert with `strconv.Atoi` at the edge (empty → 0 → defaults to 22). SQLite never tunnels.
- **Tunnel lifecycle (fixed in Phase 1, don't regress):** on a successful connect, the SSH tunnel must stay open for the connection's lifetime — it rides `ConnectedMsg.Tunnel` → stored on `Model.ActiveTunnel` → closed on replace and on exit. Closing it right after `driver.Connect` kills the live pool (that was a sev-5 bug). For Phase 2 in-app browse, `ActiveTunnel` stays open while you browse and closes on disconnect/quit (currently the hand-off holds it for lazysql's lifetime — `handoff.go`).
- **Hand-off is the fallback.** Keep `handoff.go` working until the in-app browser reaches parity (Implementation Order). Gate it behind a key so users can still reach lazysql for anything not yet built.
- **Build gate:** `go build ./...` must stay green for the WHOLE module (the tview app too) after any shared change. Adding deps can bump shared transitive deps — re-run it.
- **Testing:** build the `Model`, call handler methods, assert `(Model, tea.Cmd)` + mutated fields. Isolate pure logic (grid window math, DML change-merge, tree flatten) into testable functions. A temp-sqlite integration test covers end-to-end with no server.
- **Versioning:** `-X main.version` → `m.Version` → footer. `make install` (tagged) vs `make install-dev` (`dev-<sha>`). Keep new entrypoint wiring consistent.

## Subsystems, approach, and effort

Ordered by risk. Effort = focused full-time dev.

### 1. App shell / focus + layout — ~2–3 days
Hand-rolled focus manager + `Screen`/`Focus` enums + lipgloss split layout + keymap adapter (`tea.KeyMsg`→`keymap.Key`). Foundational; everything hangs off it.

### 2. Schema tree + sidebar — ~7–10 days
Hand-rolled tree: flat `[]Node{Name, FullPath, Type, Depth, Expanded, Children}`, cursor over the flattened visible set, lazy-load children on expand (`GetTables/Views/Functions/...` in a `tea.Cmd`), fuzzy search with ancestor-expand (port `prioritizeResult` + `stripColorTags`). Sidebar = stacked fields with focus cycling + sticky scroll.
- **Hard:** no parent pointers (store `FullPath`), async load ordering (no `App.Draw()` from goroutines — must route results as `Msg`s), search-expand vs user-expand state, viewport scroll with variable field heights.

### 3. Results grid + DML — **the dominant cost, ~4–6 weeks**
The reason Phase 2 is big. Bubble Tea has **no table widget that scales** — `bubbles/table` doesn't virtualize or scroll horizontally. Hand-roll everything:
- **Virtualized render:** window of visible rows from scroll offset; lipgloss `JoinHorizontal` per row; frozen header; ANSI-stripped width measurement; cache the visible page.
- **Horizontal scroll:** manual `ScrollX` + visible-column range for wide tables (50+ cols); truncate vs wrap.
- **Cell focus + inline edit:** spawn a temp `textinput` over the focused cell, Tab/Shift+Tab to adjacent cells (Excel-like), materialize into `PendingChanges`.
- **DML pending changes:** port `AppendNewChange` merge/replace/delete logic into flat state; color-code edited/inserted/deleted rows; commit via `ExecutePendingChanges`; confirm dialog.
- **Multi-view menu:** Records / Columns / Constraints / FK / Indexes — branch render on a `CurrentView` enum.
- **FK jump:** detect FK columns, infer `WHERE` from the cell, jump to target table with the filter applied (filter must survive pagination); breadcrumb back.
- **Async + cancel:** offset/limit tracked separately from scroll; loading spinner; cancel mid-fetch (`context` in the Cmd); guard against stale offset after sort/filter.
- **Hard:** virtualization under load, horizontal scroll, smooth inline-edit, cell-metadata indexing across row insert/delete, FK-jump filter context, external-editor (`Ctrl+E`) via `tea.ExecProcess`.

### 4. SQL editor — ~3–4 days (with `vimtea`)
Adopt `vimtea` (redis-tui-proven) for the vim stack; wire the **reusable** `sql_lexer` (highlighting, `tcell`→`lipgloss` conversion) + `sql_completer` (autocomplete popup). Execute via `ExecuteQuery`/`ExecuteDMLStatement` → route results to the grid; append to query history.
- **Hard:** autocomplete popup positioning/clipping; highlight rune-vs-visual-width with tabs; external editor. Low–medium risk because the heavy logic is reused.

### 5. Cross-cutting — ~3–5 days
Pagination controls + async, modals (confirm/value-editor/query-preview), JSON viewer (pretty-print + collapse), CSV export (`helpers/csv.go`), query history modal, clipboard (`lib/`), saved queries if present.

**Honest total: ~6–8 weeks** for true parity, dominated by the grid. The other subsystems sum to ~2.5–3 weeks; the grid is the long pole and the main schedule risk (+1–2 weeks if virtualization/horizontal-scroll stutters at scale).

## Implementation Order (recommended: incremental, keep hand-off as fallback)

- **2.0 — Browse MVP (~1.5–2 wks):** app shell + focus (1) + tree (read part of 2) + read-only records grid with pagination + vertical scroll (subset of 3). Tree select → `GetRecords` → grid. **No edit, no editor, no horizontal-scroll polish.** Switch the connect path from hand-off to in-app browse for read-only; keep `lazysql` hand-off behind a key (e.g. `o`) for everything not yet built.
- **2.1 — SQL editor (~0.5 wk):** `vimtea` + lexer + completer → `ExecuteQuery` → grid. Query history.
- **2.2 — DML (~1.5 wks):** cell edit, insert/delete, pending changes, commit/rollback, color coding, confirm.
- **2.3 — Full grid + meta (~2 wks):** horizontal scroll, sorting, filtering, multi-view menu (Columns/Constraints/FK/Indexes), sidebar inspector, FK jump, JSON viewer, CSV export, multi-tab.
- **2.4 — Polish (~0.5 wk):** mouse cell-select, external editor, perf pass on large tables, keybinding audit, retire the hand-off.

Each phase is independently shippable; the hand-off stays as the safety net until 2.3.

### Phase 2.0 — concrete first steps (do in order)

A fresh session should start exactly here. Each step builds + tests green before the next.

1. **Types.** In `internal/tui/types`: add `Screen` values (`ScreenBrowse`, `ScreenCellEdit`, `ScreenConfirm` already exists as `ScreenConfirmDelete` — reuse/rename) and a `Focus` type (`FocusTree`, `FocusGrid`). Add `Msg`s: `DatabasesLoadedMsg{DBs []string; Err error}`, `TablesLoadedMsg{DB string; Tables map[string][]string; Err error}`, `RecordsLoadedMsg{Rows [][]string; Total int; Err error}`.
2. **Commands.** In `internal/tui/commands`: add a new file `commands_browse.go` with `tea.Cmd` factories — `LoadDatabases()`, `LoadTables(db)`, `LoadRecords(db, table, where, sort string, offset, limit int)` — each calling the driver method and returning the typed `Msg`. The driver comes from the connection just made; store the live `drivers.Driver` on the Model (or on Commands) after connect instead of tearing it down. NOTE: Phase 1's `Connect` validates then the hand-off owns the session; for in-app browse you keep the `drivers.Driver` AND the tunnel alive — adjust `Connect` to return the live driver in `ConnectedMsg` (add a field) rather than discarding it.
3. **Tree (read-only).** New `internal/tui/ui/tree.go`: flat `[]node{Name, FullPath, Type, Depth, Expanded}`, cursor over the flattened-visible slice, `j/k`+arrows nav, Enter to expand (lazy-load tables via `LoadTables`) / select a table. On table select → `FocusGrid` + `LoadRecords`.
4. **Grid (read-only, paginated).** New `internal/tui/ui/grid.go`: render a window of rows (`offset..offset+limit`) with a frozen header, vertical scroll within the page, `n`/`p` for next/prev page (re-issue `LoadRecords` with new offset), column widths from max visible cell width (cap + truncate). Keep it read-only — no edit yet. Pure helpers (`visibleWindow`, `colWidths`) live here and get unit tests.
5. **Browse screen + focus.** New `internal/tui/ui/browse.go` + route in `update.go`: `ScreenBrowse` lays out `tree | grid` with `tab` to switch `Focus`; dispatch keys to the focused pane. `View` composes via `lipgloss.JoinHorizontal`.
6. **Wire the connect path.** Change `handleConnectedMsg` (currently calls `handoffToLazysql`) to instead store the live driver + tunnel and switch to `ScreenBrowse` → `LoadDatabases`. Keep `handoffToLazysql` reachable behind a key (e.g. `o` = "open in lazysql") for everything 2.0 doesn't cover (editing, SQL, meta-views).
7. **Verify.** `go build ./...`, `go vet`, `go test ./internal/tui/...`, plus a temp-sqlite integration test: create a table, connect, assert tree lists it and `LoadRecords` returns the rows. Then a real interactive run against frost-dev.

Deliverable of 2.0: connect in lazytea → see the schema tree → pick a table → page through rows, all in-app, through the SSH tunnel. Editing/SQL still hand off to lazysql.

## Testing

Same as Phase 1 — no `teatest`, no live `tea.Program`. Build the `Model`, call handler methods, assert `(Model, tea.Cmd)` + state. The `*Commands` DI seam runs query tests against a stub driver (or `go-sqlmock`, already a dep). Reuse `helpers/ssh_tunnel_test.go` harness for any tunnel-path test. Grid: unit-test the virtualization window math + DML change-merge logic directly (pure functions — isolate them from rendering). A real sqlite integration test (temp db → tree → records → edit → commit) covers the end-to-end without a server.

## Risks

- **Grid virtualization + horizontal scroll** — the whole bet. No precedent widget; redis-tui's list is 1-D. Mitigate: isolate the render-window + width-measure as pure, testable functions first; prototype against a 100k-row sqlite table before committing to the full grid.
- **DML pending-change logic is subtle** — the tview `AppendNewChange` merge rules are intricate. Port + unit-test in isolation before wiring UI.
- **Scope creep** — "parity" is large. The incremental order above lets you stop at 2.0/2.1 and still have a useful tool (browse + query), with editing as a follow-on.
- **vimtea coupling** — if its API constrains us, fall back to `bubbles/textarea` + hand-rolled vim (the lexer/completer/undo logic is ours regardless).
- **Keymap fidelity** — adapting tcell input to the reused keymap resolver must preserve user bindings; wrap, don't rewrite.

## References

- Phase 1 plan + boundary: `BUBBLETEA_PLAN.md`
- Template: `~/dev/redis-tui` — `internal/ui/` (grid/list/editor/screen routing), `vimtea` usage
- Parity target: `components/{tree,sidebar,results_table,sql_editor,home,pagination,json_viewer,result_table_filter,tabbed_menu,sql_lexer,sql_completer}.go`
- Reusable data layer: `drivers/driver.go` (the contract), `models/models.go`, `helpers/`, `app/keymap.go`
