# cellar

A keyboard-driven terminal client for SQL databases — schema browser, results grid, and SQL editor in one Bubble Tea app. It speaks MySQL, PostgreSQL, SQLite, and SQL Server, and tunnels through SSH bastions natively, including ProxyCommand setups like AWS SSM.

<!-- screenshots: drop images here once captured -->
<!-- ![browse](images/browse.png) -->
<!-- ![editor](images/editor.png) -->

## Why

I wanted a single TUI where I could open a connection, walk the schema, page through a table, edit a few rows, and run a quick query — without juggling a GUI client and an SSH config. cellar is that: everything lives in one screen, the whole thing is keyboard-first, and the SSH tunnel is built in so reaching a database behind a bastion is just part of the connection.

## Requirements

Go 1.24 or newer to build from source. No other runtime dependencies — it's a single static binary.

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
sqlserver://user:pass@host:1433/dbname
```

Connections are saved to `~/.config/cellar/config.toml`. Mark a connection read-only in the form and cellar refuses every write for it — edits, inserts, deletes, and any non-SELECT query.

### SSH tunneling

Toggle SSH in the add/edit form (`ctrl+s`) and fill in the bastion host, port, user, and either a private key or a password. cellar forwards the database connection through that bastion for the life of the session. For hosts you only reach through a wrapper — AWS SSM, for example — put the full command in the proxy-command field and cellar runs it instead of dialing directly. Passphrases and passwords are kept in memory only and never written to the config file.

## Using it

`enter` on a connection opens the browser: schema tree on the left, results grid on the right. Pick a table to load its first page, then page, scroll, sort, and filter from the grid. Edit cells in place and stage inserts/deletes; staged changes are color-coded and commit together in one transaction with `ctrl+s`. Press `e` for the SQL editor — syntax-highlighted, with autocomplete and undo — and `ctrl+r` to run a query into the same grid. Open several tables at once as tabs.

Press `?` anywhere for the full keymap. The essentials:

### Connection list

| Key | Action |
|---|---|
| `enter` | open (in-app browse) |
| `a` / `e` | add / edit |
| `D` / `d` | duplicate / delete |
| `t` / `r` | test / reload |
| `?` / `q` | help / quit |

### Schema tree

| Key | Action |
|---|---|
| `j`/`k`, arrows | navigate |
| `enter`, `→`/`l` | open table / expand |
| `←`/`h` | collapse |
| `/` | fuzzy search |
| `g` / `G` | top / bottom |
| `tab` | focus grid |

### Results grid

| Key | Action |
|---|---|
| `h`/`l` | select column |
| `j`/`k` | move row |
| `n` / `p` | next / previous page |
| `c` / `C` | edit cell / set NULL·EMPTY·DEFAULT |
| `o` / `d` | add row / toggle delete |
| `ctrl+s` / `u` | commit / discard staged changes |
| `s` / `/` / `i` | sort / filter / metadata views |
| `enter` / `⌫` | foreign-key jump / back |
| `v` / `J` | cell viewer / JSON view |
| `x` / `ctrl+y` | export / copy cell |

### SQL editor (`e`)

| Key | Action |
|---|---|
| `ctrl+r` | run query |
| `tab` | accept completion |
| `ctrl+z` | undo |
| `ctrl+s` / `ctrl+q` | save query / back |

### Tabs

| Key | Action |
|---|---|
| `T` | open selected table in a new tab |
| `]` / `[` | next / previous tab |
| `ctrl+w` | close tab |

## Credits

Inspired by [jorgerojas26/lazysql](https://github.com/jorgerojas26/lazysql). Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## License

MIT — see [LICENSE.txt](LICENSE.txt).
