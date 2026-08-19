"""
rig.py — Single source of truth for the NOOR avatar rig.

Both build_avatar.py (mesh + skeleton generator) and build_signs.py
(animation compiler) import this module, so the bone codes used in the
.hta payloads can never drift from the codes baked into the .glb.

Coordinate system: glTF standard — Y up, Z forward (toward viewer), right-handed.
Bind pose: T-pose. Every joint has identity rotation, so each bone's local
axes are aligned with world axes. That keeps the animation math readable.

Bone axis conventions in bind pose:
  spine chain  → +Y
  left arm     → +X          right arm → -X
  left fingers → +X          right fingers → -X
  legs         → -Y

Therefore, for a finger pointing +X (left hand):
  curl (toward -Y / palm)  = rotation about Z by a NEGATIVE angle
  spread (fan apart)       = rotation about Y
And mirrored for the right hand (finger points -X): curl = POSITIVE Z rotation.
`curl_sign(side)` below encapsulates that so callers never get it wrong.
"""

# ─────────────────────────────────────────────────────────────────────────────
# Bone table:  name -> (parent, local_offset_from_parent, shape)
# shape = (radius_at_base, radius_at_tip) or None for non-rendered helper bones.
# Lengths are implied by the child's offset; leaf bones carry an explicit tip.
# ─────────────────────────────────────────────────────────────────────────────

FPS = 24
ANGLE_DIVISOR = 2.7          # degrees are stored as int(deg / 2.7) in .hta
TRANSITION_SEC = 0.18

# finger segment lengths (proximal, middle, distal)
_F = {
    'thumb':  [0.040, 0.032, 0.026],
    'index':  [0.042, 0.028, 0.021],
    'middle': [0.045, 0.030, 0.022],
    'ring':   [0.041, 0.027, 0.020],
    'pinky':  [0.034, 0.022, 0.018],
}
# where each finger's root sits on the palm: (along_x, up_y, across_z)
_KNUCKLE = {
    'thumb':  (0.020, -0.012,  0.030),
    'index':  (0.072, -0.002,  0.026),
    'middle': (0.076, -0.002,  0.008),
    'ring':   (0.073, -0.002, -0.010),
    'pinky':  (0.066, -0.003, -0.028),
}

SKIN, CLOTH, FACE = 'skin', 'cloth', 'face'


def _build_bones():
    B = []          # list of (name, parent, offset, radii, material, tip_len)

    def add(name, parent, off, radii=None, mat=SKIN, tip=0.0):
        B.append((name, parent, off, radii, mat, tip))

    # ── torso ────────────────────────────────────────────────────────────────
    add('root',   None,    (0.0, 0.000, 0.0))
    add('hips',   'root',  (0.0, 0.880, 0.0), (0.132, 0.120), CLOTH)
    add('spine',  'hips',  (0.0, 0.120, 0.0), (0.120, 0.140), CLOTH)
    add('chest',  'spine', (0.0, 0.160, 0.0), (0.148, 0.120), CLOTH)
    add('neck',   'chest', (0.0, 0.215, 0.0), (0.046, 0.044), SKIN)
    add('head',   'neck',  (0.0, 0.062, 0.0), (0.090, 0.090), SKIN, tip=0.185)

    # ── arms + hands ─────────────────────────────────────────────────────────
    for side, sx in (('L', 1.0), ('R', -1.0)):
        add(f'shoulder{side}', 'chest',           (sx * 0.062, 0.160, 0.0), (0.064, 0.052), CLOTH)
        add(f'upperArm{side}', f'shoulder{side}', (sx * 0.098, 0.000, 0.0), (0.052, 0.041), CLOTH)
        add(f'forearm{side}',  f'upperArm{side}', (sx * 0.268, 0.000, 0.0), (0.041, 0.031), SKIN)
        add(f'hand{side}',     f'forearm{side}',  (sx * 0.242, 0.000, 0.0), (0.034, 0.030), SKIN)

        for fname, segs in _F.items():
            kx, ky, kz = _KNUCKLE[fname]
            r0 = 0.0115 if fname != 'thumb' else 0.0135
            for i, ln in enumerate(segs):
                bone   = f'{fname}{i+1}{side}'
                parent = f'hand{side}' if i == 0 else f'{fname}{i}{side}'
                off    = (sx * kx, ky, sx * kz) if i == 0 else (sx * segs[i-1], 0.0, 0.0)
                r      = r0 * (1.0 - 0.16 * i)
                tip    = ln if i == len(segs) - 1 else 0.0
                add(bone, parent, off, (r, r * 0.82), SKIN, tip=tip)

    # ── legs ─────────────────────────────────────────────────────────────────
    for side, sx in (('L', 1.0), ('R', -1.0)):
        add(f'thigh{side}', 'hips',         (sx * 0.092, -0.060, 0.0), (0.082, 0.062), CLOTH)
        add(f'shin{side}',  f'thigh{side}', (0.0, -0.395, 0.0),        (0.062, 0.044), CLOTH)
        add(f'foot{side}',  f'shin{side}',  (0.0, -0.385, 0.0),        (0.042, 0.034), CLOTH, tip=0.090)
    return B


