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

## Identity Headers

In HTTP mode, tsexpose adds these headers to every proxied request:

| Header | Value |
|--------|-------|
| `X-Tailscale-User-Login` | User's login name |
| `X-Tailscale-User-Name` | User's display name |
| `X-Tailscale-User-Profile-Pic` | Profile picture URL |
| `X-Tailscale-Node-Name` | Connecting node's name |

Your backend can read these headers directly — no auth middleware needed.

## License

MIT
