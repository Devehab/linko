<div align="center">

# linko

**Turn any local port into a public HTTPS URL — on your own domain.**

[![CI](https://github.com/Devehab/linko/actions/workflows/ci.yml/badge.svg)](https://github.com/Devehab/linko/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Devehab/linko?color=f6821f)](https://github.com/Devehab/linko/releases)
[![Go](https://img.shields.io/badge/go-1.23%2B-00ADD8)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[Install](#install) · [Quick start](#quick-start) · [Commands](#command-reference) · [Cookbook](#cookbook) · [Troubleshooting](#troubleshooting) · [العربية](GUIDE.md)

</div>

---

```console
$ linko 3000

✓ DNS record created (x92ka.example.com)
✓ Route published (x92ka.example.com -> http://localhost:3000)
✓ Tunnel connected

  Public URL  https://x92ka.example.com
  Forwards    http://localhost:3000
  Tunnel      example-linko-tunnel

  Press Ctrl+C to stop
```

No port forwarding. No reverse proxy. No SSL certificates. No manual DNS records.

## Why

`ngrok` is excellent until you want your own domain, no session limits and no
interstitial warning page. `linko` gives you that on top of your own Cloudflare
account — one command in, one public URL out.

|  | |
| --- | --- |
| **Your domain** | Every URL lives under a domain you control. |
| **Stable URLs** | A port keeps its URL. Restarting your app does not change the link you shared. |
| **Automatic HTTPS** | Cloudflare issues and renews the certificate. |
| **One binary** | Written in Go. No Node, no Python, no runtime. |
| **One tunnel, many projects** | Each project is a hostname routed to a different local port. |
| **Runs anywhere** | Foreground, background, or started automatically at every boot. |
| **Self-cleaning** | `--temp` URLs and their DNS records disappear when you quit. |

## Requirements

| | Required | Notes |
| --- | --- | --- |
| Cloudflare account | ✅ | Free plan is enough — [sign up](https://dash.cloudflare.com/sign-up) |
| A domain you own | ✅ | Bought anywhere: Namecheap, GoDaddy, Google, anywhere |
| **Its DNS managed by Cloudflare** | ✅ | See below — free, and you keep your registrar |
| `cloudflared` | ❌ | Downloaded automatically on first use |
| Go | ❌ | Only if you build from source |
| Node, Python, Docker | ❌ | Not used at all |

> [!IMPORTANT]
> **Owning a domain is not enough — its nameservers must point at Cloudflare.**
> `linko` creates DNS records through the Cloudflare API, so Cloudflare has to
> be the authoritative DNS for that domain. This is free and takes about ten
> minutes, most of it waiting.

<details>
<summary><b>Moving a domain's DNS to Cloudflare (free, keeps your registrar)</b></summary>

You are **not** transferring ownership. The domain stays registered where you
bought it, you keep paying the same registrar, and you can undo this any time.
Only the nameservers change.

1. **Create a free account** at
   [dash.cloudflare.com/sign-up](https://dash.cloudflare.com/sign-up).

2. **Add your domain.** Dashboard → **Add a site** → type `example.com` →
   choose the **Free** plan.

3. **Let it scan.** Cloudflare copies your existing DNS records automatically.
   Check the list against your current provider — especially `MX` records if
   you use email on that domain, or your mail will stop.

4. **Copy the two nameservers** Cloudflare shows you, for example:

   ```
   dana.ns.cloudflare.com
   rick.ns.cloudflare.com
   ```

5. **Paste them at your registrar.** Sign in where you bought the domain, find
   *Nameservers* / *DNS settings* / *Custom DNS*, remove what is there, and put
   Cloudflare's two in. Registrar-specific guides:
   [Namecheap](https://www.namecheap.com/support/knowledgebase/article.aspx/767/10/how-to-change-dns-for-a-domain/) ·
   [GoDaddy](https://www.godaddy.com/help/change-nameservers-for-my-domain-664) ·
   [Google Domains / Squarespace](https://support.squarespace.com/hc/en-us/articles/4404183898125) ·
   [Hostinger](https://support.hostinger.com/en/articles/1583227-how-to-change-nameservers-at-hostinger)

6. **Wait.** Usually under an hour, occasionally up to 24. Cloudflare emails
   you when the domain becomes **Active**.

Check it yourself at any time:

```bash
dig NS example.com +short          # should list *.ns.cloudflare.com
```

Once the domain shows **Active** in the dashboard, run `linko init`.

**Do not have a domain yet?** Cloudflare
[sells them at cost](https://developers.cloudflare.com/registrar/) with DNS
already set up — nothing to move.

</details>

## Install

### macOS and Linux

```bash
curl -fsSL https://raw.githubusercontent.com/Devehab/linko/main/install.sh | bash
```

The script detects your platform, downloads the matching binary, and puts it in a
directory already on your `PATH`. If it has to fall back to `~/.local/bin`, it
adds that line to your shell profile for you.

```bash
linko --version
```

<details>
<summary><b>Other ways to install</b></summary>

**Homebrew-managed Go, or any Go 1.23+**

```bash
go install github.com/Devehab/linko@latest
```

**From source**

```bash
git clone https://github.com/Devehab/linko.git
cd linko
make deps && make verify && make build
```

**Windows** — download the `.zip` from [Releases](https://github.com/Devehab/linko/releases),
unzip it, and put `linko.exe` somewhere on your `PATH`.

**Pin a version or a directory**

```bash
LINKO_VERSION=v0.2.0 LINKO_INSTALL="$HOME/bin" \
  bash -c "$(curl -fsSL https://raw.githubusercontent.com/Devehab/linko/main/install.sh)"
```

</details>

## Quick start

### 1 · Create a Cloudflare API token

Go to [dash.cloudflare.com/profile/api-tokens](https://dash.cloudflare.com/profile/api-tokens)
→ **Create Token** → **Create Custom Token**.

Add **both** permission rows to the **same** token — press **+ Add more** for the second:

| # | Type | Resource | Permission |
| --- | --- | --- | --- |
| 1 | `Zone` | `DNS` | **Edit** |
| 2 | `Account` | `Cloudflare Tunnel` | **Edit** |

Then set the resources:

- **Account Resources** → your account
- **Zone Resources** → `Include` → `Specific zone` → your domain

> [!WARNING]
> Leaving **Zone Resources** empty produces a token that authenticates
> successfully but sees no domains at all, and `linko init` fails with
> `no zone named …`. This is the single most common setup mistake.

Not sure? `linko docs` prints these steps in your terminal.

### 2 · Connect linko to your account

```bash
linko init
```

```console
Cloudflare credentials
Cloudflare API token: ················
✓ Cloudflare connected

Domain
Domain: [example.com]
✓ DNS zone found (example.com)
Base subdomain: [example.com]
✓ URLs will look like https://abc12.example.com

Tunnel
Tunnel name: [example-linko-tunnel]
✓ Tunnel created
✓ Configuration saved to ~/.linko/config.json
✓ cloudflared installed

You're ready.
```

> [!IMPORTANT]
> When asked for **Base subdomain**, answer with your bare domain
> (`example.com`), not `demo.example.com`. See
> [the one-level rule](#the-one-level-rule).

### 3 · Publish something

```bash
npm run dev          # your app, on :3000
linko 3000           # in another terminal
```

## The core idea: a port keeps its URL

This is the behaviour that makes `linko` pleasant to live with.

```bash
linko 3000     # first run  → https://x92ka.example.com
# Ctrl+C
linko 3000     # next run   → https://x92ka.example.com  (the same one)
```

A link you already shared keeps working across restarts, and your DNS does not
fill up with dead records. When you *do* want to change it:

```bash
linko 3000 --new          # mint a fresh random URL, retire the old one
linko 3000 --name crm     # choose the name yourself
linko 3000 --temp         # throwaway: deleted the moment you quit
```

## Command reference

### Global

| Flag | Effect |
| --- | --- |
| `--no-color` | Disable coloured output |
| `--version` | Print the version |
| `-h`, `--help` | Help for any command |

### `linko init`

One-time setup: verify the token, find the DNS zone, create the tunnel, save
everything to `~/.linko/config.json` with mode `0600`.

| Flag | Effect |
| --- | --- |
| `--token <t>` | Cloudflare API token (or set `LINKO_API_TOKEN`) |
| `--domain <d>` | Domain managed by Cloudflare, e.g. `example.com` |
| `--base <b>` | Base subdomain for generated URLs |
| `--tunnel <n>` | Tunnel name (default `<domain>-linko-tunnel`) |
| `--force` | Overwrite an existing configuration |
| `-y`, `--yes` | Non-interactive: fail instead of prompting |
| `--skip-download` | Do not download `cloudflared` now |

```bash
# fully non-interactive, for CI or a fresh machine
export LINKO_API_TOKEN='cfut_…'
linko init --yes --domain example.com --base example.com
```

### `linko <port>` · `linko start <port>`

Publish a local service. `linko 3000` is shorthand for `linko start 3000`.

| Flag | Effect |
| --- | --- |
| `-n`, `--name <n>` | Subdomain to use (default: reuse this port's URL) |
| `-r`, `--new` | Mint a new random subdomain, retiring this port's current one |
| `--temp` | Delete the hostname when the tunnel stops |
| `-d`, `--detach` | Run in the background and return to the prompt |
| `-o`, `--open` | Open the URL in your browser once it answers |
| `--replace` | Replace the hostname if it already points somewhere else |
| `-y`, `--yes` | Do not ask questions |
| `-v`, `--verbose` | Stream `cloudflared` logs |
| `--loglevel <l>` | `debug` · `info` · `warn` · `error` · `fatal` |

**Accepted targets**

| You type | It means |
| --- | --- |
| `linko 3000` | `http://localhost:3000` |
| `linko :3000` | `http://localhost:3000` |
| `linko 127.0.0.1:8080` | a specific address |
| `linko https://localhost:8443` | an origin that speaks HTTPS |
| `linko tcp://localhost:22` | a raw TCP service |

### `linko list`

Show what you have published.

```console
$ linko list

NAME    URL                        TARGET                  KIND
api     https://api.example.com -> http://localhost:8080   persistent
web     https://web.example.com -> http://localhost:3000   persistent

· 2 route(s) · tunnel example-linko-tunnel · ~/.linko/config.json
```

| Flag | Effect |
| --- | --- |
| `--remote` | Read the live routes from Cloudflare instead of the local file |

### `linko ps` / `linko stop`

Background tunnels.

```console
$ linko ps

NAME   URL                        TARGET                  PROCESS
web    https://web.example.com -> http://localhost:3000   pid 41288
```

```bash
linko stop web       # stop one
linko stop --all     # stop everything
```

### `linko status`

Tunnel health, live edge connections, published routes, and anything running in
the background.

```console
$ linko status

Cloudflare Tunnel
  Name        example-linko-tunnel
  ID          8f3c1a92-…
  Account     Acme
  Domain      example.com
  Status      connected (4 connections via AMS, FRA)

Routes
  · https://web.example.com -> http://localhost:3000
```

### `linko remove <name…>`

Delete a hostname: the tunnel route, the DNS record, and the local entry.

| Flag | Effect |
| --- | --- |
| `--all` | Remove every route linko knows about |
| `-y`, `--yes` | Skip the confirmation |

> [!NOTE]
> `linko` never deletes a DNS record that does not point at a Cloudflare
> tunnel, so it cannot clobber your production records by accident.

### `linko service`

Keep a tunnel running across reboots — a `launchd` agent on macOS, a `systemd`
user unit on Linux, both with automatic restart.

```bash
linko service install 3000 --name crm
linko service list
linko service uninstall crm
```

### `linko doctor`

Eight checks in order, from `cloudflared` being present to live edge
connections. Exits non-zero on failure, so it works inside scripts.

| Flag | Effect |
| --- | --- |
| `--fix` | Repair a dead token, a deleted tunnel, missing DNS records and routes |
| `-y`, `--yes` | Repair without asking |

```console
$ linko doctor

✓ cloudflared installed (2025.2.0)
✓ config valid (~/.linko/config.json)
✓ API token valid
✓ DNS zone reachable (example.com)
✓ tunnel exists (example-linko-tunnel)
✓ tunnel configuration readable (2 routes)
✓ DNS configured for 2 routes
✓ connection active (4 edge connections)

Everything looks good.
```

### `linko docs`

The setup guide in your terminal. `--open` launches the full page in a browser.

## Cookbook

**Show a client your work in progress**

```bash
linko 3000 --name preview --open
```

**Two services at once, one tunnel**

```bash
linko 3000 -n web   # terminal 1
linko 8080 -n api   # terminal 2
```

**A staging box that must survive reboots**

```bash
linko service install 8080 --name staging
linko service list
```

**A webhook endpoint you can leave running all week**

```bash
linko 4000 -n hooks -d
linko ps
```

**A one-off link that cleans up after itself**

```bash
linko 5173 --temp
```

**Your app only listens on the loopback interface**

```bash
linko 127.0.0.1:3000
```

**Rotate a URL you accidentally shared too widely**

```bash
linko 3000 --new
```

## The one-level rule

Cloudflare's free Universal SSL covers exactly two names:

```
example.com        the apex
*.example.com      one level
```

A wildcard in a certificate does **not** cross a dot:

| URL | Works? |
| --- | --- |
| `demo.example.com` | ✅ matches `*.example.com` |
| `demo.app.example.com` | ❌ two levels — no free certificate covers it |

> [!CAUTION]
> This is the hardest failure to diagnose in the whole system. DNS is correct,
> the tunnel is connected, and `linko doctor` is entirely green — yet the
> browser refuses with `ERR_SSL_VERSION_OR_CIPHER_MISMATCH`, because the
> rejection happens before any request reaches your tunnel. `linko init` warns
> you about it up front.

Need two levels? That requires Cloudflare
[Advanced Certificate Manager](https://developers.cloudflare.com/ssl/edge-certificates/advanced-certificate-manager/)
($10/month per zone) and then **Total TLS**. The free alternatives — subdomain
zones and partial CNAME setups — are limited to Enterprise and Business plans.

**Simplest free answer:** put the distinction in the name itself —
`myapp-dev.example.com` rather than `myapp.dev.example.com`.

## Configuration

Everything lives in `~/.linko/`:

```
~/.linko/
├── config.json      credentials and routes  (mode 0600)
├── bin/cloudflared  private copy, downloaded on first use
├── run/<name>.pid   one per background tunnel
└── logs/<name>.log  one per background tunnel
```

### Environment variables

| Variable | Effect |
| --- | --- |
| `LINKO_API_TOKEN` | Overrides the stored Cloudflare token |
| `LINKO_HOME` | Configuration directory (default `~/.linko`) |
| `NO_COLOR` | Disable coloured output |

## How it works

`linko` creates a single Cloudflare Zero Trust tunnel in your account. Each
project becomes a hostname inside that tunnel, routed to a different local port.

```
  crm.example.com   ──┐
                      │                     ┌──▶ localhost:3000
  api.example.com   ──┼──▶ Cloudflare Edge ─┼──▶ localhost:8080
                      │      (one tunnel)   │
  test.example.com  ──┘                     └──▶ localhost:5173
                                ▲
                                │  outbound connection only
                          cloudflared on your machine
```

Running `linko 3000`:

1. creates a proxied `CNAME` pointing at `<tunnel-id>.cfargotunnel.com`
2. adds an ingress rule mapping the hostname to `http://localhost:3000`
3. starts `cloudflared`, which opens an **outbound** connection to Cloudflare

Nothing on your machine listens for inbound connections, which is why no
firewall or router change is ever needed.

## Troubleshooting

| Message | Cause | Fix |
| --- | --- | --- |
| `no zone named "example.com"` | Token is missing the `Zone` permission, or its Zone Resources are empty | Add `Zone → DNS → Edit` and include your domain. `linko` lists the zones the token *can* see. |
| `Cloudflare refused to create the DNS record` `code 10000` | DNS permission is `Read`, not `Edit` | Finding the zone only needs read access, so setup passes and the first write fails. Switch it to **Edit**. |
| `Could not create the tunnel` `code 10000` | Token covers DNS but not tunnels | Add `Account → Cloudflare Tunnel → Edit` to the **same** token. |
| `ERR_SSL_VERSION_OR_CIPHER_MISMATCH` | Hostname is two levels deep | `linko init --force --base example.com`, or buy ACM. |
| `no API token: pass --token or set LINKO_API_TOKEN` | `linko init --yes` with no token | `export LINKO_API_TOKEN='…'` or pass `--token`. |
| `command not found: linko` | Install directory is not on `PATH` | Reopen the terminal, or run `exec $SHELL`. |
| `HTTP 502` · `Error 1033` | Tunnel is up but nothing is listening | Check `curl http://localhost:3000`. If your app binds the loopback only, use `linko 127.0.0.1:3000`. |

### When things change behind your back

Tokens expire. People delete tunnels from the Zero Trust dashboard. DNS records
get removed by hand. `linko` detects all three, explains what happened in plain
terms, and repairs it — it never just prints a Cloudflare error code.

**The token stopped working**

```console
$ linko 3000

✗ Cloudflare is no longer accepting the stored API token.
· It was most likely deleted, edited, or it expired.

  Create a Cloudflare API token
  1. https://dash.cloudflare.com/profile/api-tokens
  2. Add both permission rows (+ Add more):
       Zone     →  DNS               →  Edit
       Account  →  Cloudflare Tunnel →  Edit

New Cloudflare API token: ················
✓ Token updated and saved to ~/.linko/config.json
✓ Tunnel connected
```

The new token is verified against Cloudflare *before* it is saved, so a typo
cannot lock you out of your own configuration.

**The tunnel was deleted**

`linko` recreates it (or adopts one of the same name), fetches a fresh tunnel
token, **re-points every DNS record at the new tunnel** — a new tunnel means a
new CNAME target — and restores every route.

**A DNS record was removed**

Recreated on the next `linko <port>`, or for all of them at once:

```bash
linko doctor --fix
```

`--fix` repairs a dead token, a deleted tunnel, missing DNS records and missing
routes, then re-runs the full check. Add `--yes` to skip the confirmations.

When in doubt:

```bash
linko doctor
```

### Start completely fresh

```bash
linko remove --all --yes
rm -rf ~/.linko
linko init
```

## Security

- **Published URLs are public.** Anyone with the link reaches your local
  service. Do not expose anything sensitive; put
  [Cloudflare Access](https://developers.cloudflare.com/cloudflare-one/policies/access/)
  in front of the hostname if you need authentication.
- **The token lives in `~/.linko/config.json`** with mode `0600`. Never commit
  it. For teams and CI, use `LINKO_API_TOKEN` instead.
- **The tunnel token is passed to `cloudflared` via the environment**, never on
  the command line, so it does not appear in `ps`.
- **Foreign DNS records are never touched.** If a hostname points at your real
  site, `linko` refuses to overwrite or delete it.

## Development

```bash
make deps      # go mod tidy
make test      # run the test suite
make race      # tests with the race detector
make cover     # coverage report
make verify    # gofmt + go vet + tests
make build     # ./linko
make release   # cross-compiled archives in dist/
make e2e       # full end-to-end run against a real Cloudflare account
```

### Layout

```
main.go                  entry point
cmd/                     Cobra commands
  init.go                  setup wizard and permission diagnostics
  start.go                 publish, sticky names, DNS + ingress
  background.go            detach, pid files, process control
  service.go               launchd / systemd units
  list.go status.go remove.go doctor.go docs.go stop.go
cloudflare/              Cloudflare API client — net/http only
  api.go                   auth, accounts, zones
  tunnel.go                tunnels and ingress rules
  dns.go                   CNAME records
config/config.go         ~/.linko/config.json
internal/cloudflared/    locate, download and run cloudflared
internal/naming/         subdomain generation and validation
internal/target/         "3000" -> "http://localhost:3000"
internal/ui/             coloured output, prompts, OSC 8 hyperlinks
```

### Design notes

- The tunnel is **remotely managed** (`config_src: cloudflare`) — no local YAML
  to keep in sync.
- The ingress list is normalised before every write: exactly one catch-all rule,
  always last.
- The config file is written atomically (temp file + rename) with mode `0600`.
- One dependency: [Cobra](https://github.com/spf13/cobra). Everything else is
  the standard library.
- Every URL is emitted as an OSC 8 hyperlink where the terminal supports it, and
  as plain text where it does not.

## License

[MIT](LICENSE)
