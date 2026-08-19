#!/usr/bin/env python3
"""
build_avatar.py — generates NOOR.glb, an original rigged humanoid avatar.

Everything here is synthesised from scratch: no third-party model, no textures,
no external dependencies beyond numpy. The output is a valid glTF 2.0 binary
containing

  • a 50-bone skeleton (64% of it in the hands, where sign language lives)
  • three primitives:
        Body_Skin   — head, neck, forearms, hands, fingers   (material: skin)
        Body_Cloth  — torso, upper arms, legs                (material: cloth)
        Face        — brows, eyes, lips  + 16 morph targets   (material: face)
  • linear-blend skinning weights with smooth blending across every joint

Because skin and clothing live in separate primitives with separate materials,
recolouring either at runtime is a one-line material tweak — see
`SLA.setAppearance()` in web/sla-core.js.

Usage:  python3 build_avatar.py [-o ../data/avatar/NOOR.glb]
"""
import argparse
import json
import struct
import sys
from pathlib import Path

import numpy as np

sys.path.insert(0, str(Path(__file__).parent))
from rig import (BONES, BONE_NAMES, BONE_PARENT, BONE_OFFSET, BONE_RADII,
                 BONE_MAT, BONE_TIP, MORPHS, MORPH_INDEX, RIG_VERSION,
                 world_rest, SKIN, CLOTH, FACE)

RING = 12          # vertices around a limb cross-section
LEVELS = 5         # cross-sections along a limb


# ═══════════════════════════════════════════════════════════════════════════
# geometry helpers
# ═══════════════════════════════════════════════════════════════════════════
def _basis(direction):
    """Orthonormal frame whose Z axis follows `direction`."""
    d = np.asarray(direction, dtype=np.float64)
    d /= np.linalg.norm(d)
    up = np.array([0.0, 1.0, 0.0]) if abs(d[1]) < 0.9 else np.array([1.0, 0.0, 0.0])
    u = np.cross(up, d)
    u /= np.linalg.norm(u)
    v = np.cross(d, u)
    return u, v, d


def capsule(p0, direction, length, r0, r1, ring=RING, levels=LEVELS, round_tip=True):
    """Tapered capsule from p0 along `direction`. Returns (verts, norms, faces, t)
    where `t` is each vertex's 0..1 parameter along the bone — used for weight blending."""
    u, v, d = _basis(direction)
    verts, norms, tpar = [], [], []

    profile = []
    for i in range(levels):
        s = i / (levels - 1)
        profile.append((s, r0 + (r1 - r0) * s))
    if round_tip:                                   # dome the far end
        for k in (0.35, 0.70, 1.0):
            profile.append((1.0 + k * (r1 / max(length, 1e-6)) * 0.9,
                            r1 * float(np.cos(k * np.pi / 2)) ** 0.7))

    for s, r in profile:
        centre = np.asarray(p0) + d * (s * length)
        for j in range(ring):
            a = 2.0 * np.pi * j / ring
            n = u * np.cos(a) + v * np.sin(a)
            verts.append(centre + n * r)
            norms.append(n if r > 1e-6 else d)
            tpar.append(min(s, 1.0))

    faces = []
    rows = len(profile)
    for i in range(rows - 1):
        for j in range(ring):
            a = i * ring + j
            b = i * ring + (j + 1) % ring
            c = (i + 1) * ring + (j + 1) % ring
            e = (i + 1) * ring + j
            faces += [(a, b, c), (a, c, e)]
    # flat cap at the base
    base_c = len(verts)
    verts.append(np.asarray(p0, dtype=np.float64))
    norms.append(-d)
    tpar.append(0.0)
    for j in range(ring):
        faces.append((base_c, (j + 1) % ring, j))

    return (np.array(verts), np.array(norms), np.array(faces, dtype=np.int64),
            np.array(tpar))


