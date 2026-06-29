# appfossil

An interactive terminal tool for finding unused macOS apps — the ones frozen in
time on your disk. It scans your installed applications, figures out when each
was last opened, how much disk space it takes, and how it was installed
(Homebrew cask, Mac App Store, or a manual `.dmg`/`.pkg`), then presents
everything in a sortable, filterable TUI so you can spot the apps gathering
dust.

It is **report-only** — it never uninstalls or moves anything.

## Features

- Scans `/Applications`, `/Applications/Utilities`, and `~/Applications`
  (optionally `/System/Applications`).
- Multi-signal last-used detection (see [Usage accuracy](#usage-accuracy)),
  falling back gracefully and labeling where each date came from. Approximate
  dates are shown with a `~` prefix.
- On-disk size per app.
- Install-source detection:
  - **App Store** — bundle contains a `_MASReceipt`.
  - **brew: <token>** — matched against installed Homebrew casks.
  - **System** — ships with macOS.
  - **Manual** — everything else (dmg/pkg/drag-install).
- Interactive TUI: sort by staleness / size / name, filter by source, filter by
  name, stale-only toggle, and a detail panel per app.
- Non-interactive `--json` and plain-text reports for scripting.

## Install / Build

Requires Go 1.24+ and macOS.

```bash
go build -o appfossil .
```

Optionally install it on your `PATH`:

```bash
go install github.com/cibermania5/appfossil@latest
```

## Usage

Launch the interactive UI:

```bash
./appfossil
```

Flags:

| Flag               | Description                                          | Default |
| ------------------ | ---------------------------------------------------- | ------- |
| `-days N`          | Staleness threshold in days                          | `90`    |
| `-include-system`  | Also scan `/System/Applications`                     | `false` |
| `-json`            | Print a JSON report instead of the UI                | `false` |
| `-md FILE`         | Write a Markdown report to `FILE` (`-` for stdout)   | _off_   |
| `-stale-only`      | Only include stale apps in the report output         | `false` |

When stdout is piped (or `-json`/`-md` is set), a report is produced instead of
the UI:

```bash
./appfossil -json > apps.json
./appfossil -stale-only                 # piped text report of stale apps
./appfossil -days 180 -json | jq '.[] | select(.stale)'

# Markdown reports
./appfossil -md report.md               # full report to a file
./appfossil -md report.md -days 180     # 6-month threshold
./appfossil -stale-only -md -           # stale apps as Markdown to stdout
```

The Markdown report contains a summary (counts and reclaimable size), a table of
stale apps, and a table of all apps — handy for sharing or committing a cleanup
checklist.

Reports include local app paths, bundle IDs, install sources, and last-used
dates. Treat `-json` and `-md` output as personal metadata before sharing or
committing it.

### Key bindings (TUI)

| Key            | Action                          |
| -------------- | ------------------------------- |
| `↑`/`k`, `↓`/`j` | Move cursor                   |
| `pgup`/`pgdn`  | Page up / down                  |
| `g` / `G`      | Jump to top / bottom            |
| `enter`        | Toggle the detail panel         |
| `s`            | Cycle sort (stale / size / name)|
| `f`            | Cycle source filter             |
| `t`            | Toggle stale-only               |
| `/`            | Filter by name (`esc` clears)   |
| `r`            | Rescan                          |
| `q`            | Quit                            |

## Usage accuracy

For each app, appfossil takes the last-used date from the strongest signal it
can read, in this order (the `Date From` column / `last_used_from` JSON field
tells you which one was used):

1. **`usage history`** — Apple's CoreDuet `knowledgeC.db`, which logs precise app
   launch/focus events. This is the most accurate source. **It is protected by
   macOS TCC**, so it can only be read when elevated:
   - **Run with `sudo`:** `sudo ./appfossil` reads the system-wide store at
     `/private/var/db/CoreDuet/Knowledge/knowledgeC.db`.
   - **Or grant Full Disk Access** to your terminal
     (**System Settings -> Privacy & Security -> Full Disk Access**), which lets
     the per-user store at
     `~/Library/Application Support/Knowledge/knowledgeC.db` be read without sudo.
2. **`Spotlight`** — `kMDItemLastUsedDate`. Reliable when present, but macOS
   often leaves it empty.
3. **`Library activity`** — the newest modification time across the app's
   `~/Library` data (containers, saved window state, preferences, caches, HTTP
   storage). A good lower bound on when the app last ran.
4. **`file date`** — the app's executable/bundle modification time. Weakest
   signal; reflects when the app was last *updated*, not opened.

When `knowledgeC.db` can't be read, the tool prints a one-line hint suggesting
`sudo`/Full Disk Access, and reports show `Date accuracy: approximate`.

> When run under `sudo`, appfossil resolves your real home directory via
> `SUDO_USER`, so per-user paths still point at your account rather than
> `/var/root`.

## Caveats

- Homebrew matching only covers casks that install a `.app` bundle; CLI-only
  casks (e.g. `mactex`) won't appear as brew-managed apps.
- Even as root, some macOS versions still gate `knowledgeC.db` behind Full Disk
  Access; if `sudo` doesn't yield precise dates, grant FDA to your terminal.

## How it works

```
internal/
  model/   AppInfo type, source/size/last-used formatting
  scan/    bundle enumeration, plutil/mdls metadata, du-style sizing,
           Homebrew cask map, install-source classification,
           knowledgeC.db usage history + ~/Library activity signals
  tui/     bubbletea model, custom color-coded table, detail panel
main.go    flags + TUI / JSON / text report dispatch
```

## License

This project is licensed under the [MIT License](LICENSE). You may use, modify,
and distribute it for any purpose, including commercially, as long as you keep
the copyright notice and license text with any copy or derivative work.
