# linko

> Turn any local port into a public HTTPS URL — on your own domain, through Cloudflare Tunnel.

```console
$ linko 3000

✓ Route published (x92ka.demo.example.com -> http://localhost:3000)
✓ Tunnel connected

  Public URL  https://x92ka.demo.example.com
  Forwards    http://localhost:3000

  Press Ctrl+C to stop
```

No port forwarding. No reverse proxy. No SSL certificates. No manual DNS records.

## Why

`ngrok` is great until you want your own domain, no session limits and no interstitial
warning page. `linko` gives you that experience on top of your own Cloudflare account:
one command in, one public URL out.

- **Your domain** — every URL lives under a base subdomain you choose.
- **Automatic HTTPS** — Cloudflare handles the certificates.
- **One binary** — written in Go. No Node, no Python, no dependencies.
- **One tunnel, many projects** — each project is a hostname routed to a different local port.
- **Self-cleaning** — random hostnames are removed when you quit; named ones persist.

## Requirements

- A free Cloudflare account
- A domain whose nameservers point at Cloudflare

`cloudflared` is downloaded automatically on first use.

## Install

**macOS / Linux**

```bash
curl -fsSL https://raw.githubusercontent.com/ibtkrgo/linko/main/install.sh | bash
```

**Go**

```bash
go install github.com/ibtkrgo/linko@latest
```

**From source**

```bash
git clone https://github.com/ibtkrgo/linko.git
cd linko
make deps && make verify && make build
```

**Windows** — download the `.zip` from [Releases](https://github.com/ibtkrgo/linko/releases)
and put `linko.exe` somewhere on your `PATH`.

## Setup

Create a Cloudflare API token at
[dash.cloudflare.com/profile/api-tokens](https://dash.cloudflare.com/profile/api-tokens)
with exactly two permissions:

| Type    | Resource          | Permission |
| ------- | ----------------- | ---------- |
| Zone    | DNS               | Edit       |
| Account | Cloudflare Tunnel | Edit       |

Then run the wizard:

```bash
linko init
```

It verifies the token, finds your DNS zone, creates the tunnel and saves everything to
`~/.linko/config.json` (mode `0600`).

Non-interactive:

```bash
linko init --yes \
  --token "$CF_API_TOKEN" \
  --domain example.com \
  --base demo.example.com
```

## Usage

```bash
linko 3000                 # random subdomain, removed when you quit
linko 3000 --name crm      # https://crm.demo.example.com, persistent
linko 3000 --keep          # keep a random hostname too
linko 8080 -n api          # a second project, same tunnel

linko list                 # what is published
linko status               # tunnel connection state
linko remove crm           # delete a hostname (route + DNS record)
linko doctor               # check the whole setup
```

Target formats: `3000`, `:3000`, `localhost:3000`, `127.0.0.1:8080`,
`https://localhost:8443`, `tcp://localhost:22`.

### Commands

| Command                | What it does                            |
| ---------------------- | --------------------------------------- |
| `linko init`           | One-time setup                          |
| `linko <port>`         | Shorthand for `linko start <port>`      |
| `linko start <port>`   | Publish a local service                 |
| `linko list`           | Show published hostnames (`--remote`)   |
| `linko status`         | Tunnel state and active connections     |
| `linko remove <name>`  | Delete a hostname (`--all`, `--yes`)    |
| `linko doctor`         | Diagnose the setup, exit 1 on failure   |

### Environment variables

| Variable           | Effect                                            |
| ------------------ | ------------------------------------------------- |
| `LINKO_API_TOKEN`  | Overrides the stored Cloudflare token             |
| `LINKO_HOME`       | Config directory (default `~/.linko`)             |
| `NO_COLOR`         | Disable coloured output                           |

## How it works

`linko` creates a single Cloudflare Zero Trust tunnel in your account. Each project
becomes a hostname inside that tunnel, routed to a different local port.

```
  crm.demo.example.com  ──┐
                          │                     ┌──▶ localhost:3000
  api.demo.example.com  ──┼──▶ Cloudflare Edge ─┼──▶ localhost:8080
                          │      (one tunnel)   │
  test.demo.example.com ──┘                     └──▶ localhost:5173
                                   ▲
                                   │  outbound connection only
                             cloudflared on your machine
```

Running `linko 3000`:

1. creates a proxied `CNAME` pointing at `<tunnel-id>.cfargotunnel.com`
2. adds an ingress rule mapping the hostname to `http://localhost:3000`
3. starts `cloudflared`, which opens an **outbound** connection to Cloudflare

Nothing listens for inbound connections on your machine, so no firewall or router
changes are needed.

## Documentation

A full guide (Arabic, RTL) is in [`docs/index.html`](docs/index.html) — open it in a
browser, or publish it with GitHub Pages.

## Development

```bash
make deps      # go mod tidy
make test      # run the test suite
make race      # tests with the race detector
make cover     # coverage report
make verify    # gofmt + go vet + tests
make build     # ./linko
make release   # cross-compiled archives in dist/
```

### Layout

```
main.go                        entry point
cmd/                           Cobra commands (init, start, list, remove, status, doctor)
cloudflare/                    Cloudflare API client — net/http only
  api.go                         auth, accounts, zones
  tunnel.go                      tunnels and ingress rules
  dns.go                         CNAME records
config/config.go               ~/.linko/config.json
internal/cloudflared/          locate, download and run cloudflared
internal/naming/               subdomain generation and validation
internal/target/               "3000" -> "http://localhost:3000"
internal/ui/                   coloured output and prompts
```

### Design notes

- The tunnel is **remotely managed** (`config_src: cloudflare`) — no local YAML.
- The tunnel token is passed to `cloudflared` via `TUNNEL_TOKEN`, never on the command
  line, so it does not show up in `ps`.
- The ingress list is normalised before every write: exactly one catch-all rule, last.
- The config file is written atomically (temp file + rename) with mode `0600`.
- One dependency: [Cobra](https://github.com/spf13/cobra). Everything else is stdlib.

## Security

`linko` never deletes a DNS record that does not point at a Cloudflare tunnel, so it
cannot clobber your production records by accident.

Published URLs are **public**. Anyone with the link can reach your local service — do
not publish anything sensitive. For access control, put Cloudflare Access in front of
the hostname from the Zero Trust dashboard.

## License

[MIT](LICENSE)
