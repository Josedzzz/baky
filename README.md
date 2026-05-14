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

### Code Structure

- **internal/backup**: Handles creating backups and monitoring file changes
- **internal/config**: Manages configuration persistence and retrieval
- **internal/restore**: Handles backup extraction and restoration with conflict resolution
- **internal/tui**: Implements the terminal user interface with Bubble Tea

## Roadmap

### Completed

- Backup path management
- Automatic backup scheduling and monitoring
- View backups in TUI
- Restore to original or custom location
- Restore history tracking

### In Progress

- Improve restore feedback and progress indication

### Planned

- **Version Comparison**: File-level diff between backup versions
- **Selective Restore**: Choose individual files to restore instead of entire backup
- **Backup Verification**: Validate backup integrity and test extraction
- **Compression Options**: Allow different compression levels for backups
- **Remote Storage**: Support for cloud storage backends (S3, etc.)
- **Scheduling UI**: Visual scheduling configuration instead of frequency presets
- **Restore Dry-run**: Preview what would be restored without making changes
- **Search**: Search through files in backups
- **Backup Size Analysis**: View which paths consume the most backup space
- **Configuration Import/Export**: Share configs between systems

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

MIT License - feel free to use this project however you like!

## Support

For bugs, questions, or feature requests, please open an issue on GitHub.
