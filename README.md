# tsexpose

Expose a local HTTP port on your Tailscale network using [tsnet](https://pkg.go.dev/tailscale.com/tsnet).

tsexpose creates a node on your tailnet and reverse-proxies traffic to a local service. It automatically injects Tailscale identity headers into forwarded requests, giving your app zero-config authentication.

## Install

```
go install github.com/piotrekwitkowski/tsexpose@latest
```

## Usage

```
tsexpose -local-port 3000
tsexpose -local-port 8080 -ts-port 443 -hostname myapp
tsexpose -local-port 5432 -hostname my-db -ephemeral
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-local-port` | `8000` | Local port to expose (required) |
| `-ts-port` | same as `-local-port` | Port to listen on in the tailnet |
| `-hostname` | `tsexpose` | Tailscale hostname for this node |
| `-auth-key` | | Tailscale auth key (or set `TS_AUTHKEY` env) |
| `-state-dir` | `~/.tsexpose/<hostname>` | Directory to store Tailscale state |
| `-ephemeral` | `false` | Remove node from tailnet on exit |
| `-ts-logs` | `false` | Enable Tailscale logging to log.tailscale.net |
| `-install` | `false` | Install as macOS launchd service (auto-start at login) |
| `-uninstall` | `false` | Remove macOS launchd service |

## Identity Headers

In HTTP mode, tsexpose adds these headers to every proxied request:

| Header | Value |
|--------|-------|
| `X-Tailscale-User-Login` | User's login name |
| `X-Tailscale-User-Name` | User's display name |
| `X-Tailscale-User-Profile-Pic` | Profile picture URL |
| `X-Tailscale-Node-Name` | Connecting node's name |

Your backend can read these headers directly — no auth middleware needed.

## Auto-Start on macOS

tsexpose can install itself as a launchd service that starts automatically at login:

```
tsexpose --install -local-port 3000 -hostname myapp
tsexpose --install -local-port 8080 -ts-port 443 -hostname myapp -auth-key tskey-auth-XXXXX
```

This generates a plist in `~/Library/LaunchAgents/` and loads it immediately.

To stop and remove:

```
tsexpose --uninstall -hostname myapp
```

Logs go to `/tmp/tsexpose-<hostname>.log`.


## License

MIT