def ellipsoid(centre, radii, lat=14, lon=18):
    verts, norms, faces = [], [], []
    for i in range(lat + 1):
        th = np.pi * i / lat
        for j in range(lon):
            ph = 2 * np.pi * j / lon
            n = np.array([np.sin(th) * np.cos(ph), np.cos(th), np.sin(th) * np.sin(ph)])
            verts.append(np.asarray(centre) + n * np.asarray(radii))
            norms.append(n / np.linalg.norm(n))
    for i in range(lat):
        for j in range(lon):
            a = i * lon + j
            b = i * lon + (j + 1) % lon
            c = (i + 1) * lon + (j + 1) % lon
            e = (i + 1) * lon + j
            faces += [(a, b, c), (a, c, e)]
    return np.array(verts), np.array(norms), np.array(faces, dtype=np.int64)


def disc(centre, rx, ry, normal_z=1.0, ring=16):
    """Flat elliptical disc facing ±Z — used for eyes, brows and lips."""
    verts = [np.asarray(centre, dtype=np.float64)]
    norms = [np.array([0.0, 0.0, normal_z])]
    for j in range(ring):
        a = 2 * np.pi * j / ring
        verts.append(np.asarray(centre) + np.array([np.cos(a) * rx, np.sin(a) * ry, 0.0]))
        norms.append(np.array([0.0, 0.0, normal_z]))
    faces = [(0, j + 1, (j + 1) % ring + 1) for j in range(ring)]
    return np.array(verts), np.array(norms), np.array(faces, dtype=np.int64)


# ═══════════════════════════════════════════════════════════════════════════
# mesh assembly
# ═══════════════════════════════════════════════════════════════════════════
class Prim:
    def __init__(self, name, material):
        self.name, self.material = name, material
        self.V, self.N, self.F = [], [], []
        self.J, self.W = [], []
        self.n = 0

    def add(self, verts, norms, faces, joints, weights):
        self.F.append(np.asarray(faces) + self.n)
        self.V.append(verts); self.N.append(norms)
        self.J.append(joints); self.W.append(weights)
        self.n += len(verts)

    def pack(self):
        J = np.concatenate(self.J).astype(np.uint16)
        W = np.concatenate(self.W).astype(np.float32)
        J[W <= 0.0] = 0            # glTF: unused joint slots must be index 0
        return (np.concatenate(self.V).astype(np.float32),
                np.concatenate(self.N).astype(np.float32),
                np.concatenate(self.F).astype(np.uint32), J, W)


