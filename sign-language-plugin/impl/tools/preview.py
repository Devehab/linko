#!/usr/bin/env python3
"""
preview.py — a small software renderer used as the project's visual test harness.

Chromium is unavailable in CI here, so instead of trusting that the avatar and
the signs look right, this renders them offline: it reads NOOR.glb, applies a
pose (or samples a compiled .hta clip), runs linear-blend skinning + morph
targets on the CPU, and rasterises a shaded PNG with a z-buffer.

That makes every sign visually reviewable without a browser, and gives us
regression images we can diff.

Examples
    python3 preview.py --pose rest -o /tmp/rest.png
    python3 preview.py --hta ../data/signs/ase/@a.hta --frames 1 --view hand
    python3 preview.py --contact-sheet ../data/signs/ase -o /tmp/alphabet.png
"""
import argparse
import json
import math
import struct
import sys
from pathlib import Path

import numpy as np
from PIL import Image

sys.path.insert(0, str(Path(__file__).parent))
from rig import (BONE_NAMES, BONE_PARENT, BONE_OFFSET, BONE_CODE, CODE_BONE,
                 CODE_MORPH, MORPH_INDEX, MORPHS, ANGLE_DIVISOR, FPS)

DEFAULT_GLB = Path(__file__).parent.parent / 'data/avatar/NOOR.glb'


# ═══════════════════════════════════════════════════════════════════════════
# GLB reader (only what we emit)
# ═══════════════════════════════════════════════════════════════════════════
CT = {5120: 'i1', 5121: 'u1', 5122: 'i2', 5123: 'u2', 5125: 'u4', 5126: 'f4'}
NC = {'SCALAR': 1, 'VEC2': 2, 'VEC3': 3, 'VEC4': 4, 'MAT4': 16}


def load_glb(path):
    raw = Path(path).read_bytes()
    magic, ver, total = struct.unpack_from('<III', raw, 0)
    assert magic == 0x46546C67, 'not a GLB'
    off, js, bin_ = 12, None, None
    while off < total:
        ln, ty = struct.unpack_from('<II', raw, off)
        chunk = raw[off + 8: off + 8 + ln]
        if ty == 0x4E4F534A:
            js = json.loads(chunk)
        elif ty == 0x004E4942:
            bin_ = chunk
        off += 8 + ln
    return js, bin_


def accessor(g, b, i):
    a = g['accessors'][i]
    v = g['bufferViews'][a['bufferView']]
    n = NC[a['type']]
    arr = np.frombuffer(b, dtype=CT[a['componentType']],
                        count=a['count'] * n, offset=v.get('byteOffset', 0))
    return arr.reshape(a['count'], n) if n > 1 else arr


