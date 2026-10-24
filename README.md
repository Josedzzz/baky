# baky

Terminal tool for backing up the infrastructure of the GRID Research Group from a single place:

- **File backup**: compresses files and folders into `tar.gz` archives sent to a backup destination (for example a NAS), manually, on a schedule or when changes are detected, and restores them.
- **Config backup**: watches configuration folders (Firewall, VPN, DNS...) and records every change as a commit in a Git repository, with history and a line-by-line `diff`.

It is written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea). It is a single program with two modes: a terminal interface (TUI) for the administrator and a background service that runs the automatic backups.

Project developed as part of the **MAGITI** degree project: *Modelo de Automatización para la Gestión de la Infraestructura de TI del Grupo de Investigación GRID* (Automation Model for the IT Infrastructure Management of the GRID Research Group).

## Origin

baky unifies three tools that used to be installed and maintained separately:

| Original tool | What it did | Where it ended up in baky |
|---|---|---|
| `baky` | File backup to `tar.gz` and restore | `internal/backup`, `internal/restore` |
| [`magiti-config-backup`](https://github.com/MiguelFVasquez/magiti-config-backup) | Bash daemon that versioned configurations in Git | `internal/gitbackup`, rewritten in Go |
| [`magiti-backup-tui`](https://github.com/MiguelFVasquez/magiti-backup-tui) | TUI to control that daemon | *Config backup* and *Service* menus |

The administrator now installs a single binary, a single service and edits a single configuration file.

## Usage

```
baky                               Opens the terminal interface
baky daemon                        Runs the automatic backups (used by systemd)
baky import-magiti <config.json>   Imports the watched folders of the magiti-config-backup daemon
baky help                          Shows the help
```

### Interface menus

```
BAKY
  File backup     Manage paths · Back up now · View backups / Restore · Backup destination
  Config backup   Watched folders · Change history · Repository
  Service         Service status · Live logs
  Help
  Quit
```

Common keys: `↑/↓` or `k/j` to navigate, `enter` to select, `esc` to go back, `?` for help and `q` to quit.

The automatic backups are run by the service, not by the interface: you can close the TUI without interrupting them. Changes made from the TUI (adding a path, watching a new folder) are applied by the service by itself, without restarting it.

## Quick install

```bash
git clone https://github.com/Josedzzz/baky.git
cd baky
go build -o baky ./cmd
sudo install -m 0755 baky /usr/local/bin/baky
```

Requirements: Go 1.24 or later to build, `git` for the config backup and `systemd` for the service.

The full installation (service, permissions and config repository) is in [docs/installation.md](docs/installation.md).

## How it works

### File backup

Each configured path has a frequency:

| Frequency | When it is backed up |
|---|---|
| Daily | Every day, during the 3 AM hour |
| Weekly | On Sundays, during the 3 AM hour |
| On change | A few seconds after a file in the path changes |

Backups are stored at the destination as `[name]_YYYYMMDD_HHMMSS.tar.gz`. When restoring you can choose the original location or another path, and what to do if there are files already:

- **Overwrite**: replaces the files that are in the backup and keeps the rest.
- **Rename**: keeps what exists under a name with the date and restores the backup.
- **Skip**: does not restore if there are files already.

Before restoring, baky shows what would change at the chosen destination:

```
RESTORE CHANGES
M modified: 1   + created: 1   = unchanged: 1

M report.txt  (modified)
+ deleted.txt  (created)
= same.txt  (unchanged)
```

With `enter` on a file you see its line-by-line differences against the current file (in red what is removed, in green what comes back), and `r` continues with the restore. This preview changes nothing. The line-by-line comparison uses `git`, and only applies to text files.

The backup is always extracted to a temporary folder first: a damaged backup never changes what already exists at the destination.

### Config backup

Each watched folder is mapped to a folder inside the config repository (for example `/etc/openvpn` → `vpn`). When a file changes, the service copies it to the repository, creates one commit per file and pushes it to the remote:

```
auto-backup: updated nat.conf (2026-07-24 21:46:14)
auto-backup: deleted config-test.conf (2026-07-24 21:39:09)
```

In addition:

- On startup it syncs everything: changes made while the service was stopped are not lost.
- It ignores editor temporary files (`*.swp`, `*~`, `4913`).
- If the push to the remote fails, the commits stay in the local repository and the push is retried.
- If a watched folder stops being accessible, what was already backed up is kept.

#### Validating every change

Each watched folder can have two optional commands, set with `c` in *Watched folders*. `{dir}` is replaced by the folder that holds the configuration:

| Command | Example with Shorewall | When it runs |
|---|---|---|
| Validation | `sudo -n shorewall check {dir}` | After every detected change, and before a restore |
| Apply | `sudo -n shorewall try {dir}` | Only when a file is restored from the history |

When a change does not pass validation, baky still backs it up (the backup reflects what is on disk), but marks it: the commit ends with `[validation failed]`, stores the validator's error, shows up in red in the history and leaves a warning in the log. This way a syntax error is known a few seconds after it is saved, before anyone restarts the service.

baky never applies a change it detects on its own: it only validates it.

The commands run exactly as written, without a shell. If they need privileges they must start with `sudo -n`, and the service user must have that permission without a password (see [docs/installation.md](docs/installation.md)).

#### Restoring an earlier configuration

In *Config backup → Change history* each line is a backed-up change. When you pick one with `enter`, baky compares the current file with the version saved in that change and shows what restoring it would do:

```
── Changes from line 1 of the current file ──
  -A INPUT -p tcp --dport 22 -j ACCEPT
- -A INPUT -p tcp --dport 8080 -j ACCEPT      current line that is removed
+ -A INPUT -p tcp --dport 80 -j ACCEPT        line that comes back
  -A INPUT -p tcp --dport 443 -j ACCEPT
```

`r` restores that version, after confirmation. If the folder has validation and apply commands, the restore does not touch the real configuration until it knows it is good:

1. It copies the folder to a temporary directory and puts the version to restore there.
2. It runs the validation on that copy. If it fails, it stops.
3. It runs the apply command on that copy. If it fails, it stops.
4. Only then does it write the file to the watched folder, keeping its owner and permissions. If the user cannot write there, it writes the file through `sudo` with the `baky-install` helper, which only writes inside the folders listed in `/etc/baky/restore-dirs`.

The service detects the restore and records it as one more change, so it can also be undone. If the chosen change deleted the file, it holds no version to restore: pick an earlier one.

The config repository is a dedicated repository, separate from the code. At GRID it is [`grid-config-data`](https://github.com/MiguelFVasquez/grid-config-data).

Commits made before the messages were translated to English (`auto-backup: actualizado ...`, `[validación fallida]`) still show up in the history, and invalid ones are still marked.

## Files

They all live in `~/.config/baky/` of the user that runs baky. The `BAKY_CONFIG` variable lets you use another location for `config.json`; the other files go next to it.

| File | Contents |
|---|---|
| `config.json` | Settings: destination, paths, repository and watched folders |
| `state.json` | Backup and restore history, last backup of each path |
| `baky.log` | Activity log of the service |

Example `config.json`:

```json
{
  "nas_path": "/mnt/nas/backups",
  "backup_paths": [
    { "path": "/home/grid/documents", "frequency": "daily" },
    { "path": "/etc/nginx/nginx.conf", "frequency": "on_change" }
  ],
  "config_backup": {
    "repo_dir": "/home/grid/grid-config-data",
    "branch": "main",
    "watches": [
      { "source": "/etc/openvpn", "dest": "vpn" },
      {
        "source": "/etc/shorewall",
        "dest": "firewall",
        "check": "sudo -n shorewall check {dir}",
        "apply": "sudo -n shorewall try {dir}"
      }
    ]
  }
}
```

You don't need to edit it by hand: everything is configured from the interface.

## Project structure

```
baky/
├── cmd/main.go            Entry point: TUI, daemon and import
├── internal/
│   ├── backup/            File backup (tar.gz) and its scheduling
│   ├── restore/           Listing, comparing and restoring backups
│   ├── gitbackup/         Versioned config backup in Git
│   ├── daemon/            Service: runs both engines and reloads the configuration
│   ├── service/           Control of the systemd service
│   ├── config/            Configuration and state shared by the TUI and the service
│   ├── logging/           Writing and reading the service log
│   ├── watch/             Recursive folder watching (fsnotify)
│   ├── textdiff/          Line-by-line comparison of two versions of a file
│   └── tui/               Terminal interface, one file per screen
├── deploy/                systemd unit, sudoers rule and the baky-install helper
└── docs/                  Installation and test logs
```

The code, the interface and the documentation are all in English.

## Testing

### Automated tests

```bash
go test ./...          # all tests
go test -race ./...    # also detects race conditions
go vet ./...           # static analysis
```

They do not use the network or touch the real configuration: each test works in temporary folders, and the config backup is tested against a local Git repository that acts as the remote.

| Package | What its tests cover |
|---|---|
| `internal/backup` | Creating the `tar.gz` (folders, single file, symlinks) and when a scheduled backup is due |
| `internal/restore` | Restore with overwrite, rename and skip; damaged backup; single file; preview of the changes |
| `internal/gitbackup` | Creating, modifying, deleting and renaming files; subfolders; moved folders; temporary files; push failure and retry; validating every change; comparing and restoring an earlier version with prior validation |
| `internal/daemon` | The whole service with both engines and a live configuration change |
| `internal/config` | Saving, history in `state.json`, migrating old configurations, damaged file, watched folders |
| `internal/tui` | Navigating the menus and the main flows, by sending keys to the model |
| `internal/logging`, `internal/watch`, `internal/textdiff` | Log format and reading; recursive watching; line-by-line comparison |

### Manual testing without installing anything

You can test everything in a temporary folder, without systemd, without `sudo` and without touching `~/.config/baky`. The `BAKY_CONFIG` variable makes baky use a separate configuration.

**1. Prepare the test environment**

```bash
go build -o baky ./cmd

export TEST=/tmp/baky-test
export BAKY_CONFIG=$TEST/cfg/config.json
mkdir -p $TEST/cfg $TEST/etc/firewall $TEST/docs $TEST/nas

# A local "remote" repository and its clone, which acts as the config repository
git init --bare --initial-branch=main $TEST/remote.git
git init --initial-branch=main $TEST/repo
git -C $TEST/repo remote add origin $TEST/remote.git

echo "rule 1" > $TEST/etc/firewall/rules.conf
echo "hello" > $TEST/docs/report.txt
```

**2. Start the service in a terminal**

```bash
./baky daemon
```

It stays in the foreground showing the log. Stop it with `Ctrl+C`.

**3. Open the interface in another terminal**

```bash
export TEST=/tmp/baky-test
export BAKY_CONFIG=$TEST/cfg/config.json
./baky
```

The main menu will show the service as inactive: that is expected, because in this test the daemon is not running under systemd. For the same reason, the start and stop keys of *Service → Service status* do not apply here.

**4. Test the config backup**

1. *Config backup → Repository*: `e`, type `/tmp/baky-test/repo`, `enter`.
2. *Config backup → Watched folders*: `a`, type `/tmp/baky-test/etc/firewall`, `enter`, accept the name `firewall` with `enter`.
3. In a third terminal, modify, create and delete files:

   ```bash
   echo "rule 2" >> /tmp/baky-test/etc/firewall/rules.conf
   mkdir /tmp/baky-test/etc/firewall/extra
   echo "nat" > /tmp/baky-test/etc/firewall/extra/nat.conf
   rm /tmp/baky-test/etc/firewall/rules.conf
   ```

4. Expected result, a few seconds after each change:
   - In *Change history* (`r` to refresh): one commit per file, `auto-backup: updated ...` or `auto-backup: deleted ...`.
   - In *Change history*, `enter` on an earlier change shows the differences with the current file, and `r` restores it.
   - In *Service → Live logs*: `Commit created` and `Push succeeded`.
   - In the remote: `git -C /tmp/baky-test/remote.git log --oneline main` shows the same commits.

To test the sync on startup: stop the daemon with `Ctrl+C`, change a file in the watched folder and start it again. The change must be backed up on startup.

**5. Test the file backup**

1. *File backup → Backup destination*: `e`, type `/tmp/baky-test/nas`, `enter`; `t` should say it is accessible.
2. *File backup → Manage paths*: `a`, type `/tmp/baky-test/docs`, `enter`.
3. *File backup → Back up now*: `enter`. A `docs_YYYYMMDD_HHMMSS.tar.gz` should appear in `/tmp/baky-test/nas`.
4. *View backups / Restore*: `enter` on the backup, `c`, type `/tmp/baky-test/restored`, `enter`. The list of restore changes appears (here, `report.txt` is created); `r` to continue and `enter` to confirm. The file `report.txt` should appear in that folder.
5. Preview with differences: modify the original file (`echo "another line" >> /tmp/baky-test/docs/report.txt`), go back to *View backups / Restore*, `enter` on the backup and `o` (original location). The file shows up as "modified"; `enter` shows the line that would be removed.
6. On-change backup: in *Manage paths* press `f` until you see `[on change]`, then modify a file:

   ```bash
   echo "change" >> /tmp/baky-test/docs/report.txt
   ```

   About 5 seconds later the daemon creates a new `tar.gz`, and the log shows `On-change backup started` and `Backup completed`.

**6. Clean up**

```bash
rm -rf /tmp/baky-test
unset TEST BAKY_CONFIG
```

Testing on a real installation (systemd service and remote repository) is in step 7 of [docs/installation.md](docs/installation.md).

## To do

- Backup retention policy (delete the oldest ones automatically).
- Encryption of the backups and of the config repository.
- Alerts when the service fails.
- Comparing backup versions from the interface (`internal/restore/diff.go` already computes the differences).
- Consistent backup of running virtual machines.

## Documentation

- [Installation and setup](docs/installation.md)
- [Config daemon test log](docs/daemon-tests.md)
- [TUI test log](docs/tui-tests.md)

## Authors

- Jose D. Amaya — MAGITI Project Researcher
- Juan M. Florez — MAGITI Project Researcher

Based on an interview with Luis E. Sepúlveda, GRID Systems Administrator (June 22, 2026).

## License

MIT. See [LICENSE](LICENSE).
