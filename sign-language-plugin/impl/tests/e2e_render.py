#!/usr/bin/env python3
"""
e2e_render.py — end-to-end proof, rendered.

Does exactly what the browser does, in the same order, against the running
server: POST /v1/sign/translate, flatten the three-level response into a play
queue, resolve '@' ids against the offline dictionary, decode each payload, and
sample the resulting clip over time. Then it rasterises the frames into a
filmstrip so the output can be *looked at* rather than merely asserted.

Usage:
    node server/server.js --port 8787 &
    python3 tests/e2e_render.py "What is your name?" -o /tmp/strip.png
"""
import argparse
import json
import sys
import urllib.request
from pathlib import Path

from PIL import Image, ImageDraw

sys.path.insert(0, str(Path(__file__).parent.parent / 'tools'))
from preview import Avatar, render, sample_hta, hta_duration   # noqa: E402

BASE = 'http://127.0.0.1:8787'
TOKEN = 'demo_2f6a91c4e7b3'
SIGNS = Path(__file__).parent.parent / 'data/signs/ase'
TRANSITION = 0.18


def post(path, payload):
    req = urllib.request.Request(
        BASE + path, data=json.dumps(payload).encode(),
        headers={'Content-Type': 'application/json',
                 'X-Api-Key': 'sla_pub_9f2c41d7e8b64a05'})
    with urllib.request.urlopen(req, timeout=10) as r:
        return json.load(r)


def flatten(x, out=None):
    """Same recursive flatten the plugin shell performs on the wire response."""
    out = [] if out is None else out
    if isinstance(x, list):
        for v in x:
            flatten(v, out)
    elif isinstance(x, str) and x:
        out.append(x)
    return out


def resolve(item):
    """'@a' -> offline dictionary lookup; anything else is already a payload."""
    if item.startswith('@'):
        p = SIGNS / f'{item}.hta'
        return p.read_text() if p.exists() else None
    return item


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('text', nargs='?', default='What is your name?')
    ap.add_argument('-o', '--out', default='/tmp/e2e_strip.png')
    ap.add_argument('--frames', type=int, default=12)
    ap.add_argument('--view', default='bust')
    ap.add_argument('--size', type=int, nargs=2, default=[260, 340])
    a = ap.parse_args()

    print(f'  text     : {a.text!r}')

    detail = post('/v1/sign/translate',
                  {'token': TOKEN, 'q': a.text, 'lang': 'ase', 'format': 'json'})
    for s in detail['sentences']:
        parts = []
        for g in s['glosses']:
            if g['type'] == 'dropped':
                parts.append(f"({g['source']}→∅)")
            elif g['type'] == 'sign':
                parts.append(g['gloss'])
            else:
                parts.append('-'.join(i[1:] for i in g['ids']))
        nmm = f"  [{s['nmm']}]" if s['nmm'] else ''
        print(f'  glosses  : {" ".join(parts)}{nmm}')
    print(f"  engine   : {detail['meta']['engine']} · {detail['meta']['latencyMs']} ms · "
          f"cache {'HIT' if detail['meta']['cacheHit'] else 'miss'}")

    wire = post('/v1/sign/translate', {'token': TOKEN, 'q': a.text, 'lang': 'ase'})
    queue = flatten(wire) + ['@rest']
    payload_bytes = len(json.dumps(wire))
    print(f'  queue    : {len(queue)} clips, wire = {payload_bytes/1024:.2f} KB')

    # Build a timeline the way the mixer does: each clip runs its own duration,
    # with the next one cross-fading in TRANSITION seconds early.
    timeline, t0 = [], 0.0
    for item in queue:
        hta = resolve(item)
        if not hta:
            print(f'  ! missing asset for {item}')
            continue
        d = hta_duration(hta)
        timeline.append((t0, t0 + d, hta, item))
        t0 += max(d - TRANSITION, 0.05)
    total = t0
    print(f'  duration : {total:.2f} s')

    av = Avatar()
    W, H = a.size
    strip = Image.new('RGB', (W * a.frames, H + 22), (11, 14, 20))
    draw = ImageDraw.Draw(strip)

    for k in range(a.frames):
        t = total * k / max(a.frames - 1, 1)
        active = [c for c in timeline if c[0] <= t <= c[1]] or \
                 [min(timeline, key=lambda c: abs(c[0] - t))]
        start, _, hta, item = active[-1]
        pose, morphs = sample_hta(hta, t - start)
        img = render(av.skin(pose, morphs), W, H, a.view)
        strip.paste(img, (k * W, 22))
        label = item[1:] if item.startswith('@') else 'sign'
        draw.text((k * W + 6, 5), f'{t:0.2f}s  {label}', fill=(123, 180, 255))

    strip.save(a.out)
    print(f'  ✓ {a.out}')


if __name__ == '__main__':
    main()