class Avatar:
    def __init__(self, path=DEFAULT_GLB):
        g, b = load_glb(path)
        self.g, self.b = g, b
        self.joints = [g['nodes'][j]['name'] for j in g['skins'][0]['joints']]
        self.jidx = {n: i for i, n in enumerate(self.joints)}
        ibm = accessor(g, b, g['skins'][0]['inverseBindMatrices']).reshape(-1, 4, 4)
        self.ibm = np.transpose(ibm, (0, 2, 1))      # column-major -> row-major

        self.prims = []
        for mesh in g['meshes']:
            for p in mesh['primitives']:
                mat = g['materials'][p['material']]
                col = mat['pbrMetallicRoughness']['baseColorFactor'][:3]
                entry = {
                    'V': accessor(g, b, p['attributes']['POSITION']).astype(np.float64),
                    'N': accessor(g, b, p['attributes']['NORMAL']).astype(np.float64),
                    'J': accessor(g, b, p['attributes']['JOINTS_0']).astype(np.int32),
                    'W': accessor(g, b, p['attributes']['WEIGHTS_0']).astype(np.float64),
                    'F': accessor(g, b, p['indices']).reshape(-1, 3).astype(np.int32),
                    'color': np.array(col),
                    'targets': None,
                }
                if 'targets' in p:
                    entry['targets'] = np.stack(
                        [accessor(g, b, t['POSITION']).astype(np.float64) for t in p['targets']])
                self.prims.append(entry)

    # ── posing ────────────────────────────────────────────────────────────
    def world_matrices(self, pose):
        """pose: {bone: (rx, ry, rz) in degrees}. Returns (J,4,4) world matrices."""
        out = np.zeros((len(self.joints), 4, 4))
        cache = {}

        def solve(name):
            if name in cache:
                return cache[name]
            m = np.eye(4)
            m[:3, 3] = BONE_OFFSET[name]
            r = pose.get(name)
            if r is not None:
                if len(r) == 4:
                    m[:3, :3] = quat_to_matrix(r)
                elif any(abs(x) > 1e-9 for x in r):
                    m[:3, :3] = euler_xyz(*[math.radians(v) for v in r])
            par = BONE_PARENT[name]
            w = solve(par) @ m if par else m
            cache[name] = w
            return w

        for n in self.joints:
            out[self.jidx[n]] = solve(n)
        return out

    def skin(self, pose, morphs=None):
        W = self.world_matrices(pose)
        skmat = np.einsum('jab,jbc->jac', W, self.ibm)     # joint palette
        out = []
        for p in self.prims:
            V = p['V']
            if p['targets'] is not None and morphs is not None:
                infl = np.asarray([morphs.get(m, 0.0) for m in MORPHS])
                if infl.any():
                    V = V + np.einsum('t,tvc->vc', infl, p['targets'])
            Vh = np.concatenate([V, np.ones((len(V), 1))], axis=1)
            Nh = np.concatenate([p['N'], np.zeros((len(V), 1))], axis=1)
            acc_v = np.zeros((len(V), 4))
            acc_n = np.zeros((len(V), 4))
            for k in range(4):
                w = p['W'][:, k]
                if not w.any():
                    continue
                M = skmat[p['J'][:, k]]
                acc_v += w[:, None] * np.einsum('vab,vb->va', M, Vh)
                acc_n += w[:, None] * np.einsum('vab,vb->va', M, Nh)
            n = acc_n[:, :3]
            n /= np.maximum(np.linalg.norm(n, axis=1, keepdims=True), 1e-9)
            out.append((acc_v[:, :3], n, p['F'], p['color']))
        return out


def quat_from_euler_deg(rx, ry, rz):
    """Must match THREE.Quaternion.setFromEuler(euler, 'XYZ')."""
    x, y, z = (math.radians(v) for v in (rx, ry, rz))
    c1, c2, c3 = math.cos(x / 2), math.cos(y / 2), math.cos(z / 2)
    s1, s2, s3 = math.sin(x / 2), math.sin(y / 2), math.sin(z / 2)
    return (s1 * c2 * c3 + c1 * s2 * s3,
            c1 * s2 * c3 - s1 * c2 * s3,
            c1 * c2 * s3 + s1 * s2 * c3,
            c1 * c2 * c3 - s1 * s2 * s3)


def slerp(a, b, u):
    """Spherical interpolation, matching THREE.QuaternionLinearInterpolant."""
    d = sum(x * y for x, y in zip(a, b))
    if d < 0:
        b, d = tuple(-x for x in b), -d
    if d > 0.9995:
        r = tuple(x + (y - x) * u for x, y in zip(a, b))
        n = math.sqrt(sum(v * v for v in r)) or 1.0
        return tuple(v / n for v in r)
    th = math.acos(max(-1.0, min(1.0, d)))
    st = math.sin(th)
    wa, wb = math.sin((1 - u) * th) / st, math.sin(u * th) / st
    return tuple(x * wa + y * wb for x, y in zip(a, b))


def quat_to_matrix(q):
    x, y, z, w = q
    return np.array([
        [1 - 2 * (y * y + z * z), 2 * (x * y - z * w),     2 * (x * z + y * w)],
        [2 * (x * y + z * w),     1 - 2 * (x * x + z * z), 2 * (y * z - x * w)],
        [2 * (x * z - y * w),     2 * (y * z + x * w),     1 - 2 * (x * x + y * y)],
    ])


def euler_xyz(x, y, z):
    cx, sx = math.cos(x), math.sin(x)
    cy, sy = math.cos(y), math.sin(y)
    cz, sz = math.cos(z), math.sin(z)
    Rx = np.array([[1, 0, 0], [0, cx, -sx], [0, sx, cx]])
    Ry = np.array([[cy, 0, sy], [0, 1, 0], [-sy, 0, cy]])
    Rz = np.array([[cz, -sz, 0], [sz, cz, 0], [0, 0, 1]])
    return Rx @ Ry @ Rz


