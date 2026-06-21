# OrbitLink Machine

Machine-side client that connects to OrbitLink edge over TLS/yamux (frpc-style control + work conn protocol).

Control-plane HTTP is only used for one-time `authtoken` registration.

## Build

```bash
go build -ldflags "-X github.com/orbitlink/orbitlink-machine/internal/buildmeta.ControlPlaneURL=https://api.example.com" \
  -o orbitlink-machine ./cmd
```

## Usage

```bash
# Register machine identity
orbitlink-machine authtoken <token>

# Run daemon (edge connection only)
orbitlink-machine run
```

Config and logs live under the workdir (default OS-specific path, overridable with `--workdir`).
