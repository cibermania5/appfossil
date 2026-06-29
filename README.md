# appfossil

![macOS](https://img.shields.io/badge/platform-macOS-000000?style=for-the-badge&logo=apple&logoColor=white)
![Go](https://img.shields.io/badge/go-1.24+-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-4CAF50?style=for-the-badge)
![Report only](https://img.shields.io/badge/mode-report--only-E91E63?style=for-the-badge)

An interactive terminal tool for finding unused macOS apps — the ones frozen in
time on your disk. It scans your installed applications, figures out when each
was last opened, how much disk space it takes, and how it was installed
(Homebrew cask, Mac App Store, or a manual `.dmg`/`.pkg`), then presents
everything in a sortable, filterable TUI so you can spot the apps gathering
dust.

> [!IMPORTANT]
> **Report-only** — appfossil never uninstalls or moves anything. It only
> shows what is on your Mac so you can decide what to keep.

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

### TUI color legend

In the interactive UI, row colors reflect how long ago an app was last used
(relative to your `-days` threshold, default **90**):

| Color | Meaning | Example |
| --- | --- | --- |
| 🟢 **Green** | Recently used (under half the threshold) | opened 12 days ago |
| 🟡 **Yellow** | Aging (past half the threshold) | opened ~60 days ago |
| 🔴 **Red** | Stale (at or beyond the threshold) | opened ~142 days ago |
| 🟥 **Deep red** | Never opened / unknown last use | no usage record |

## Examples

The samples below use **fictitious apps** to show what output looks like. Your
machine will list real apps and paths.

### Interactive TUI

```bash
./appfossil
# or: task run
```

```
 appfossil · 6 apps · 3 stale (>90d) · 2.3 GB reclaimable · sort: stale · filter: all

  APP                  LAST USED     SIZE       SOURCE
  ArchiveRipper 2019   ~892d ago     1.2 GB     Manual          ← stale (red)
  PixelForge Studio    ~142d ago     890.0 MB   Manual          ← stale (red)
  WidgetLab            Never         128.0 MB   App Store       ← never (deep red)
  CloudSync Pro        12d ago       245.0 MB   brew: cloudsync ← fresh (green)
  DailyNotes           Today         54.0 MB    Manual          ← fresh (green)
  Calculator           3d ago        4.5 MB     System          ← fresh (green)

  ↑/↓ move · enter details · s sort · f filter source · t stale-only · / search · q quit
```

### Plain-text report

```bash
./appfossil -days 90
# piped output when stdout is not a terminal
```

```
appfossil report
6 apps · 3 stale (>90d) · 2.3 GB reclaimable
Note: dates are approximate. Re-run with sudo (or grant Full Disk Access) to read macOS usage history for precise last-used dates.

APP                    LAST USED    SIZE       SOURCE
ArchiveRipper 2019     ~892d ago    1.2 GB     Manual
PixelForge Studio      ~142d ago    890.0 MB   Manual
WidgetLab              Never        128.0 MB   App Store
CloudSync Pro          12d ago      245.0 MB   brew: cloudsync-pro
DailyNotes             Today        54.0 MB    Manual
Calculator             3d ago       4.5 MB     System
```

Filter to stale apps only:

```bash
./appfossil -stale-only -days 90
```

```
appfossil report
3 apps · 3 stale (>90d) · 2.2 GB reclaimable

APP                    LAST USED    SIZE       SOURCE
ArchiveRipper 2019     ~892d ago    1.2 GB     Manual
PixelForge Studio      ~142d ago    890.0 MB   Manual
WidgetLab              Never        128.0 MB   App Store
```

### JSON report

```bash
./appfossil -stale-only -json
# or: task report:json
```

```json
[
  {
    "name": "ArchiveRipper 2019",
    "path": "/Applications/ArchiveRipper 2019.app",
    "bundle_id": "com.example.archiveripper",
    "version": "3.1.0",
    "source": "Manual",
    "last_used": "2023-12-15T10:30:00-08:00",
    "last_used_approx": true,
    "last_used_from": "Library activity",
    "days_since_used": 892,
    "size_bytes": 1288490188,
    "stale": true
  },
  {
    "name": "PixelForge Studio",
    "path": "/Applications/PixelForge Studio.app",
    "bundle_id": "com.example.pixelforge",
    "version": "2.4.1",
    "source": "Manual",
    "last_used": "2025-02-07T14:22:00-08:00",
    "last_used_approx": true,
    "last_used_from": "Spotlight",
    "days_since_used": 142,
    "size_bytes": 933232640,
    "stale": true
  },
  {
    "name": "WidgetLab",
    "path": "/Applications/WidgetLab.app",
    "bundle_id": "com.example.widgetlab",
    "version": "1.0.2",
    "source": "App Store",
    "last_used": null,
    "last_used_approx": false,
    "last_used_from": "unknown",
    "days_since_used": -1,
    "size_bytes": 134217728,
    "stale": true
  }
]
```

Pipe into `jq` for quick summaries:

```bash
./appfossil -json | jq '[.[] | select(.stale)] | length'
./appfossil -days 180 -json | jq '.[] | select(.stale) | {name, days_since_used, size_bytes}'
```

### Markdown report

```bash
./appfossil -md report.md
# or: task report:md
./appfossil -stale-only -md -          # Markdown to stdout
./appfossil -days 180 -md spring-clean.md
```

Excerpt of the generated file:

```markdown
# appfossil report

_Generated Mon 29 Jun 2026, 17:05_

## Summary

- Apps scanned: **6**
- Stale (not used in 90+ days): **3**
- Reclaimable if stale apps removed: **2.3 GB**
- Total size of scanned apps: **2.6 GB**
- Date accuracy: **approximate (no usage history access)**

## Stale apps (not used in 90+ days)

| App | Last Used | Days Idle | Size | Source | Date From | Bundle ID |
| --- | --- | --: | --: | --- | --- | --- |
| ArchiveRipper 2019 | ~892d ago | 892 | 1.2 GB | Manual | Library activity | com.example.archiveripper |
| PixelForge Studio | ~142d ago | 142 | 890.0 MB | Manual | Spotlight | com.example.pixelforge |
| WidgetLab | Never | — | 128.0 MB | App Store | unknown | com.example.widgetlab |

## How to remove these apps

> **Warning:** appfossil never removes anything itself. Review each command and quit the app first.

### Manual installs

\`\`\`bash
mv "/Applications/ArchiveRipper 2019.app" ~/.Trash/  # ArchiveRipper 2019
mv "/Applications/PixelForge Studio.app" ~/.Trash/  # PixelForge Studio
\`\`\`
```

The Markdown report contains a summary (counts and reclaimable size), a table of
stale apps, suggested removal commands, and a table of all apps — handy for
sharing or committing a cleanup checklist.

> [!WARNING]
> Reports include local app paths, bundle IDs, install sources, and last-used
> dates. Treat `-json` and `-md` output as **personal metadata** before sharing
> or committing it.

## Install / Build

Requires Go 1.24+ and macOS.

```bash
go build -o appfossil .
```

With [Task](https://taskfile.dev) installed:

```bash
task build      # compile ./appfossil
task test       # run tests
task run        # launch the TUI
task report:md  # write report.md
```

Optionally install it on your `PATH`:

```bash
go install github.com/cibermania5/appfossil@latest
# or: task install
```

### Releases

Tagged releases are built with [GoReleaser](https://goreleaser.com) for macOS
(`darwin/amd64` and `darwin/arm64`).

Maintainers:

```bash
task release:snapshot   # local dry-run artifacts in dist/
git tag v0.1.0 && git push origin v0.1.0
task release            # publish GitHub release + archives
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
     (**System Settings → Privacy & Security → Full Disk Access**), which lets
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

> [!TIP]
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
