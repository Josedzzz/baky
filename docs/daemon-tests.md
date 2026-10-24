# Test Log — Automatic Backup Daemon

> **Historical note.** This log belongs to the original version of the daemon, written in Bash (`backup-daemon.sh`, project `magiti-config-backup`). That logic now lives in baky, rewritten in Go (`internal/gitbackup`), with the same observable behavior. The scenarios described here are covered by the automated tests of `internal/gitbackup` and `internal/daemon`. The service names, paths and files below are those of the original version, and the quoted log output is reproduced exactly as that Spanish-language version printed it; baky now prints its messages in English (for example `Commit created` and `Push succeeded`).

MAGITI Project - GRID
Component: `scripts/backup-daemon.sh`

## Objective

Validate that the automatic backup daemon meets the requirements identified in the interview with the infrastructure administrator (sections 3.3 and 4.1 of the Interview Record): real-time detection of changes to critical configuration files (Firewall, VPN) and automatic versioned backup to a Git repository, without depending on manual intervention.

## Test Environment

| Component | Detail |
|---|---|
| Operating system | Ubuntu 24.04 LTS (WSL2 on Windows) |
| Process manager | systemd (explicitly enabled in WSL2) |
| Tools | git, inotify-tools (inotifywait), jq |
| Remote repository | GitHub (private) — testing phase |
| Simulated directory | `~/simulacion-configs/` (stands in for the real production paths of the GRID server) |

> Note: the configuration files used in the tests are fictitious. No real Firewall or VPN configuration from the GRID infrastructure was used during this phase.

## Test Log

### Test 1 — Automatic backup when a file is created/modified (initial MVP)

- **What was tested:** that the daemon detects a change to a file inside a watched folder and backs it up automatically to the Git repository.
- **Procedure:**
  ```bash
  echo "regla de prueba $(date)" >> ~/simulacion-configs/firewall/reglas.conf
  ```
- **Expected result:** the daemon detects the `close_write` event, copies the file to the repository, runs `git commit` and `git push`.
- **Actual result:** ✅ Passed. Log:
  ```
  [INFO] Cambio detectado: /home/miguel/simulacion-configs/firewall/reglas.conf
  [INFO] Commit realizado para firewall/reglas.conf.
  [INFO] Push exitoso para firewall/reglas.conf.
  ```
  Also confirmed on GitHub (commit `auto-backup: reglas.conf`).

### Test 2 — Daemon persistence when the session ends

- **What was tested:** whether the daemon survives closing the terminal or session where it was started.
- **Procedure:** the daemon was started manually in a terminal, then that terminal/session was closed.
- **Expected result (before the fix):** the daemon should keep running.
- **Actual result:** ❌ Failed. Checking with `ps -ef`, the process no longer existed. The daemon depended on the terminal session where it was launched.
- **Root cause:** the script ran directly in the foreground, without a process manager to keep it alive independently of the user's session.
- **Fix applied:** a `systemd` service (`magiti-backup.service`) was implemented with `Restart=on-failure`, running as a system user, with automatic start (`enable`).
- **Retest:** `wsl --shutdown` was run (equivalent to a full system restart) and the service status was checked after restarting.
- **Result after the fix:** ✅ Passed.
  ```
  Active: active (running) since Fri 2026-07-24 20:57:07 -05; 3min 36s ago
  ```
  The service started automatically without manual intervention.

### Test 3 — Externalized configuration (`config.json`)

- **What was tested:** that the daemon correctly reads the watched directory and the folder mapping from an external configuration file (`config.json`), instead of having these values hard-coded in the script.
- **Procedure:**
  ```bash
  jq '.' scripts/config.json
  jq -r '.carpetas | to_entries[] | "\(.key)=\(.value)"' scripts/config.json
  ```
- **Expected result:** the JSON is parsed correctly and the daemon starts using those values.
- **Actual result:** ✅ Passed.
  ```
  firewall=firewall
  vpn=vpn
  ```
  The daemon's startup log confirmed: `Vigilando: /home/miguel/simulacion-configs (carpetas: vpn firewall)`.
- **Rationale:** this externalization is a prerequisite for a future interface (TUI) to change what is watched without editing the script's code directly.

### Test 4 — Handling file deletion

- **What was tested:** that when a file inside a watched folder is deleted, the daemon reflects that deletion in the Git repository (instead of leaving an orphaned file).
- **Procedure:**
  ```bash
  rm ~/simulacion-configs/vpn/config-test.conf
  ```