def build_body(rest, jidx):
    skin = Prim('Body_Skin', SKIN)
    cloth = Prim('Body_Cloth', CLOTH)
    children = {}
    for name, parent, *_ in BONES:
        children.setdefault(parent, []).append(name)

    for name, parent, off, radii, mat, tip in BONES:
        if radii is None:
            continue
        p0 = np.array(rest[name])
        kids = [c for c in children.get(name, []) if BONE_RADII[c] is not None]
        # a bone's rendered direction follows its first child, or its own tip
        if kids and tip == 0.0:
            d = np.array(rest[kids[0]]) - p0
            length = float(np.linalg.norm(d))
            if length < 1e-6:
                continue
            d /= length
            round_tip = False
        else:
            length = tip if tip > 0 else 0.05
            d = np.array(off, dtype=np.float64)
            if name.startswith('foot'):
                d = np.array([0.0, -0.25, 1.0])          # feet point forward
            if np.linalg.norm(d) < 1e-9:
                d = np.array([0.0, 1.0, 0.0])
            d /= np.linalg.norm(d)
            round_tip = True

        if name == 'head':
            hc = p0 + np.array([0, 0.092, 0.004])
            v, n, f = ellipsoid(hc, (0.086, 0.106, 0.092))
            t = np.ones(len(v))
            nv, nn, nf, _ = capsule(hc + np.array([0.0, -0.012, 0.070]),
                                    np.array([0.0, -0.55, 1.0]), 0.028,
                                    0.0125, 0.0085, ring=10, levels=3)
            f = np.concatenate([f, nf + len(v)])
            v = np.concatenate([v, nv]); n = np.concatenate([n, nn])
            t = np.concatenate([t, np.ones(len(nv))])
        else:
            v, n, f, t = capsule(p0, d, length, radii[0], radii[1], round_tip=round_tip)

        # skin weights: own bone, blended with the parent across the joint
        self_i = jidx[name]
        par_i = jidx[parent] if parent in jidx else self_i
        J = np.zeros((len(v), 4), dtype=np.uint16)
        W = np.zeros((len(v), 4), dtype=np.float32)
        blend = np.clip(t / 0.42, 0.0, 1.0)
        ws = 0.5 + 0.5 * blend
        J[:, 0] = self_i
        J[:, 1] = par_i
        W[:, 0] = ws
        W[:, 1] = 1.0 - ws
        (skin if mat == SKIN else cloth).add(v, n, f, J, W)

    # Joint fillers. Linear-blend skinning creases badly where two capsules meet
    # at a sharp angle — the shoulder and elbow show a visible pinch as soon as
    # the arm lifts. A sphere sitting at the joint, weighted evenly between the
    # two bones, rotates with the average and hides the crease.
    FILLERS = [
        ('shoulderL', 0.058, CLOTH), ('shoulderR', 0.058, CLOTH),
        ('upperArmL', 0.052, CLOTH), ('upperArmR', 0.052, CLOTH),
        ('forearmL', 0.040, SKIN),   ('forearmR', 0.040, SKIN),
        ('handL', 0.031, SKIN),      ('handR', 0.031, SKIN),
        ('thighL', 0.078, CLOTH),    ('thighR', 0.078, CLOTH),
        ('shinL', 0.058, CLOTH),     ('shinR', 0.058, CLOTH),
        ('spine', 0.118, CLOTH),     ('chest', 0.140, CLOTH),
        ('neck', 0.044, SKIN),
    ]
    for name, r, mat in FILLERS:
        parent = BONE_PARENT[name]
        v, n, f = ellipsoid(np.array(rest[name]), (r, r, r), lat=8, lon=10)
        J = np.zeros((len(v), 4), dtype=np.uint16)
        W = np.zeros((len(v), 4), dtype=np.float32)
        J[:, 0] = jidx[name]; W[:, 0] = 0.5
        J[:, 1] = jidx.get(parent, jidx[name]); W[:, 1] = 0.5
        (skin if mat == SKIN else cloth).add(v, n, f, J, W)

    return skin, cloth