BONES = _build_bones()
BONE_NAMES = [b[0] for b in BONES]
BONE_PARENT = {b[0]: b[1] for b in BONES}
BONE_OFFSET = {b[0]: b[2] for b in BONES}
BONE_RADII = {b[0]: b[3] for b in BONES}
BONE_MAT = {b[0]: b[4] for b in BONES}
BONE_TIP = {b[0]: b[5] for b in BONES}


# ─────────────────────────────────────────────────────────────────────────────
# Two-letter codes.  Bones use B*/C*/D*, morph targets use U* — exactly the
# convention the decoder relies on: first letter in "UVWXYZ" ⇒ morph target.
# ─────────────────────────────────────────────────────────────────────────────
def _codes(n, start_letters='BCD'):
    out, i = [], 0
    for a in start_letters:
        for b in 'ABCDEFGHIJKLMNOPQRSTUVWXYZ':
            if i >= n:
                return out
            out.append(a + b)
            i += 1
    raise ValueError('ran out of code space')


BONE_CODE = dict(zip(BONE_NAMES, _codes(len(BONE_NAMES))))
CODE_BONE = {v: k for k, v in BONE_CODE.items()}

MORPHS = [
    'browInnerUpL', 'browInnerUpR', 'browOuterUpL', 'browOuterUpR',
    'browDownL', 'browDownR',
    'eyeBlinkL', 'eyeBlinkR', 'eyeWideL', 'eyeWideR',
    'mouthOpen', 'mouthSmile', 'mouthFrown', 'mouthPucker',
    'mouthWide', 'tongueOut',
]
MORPH_CODE = {m: 'U' + 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'[i] for i, m in enumerate(MORPHS)}
CODE_MORPH = {v: k for k, v in MORPH_CODE.items()}
MORPH_INDEX = {m: i for i, m in enumerate(MORPHS)}


def curl_sign(side):
    """Z-rotation sign that curls a finger toward the palm for the given side."""
    return -1.0 if side == 'L' else 1.0


def world_rest():
    """Accumulated world-space rest positions (bind pose has no rotations)."""
    pos = {}
    for name, parent, off, *_ in BONES:
        px, py, pz = pos[parent] if parent else (0.0, 0.0, 0.0)
        pos[name] = (px + off[0], py + off[1], pz + off[2])
    return pos


RIG_VERSION = 'noor-rig-v1'

if __name__ == '__main__':
    import collections
    w = world_rest()
    groups = collections.Counter()
    for n in BONE_NAMES:
        if any(f in n for f in _F) or n.startswith('hand'):
            groups['hands/fingers'] += 1
        elif 'Arm' in n or 'shoulder' in n or 'forearm' in n:
            groups['arms'] += 1
        elif 'thigh' in n or 'shin' in n or 'foot' in n:
            groups['legs'] += 1
        else:
            groups['torso/head'] += 1
    print(f'{RIG_VERSION}: {len(BONE_NAMES)} bones, {len(MORPHS)} morph targets')
    for k, v in groups.most_common():
        print(f'  {v:3d}  {k}  ({100*v/len(BONE_NAMES):.0f}%)')
    print(f'  height ≈ {w["head"][1] + BONE_TIP["head"]:.3f} m')
    print(f'  codes  {BONE_CODE["root"]}..{BONE_CODE[BONE_NAMES[-1]]}  |  '
          f'{MORPH_CODE[MORPHS[0]]}..{MORPH_CODE[MORPHS[-1]]}')
