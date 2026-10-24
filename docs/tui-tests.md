# Test Log — Management TUI (magiti-backup-tui)

> **Historical note.** This log belongs to the original TUI of the `magiti-backup-tui` project. Its screens are now part of baky (the *Config backup* and *Service* menus). Two changes compared to what is described here: checking the service status no longer uses `sudo`, and adding or removing a watched folder no longer restarts the service, because the service reloads the configuration by itself. The service names, paths and files below are those of the original version, and the quoted screen texts and log output are reproduced exactly as that Spanish-language version showed them. baky's interface is now in English, and its service keys are `s` (start), `x` (stop), `r` (restart) and `u` (update status).

MAGITI Project - GRID
Component: `magiti-backup-tui` (terminal interface in Go + Bubble Tea)

## Objective

Validate that the terminal interface lets the infrastructure administrator check and manage the automatic backup daemon (`magiti-config-backup`) without having to remember `systemctl` or `git` commands or edit configuration files by hand — covering the four needs identified as priorities: service status, log review, backup history and management of watched folders.

## Test Environment

| Component | Detail |
|---|---|
| Operating system | Ubuntu 24.04 LTS (WSL2 on Windows) |
| Language / Framework | Go 1.23, Bubble Tea, Bubbles, Lip Gloss |
| Dependent service | `magiti-backup.service` (systemd), from the `magiti-config-backup` project |
| Permissions | Specific `NOPASSWD` rules in `/etc/sudoers.d/magiti-backup` for the `systemctl` subcommands used |
| Development editor | Neovim (LazyVim) |

## Test Log

### Test 1 — Service status screen (query)

- **What was tested:** that the TUI correctly reflects whether the `magiti-backup.service` service is active or inactive.
- **Expected result:** the status shown on screen matches the real result of `systemctl status`.
- **Actual result (first attempt):** Failed. The TUI always showed "INACTIVO" (inactive), even with the service running (confirmed manually with `sudo -n systemctl status magiti-backup.service`, which did show `active (running)`).
- **Root cause:** the code ran `sudo -n systemctl is-active magiti-backup.service`, but the file `/etc/sudoers.d/magiti-backup` only allowed the `start`, `stop`, `restart` and `status` subcommands without a password — **`is-active` was not included**. Without authorization, `sudo -n` failed silently and the code interpreted that failure as "service inactive".
- **Fix applied:** `/usr/bin/systemctl is-active magiti-backup.service` was added to the list of commands allowed without a password in `sudoers`.
- **Retest:** the status was checked again from the TUI after the fix.
- **Result after the fix:** Passed. The TUI correctly showed "ACTIVO" (active) in green, matching the real state of the service.
- **Lesson for the project:** every time the code adds a new `systemctl` subcommand, it must be synchronized manually with the `sudoers` file. The complete, final line still needs to be documented in the installation README to avoid this mismatch in future deployments.

### Test 2 — Service status screen (actions: start / stop / restart)

- **What was tested:** that the actions `i` (iniciar, start), `d` (detener, stop) and `r` (reiniciar, restart) run the corresponding `systemctl` subcommand and show the result on screen.
- **Procedure:** the three keys were tested from the status screen, checking the confirmation message in the TUI and comparing with `systemctl status` from an external terminal.
- **Actual result:** Passed. Each action showed the message "Inicio/Detención/Reinicio ejecutado correctamente" (start/stop/restart completed successfully), and the status refreshed automatically after each action.

### Test 3 — Live logs screen (reading and auto-refresh)

