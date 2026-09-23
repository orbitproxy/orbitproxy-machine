# OrbitProxy Machine

Thin CLI over [`orbitproxy-go`](https://github.com/orbitproxy/orbitproxy-go): `run` calls `orbitproxy.Start`; control-plane HTTP is only used for one-time `authtoken` registration.

## Build

`VERSION` is the product semver. `make` and `deploy/machine/publish.sh` both inject it as `buildmeta.SoftVersion`. Bump with `scripts/bump-version.sh 1.2.3` before a release.

```bash
# Production (TLS + cert verification to conn_addr gateway)
# Release binary name is `orbitproxy` (repo remains orbitproxy-machine).
make build CONTROL_PLANE_URL=https://cp.app.orbitproxy.cloud TLS_VERIFY=on

# Local dev (plain TCP to edge machineListener)
make build-local
```

`buildmeta.TLSVerify` is link-time only (`on` | `off`), not configurable in `orbitproxy.yml`. A bare `go build` without these ldflags has an empty `SoftVersion` and will not register.

## Usage

```bash
# Register machine identity (CLI-owned; not in the SDK). Do not use sudo.
orbitproxy authtoken <token>

# Foreground run (stops when the terminal closes). Do not use sudo.
# A TTY shows a status card; file logs stay greppable slog text.
orbitproxy run
# Force slog on stdout even in a terminal:
orbitproxy run --log=legacy
# If multiple machines exist under the root:
orbitproxy run --machine-key ck_abc
# Read local machine configs without starting anything:
orbitproxy status

# Remove this machine's OrbitProxy install: service, processes, and ~/.orbitproxy
# (including bin). If a service is installed, run with sudo.
orbitproxy reset

# OS service: starts on boot. sudo is only used to register the task;
# the process still runs as your user and reads ~/.orbitproxy.
sudo orbitproxy service install
sudo orbitproxy service status
sudo orbitproxy service stop
sudo orbitproxy service start
sudo orbitproxy service uninstall
```

If `sudo orbitproxy` cannot find the binary (common when it lives in `~/.local/bin`), pass the absolute path:

```bash
sudo "$(command -v orbitproxy)" service install
sudo "$(command -v orbitproxy)" reset
```

Data root defaults to `~/.orbitproxy` on all OS (override with `--workdir`). Per machine:

```text
<root>/<machineKey>/orbitproxy.yml
<root>/<machineKey>/logs/orbitproxy.log
<root>/<machineKey>/orbitproxy.lock
```

`run` and the OS service share that lock so they cannot both stay up for the same machine. Config is never copied to `/etc`.
