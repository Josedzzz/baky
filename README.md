# baky

A terminal-based backup utility designed for sysadmins and developers to quickly manage and automate backups of local files and directories to a NAS. Built with Go and the Bubble Tea TUI framework, it provides a fast, keyboard-driven interface for server maintenance.

## About

**baky** simplifies the backup workflow by providing an intuitive terminal user interface (TUI) to:

- Manage which files and directories to backup
- Automate backup scheduling with configurable frequencies
- Browse and restore from existing backups
- Handle file conflicts during restoration with intelligent strategies
- Track backup history and restore operations

The project prioritizes **speed**, **simplicity**, and **reliability** for system administrators who work in terminal environments.

## Features

### Phase 1: Backup Management (Complete)

- **Manage Backup Paths**: Add, edit, delete, and configure directories/files to backup
- **Configurable Schedules**: Set backup frequency per path (daily, weekly, on-change)
- **Automatic Backups**: Background watcher monitors files for changes and triggers backups
- **Backup History**: Track all backup operations with timestamps and status (success/error)
- **NAS Integration**: Configure backup destination on network storage

### Phase 2: Restore Functionality (Complete)

- **Browse Backups**: View all available backups with metadata (timestamp, size, source)
- **Restore Operations**: Extract backups to original or custom locations
- **Conflict Resolution**: Three strategies for handling existing files:
  - **Overwrite**: Replace existing files with backup content
  - **Rename**: Backup existing files with timestamp and restore new content
  - **Skip**: Preserve existing files and abort restore
- **Restore Tracking**: Log all restore operations with details and actions taken
- **Path Validation**: Ensure restore paths are absolute and parent directories exist

### Phase 3: Version Comparison (Planned)

- Compare file-level differences between backup versions (git-style diffs)
- View added, modified, and deleted files between versions
- Preview changes before restoring specific versions

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/Josedzzz/baky.git
cd baky

# Build the project
go build -o baky ./cmd

# Run the application
./baky
```

### Configuration

**baky** uses a single configuration file at `~/.config/baky/config.json` (Linux standard across all platforms).

The config file stores:

- Backup destination (NAS path)
- Monitored backup paths with schedules
- Backup history (last 100 events)
- Restore history (last 100 events)

### Basic Usage

1. **Start the app**: Run `./baky`
2. **Main Menu**: Navigate with arrow keys or vim keys (hjkl)

   - `↑/↓` or `k/j`: Navigate menu items
   - `Enter`: Select option
   - `q`: Quit

3. **Manage Backup Paths**: Add directories to backup

   - `a`: Add new path
   - `e`: Edit selected path
   - `f`: Cycle backup frequency (Daily → Weekly → On-Change)
   - `d`: Delete path

4. **View Backups**: Browse and restore existing backups

   - `↑/↓` or `k/j`: Scroll through backups
   - `PgUp/PgDn`: Scroll faster
   - `Enter`: Select backup to restore
   - `o`: Restore to original location
   - `c`: Restore to custom path

5. **Conflict Resolution**: Handle files at restore destination
   - `↑/↓`: Select action (Overwrite, Rename, or Skip)
   - `Enter`: Confirm and start restore

### Configuration File Example

```json
{
  "nas_path": "/mnt/nas/backups",
  "backup_paths": [
    {
      "path": "/home/user/documents",
      "frequency": "daily",
      "last_backup": "2026-04-14T11:07:48Z"
    },
    {
      "path": "/etc/nginx/nginx.conf",
      "frequency": "on_change",
      "last_backup": "2026-04-13T10:13:31Z"
    }
  ],
  "history": [
    {
      "path": "/home/user/documents",
      "timestamp": "2026-04-14T11:07:48Z",
      "result": "success"
    }
  ],
  "restore_history": [
    {
      "backup_filename": "documents_20260414_110748.tar.gz",
      "source_path": "/home/user/documents",
      "restore_path": "/home/user/documents",
      "timestamp": "2026-04-19T23:55:40Z",
      "result": "success",
      "action": "rename"
    }
  ]
}
```

## Architecture

### Backup Format

Backups are stored as compressed tar archives in the NAS directory:

- **Filename**: `[source_basename]_YYYYMMdd_HHmmss.tar.gz`
- **Example**: `nginx.conf_20260414_110748.tar.gz`
- **Contents**: Complete directory structure preserved in tar format

### Restore Process

1. **Path Resolution**: Match backup to original source path using config
2. **Conflict Detection**: Check if files exist at restore destination
3. **Conflict Handling**: Apply selected action (overwrite/rename/skip)
4. **Extraction**: Extract backup with intelligent directory detection
5. **Logging**: Record restore event with action and result

### Data Storage

```
~/.config/baky/config.json          # Configuration file
/mnt/nas/backups/                   # Backup destination (configurable)
  ├── documents_20260414_110748.tar.gz
  ├── nginx.conf_20260413_101331.tar.gz
  └── ...