- **What was tested:** that the logs screen shows the last lines of the `backup-daemon.log` file, colored by level (INFO/WARN/ERROR), and that it refreshes automatically without user intervention.
- **Procedure:** the logs screen was opened and, from another terminal, a real change was made to a watched file (`echo "..." >> ~/simulacion-configs/vpn/config-test.conf`).
- **Expected result:** the new log line should appear in the TUI within at most 3 seconds (the configured refresh interval), without the user pressing any key.
- **Actual result (first attempt):** Failed. The logs screen displayed correctly when opened, but it was not possible to leave it by pressing `esc` or `q` — the interface was "frozen" on that view.
- **Root cause:** in the model's `Update` function, the `switch` that routes keys (`tea.KeyMsg`) by active view did not include the `vistaLogs` case. The `updateLogs` function already existed and handled `esc`/`q` correctly, but it was never called because the `switch` lacked that branch, so the keys had no effect.
- **Fix applied:** the missing case (`case vistaLogs: return m.updateLogs(msg)`) was added to the key routing `switch`.
- **Retest:** opening the logs screen and leaving it with `esc` was repeated.
- **Result after the fix:** Passed. Entering and leaving worked correctly, and the auto-refresh every 3 seconds showed the event generated from the external terminal on screen without manual intervention.

### Test 4 — Commit history screen

- **What was tested:** that the TUI lists the latest backups made (commits of the `magiti-config-backup` repository), showing date and message legibly.
- **Procedure:** the history screen was opened and the list shown was compared with `git log` run manually on the same repository.
- **Actual result:** Passed. The list matched the real commit history, correctly showing the messages generated automatically by the daemon (e.g. "actualizado nat.conf", "eliminado config-test.conf"). Manual refresh with the `r` key worked correctly.

### Test 5 — Watched folder management: installing a missing dependency

- **What was tested:** building the project after adding the text input component (`textinput`) from the `bubbles` package, needed to capture the name of a new folder to watch.
- **Actual result (first attempt):** Failed. The build failed with the error `could not import github.com/charmbracelet/bubbles/textinput`.
- **Root cause:** the `bubbles/textinput` package was not downloaded in the local environment nor referenced in `go.mod`/`go.sum`, even though `bubbles` was already listed as a general dependency of the project.
- **Fix applied:**
  ```bash
  go get github.com/charmbracelet/bubbles/textinput
  go mod tidy
  ```
- **Result after the fix:** Passed. The build finished without errors and the module was correctly referenced in `go.mod` and `go.sum`.

### Test 6 — Watched folder management: listing the current folders

- **What was tested:** that the screen correctly shows the mapping of watched folders read from the daemon's `config.json`.
- **Actual result:** Passed. `firewall → firewall` and `vpn → vpn` were listed, with working navigation between them using the arrow keys.

### Test 7 — Watched folder management: adding a new folder

- **What was tested:** the complete flow of adding a folder from the TUI, including writing to `config.json`, physically creating the directory, and automatically restarting the service (chosen design option: immediate restart after saving, accepting a brief window of daemon downtime during the restart).
- **Procedure:**
  1. From the TUI: key `a`, the name `dns` was entered, `Enter`.
  2. External check of the configuration file:
     ```bash
     cat ~/proyectos/magiti-config-backup/scripts/config.json
     ```
  3. Check of the service restart:
     ```bash
     sudo -n systemctl status magiti-backup.service
     ```
  4. Functional test: creating a file inside the new watched folder.
     ```bash
     mkdir -p ~/simulacion-configs/dns
     echo "registro de prueba" > ~/simulacion-configs/dns/registro.conf
     ```
- **Actual result:** Passed on all four points:
  - `config.json` included `"dns": "dns"` after saving.
  - The daemon log showed a new start: `Daemon iniciado. Vigilando: ... (carpetas: vpn dns firewall)`.
  - The file `registro.conf` was detected and backed up automatically:
    ```
    [INFO] Commit realizado: actualizado registro.conf -> dns/registro.conf.
    [INFO] Push exitoso para dns/registro.conf.
    ```

### Test 8 — Watched folder management: removing a folder

- **What was tested:** that when a folder is removed from the TUI, the daemon stops watching it after the automatic restart, without deleting the physical files that already exist (design decision: removing only stops the watching, it does not delete data).
- **Procedure:**
  1. From the TUI: `dns` was selected, key `d`, confirmation with `y`.
  2. Check that `dns` no longer appears in `config.json`.
  3. Functional test: creating a new file inside the folder that is no longer watched.
     ```bash
     echo "esto no debería respaldarse" > ~/simulacion-configs/dns/otro.conf
     ```