def build_face(rest, jidx):
    """Face features + the 16 morph targets that carry ASL's grammar.

    Every feature is a patch whose vertices are individually projected onto the
    head ellipsoid, then lifted along the surface normal. A flat disc would dip
    below the skull at its edges and the brows — which encode question type in
    ASL — would disappear."""
    HR = np.array([0.086, 0.106, 0.092])
    hp = np.array(rest['head'])
    c = hp + np.array([0.0, 0.092, 0.004])
    face = Prim('Face', FACE)
    groups = {}

    def project(x, y, lift):
        k = 1.0 - (x / HR[0]) ** 2 - (y / HR[1]) ** 2
        z = HR[2] * float(np.sqrt(max(k, 0.015)))
        pt = np.array([x, y, z])
        nrm = pt / (HR ** 2)
        nrm /= np.linalg.norm(nrm)
        return c + pt + nrm * lift, nrm

    def patch(cx, cy, rx, ry, lift=0.0022, ring=18):
        """Elliptical patch lying on the head surface."""
        p0, n0 = project(cx, cy, lift)
        verts, norms = [p0], [n0]
        for j in range(ring):
            a = 2 * np.pi * j / ring
            pv, nv = project(cx + np.cos(a) * rx, cy + np.sin(a) * ry, lift)
            verts.append(pv); norms.append(nv)
        faces = [(0, j + 1, (j + 1) % ring + 1) for j in range(ring)]
        return np.array(verts), np.array(norms), np.array(faces, dtype=np.int64)

    def piece(tag, verts, norms, faces):
        start = face.n
        J = np.zeros((len(verts), 4), dtype=np.uint16)
        W = np.zeros((len(verts), 4), dtype=np.float32)
        J[:, 0] = jidx['head']; W[:, 0] = 1.0
        face.add(verts, norms, faces, J, W)
        groups[tag] = (start, face.n)

    for side, sx in (('L', 1.0), ('R', -1.0)):
        piece(f'eye{side}',   *patch(sx * 0.036, 0.008, 0.0205, 0.0150))
        piece(f'pupil{side}', *patch(sx * 0.036, 0.006, 0.0090, 0.0090, lift=0.0050))
        piece(f'brow{side}',  *patch(sx * 0.038, 0.046, 0.0250, 0.0058))

    piece('mouth',  *patch(0.0, -0.056, 0.0290, 0.0095))
    piece('tongue', *patch(0.0, -0.058, 0.0170, 0.0050, lift=0.0038))

    V = np.concatenate(face.V)
    targets = np.zeros((len(MORPHS), len(V), 3), dtype=np.float32)

    def region(tag):
        a, b = groups[tag]
        return slice(a, b)

    def centre_of(tag):
        return V[region(tag)].mean(axis=0)

    for side in ('L', 'R'):
        br = region(f'brow{side}')
        bc = centre_of(f'brow{side}')
        inner = (V[br][:, 0] - bc[0]) * (1.0 if side == 'R' else -1.0)   # toward the nose
        w_in = np.clip(0.5 + inner / 0.042, 0.0, 1.0)
        targets[MORPH_INDEX[f'browInnerUp{side}'], br, 1] = 0.017 * w_in
        targets[MORPH_INDEX[f'browOuterUp{side}'], br, 1] = 0.015 * (1.0 - w_in)
        targets[MORPH_INDEX[f'browDown{side}'], br, 1] = -0.013
        targets[MORPH_INDEX[f'browDown{side}'], br, 0] = 0.004 * (1 if side == 'R' else -1)

        for tag, scale in ((f'eye{side}', 1.0), (f'pupil{side}', 0.45)):
            rg = region(tag)
            ec = centre_of(tag)
            dy = V[rg][:, 1] - ec[1]
            targets[MORPH_INDEX[f'eyeBlink{side}'], rg, 1] = -dy * 0.94 * scale
            targets[MORPH_INDEX[f'eyeWide{side}'], rg, 1] = dy * 0.42 * scale

    mo = region('mouth')
    mc = centre_of('mouth')
    dx = V[mo][:, 0] - mc[0]
    dy = V[mo][:, 1] - mc[1]
    corner = np.clip(np.abs(dx) / 0.025, 0.0, 1.0)
    targets[MORPH_INDEX['mouthOpen'], mo, 1] = np.sign(dy) * 0.020 * (1.0 - corner * 0.7)
    targets[MORPH_INDEX['mouthOpen'], mo, 2] = -0.004 * (1.0 - corner)
    targets[MORPH_INDEX['mouthSmile'], mo, 1] = corner * np.sign(dx) * 0.0 + corner * 0.014
    targets[MORPH_INDEX['mouthSmile'], mo, 0] = dx * 0.16
    targets[MORPH_INDEX['mouthFrown'], mo, 1] = -corner * 0.013
    targets[MORPH_INDEX['mouthPucker'], mo, 0] = -dx * 0.55
    targets[MORPH_INDEX['mouthPucker'], mo, 1] = dy * 0.85
    targets[MORPH_INDEX['mouthPucker'], mo, 2] = 0.011
    targets[MORPH_INDEX['mouthWide'], mo, 0] = dx * 0.55
    targets[MORPH_INDEX['mouthWide'], mo, 1] = -dy * 0.25

    tg = region('tongue')
    targets[MORPH_INDEX['tongueOut'], tg, 2] = 0.020
    targets[MORPH_INDEX['tongueOut'], tg, 1] = -0.012
    targets[MORPH_INDEX['mouthOpen'], tg, 1] = -0.008

    # Split into two primitives so the sclera can be white and the pupils dark.
    # Both stay in the same mesh, so both must carry all 16 morph targets.
    white_tags = {'eyeL', 'eyeR'}
    w_idx, d_idx = [], []
    for tag, (a, b) in groups.items():
        (w_idx if tag in white_tags else d_idx).extend(range(a, b))
    w_idx, d_idx = np.array(w_idx), np.array(d_idx)

    Vall = np.concatenate(face.V); Nall = np.concatenate(face.N)
    Jall = np.concatenate(face.J); Wall = np.concatenate(face.W)
    Fall = np.concatenate(face.F)

    def subset(idx, material):
        remap = -np.ones(len(Vall), dtype=np.int64)
        remap[idx] = np.arange(len(idx))
        keep = np.all(remap[Fall] >= 0, axis=1)
        pr = Prim('Face', material)
        pr.add(Vall[idx], Nall[idx], remap[Fall[keep]], Jall[idx], Wall[idx])
        return pr, targets[:, idx, :]

    return subset(w_idx, 'sclera'), subset(d_idx, FACE)


