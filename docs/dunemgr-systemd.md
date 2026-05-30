# Running dunemgr as a systemd user service

`dunemgr` (no args) starts the admin UI on `127.0.0.1:8765`. To run it
in the background and on login, use the shipped systemd **user** unit.

## Install

```sh
# 1. put the binary somewhere on the unit's ExecStart path
install -Dm755 bin/dunemgr ~/.local/bin/dunemgr

# 2. install the unit
mkdir -p ~/.config/systemd/user
cp etc/systemd/dunemgr.service.example ~/.config/systemd/user/dunemgr.service

# 3. enable + start
systemctl --user daemon-reload
systemctl --user enable --now dunemgr.service

# 4. (optional) keep it running without an active login session
sudo loginctl enable-linger "$USER"
```

## First run

On first start dunemgr writes a session token to
`~/.config/dunemgr/token`. Read it with:

```sh
cat ~/.config/dunemgr/token
```

Then open <http://127.0.0.1:8765/> and log in with that token.

## Logs

```sh
journalctl --user -u dunemgr -f
```

## Notes

- The unit is **user-scoped** — no root needed except for the optional
  `loginctl enable-linger`.
- The full operator-workstation deploy script (`deploy-dunemgr.sh`) is a
  v2 concern (it only makes sense alongside OIDC + network bind); v1 is
  this manual systemd path.