- **Actual result:** Passed.
  - `config.json` no longer included the `dns` entry.
  - The daemon log showed the restart with the updated mapping: `Daemon iniciado. Vigilando: ... (carpetas: vpn firewall)`.
  - The file `otro.conf` was correctly ignored:
    ```
    [WARN] Carpeta no mapeada, se ignora: dns
    ```

### Test 9 — Watched folder management: validating the name when adding

- **What was tested:** that the text field for adding a folder rejects invalid names (spaces, uppercase letters, slashes, `..` sequences) both while typing and when confirming with Enter, preventing names that could break the 1:1 mapping or create paths outside the watched directory.
- **Procedure:**
  1. Typing a space inside the field was attempted — character blocked, it did not appear in the input.
  2. Typing an uppercase letter (`D`) was attempted — character blocked.
  3. Typing a sequence with slashes (`../../etc`) was attempted — the `/` and `.` characters were blocked individually by the `textinput` validator.
  4. The field was left empty and Enter was pressed — the message "el nombre no puede estar vacío" (the name cannot be empty) was shown without trying to save.
  5. A valid name (`dns2`) was typed and confirmed with Enter.
- **Expected result:** cases 1–4 should prevent saving; case 5 should complete successfully.
- **Actual result:** Passed in all five cases. The valid name `dns2` was saved correctly in `config.json` and the service restarted reflecting the new folder:
  ```
  Daemon iniciado. Vigilando: /home/miguel/simulacion-configs (carpetas: vpn dns2 firewall)
  ```
- **Additional observation (not a bug):** after adding `dns2`, no new commit appeared in the GitHub history until a real file was created inside the folder. This is the expected behavior: the TUI only updates `config.json`, creates the physical directory and restarts the service — it does not run any Git operations itself. The backup (commit/push) remains the sole responsibility of the daemon, which only acts on real file events (`close_write`, `delete`, `moved_to`, etc.); an empty folder generates no `inotifywait` event, so there is nothing to back up until content is added. The complete flow was confirmed by creating a test file inside `dns2/`, which did generate the corresponding commit and push.

## Bugs Found and Fixed During Testing

| # | Bug | Cause | Fix |
|---|---|---|---|
| 1 | The service status always showed "INACTIVO" (inactive) | The `systemctl` subcommand `is-active` was not authorized in `sudoers` | `is-active` was added to the list of `NOPASSWD` commands |
| 2 | The logs screen did not allow going back to the menu | Missing `vistaLogs` case in the key routing `switch` inside `Update` | The missing case was added to call `updateLogs` |
| 3 | Build failure when using `textinput` | The `bubbles/textinput` dependency was not downloaded/registered in `go.mod` | `go get` of the specific package + `go mod tidy` |

## Test Coverage

| Feature | Covered |
|---|---|
| Check the service status | Yes |
| Start / stop / restart the service from the TUI | Yes |
| Log display with auto-refresh | Yes |
| Log coloring by level (INFO/WARN/ERROR) | Yes |
| Commit history (backups made) | Yes |
| List watched folders | Yes |
| Add a watched folder (with automatic restart) | Yes |
| Remove a watched folder (with automatic restart) | Yes |
| Folder name format validation (special characters, spaces) | Yes |
| Scrolling in long lists (logs/history with many entries) | Pending |

## Conclusion

The TUI (`magiti-backup-tui`), in its current version, covers the four needs prioritized by the project team for the operational management of the backup daemon: visibility of the service status, review of activity, traceability of the backups made, and administration of the watched folders without editing configuration files by hand. The tests made it possible to detect and fix three failures — two of configuration/permissions and one of event routing logic — before considering this phase of the project stable. In addition, an input sanitization layer (folder names) was added and validated to prevent invalid configurations or paths outside the watched directory.
