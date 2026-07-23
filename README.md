# cellar

A keyboard-driven terminal client for SQL databases — schema browser, results grid, and SQL editor in one Bubble Tea app. It speaks MySQL, PostgreSQL, and SQLite, and tunnels through SSH bastions natively, including ProxyCommand setups like AWS SSM.

<!-- screenshots: drop images here once captured -->
<!-- ![browse](images/browse.png) -->
<!-- ![editor](images/editor.png) -->

## Why

I wanted a single TUI where I could open a connection, walk the schema, page through a table, inspect an index, and run a quick query — without juggling a GUI client and an SSH config. cellar is that: everything lives in one window, the whole thing is keyboard-first, and the SSH tunnel is built in so reaching a database behind a bastion is just part of the connection.

## Requirements

Go 1.25 or newer to build from source. No other runtime dependencies — it's a single static binary.

For the full keybinding experience (`ctrl+enter` to run, `ctrl+]`/`ctrl+[` and `ctrl+1..9` for tabs) you want a terminal that speaks the kitty keyboard protocol: WezTerm, kitty, Ghostty, Alacritty, iTerm2, foot, and friends. On anything older the app still works — those chords have legacy fallbacks (`ctrl+r` runs, `ctrl+pgup/pgdn` switches tabs). Compact keyboards are first-class: nothing requires function keys, Home/End, or PgUp/PgDn — `g`/`G`, `n`/`p`, and `ctrl+a`/`ctrl+e` cover all of it.

## Install

Clone and build with the Makefile, which drops the `cellar` binary into `~/.local/bin`:

```bash
git clone <your-repo-url> cellar
cd cellar
make install
```

`make install-dev` builds a version tagged with the current commit (handy while hacking), and `make build` just produces `./cellar` without installing. Override the install location with `PREFIX=/somewhere/bin make install`.

## Connections

First run lands you on the connection list. Press `a` to add one. A connection is a name plus a database URL:

```
mysql://user:pass@host:3306/dbname
postgres://user:pass@host:5432/dbname
sqlite:///absolute/path/to/file.db
```

For SQLite, a bare filesystem path works too — the URL form just saves you setting the provider by hand.

Connections are saved to `~/.config/cellar/config.toml`. Mark a connection read-only in the form and cellar refuses every write for it — any non-SELECT query gets blocked before it reaches the wire.

Once the list grows past a screenful it scrolls, and `/` filters it live by name or host — the same muscle memory as filtering the schema tree. `K`/`J` reorder entries, `t` test-connects, `D` duplicates.

