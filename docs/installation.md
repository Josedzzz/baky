# Installation and setup

This guide gets baky running on a Linux server with systemd: the binary, the service that runs the automatic backups, and the config repository.

## 1. Requirements

| Requirement | What for |
|---|---|
| Go 1.24 or later | Building baky (only on the machine where it is built) |
| `git` | Config backup |
| `systemd` | Running the service in the background |
| SSH key with access to the config repository | Pushing the commits to the remote without a password |

## 2. Build and install the binary

```bash
git clone https://github.com/Josedzzz/baky.git
cd baky
go build -o baky ./cmd
sudo install -m 0755 baky /usr/local/bin/baky
```

## 3. Choose the user

The interface and the service must run as **the same user**, because they share the files in `~/.config/baky/`. That user must be able to:

- read the configuration folders that will be watched,
- read the paths that will be backed up and write to the backup destination,
- `git push` to the config repository with their SSH key.

In the following steps that user is called `grid`; replace it with the real one.

## 4. Clone the config repository

Configurations are stored in a dedicated repository that holds no code. At GRID it is `grid-config-data`:

```bash
git clone git@github.com:MiguelFVasquez/grid-config-data.git ~/grid-config-data
```

Use the SSH address (`git@github.com:...`), not the HTTPS one: the service cannot type a password.

Check that the user can push without being asked for anything:

```bash
ssh -T git@github.com
```

If the user has no name or email configured in Git, baky signs the commits as `baky@<hostname>`.

## 5. Install the service

```bash
sed 's/CHANGE_USER/grid/' deploy/baky.service | sudo tee /etc/systemd/system/baky.service > /dev/null
sudo systemctl daemon-reload
sudo systemctl enable --now baky.service
systemctl status baky.service
```

So that the interface can start, stop and restart the service without asking for a password:

```bash
sed 's/CHANGE_USER/grid/' deploy/baky.sudoers | sudo tee /etc/sudoers.d/baky > /dev/null
sudo chmod 0440 /etc/sudoers.d/baky
sudo visudo -cf /etc/sudoers.d/baky
```

Checking the status does not need `sudo`.

## 6. Configure from the interface

Run `baky` as the chosen user:

1. **Config backup → Repository**: `e`, type the path of the clone (`/home/grid/grid-config-data`) and confirm. `t` checks that it is a valid repository.
2. **Config backup → Watched folders**: `a`, type the folder to watch (`/etc/openvpn`) and then the name it will have inside the repository (`vpn`).
3. **File backup → Backup destination**: `e`, type the destination path (`/mnt/nas/backups`). `t` checks that it is accessible.
4. **File backup → Manage paths**: `a` to add a path and `f` to choose its frequency.

The service applies every change by itself. **Service → Live logs** shows what it is doing.

### Validate the configurations (optional)

In **Watched folders**, `c` on a folder sets the command that validates it and the one that applies it. For a Shorewall firewall:

| Step | Command |
|---|---|
| Validation | `sudo -n shorewall check {dir}` |
| Apply | `sudo -n shorewall try {dir}` |

The service user must be able to run those commands with `sudo` without a password. If it is not already an administrator, the minimal rule is:

```
grid ALL=(root) NOPASSWD: /usr/sbin/shorewall check *, /usr/sbin/shorewall try *, /usr/bin/install
```

`install` is needed so that a restore can write to root-owned folders such as `/etc/shorewall`. Keep in mind that whoever can edit baky's configuration decides which commands run with those permissions.

Do not watch folders that contain private keys (for example `/etc/openvpn` with its PKI): baky backs up every file in the folder and pushes them to the repository.

## 7. Check that it works

```bash
echo "# test" | sudo tee -a /etc/openvpn/test.conf
```

Within a few seconds, **Config backup → Change history** should show a commit `auto-backup: updated test.conf (...)`, and the log should show the lines `Commit created` and `Push succeeded`.

## Migrating from magiti-config-backup

If the machine had the Bash daemon (`magiti-backup.service`):

```bash
# 1. Stop and disable the old service
sudo systemctl disable --now magiti-backup.service

# 2. Import the folders it was watching
baky import-magiti ~/proyectos/magiti-config-backup/scripts/config.json

# 3. Set the new repository from the interface (step 6.1) and install the service (step 5)
```

The old repository is not imported: it mixed the daemon's code with the backups. The configurations move to the dedicated repository, and on its first start the service backs them all up.

After checking that baky works, the unit `/etc/systemd/system/magiti-backup.service` and the rule `/etc/sudoers.d/magiti-backup` can be removed.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| The log shows `Push failed` | The service user has no SSH access to the remote, or the clone uses an HTTPS address. The commits are kept and the push is retried. |
| `Config backup disabled: ... is not a valid Git repository` | The configured path is not a Git clone. Fix it in *Repository*. |
| A change shows up as `[validation failed]` | The saved configuration does not pass the validation command. The log and the commit include the error. If the error comes from `sudo`, the passwordless permission is missing. |
| `Watched folder not accessible, skipping` | The folder does not exist or the service user cannot read it. What was already backed up is kept. |
| The interface shows the service as inactive | `systemctl status baky.service` and `journalctl -u baky.service` show the reason. |
| Starting or stopping from the interface fails | The sudoers rule from step 5 is missing. |
| The interface does not show what the service is doing | The interface and the service run as different users and use different files. |
