#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  echo "Usage: $0 <editor [args...]> <file>"
  echo "Example: $0 vim -u NORC /etc/shorewall/shorewall.conf"
}

# Need at least an editor and a file
if (($# < 2)); then
  usage
  exit 1
fi

# Last argument is the file; everything before is the editor command
FILE=${@: -1}
EDITOR_CMD=("${@:1:$#-1}")

# Optional: if user typed just 'backup <file>', use $EDITOR env or default to nano
if ((${#EDITOR_CMD[@]} == 0)); then
  if [[ -n "${EDITOR:-}" ]]; then
    IFS=' ' read -r -a EDITOR_CMD <<<"$EDITOR"
  else
    EDITOR_CMD=(nano)
  fi
fi

# Sanity checks
if [[ ! -f "$FILE" ]]; then
  echo "File does not exist: $FILE" >&2
  exit 1
fi

# Pick a checksum tool that exists
if command -v sha256sum >/dev/null 2>&1; then
  SUM=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  SUM=(shasum -a 256)
else
  echo "Need sha256sum or shasum installed." >&2
  exit 1
fi

# Create backups/ directory in current working dir
BACKUP_DIR="./backups"
mkdir -p "$BACKUP_DIR"

# Make a timestamped backup next to the file
TS=$(date -u +"%Y%m%dT%H%M%SZ")
BASENAME=$(basename "$FILE")
BACKUP_PATH="${BACKUP_DIR}/${BASENAME}.${TS}.bak"

# Preserve perms/timestamps
cp -p -- "$FILE" "$BACKUP_PATH"
echo "Backup created: $BACKUP_PATH"

# Record original checksum
ORIGSUM=$("${SUM[@]}" "$FILE" | awk '{print $1}')

# Run the editor; if it fails, keep the backup and propagate the error
if ! "${EDITOR_CMD[@]}" "$FILE"; then
  echo "Editor failed; keeping backup at $BACKUP_PATH" >&2
  exit 2
fi

# Compare after editing
NEWSUM=$("${SUM[@]}" "$FILE" | awk '{print $1}')

if [[ "$ORIGSUM" == "$NEWSUM" ]]; then
  echo "No changes detected, removing backup..."
  rm -f -- "$BACKUP_PATH"
else
  echo "Changes detected, backup kept: $BACKUP_PATH"
fi