# ═══════════════════════════════════════════════════════════════════════════
# rasteriser
# ═══════════════════════════════════════════════════════════════════════════
VIEWS = {
    #        target (x,y,z)      , half-height, azimuth°
    'full':  ((0.00, 0.95, 0.00), 0.98,  0),
    'bust':  ((0.00, 1.30, 0.00), 0.44,  0),
    'face':  ((0.00, 1.55, 0.00), 0.17,  0),
    'hand':  ((-0.185, 1.345, 0.16), 0.135, -20),
    'hands': ((0.00, 1.20, 0.00), 0.52,  0),
}


def render(parts, w=420, h=560, view='full', bg=(14, 18, 27)):
    target, half, az = VIEWS[view]
    a = math.radians(az)
    cam_dir = np.array([math.sin(a), 0.0, math.cos(a)])
    right = np.array([math.cos(a), 0.0, -math.sin(a)])
    up = np.array([0.0, 1.0, 0.0])
    aspect = w / h
    img = np.zeros((h, w, 3), dtype=np.float64)
    img[:, :] = np.array(bg) / 255.0
    zbuf = np.full((h, w), 1e9)
    light = np.array([0.45, 0.75, 0.85])
    light /= np.linalg.norm(light)

    for V, N, F, col in parts:
        rel = V - np.array(target)
        sx = rel @ right
        sy = rel @ up
        sz = -(rel @ cam_dir)      # depth grows away from the camera
        px = (sx / (half * aspect) * 0.5 + 0.5) * w
        py = (1.0 - (sy / half * 0.5 + 0.5)) * h
        lam = np.clip(N @ light, 0, 1)
        shade = 0.30 + 0.70 * lam
        rim = np.clip(1.0 - np.abs(N @ cam_dir), 0, 1) ** 3 * 0.35

        for tri in F:
            i0, i1, i2 = tri
            x0, y0 = px[i0], py[i0]; x1, y1 = px[i1], py[i1]; x2, y2 = px[i2], py[i2]
            area = (x1 - x0) * (y2 - y0) - (x2 - x0) * (y1 - y0)
            if area >= -1e-9:                      # backface / degenerate
                continue
            minx = max(int(min(x0, x1, x2)), 0); maxx = min(int(max(x0, x1, x2)) + 1, w - 1)
            miny = max(int(min(y0, y1, y2)), 0); maxy = min(int(max(y0, y1, y2)) + 1, h - 1)
            if minx > maxx or miny > maxy:
                continue
            ys, xs = np.mgrid[miny:maxy + 1, minx:maxx + 1]
            xf, yf = xs + 0.5, ys + 0.5
            w0 = ((x1 - x0) * (yf - y0) - (xf - x0) * (y1 - y0)) / area
            w1 = ((x2 - x1) * (yf - y1) - (xf - x1) * (y2 - y1)) / area
            w2 = 1.0 - w0 - w1
            m = (w0 >= 0) & (w1 >= 0) & (w2 >= 0)
            if not m.any():
                continue
            z = w1 * sz[i0] + w2 * sz[i1] + w0 * sz[i2]
            sub = zbuf[miny:maxy + 1, minx:maxx + 1]
            m &= z < sub
            if not m.any():
                continue
            sh = w1 * shade[i0] + w2 * shade[i1] + w0 * shade[i2]
            rm = w1 * rim[i0] + w2 * rim[i1] + w0 * rim[i2]
            c = col[None, None, :] * sh[..., None] + rm[..., None] * 0.55
            sub[m] = z[m]
            tile = img[miny:maxy + 1, minx:maxx + 1]
            tile[m] = np.clip(c, 0, 1)[m]
    return Image.fromarray((np.clip(img, 0, 1) * 255).astype(np.uint8))