# ═══════════════════════════════════════════════════════════════════════════
# GLB writer
# ═══════════════════════════════════════════════════════════════════════════
class Glb:
    def __init__(self):
        self.bin = bytearray()
        self.views, self.accessors = [], []

    def _view(self, data, target=None):
        while len(self.bin) % 4:
            self.bin.append(0)
        off = len(self.bin)
        self.bin += data
        v = {'buffer': 0, 'byteOffset': off, 'byteLength': len(data)}
        if target:
            v['target'] = target
        self.views.append(v)
        return len(self.views) - 1

    def acc(self, array, comp_type, type_, target=None, minmax=False, normalized=False):
        data = array.tobytes()
        vi = self._view(data, target)
        a = {'bufferView': vi, 'componentType': comp_type,
             'count': len(array), 'type': type_}
        if normalized:
            a['normalized'] = True
        if minmax:
            a['min'] = array.min(axis=0).tolist()
            a['max'] = array.max(axis=0).tolist()
        self.accessors.append(a)
        return len(self.accessors) - 1

    def write(self, gltf, path):
        gltf['buffers'] = [{'byteLength': len(self.bin)}]
        gltf['bufferViews'] = self.views
        gltf['accessors'] = self.accessors
        js = json.dumps(gltf, separators=(',', ':')).encode()
        js += b' ' * ((4 - len(js) % 4) % 4)
        bn = bytes(self.bin) + b'\0' * ((4 - len(self.bin) % 4) % 4)
        total = 12 + 8 + len(js) + 8 + len(bn)
        with open(path, 'wb') as f:
            f.write(struct.pack('<III', 0x46546C67, 2, total))
            f.write(struct.pack('<II', len(js), 0x4E4F534A)); f.write(js)
            f.write(struct.pack('<II', len(bn), 0x004E4942)); f.write(bn)
        return total


FLOAT, USHORT, UINT = 5126, 5123, 5125
ARRAY_BUFFER, ELEMENT_ARRAY_BUFFER = 34962, 34963


