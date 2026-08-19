---
name: linko
description: Give a service running on this machine a public HTTPS URL on the user's own domain, through Cloudflare Tunnel. Use when the user wants to preview or share a local dev server, look at their work from a phone or another device, receive webhooks on localhost, or reach a home server that has no public IP.
---

# linko

`linko` turns a local port into a public HTTPS URL on a domain the user owns:

```
linko 3000 -d          →  https://x92ka.example.com  →  http://localhost:3000
```

It drives Cloudflare Tunnel: creates the DNS record, the ingress route and the
certificate, and runs `cloudflared`. The connection is **outbound only**, so it
works behind CGNAT, on a home server, or on any machine with no public IP and no
port forwarding.

## Use this when

- The user wants to see their dev server **from a phone** or another device.
- They want to send a working link to someone — a client, a teammate.
- A third party must reach localhost: a webhook, an OAuth callback, a device on
  the LAN, a QR-code test.
- They are on a **home server / Raspberry Pi / NAS behind CGNAT** and want a
  project reachable on their own domain.

Do **not** use it to expose databases, admin panels, `.env` files, or anything
that is not meant to be world-readable. See [Before you publish](#before-you-publish).

## Before you publish

**A published URL is public.** Anyone who has the link reaches the service. It
has no authentication in front of it.

So: **ask the user before publishing anything you were not explicitly asked to
publish**, and never publish a port you have not identified. If the user says
"share my app", confirm which port and what runs on it first.

Never expose: databases (5432, 3306, 6379, 27017), admin panels, file servers
rooted at `$HOME` or `/`, or anything reachable at `0.0.0.0` that you did not
start yourself.

## Preflight

Run these in order. Each one tells you what to do next.

```bash
# 1 · is it installed?
command -v linko || echo "not installed"

# 2 · is it configured?
linko status >/dev/null 2>&1 || echo "not set up"

# 3 · is anything actually listening on the port?
curl -sf -o /dev/null http://localhost:3000 || echo "nothing on 3000"
```

Step 3 matters. Publishing a port with nothing behind it produces a live URL
that returns **HTTP 502 / Cloudflare error 1033**, which looks like a linko bug
and is not one. Start the user's server first.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Devehab/linko/main/install.sh | bash
```

One static binary, no runtime. If `linko` is still not found afterwards, the
install directory is not on `PATH` yet — `exec $SHELL`, or open a new terminal.

With Go 1.23+ available: `go install github.com/Devehab/linko@latest`.

Windows outside Git Bash cannot run the one-liner; download the `.zip` from the
releases page instead.

## Setup — this part is the user's, not yours

`linko init` needs a Cloudflare API token. **Do not ask the user to paste a
token into the conversation, and never echo one.** Ask them to run it themselves:

```bash
linko init
```

It asks for the token, then lists the domains that token can see and takes a
number. There is no second question about a base — the domain they pick is it.

The token needs **both** rows on the **same** token, both set to **Edit**:

| Type | Resource | Permission |
| --- | --- | --- |
| `Zone` | `DNS` | Edit |
| `Account` | `Cloudflare Tunnel` | Edit |

…plus **Zone Resources → Include → Specific zone → their domain**. Leaving Zone
Resources empty is the single most common setup failure: the token authenticates
fine and then sees no domains at all.

Created at <https://dash.cloudflare.com/profile/api-tokens>.

**Requirement to state up front:** the domain's nameservers must point at
Cloudflare. Owning the domain is not enough. This is free and takes about ten
minutes — `linko docs` prints the steps.

Non-interactive (CI, or a machine you are provisioning), only if the token is
already in the environment and you never print it:

```bash
linko init --yes --domain example.com     # reads $LINKO_API_TOKEN
```

## Publishing

**Always use `-d`.** Without it `linko` runs in the foreground and blocks until
Ctrl+C — which, in an agent session, means the command never returns.

```bash
linko 3000 -d                  # background, keeps its URL between runs
linko 3000 -d --name preview   # a name you choose → preview.example.com
linko 3000 -d --temp           # deleted when the tunnel stops
linko 3000 -d --new            # mint a fresh random URL, retire the old one
```

Targets other than a bare port:

```bash
linko 127.0.0.1:3000           # app binds the loopback only
linko https://localhost:8443   # origin speaks HTTPS
linko tcp://localhost:22       # raw TCP
```

**A port keeps its URL.** Re-running `linko 3000 -d` returns the same address it
returned last time, so the command is safe to repeat and a link the user already
sent keeps working. Only pass `--new` if they explicitly want to rotate it.

**Names must be one label deep.** `--name preview` is fine; `--name preview.app`
produces a two-level hostname that no free Cloudflare certificate covers, and
the browser fails with `ERR_SSL_VERSION_OR_CIPHER_MISMATCH` before the request
ever reaches the tunnel. Use `preview-app`, not `preview.app`.

## Reading the URL back

`linko list` prints a table for humans. To get the URL programmatically, read
the config — it is JSON:

```bash
jq -r '.routes[] | select(.port == 3000) | "https://" + .hostname' ~/.linko/config.json
```

> **`~/.linko/config.json` also holds the Cloudflare API token and the tunnel
> token.** Never `cat` it, never print it, never paste it into a message or a
> commit. Select the fields you need with `jq` and nothing else.

`$LINKO_HOME` overrides `~/.linko` if it is set.

## Verifying it actually works

Do not report success from the fact that the command exited 0. DNS needs a few
seconds to propagate. Poll:

```bash
URL=$(jq -r '.routes[] | select(.port == 3000) | "https://" + .hostname' ~/.linko/config.json)

for i in $(seq 1 20); do
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$URL")
  [ "$code" = "200" ] && break
  sleep 3
done
echo "$URL → $code"
```

A healthy result is **HTTP 200 with a `cf-ray` response header** — the header
proves the request went through Cloudflare rather than resolving somewhere else:

```bash
curl -sI "$URL" | grep -i '^cf-ray'
```

Then confirm the tunnel side:

```bash
linko status     # tunnel health, live edge connections, routes
linko ps         # what is running in the background
linko doctor     # eight checks; exits non-zero, so it works in scripts
```

## When something is wrong

`linko doctor --fix --yes` repairs an expired token, a tunnel deleted from the
dashboard, missing DNS records and missing routes, then re-runs the full check.
Try it before diagnosing by hand.

| Symptom | Cause | Fix |
| --- | --- | --- |
| `HTTP 502` · `Error 1033` | Tunnel is up, nothing is listening on the port | Start the server. If it binds the loopback only, publish `127.0.0.1:3000`. |
| `ERR_SSL_VERSION_OR_CIPHER_MISMATCH` | Hostname is two levels deep | Use a one-label name — `preview-app`, not `preview.app`. |
| `no zone named "…"` | Token is missing `Zone` permission, or Zone Resources is empty | Re-issue the token with both rows and the domain included. |
| `Authentication error (code 10000)` on a DNS write | DNS permission is `Read`, not `Edit` | Finding the zone only needs read access, so setup passes and the first write fails. Switch it to Edit. |
| `Authentication error (code 10000)` creating the tunnel | Token covers DNS but not tunnels | Add `Account → Cloudflare Tunnel → Edit` to the **same** token. |
| `command not found: linko` | Install directory not on `PATH` | `exec $SHELL`, or open a new terminal. |
| `linko is not set up yet` | No config | The user runs `linko init`. |

`linko` never deletes a DNS record that does not point at one of its tunnels, so
it cannot take down the user's real site.

## Cleaning up

Leaving a tunnel running leaves a public URL live. When the task is done, say so
and offer to close it:

```bash
linko stop web            # stop one background tunnel (URL survives, stops serving)
linko stop --all          # stop all of them
linko remove preview      # delete the URL, its route and its DNS record
linko remove --all --yes  # delete every published URL
```

`stop` leaves the hostname registered so the next `linko 3000 -d` reuses it.
`remove` deletes it for good.

## Keeping it up

For something meant to stay reachable — a home server, a webhook endpoint, a
demo that must survive a reboot:

```bash
linko service install 8096 --name media   # launchd on macOS, systemd --user on Linux
linko service list
linko service uninstall media
```

It restarts automatically if the tunnel drops.

## Recipes

**Check a dev server from a phone**

```bash
npm run dev &                 # or whatever starts it
curl -sf -o /dev/null http://localhost:3000 || sleep 3
linko 3000 -d --name preview
# → https://preview.example.com — opens on any device, no VPN, no same-network requirement
```

**A webhook endpoint Stripe or GitHub can actually reach**

```bash
linko 4000 -d --name hooks
# register https://hooks.example.com/webhook with the provider
```

**Two services at once, one tunnel**

```bash
linko 3000 -d --name web
linko 8080 -d --name api
```

**A home server with no public IP**

```bash
linko service install 8096 --name media     # survives reboots
linko service install 3001 --name notes
```

**A throwaway link that cleans up after itself**

```bash
linko 5173 -d --temp
```

## Command reference

| Command | Purpose |
| --- | --- |
| `linko init` | One-time setup: token, domain, tunnel |
| `linko <port> -d` | Publish a port in the background |
| `linko list` | What is published (`--remote` reads Cloudflare) |
| `linko status` | Tunnel health, live connections, routes |
| `linko ps` | Background tunnels running now |
| `linko stop <name>` | Stop one, or `--all` |
| `linko remove <name>` | Delete a URL, its route and DNS record |
| `linko doctor [--fix]` | Eight checks, and repairs |
| `linko service install <port> --name <n>` | Start at every boot |
| `linko domain` | Change the domain being published to |
| `linko token` | Replace the API token (verified before saving) |
| `linko uninstall` | Remove linko and its Cloudflare traces |
| `linko docs` | The full guide in the terminal |

Full flag reference and the complete error catalogue: <https://devehab.github.io/linko/guide.html>

## Rules

1. Ask before publishing anything the user did not ask you to publish.
2. Always `-d`. A foreground `linko` never returns.
3. Never print, echo, copy or commit the API token, the tunnel token, or the
   contents of `~/.linko/config.json`.
4. Check something is listening on the port **before** publishing it.
5. Verify with a real HTTP request, not with the exit code.
6. One-label names only.
7. Tell the user the URL is public, and clean up when the task ends.