On Postgres you can set a default schema (the form's `Schema` field, or `--schema` on the CLI). When set, cellar auto-expands that schema and drops the cursor on it when you connect, so you land in its tables instead of `hdb_catalog`/`information_schema`. It doesn't hide the other schemas — they're still there, just collapsed.

### Adding connections from the command line

If your URLs come from somewhere else — a Vault cred script, an `.envrc`, whatever — you can push them into cellar without the form:

```bash
cellar --add-connection --name "orders-prod" --url "mysql://user:pass@host:3306/orders" --read-only
```

It upserts by name (re-running replaces the entry instead of duplicating it), so it's safe to call from a script that refreshes short-lived credentials. The provider is inferred from the URL scheme; pass `--provider` to override. Add `--schema public` to set a Postgres default schema, drop `--read-only` for a writable connection, and `--config` to target a non-default config file. The whole thing writes to `~/.config/cellar/config.toml` and exits without opening the TUI.

### Vault-resolved credentials

If your database creds are short-lived — generated on demand by a Vault script, say — you don't have to keep re-importing a fresh URL. Give the connection a `Vault Command` (the last field in the add/edit form, or `--vault-command` on the CLI) and cellar runs that command every time you connect, reads its stdout, and dials whatever URL it prints. The command owns credential generation; cellar just asks for a URL when it needs one.

```bash
cellar --add-connection --name "orders-prod" \
  --vault-command "bash /path/to/creds.sh url orders" --read-only
```

The command runs to completion before the dial, so it can log into Vault, mint a lease, and print `mysql://user:pass@host:3306/orders`. Only the command string is written to `config.toml` — the resolved credentials live in memory for that session and never touch disk, which is the whole point over baking a temporary URL into the config. If the command fails or prints nothing, the connect is aborted and its stderr comes back as the error, so you can see what Vault complained about. cellar splits the command on whitespace and runs it directly (no shell), so use an absolute path with no spaces, and make sure whatever env the command needs — Vault address, login token, etc. — is present in the shell you launch cellar from.

### SSH tunneling

Toggle SSH in the add/edit form (`ctrl+s`) and fill in the bastion host, port, user, and either a private key or a password. cellar forwards the database connection through that bastion for the life of the session. For hosts you only reach through a wrapper — AWS SSM, for example — put the full command in the proxy-command field and cellar runs it instead of dialing directly. Passphrases and passwords are kept in memory only and never written to the config file.

## Using it

`enter` on a connection opens the browser: schema tree on the left, results grid on the right. Tables show as `•`, views as `◇`, each in their own group. Pick one to load its first page, then page, scroll, sort, and filter from the grid. Press `i` on anything — a tree node or the open table — and a floating inspector pops up with its columns, indexes, foreign keys, and full DDL (or the view's definition), each a tab, `y` to copy the lot. That inspector is the fastest way to answer "what's actually on this table" without leaving your spot.

Press `e` for the SQL editor. It keeps the schema tree beside you as a sidebar (`ctrl+b` hides it; `tab` cycles sidebar → editor → results, `shift+tab` cycles backwards, and `enter` on a table in the sidebar types its name into your query, quoted if the dialect needs it). `ctrl+enter` runs the statement under the cursor, `ctrl+shift+enter` runs the whole buffer statement by statement. Queries run async — `esc` cancels a slow one instead of you killing the app.

Long cell value clipped in the grid? `v` floats a wrapped, scrollable peek over the grid; `V` opens the full-screen viewer; `w` toggles full-width cells inline. Query results (not table pages) can also flip to a JSON view with `J` or export to CSV/JSON with `x`.

Press `?` or `ctrl+g` anywhere for the full keymap. The essentials:

### Connection list

| Key | Action |
|---|---|
| `enter` | open (in-app browse) |
| `/` | filter list (esc clears) |
| `a` / `e` | add / edit |
| `D` / `d` | duplicate / delete |
| `K` / `J` | move up / down |
| `t` / `r` | test / reload |
| `?` / `q` | help / quit |

### Schema tree

| Key | Action |
|---|---|
| `j`/`k`, arrows | navigate |
| `enter`, `→`/`l` | open table/view / expand |
| `←`/`h` | collapse |
| `/` | fuzzy search |
| `i` | inspect (columns / indexes / FKs / DDL) |
| `g` / `G` | top / bottom |
| `tab` | focus grid |

### Results grid

| Key | Action |
|---|---|
| `h`/`l`, `j`/`k` | move column / row |
| `n` / `p` | next / previous page |
| `s` / `/` | sort / filter (WHERE) |
| `i` | inspect the table |
| `enter` / `⌫` | foreign-key jump / back |
| `v` / `V` | peek popup / full cell view |
| `w` / `J` | wide cells / JSON view |
| `d` / `o` | DELETE / INSERT SQL into the editor (review, then run) |
| `x` / `y` | export / copy |
| `esc` | cancel a running load, else back out |

### SQL editor (`e`)

| Key | Action |
|---|---|
| `ctrl+enter` | run statement at cursor (`ctrl+r` legacy fallback) |
| `ctrl+shift+enter` | run all statements |
| `esc` | cancel the running query / dismiss popup / leave |
| `ctrl+b` | toggle schema sidebar |
| `ctrl+x` | zoom the focused pane (tmux-style) — `tab` flips which pane is full |
| `ctrl+t` / `ctrl+w` | new / close query buffer |
| `ctrl+]`/`ctrl+[`, `ctrl+1..9` | switch / jump query buffer |
| `ctrl+space` | completion popup (auto-shows at 2+ chars) |
| `ctrl+n` / `ctrl+p` | engage completion — then `↑`/`↓` move, `tab` accepts |
| `ctrl+/` | toggle comment |
| `ctrl+z` / `ctrl+y` | undo / yank line |
| `ctrl+s` / `ctrl+shift+s` | save (names an `untitled` buffer) / save as |
| `ctrl+o` | saved queries + history picker |

Query buffers are tabs: each shows as `1:name` in a bar above the editor, named `untitled` until you save it — `ctrl+s` prompts for a name and that name becomes the tab title and the saved-query entry in one step. Buffers, names, and your sidebar preference persist per connection, so disconnecting and coming back restores your workspace exactly.

The completion popup stays out of your way by design: it only auto-appears once you've typed two characters of a word, `esc` dismisses it and it stays dismissed until you move to a different word, and arrow keys keep moving your cursor until you engage the popup with `ctrl+n`/`ctrl+p`. `ctrl+space` summons it on demand — handy right after typing `table.`.

## Config

Everything lives under `[application]` in `~/.config/cellar/config.toml`. The one you're most likely to touch is `QueryRowLimit`: editor SELECTs fetch at most this many rows (default 5000) so a careless `SELECT *` on a big table can't eat your terminal's memory — the status line tells you when a result got capped. Set it to `-1` for unlimited. Table browsing is unaffected; that's paged server-side.

A per-repo `.cellar.toml` in your working directory replaces the global connection list wholesale — useful for project-scoped credentials.

You can read and write `[application]` settings from the CLI without opening the file: `cellar config list` shows everything, `cellar config get QueryRowLimit` reads one, and `cellar config set BackupDir ~/cellar-backups` writes one back (keys are case-insensitive, connections and other settings are preserved).

For everything else there's a command palette: press `:` on the connections screen, the browse screen, or the editor's results pane. It takes `backup [path]`, `import <backup.tar.gz>`, `set <key> <value>`, `get <key>`, `settings`, `help`, and `quit` (`q`), with the available commands listed right in the modal. New commands get added to a registry, so features can land there without claiming another keybind.

There's also an in-app settings screen: press `,` on the connections or browse screen. It covers the everyday knobs — `BackupDir`, `DefaultPageSize`, `QueryRowLimit`, `MaxQueryHistoryPerConnection`, `DisableSidebar` — with `enter` to edit (booleans just toggle), saved straight to the config file and applied live where that makes sense. Deeper settings stay in the TOML; the screen tells you where.

## Backup and restore

`cellar export` archives your whole config dir — connections, saved queries, query buffers, and history — into a timestamped `tar.gz`. It lands in `BackupDir` if you've set one (see above), else the current directory, and you can pass an explicit path instead (`cellar export ~/some/backup.tar.gz`). The same export is one keypress in the app: `x` on the settings screen (`,`). Mind where you put it: the archive contains your connection credentials.

`cellar import <backup.tar.gz>` restores one. It never merges — your current config dir is moved aside to `cellar.pre-import-<timestamp>` first, so a restore is always reversible. Delete the aside copy once you're happy.

## Credits

Inspired by [jorgerojas26/lazysql](https://github.com/jorgerojas26/lazysql). Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## License

MIT — see [LICENSE.txt](LICENSE.txt).