def main(out_path):
    rest = world_rest()
    jidx = {n: i for i, n in enumerate(BONE_NAMES)}

    skin_p, cloth_p = build_body(rest, jidx)
    (white_p, white_t), (dark_p, dark_t) = build_face(rest, jidx)

    g = Glb()
    body_prims, face_prims = [], []
    for p, morph in ((skin_p, None), (cloth_p, None),
                     (white_p, white_t), (dark_p, dark_t)):
        V, N, F, J, W = p.pack()
        attrs = {
            'POSITION': g.acc(V, FLOAT, 'VEC3', ARRAY_BUFFER, minmax=True),
            'NORMAL':   g.acc(N, FLOAT, 'VEC3', ARRAY_BUFFER),
            'JOINTS_0': g.acc(J, USHORT, 'VEC4', ARRAY_BUFFER),
            'WEIGHTS_0': g.acc(W, FLOAT, 'VEC4', ARRAY_BUFFER),
        }
        prim = {'attributes': attrs,
                'indices': g.acc(F.reshape(-1, 1), UINT, 'SCALAR', ELEMENT_ARRAY_BUFFER),
                'material': {SKIN: 0, CLOTH: 1, FACE: 2, 'sclera': 3}[p.material]}
        if morph is not None:
            prim['targets'] = [{'POSITION': g.acc(t, FLOAT, 'VEC3', ARRAY_BUFFER, minmax=True)}
                               for t in morph]
            prim['extras'] = {'targetNames': MORPHS}
        (face_prims if morph is not None else body_prims).append(prim)

    # inverse bind matrices (bind pose is translation-only, column-major)
    ibm = np.zeros((len(BONE_NAMES), 16), dtype=np.float32)
    for n, i in jidx.items():
        m = np.eye(4)
        m[:3, 3] = -np.array(rest[n])
        ibm[i] = m.T.reshape(-1)      # glTF wants column-major
    ibm_acc = g.acc(ibm, FLOAT, 'MAT4')

    nodes = []
    for name, parent, off, *_ in BONES:
        nodes.append({'name': name, 'translation': list(off)})
    for i, (name, parent, *_) in enumerate(BONES):
        if parent is not None:
            nodes[jidx[parent]].setdefault('children', []).append(i)
    body_node = len(nodes)
    nodes.append({'name': 'NOOR_Body', 'mesh': 0, 'skin': 0})
    face_node = len(nodes)
    nodes.append({'name': 'NOOR_Face', 'mesh': 1, 'skin': 0})

    gltf = {
        'asset': {'version': '2.0',
                  'generator': f'sign-language-plugin build_avatar.py ({RIG_VERSION})'},
        'scene': 0,
        'scenes': [{'nodes': [jidx['root'], body_node, face_node]}],
        'nodes': nodes,
        'meshes': [
            {'name': 'NOOR_Body', 'primitives': body_prims},
            {'name': 'NOOR_Face', 'primitives': face_prims,
             'weights': [0.0] * len(MORPHS),
             'extras': {'targetNames': MORPHS}},
        ],
        'skins': [{'name': 'NOOR_Skeleton', 'inverseBindMatrices': ibm_acc,
                   'skeleton': jidx['root'],
                   'joints': [jidx[n] for n in BONE_NAMES]}],
        'materials': [
            {'name': 'skin', 'pbrMetallicRoughness': {
                'baseColorFactor': [0.85, 0.66, 0.52, 1.0],
                'metallicFactor': 0.0, 'roughnessFactor': 0.72}},
            {'name': 'cloth', 'pbrMetallicRoughness': {
                'baseColorFactor': [0.12, 0.35, 0.72, 1.0],
                'metallicFactor': 0.0, 'roughnessFactor': 0.85}},
            {'name': 'face', 'doubleSided': True, 'pbrMetallicRoughness': {
                'baseColorFactor': [0.14, 0.11, 0.11, 1.0],
                'metallicFactor': 0.0, 'roughnessFactor': 0.55}},
            {'name': 'sclera', 'doubleSided': True, 'pbrMetallicRoughness': {
                'baseColorFactor': [0.94, 0.94, 0.92, 1.0],
                'metallicFactor': 0.0, 'roughnessFactor': 0.35}},
        ],
        'extras': {
            'rigVersion': RIG_VERSION,
            'boneNames': BONE_NAMES,
            'morphNames': MORPHS,
            'faceMeshName': 'NOOR_Face',
        },
    }

    out = Path(out_path)
    out.parent.mkdir(parents=True, exist_ok=True)
    size = g.write(gltf, out)

    prims = (skin_p, cloth_p, white_p, dark_p)
    tris = sum(len(np.concatenate(p.F)) for p in prims)
    verts = sum(p.n for p in prims)
    print(f'✓ {out}')
    print(f'  {size/1024:.1f} KB · {verts} vertices · {tris} triangles')
    print(f'  {len(BONE_NAMES)} joints · {len(MORPHS)} morph targets · {len(prims)} primitives')
    return 0


if __name__ == '__main__':
    ap = argparse.ArgumentParser()
    ap.add_argument('-o', '--out', default=str(Path(__file__).parent.parent / 'data/avatar/NOOR.glb'))
    sys.exit(main(ap.parse_args().out))