- **Expected result:** the daemon detects the `DELETE` event, removes the corresponding file in the repo and commits the deletion.
- **Actual result:** ✅ Passed.
  ```
  [INFO] Evento detectado: DELETE en /home/miguel/simulacion-configs/vpn/config-test.conf
  [INFO] Commit realizado: eliminado config-test.conf -> vpn/config-test.conf.
  [INFO] Push exitoso para vpn/config-test.conf.
  ```

### Test 5 — Handling file renames

- **What was tested:** that renaming/moving a file (`mv`) is correctly reflected as a deletion of the old name and a creation of the new one.
- **Procedure:**
  ```bash
  echo "contenido" > ~/simulacion-configs/firewall/temporal.conf
  mv ~/simulacion-configs/firewall/temporal.conf ~/simulacion-configs/firewall/renombrado.conf
  ```
- **Expected result:** two events (`MOVED_FROM` and `MOVED_TO`), resulting in a commit deleting the old file and a commit creating the new one.
- **Actual result:** ✅ Passed.
  ```
  [INFO] Evento detectado: MOVED_FROM en .../firewall/temporal.conf
  [INFO] El archivo eliminado no existía en el repo, nada que hacer: firewall/temporal.conf
  [INFO] Evento detectado: MOVED_TO en .../firewall/renombrado.conf
  [INFO] Commit realizado: actualizado renombrado.conf -> firewall/renombrado.conf.
  ```
  Note: the message "no existía en el repo" (did not exist in the repo) is correct in this particular case, since `temporal.conf` was never backed up before the rename (bug detected and fixed in Test 6).

### Test 6 — Handling nested subfolders inside a watched folder

- **What was tested:** that files inside subfolders (e.g. `firewall/reglas_extra/nat.conf`) are backed up preserving their folder structure, not just the file name.
- **Procedure:**
  ```bash
  mkdir -p ~/simulacion-configs/firewall/reglas_extra
  echo "nat rule" > ~/simulacion-configs/firewall/reglas_extra/nat.conf
  ```
- **Expected result (first attempt):** the file should be backed up to `firewall/reglas_extra/nat.conf` inside the repo.
- **Actual result (first attempt):** ❌ Failed silently.
  ```
  [WARN] Evento no manejado explícitamente: CLOSE_WRITE,CLOSE
  ```
- **Root cause:** the script's `case` expected the literal value `CLOSE_WRITE:CLOSE_WRITE`, but `inotifywait` reports the event as `CLOSE_WRITE,CLOSE` (separated by a comma, not a colon). This affected **all** file creations/modifications, not just those in subfolders — the bug was detected thanks to this test.
- **Fix applied:** the `case` match was changed from an exact string to a pattern (`*CLOSE_WRITE*`), more robust to variations in the exact event format.
- **Retest:**
  ```bash
  echo "nat rule v2" > ~/simulacion-configs/firewall/reglas_extra/nat.conf
  ```
- **Result after the fix:** ✅ Passed.
  ```
  [INFO] Commit realizado: actualizado nat.conf -> firewall/reglas_extra/nat.conf.
  [INFO] Push exitoso para firewall/reglas_extra/nat.conf.
  ```
  Also verified with `ls -la`, confirming that the `reglas_extra/` subfolder was correctly preserved inside `firewall/` in the repository.

## Bugs Found and Fixed During Testing

| # | Bug | Cause | Fix |
|---|---|---|---|
| 1 | Daemon does not survive the session ending | Running in the foreground without a process manager | systemd service with `Restart=on-failure` |
| 2 | Creation/modification events were not processed (`CLOSE_WRITE,CLOSE`) | Exact string match in `case`, which did not match the real `inotifywait` format | Switched to a pattern match (`*CLOSE_WRITE*`) |

## Test Coverage

| Scenario | Covered |
|---|---|
| File creation/modification | ✅ |
| File deletion | ✅ |
| File rename/move | ✅ |
| Nested subfolders | ✅ |
| Persistence across a system restart | ✅ |
| External configuration (config.json) | ✅ |
| Syntax validation before commit | ⏸️ Pending (out of scope for v1) |
| Encryption at rest of the repository | ⏸️ Pending (out of scope for v1) |

## Conclusion

The automatic backup daemon (`backup-daemon.sh`), in its current version (v1), meets the measurable success criteria defined by the interviewee in section 4.2 of the Interview Record: continuous visibility of the state of the watched files, automatic backup without manual intervention, and full traceability of changes through the Git commit history. The tests made it possible to identify and fix two failures before moving on to the next phase of the project (the management TUI).