```

## Tech Stack

- **Language**: Go 1.24+
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **UI Components**: [Bubbles](https://github.com/charmbracelet/bubbles)
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss)

## Project Structure

```
baky/
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── backup/                 # Backup logic & watcher
│   │   ├── backup.go           # Backup creation
│   │   └── watcher.go          # File change detection
│   ├── config/                 # Configuration management
│   │   └── config.go           # Config loading/saving
│   ├── restore/                # Restore functionality
│   │   ├── types.go            # Data structures
│   │   ├── scanner.go          # Backup scanning
│   │   ├── extractor.go        # Tar extraction
│   │   ├── restorer.go         # Restore operations
│   │   └── list.go             # Backup filtering
│   └── tui/                    # Terminal UI
│       ├── model.go            # TUI state model
│       ├── view.go             # TUI rendering
│       └── styles.go           # UI styling
├── go.mod & go.sum             # Go module files
└── README.md                   # This file
```

## Development

### Building

```bash
go build -o baky ./cmd
```

### Running Tests

```bash
go test ./...
```

## Roadmap

### Phase 1-2: Core Features (Completed)

- Backup path management with add/edit/delete
- Automatic backup scheduling (daily, weekly, on-change)
- File watcher for on-change backups
- View backups in TUI with scrollable list
- Restore to original or custom location
- Restore with conflict resolution (overwrite, rename, skip)
- Restore history tracking
- Backup history with filtering and sorting

### Phase 3: Version Comparison (High Priority)

- [ ] **Backup Diff Viewer** — Compare two backup versions and show added, modified, and deleted files, similar to `git diff --stat`
- [ ] **File Content Diff** — Show line-by-line differences for text files between two backup versions
- [ ] **Change Preview** — Preview what files would change before restoring a specific backup version
- [ ] **Restore Selectively from Diff** — Select individual files from the diff view to restore, rather than restoring the entire backup

### Phase 4: GitHub Integration (High Priority)

- [ ] **GitHub as Backup Destination** — Add support for pushing backups to GitHub Releases or repository storage
- [ ] **GitHub Auth** — Token-based authentication (env var, config file, or interactive prompt)
- [ ] **Release Management** — Automatically create GitHub releases for backup snapshots
- [ ] **Backup Sync** — Sync local backups to GitHub with progress indication
- [ ] **Restore from GitHub** — Browse and restore backups stored in GitHub releases
- [ ] **Cross-platform** — Works with GitHub CLI (`gh`) or direct API calls

### Phase 5: Backup Quality & Reliability

- [ ] **Backup Integrity Verification** — Verify tar.gz integrity after creation (checksum, test extraction)
- [ ] **Backup Encryption** — Optional AES encryption for backups at rest (encrypt before storing to NAS)
- [ ] **Compression Level Options** — Let users choose gzip compression level (fast, balanced, best)
- [ ] **Backup Retention Policies** — Auto-delete old backups based on rules (keep last N, keep by age, etc.)
- [ ] **Backup Dry-run Mode** — Simulate a backup to show what files would be included
- [ ] **Exclude Patterns** — `.gitignore`-style glob patterns to exclude certain files/directories from backups

### Phase 6: Restore Enhancements

- [ ] **Selective File Restore** — Browse contents of a backup archive and choose individual files to restore instead of the entire backup
- [ ] **Restore Dry-run** — Preview what files would be restored, where they would go, and what conflicts would arise, without making any changes
- [ ] **Restore Progress Bar** — Show real-time progress during extraction (files extracted / total files, bytes processed)
- [ ] **Batch Restore** — Select multiple backups at once and restore them in sequence
- [ ] **Restore to Temporary Directory** — Extract to a temp directory and let the user review before confirming the final location

### Phase 7: Storage & Destinations

- [ ] **Multi-destination Support** — Backup to multiple destinations simultaneously (NAS + GitHub + S3)
- [ ] **S3-Compatible Storage** — Support for AWS S3, MinIO, DigitalOcean Spaces, etc.
- [ ] **SFTP/SCP Remote** — Backup directly to remote servers via SSH
- [ ] **Local Directory** — Simple local directory as backup destination
- [ ] **Destination Health Checks** — Test connectivity and write permissions to all configured destinations

### Phase 8: Scheduling & Automation

- [ ] **TUI Scheduling UI** — Visual date/time picker for backup schedules instead of fixed frequency presets
- [ ] **Custom Cron Expressions** — Full cron expression support for advanced scheduling
- [ ] **Run on System Events** — Trigger backups on system events (login, network mount, USB plug)
- [ ] **Quiet / Headless Mode** — Run backups from cron or systemd timers with logging only, no TUI
- [ ] **Desktop Notifications** — Desktop notification on backup success/failure

### Phase 9: User Experience

- [ ] **Search Backups** — Search through backup filenames, source paths, and timestamps
- [ ] **Backup Statistics Dashboard** — Summary view with total backups, size, success rate, etc.
- [ ] **Inline Log Viewer** — View backup/restore logs directly in the TUI with filtering
- [ ] **Configuration Profiles** — Multiple named profiles for different backup scenarios (work, home, server)
- [ ] **Configuration Import/Export** — Share configs between machines via JSON export/import
- [ ] **Backup Size Analysis** — Visual breakdown of which paths consume the most backup storage
- [ ] **Color Themes** — Light/dark mode toggle and custom color schemes
- [ ] **Help System** — In-app help with keybindings and feature explanations

### Phase 10: Advanced Features

- [ ] **Incremental Backups** — Only backup files that changed since last full backup (significant space savings)
- [ ] **Backup Diff Since Last Backup** — Show what files changed between the current state and the last backup
- [ ] **Pre/Post Backup Hooks** — Run custom scripts before and after backups (e.g., stop a service, dump a DB)
- [ ] **Backup Annotations** — Add custom labels or notes to backups for easier identification
- [ ] **Bandwidth Limiting** — Throttle upload speed for remote backups
- [ ] **Email/Webhook Notifications** — Send backup reports via email or webhook
- [ ] **Snapshot Comparison Over Time** — Graph showing backup sizes and counts over time

## Configuration

### Linux Standard

**baky** uses the Linux standard configuration directory (`~/.config/baky/config.json`) across all platforms for consistency and follows XDG Base Directory specification.

### Backup Naming

Backup filenames follow a consistent pattern to enable parsing and sorting:

- Format: `[source_name]_YYYYMMdd_HHmmss.tar.gz`
- Sortable: Oldest to newest alphabetically
- Unique: Timestamp ensures no collisions

## Troubleshooting

### Backup destination not accessible

- Ensure NAS path exists and is writable
- Check network connectivity to NAS
- Verify permissions: `chmod 755 /mnt/nas/backups`

### Restore fails with "parent directory does not exist"

- Create the parent directory before restoring to a custom path
- Ensure the path is absolute (starts with `/`)

### No backups found

- Verify NAS path is configured correctly
- Check that backup files exist in the NAS directory
- Ensure backup files follow the naming convention

## Contributing

This project is in active development. Feel free to submit issues and enhancement requests!

## License

MIT License — feel free to use this project however you like!

## Support

For bugs, questions, or feature requests, please open an issue on GitHub.