# ═══════════════════════════════════════════════════════════════════════════
# .hta sampling
# ═══════════════════════════════════════════════════════════════════════════
def sample_hta(hta, t):
    """Decode an .hta payload and evaluate it at time t (seconds).

    Deliberately mirrors web/src/core.js decodeHTA() + three.js sampling:
    each keyframe's euler triple becomes a quaternion, and interpolation is
    SLERP between those quaternions. Interpolating the euler angles instead
    would trace a different path between keyframes — tests/test.js asserts the
    two implementations agree, and caught exactly that discrepancy.
    """
    pose, morphs = {}, {}
    for track in hta.split('?'):
        if not track:
            continue
        parts = track.split('*')
        code, frames = parts[0], parts[1]
        times = [int(v or 0) / FPS for v in frames.split('#')]
        is_morph = code[0] in 'UVWXYZ'
        chans = parts[2:]
        n = len(times)

        def chan(k):
            """Channel k expanded to n values, holding the last one."""
            if k >= len(chans):
                return [0.0] * n
            xs = [float(v or 0) for v in chans[k].split('#')]
            out, last = [], 0.0
            for i in range(n):
                if i < len(xs):
                    last = xs[i]
                out.append(last)
            return out

        if t <= times[0]:
            i, u = 0, 0.0
        elif t >= times[-1]:
            i, u = n - 1, 0.0
        else:
            i = max(j for j in range(n) if times[j] <= t)
            span = times[min(i + 1, n - 1)] - times[i]
            u = 0.0 if span <= 0 else (t - times[i]) / span
        j = min(i + 1, n - 1)

        if is_morph:
            v = chan(0)
            morphs[CODE_MORPH[code]] = (v[i] + (v[j] - v[i]) * u) / 100.0
        else:
            xs, ys, zs = chan(0), chan(1), chan(2)
            qa = quat_from_euler_deg(xs[i] * ANGLE_DIVISOR, ys[i] * ANGLE_DIVISOR,
                                     zs[i] * ANGLE_DIVISOR)
            qb = quat_from_euler_deg(xs[j] * ANGLE_DIVISOR, ys[j] * ANGLE_DIVISOR,
                                     zs[j] * ANGLE_DIVISOR)
            pose[CODE_BONE[code]] = slerp(qa, qb, u) if u > 0 else qa
    return pose, morphs


def hta_duration(hta):
    end = 0.0
    for track in hta.split('?'):
        if track:
            end = max(end, max(int(v or 0) for v in track.split('*')[1].split('#')) / FPS)
    return end


# ═══════════════════════════════════════════════════════════════════════════
def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--glb', default=str(DEFAULT_GLB))
    ap.add_argument('--hta')
    ap.add_argument('--pose', choices=['rest', 'bind'], default='bind')
    ap.add_argument('--frames', type=int, default=4)
    ap.add_argument('--view', default='full', choices=list(VIEWS))
    ap.add_argument('--contact-sheet')
    ap.add_argument('--cols', type=int, default=9)
    ap.add_argument('--size', type=int, nargs=2, default=[420, 560])
    ap.add_argument('-o', '--out', default='/tmp/preview.png')
    a = ap.parse_args()

    av = Avatar(a.glb)
    W, H = a.size

    if a.contact_sheet:
        files = sorted(Path(a.contact_sheet).glob('*.hta'))
        tiles = []
        for f in files:
            hta = f.read_text()
            d = hta_duration(hta)
            pose, morphs = sample_hta(hta, d * 0.55)
            im = render(av.skin(pose, morphs), W, H, a.view)
            tiles.append((f.stem, im))
        cols = a.cols
        rows = (len(tiles) + cols - 1) // cols
        sheet = Image.new('RGB', (cols * W, rows * H), (14, 18, 27))
        from PIL import ImageDraw
        for k, (name, im) in enumerate(tiles):
            x, y = (k % cols) * W, (k // cols) * H
            sheet.paste(im, (x, y))
            ImageDraw.Draw(sheet).text((x + 8, y + 6), name, fill=(140, 200, 255))
        sheet.save(a.out)
        print(f'✓ {a.out}  ({len(tiles)} signs, {cols}×{rows})')
        return

    if a.hta:
        hta = Path(a.hta).read_text()
        d = hta_duration(hta)
        n = a.frames
        ims = []
        for k in range(n):
            t = d * (k + 0.5) / n if n > 1 else d * 0.55
            pose, morphs = sample_hta(hta, t)
            ims.append(render(av.skin(pose, morphs), W, H, a.view))
        sheet = Image.new('RGB', (W * n, H), (14, 18, 27))
        for k, im in enumerate(ims):
            sheet.paste(im, (k * W, 0))
        sheet.save(a.out)
        print(f'✓ {a.out}  ({n} frames over {d:.2f}s)')
        return

    render(av.skin({}, None), W, H, a.view).save(a.out)
    print(f'✓ {a.out}  (bind pose, view={a.view})')


if __name__ == '__main__':
    main()
