#!/usr/bin/env python3
"""
build_signs.py — compiles the sign dictionary into .hta payloads.

Two kinds of entry come out of this:

  1. The fingerspelling alphabet (a–z, 0–9, punctuation). These are *derived*,
     not hand-authored: each ASL letter is described once as a handshape — a
     curl value per finger plus a wrist orientation — and the compiler turns
     that description into joint rotations. That is what makes a 40-odd entry
     offline dictionary tractable to build and to review.

  2. Word signs, authored against the five parameters linguists actually use to
     describe a sign: HANDSHAPE, LOCATION, ORIENTATION, MOVEMENT and NON-MANUAL
     MARKERS. Location is a wrist target in world space, resolved by the IK in
     kinematics.py. Non-manual markers are not decoration — in ASL a raised brow
     is what turns a clause into a yes/no question — so they are compiled into
     the same clip as morph-target tracks.

Output is HTA, identical in grammar to what the backend serves and what
web/sla-core.js decodes:

    Track  := Code "*" Frames "*" ChanX [ "*" ChanY "*" ChanZ ]
    HTA    := Track ( "?" Track )*

Angles are stored as int(degrees / 2.7); morph influence as int(0..100);
an empty value means zero. Bones that never leave the rest pose are omitted
entirely and the decoder restores them.

Usage:  python3 build_signs.py [--out ../data/signs/ase]
"""
import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
import kinematics as K
from rig import (BONE_CODE, MORPH_CODE, ANGLE_DIVISOR, FPS, curl_sign,
                 RIG_VERSION)

FINGERS = ['thumb', 'index', 'middle', 'ring', 'pinky']

# How much each joint of a finger bends at curl = 1.0 (degrees).
JOINT_CURL = {
    'thumb':  (42.0, 46.0, 34.0),
    'index':  (80.0, 90.0, 54.0),
    'middle': (82.0, 92.0, 56.0),
    'ring':   (80.0, 90.0, 54.0),
    'pinky':  (78.0, 88.0, 52.0),
}
BASE_SPREAD = {'thumb': -30.0, 'index': 6.0, 'middle': 0.0, 'ring': -6.0, 'pinky': -12.0}
SPREAD_RANGE = {'thumb': 24.0, 'index': 16.0, 'middle': 5.0, 'ring': -9.0, 'pinky': -18.0}
THUMB_ABDUCT = 46.0


# ═══════════════════════════════════════════════════════════════════════════
# HANDSHAPE inventory
#   curl:   0 = fully extended, 1 = folded into the palm
#   spread: 0 = fingers together, 1 = fanned apart
#   abduct: thumb away from the palm (0 = tucked across, 1 = sticking out)
# ═══════════════════════════════════════════════════════════════════════════
def H(thumb, index, middle, ring, pinky, spread=0.0, abduct=0.0, **kw):
    d = dict(curl=dict(zip(FINGERS, (thumb, index, middle, ring, pinky))),
             spread=spread, abduct=abduct)
    d.update(kw)
    return d


HANDSHAPES = {
    # ── ASL manual alphabet ────────────────────────────────────────────────
    'A': H(0.05, 1.00, 1.00, 1.00, 1.00, abduct=0.25),
    'B': H(0.95, 0.00, 0.00, 0.00, 0.00),
    'C': H(0.40, 0.45, 0.45, 0.45, 0.45, spread=0.15, abduct=0.55),
    'D': H(0.30, 0.00, 0.90, 0.92, 0.92, abduct=0.30),
    'E': H(0.85, 0.80, 0.80, 0.80, 0.80),
    'F': H(0.70, 0.75, 0.00, 0.00, 0.00, spread=0.45, abduct=0.35),
    'G': H(0.15, 0.00, 1.00, 1.00, 1.00, abduct=0.55, orient=('back', 'left')),
    'H': H(0.85, 0.00, 0.00, 1.00, 1.00, orient=('back', 'left')),
    'I': H(0.85, 1.00, 1.00, 1.00, 0.00),
    'J': H(0.85, 1.00, 1.00, 1.00, 0.00, motion='J'),
    'K': H(0.25, 0.00, 0.00, 1.00, 1.00, spread=0.55, abduct=0.45),
    'L': H(0.00, 0.00, 1.00, 1.00, 1.00, abduct=1.00),
    'M': H(0.95, 0.85, 0.85, 0.85, 1.00),
    'N': H(0.95, 0.85, 0.85, 1.00, 1.00),
    'O': H(0.60, 0.62, 0.62, 0.62, 0.62, spread=0.10, abduct=0.45),
    'P': H(0.25, 0.00, 0.00, 1.00, 1.00, spread=0.55, abduct=0.45,
           orient=('fwd', 'down')),
    'Q': H(0.15, 0.00, 1.00, 1.00, 1.00, abduct=0.55, orient=('back', 'down')),
    'R': H(0.85, 0.00, 0.00, 1.00, 1.00, cross=True),
    'S': H(0.72, 1.00, 1.00, 1.00, 1.00),
    'T': H(0.55, 0.90, 1.00, 1.00, 1.00, abduct=0.20),
    'U': H(0.85, 0.00, 0.00, 1.00, 1.00),
    'V': H(0.85, 0.00, 0.00, 1.00, 1.00, spread=1.00),
    'W': H(0.90, 0.00, 0.00, 0.00, 1.00, spread=0.80),
    'X': H(0.70, 0.55, 1.00, 1.00, 1.00, hook=True),
    'Y': H(0.00, 1.00, 1.00, 1.00, 0.00, abduct=1.00),
    'Z': H(0.35, 0.00, 1.00, 1.00, 1.00, motion='Z'),

    # ── digits ─────────────────────────────────────────────────────────────
    '0': H(0.60, 0.62, 0.62, 0.62, 0.62, spread=0.10, abduct=0.45),
    '1': H(0.85, 0.00, 1.00, 1.00, 1.00),
    '2': H(0.85, 0.00, 0.00, 1.00, 1.00, spread=1.00),
    '3': H(0.00, 0.00, 0.00, 1.00, 1.00, spread=0.70, abduct=1.00),
    '4': H(0.90, 0.00, 0.00, 0.00, 0.00, spread=0.55),
    '5': H(0.00, 0.00, 0.00, 0.00, 0.00, spread=1.00, abduct=1.00),
    '6': H(0.10, 0.00, 0.00, 0.00, 0.80, spread=0.35),
    '7': H(0.15, 0.00, 0.00, 0.80, 0.00, spread=0.35),
    '8': H(0.20, 0.00, 0.80, 0.00, 0.00, spread=0.35),
    '9': H(0.70, 0.75, 0.00, 0.00, 0.00, spread=0.45, abduct=0.35),

    # ── shapes used by word signs ──────────────────────────────────────────
    'FLAT':  H(0.30, 0.00, 0.00, 0.00, 0.00, spread=0.10),
    'OPEN':  H(0.00, 0.00, 0.00, 0.00, 0.00, spread=1.00, abduct=1.00),
    'FIST':  H(0.72, 1.00, 1.00, 1.00, 1.00),
    'POINT': H(0.85, 0.00, 1.00, 1.00, 1.00),
    'CLAW':  H(0.45, 0.50, 0.50, 0.50, 0.50, spread=0.55, abduct=0.50),
    'ILY':   H(0.00, 0.00, 1.00, 1.00, 0.00, abduct=1.00, spread=0.60),
    'BENT':  H(0.40, 0.35, 0.35, 0.35, 0.35, spread=0.20),
    'REST':  H(0.22, 0.22, 0.24, 0.26, 0.28, spread=0.12),
}


# ═══════════════════════════════════════════════════════════════════════════
# LOCATION inventory — wrist targets in world space for the RIGHT (dominant)
# hand. mirror_x() flips them for the left. Solved by the IK in kinematics.py.
# ═══════════════════════════════════════════════════════════════════════════
LOCATIONS = {
    'rest':     (-0.215, 0.995, 0.045),
    'neutral':  (-0.150, 1.170, 0.285),
    'spell':    (-0.190, 1.290, 0.230),
    'chest':    (-0.090, 1.215, 0.250),
    'heart':    (-0.050, 1.240, 0.190),
    'chin':     (-0.050, 1.420, 0.175),
    'mouth':    (-0.045, 1.450, 0.160),
    'nose':     (-0.040, 1.490, 0.150),
    'brow':     (-0.070, 1.550, 0.140),
    'temple':   (-0.130, 1.535, 0.100),
    'ear':      (-0.145, 1.510, 0.040),
    'shoulder': (-0.180, 1.330, 0.195),
    'out':      (-0.330, 1.245, 0.240),
    'wide':     (-0.400, 1.290, 0.155),
    'high':     (-0.215, 1.520, 0.220),
    'low':      (-0.145, 1.060, 0.265),
    'across':   (0.075, 1.245, 0.235),
    'far':      (-0.180, 1.230, 0.380),
}

# (palm, fingers) defaults per location
DEFAULT_ORIENT = {
    'rest':     ('down', 'left'),
    'low':      ('up', 'fwd'),
    'neutral':  ('up', 'fwd'),
    'across':   ('back', 'left'),
}
GENERIC_ORIENT = ('fwd', 'up')


def mirror_x(p):
    return (-p[0], p[1], p[2])


def mirror_dir(v):
    if isinstance(v, str):
        return {'left': 'right', 'right': 'left'}.get(v, v)
    return (-v[0], v[1], v[2])


def resolve_loc(loc, side, offset=None):
    base = LOCATIONS[loc] if isinstance(loc, str) else loc
    if side == 'L':
        base = mirror_x(base)
    if offset:
        off = offset if side == 'R' else mirror_x(offset)
        base = tuple(a + b for a, b in zip(base, off))
    return base


# ═══════════════════════════════════════════════════════════════════════════
# Pose construction
# ═══════════════════════════════════════════════════════════════════════════
def hand_pose(shape_name, side):
    """Expand a handshape into per-joint euler rotations for one hand."""
    h = HANDSHAPES[shape_name]
    cs = curl_sign(side)
    mir = 1.0 if side == 'L' else -1.0
    pose = {}
    for f in FINGERS:
        c = h['curl'][f]
        base = BASE_SPREAD[f] + SPREAD_RANGE[f] * h['spread']
        if f == 'thumb':
            base += THUMB_ABDUCT * h['abduct']
        for j in range(3):
            ang = JOINT_CURL[f][j] * c
            if h.get('hook') and f == 'index':
                ang = JOINT_CURL[f][j] * (0.15 if j == 0 else 1.0)
            ry = base * mir if j == 0 else 0.0
            if h.get('cross') and f in ('index', 'middle') and j == 0:
                ry += (18.0 if f == 'index' else -18.0) * mir
            pose[f'{f}{j+1}{side}'] = (0.0, ry, cs * ang)
    return pose


def side_pose(side, loc='rest', shape='REST', palm=None, fingers=None,
              offset=None, lift=0.0):
    dp, df = DEFAULT_ORIENT.get(loc, GENERIC_ORIENT) if isinstance(loc, str) \
        else GENERIC_ORIENT
    shape_orient = HANDSHAPES[shape].get('orient')
    if shape_orient and loc == 'spell':
        dp, df = shape_orient
    palm = dp if palm is None else palm
    fingers = df if fingers is None else fingers
    if side == 'L':
        palm, fingers = mirror_dir(palm), mirror_dir(fingers)
    p = K.solve_arm(side, resolve_loc(loc, side, offset),
                    palm=palm, fingers=fingers, shoulder_lift=lift)
    p.update(hand_pose(shape, side))
    return p


def build_pose(loc='rest', shape='REST', palm=None, fingers=None, offset=None,
               lift=0.0, loc2='rest', shape2='REST', palm2=None, fingers2=None,
               offset2=None, lift2=0.0, face=None, head=(0, 0, 0), torso=(0, 0, 0)):
    """Compose a full-body pose. Unsuffixed arguments drive the dominant (right)
    hand; the `*2` arguments drive the non-dominant (left) hand."""
    p = {}
    p.update(side_pose('R', loc, shape, palm, fingers, offset, lift))
    p.update(side_pose('L', loc2, shape2, palm2, fingers2, offset2, lift2))
    if any(head):
        p['head'] = tuple(head)
        p['neck'] = tuple(v * 0.45 for v in head)
    if any(torso):
        p['chest'] = tuple(torso)
    return p, dict(face or {})


# ═══════════════════════════════════════════════════════════════════════════
# HTA encoding
# ═══════════════════════════════════════════════════════════════════════════
def _fmt(v):
    i = int(round(v))
    return '' if i == 0 else str(i)


def encode_hta(keyframes):
    """keyframes: [(frame_int, pose_dict, morph_dict)] -> HTA string."""
    frames = [k[0] for k in keyframes]
    fs = '#'.join(str(f) for f in frames)
    bones, morphs = set(), set()
    for _, pose, mo in keyframes:
        bones |= set(pose)
        morphs |= set(mo)

    out = []
    for b in sorted(bones, key=lambda x: BONE_CODE[x]):
        chans = [[k[1].get(b, (0.0, 0.0, 0.0))[a] / ANGLE_DIVISOR for k in keyframes]
                 for a in range(3)]
        if all(abs(x) < 0.5 for ch in chans for x in ch):
            continue                                   # static bone: omit it
        out.append(f'{BONE_CODE[b]}*{fs}*' +
                   '*'.join('#'.join(_fmt(x) for x in ch) for ch in chans))
    for m in sorted(morphs, key=lambda x: MORPH_CODE[x]):
        vals = [k[2].get(m, 0.0) * 100.0 for k in keyframes]
        if all(abs(x) < 0.5 for x in vals):
            continue
        out.append(f'{MORPH_CODE[m]}*{fs}*' + '#'.join(_fmt(x) for x in vals))
    return '?'.join(out)


# ═══════════════════════════════════════════════════════════════════════════
# NON-MANUAL MARKERS — grammatical, not cosmetic
# ═══════════════════════════════════════════════════════════════════════════
BROW_UP = {'browInnerUpL': 0.9, 'browInnerUpR': 0.9,      # yes/no question, topic
           'browOuterUpL': 0.75, 'browOuterUpR': 0.75}
BROW_DOWN = {'browDownL': 0.95, 'browDownR': 0.95}        # wh- question
SMILE = {'mouthSmile': 0.85}
FROWN = {'mouthFrown': 0.8}
OPEN = {'mouthOpen': 0.65}
PUCKER = {'mouthPucker': 0.75}
WIDE = {'mouthWide': 0.7}
SQUINT = {'eyeBlinkL': 0.45, 'eyeBlinkR': 0.45}


def letter_sign(ch):
    """rest -> handshape -> hold -> rest, with motion for J and Z."""
    up = ch.upper()
    shape = HANDSHAPES[up]
    motion = shape.get('motion')

    kf = [(0, *build_pose())]
    kf.append((4, *build_pose(loc='spell', shape=up)))
    if motion == 'J':                                  # J traces a hook downward
        kf.append((8, *build_pose(loc='spell', shape='J', offset=(0.0, -0.05, 0.0))))
        kf.append((12, *build_pose(loc='spell', shape='J',
                                   offset=(0.06, -0.09, -0.02))))
        end = 17
    elif motion == 'Z':                                # Z traces a zig-zag
        kf.append((7, *build_pose(loc='spell', shape='Z', offset=(-0.10, 0.02, 0.0))))
        kf.append((10, *build_pose(loc='spell', shape='Z',
                                   offset=(0.02, -0.07, 0.0))))
        kf.append((13, *build_pose(loc='spell', shape='Z',
                                   offset=(-0.10, -0.14, 0.0))))
        end = 18
    else:
        kf.append((9, *build_pose(loc='spell', shape=up)))
        end = 13
    kf.append((end, *build_pose()))
    return encode_hta(kf)


# ═══════════════════════════════════════════════════════════════════════════
# WORD SIGNS
# ═══════════════════════════════════════════════════════════════════════════
WORD_SIGNS = {
    'HELLO': [(0, {}),
              (5, dict(loc='temple', shape='FLAT', palm='fwd', fingers='up', face=SMILE)),
              (11, dict(loc='out', shape='FLAT', palm='fwd', fingers='up', face=SMILE)),
              (16, {})],

    'THANK-YOU': [(0, {}),
                  (5, dict(loc='chin', shape='FLAT', palm='back', fingers='up', face=SMILE)),
                  (12, dict(loc='neutral', shape='FLAT', palm='up', fingers='fwd',
                            face=SMILE, head=(10, 0, 0))),
                  (17, {})],

    'YES': [(0, {}),                                    # a nodding fist
            (4, dict(loc='chest', shape='FIST', palm='fwd', fingers='up', head=(-12, 0, 0))),
            (7, dict(loc='chest', shape='FIST', palm='fwd', fingers='fwd', head=(10, 0, 0))),
            (10, dict(loc='chest', shape='FIST', palm='fwd', fingers='up', head=(-12, 0, 0))),
            (13, dict(loc='chest', shape='FIST', palm='fwd', fingers='fwd', head=(8, 0, 0))),
            (18, {})],

    'NO': [(0, {}),
           (4, dict(loc='chest', shape='OPEN', palm='down', fingers='fwd',
                    face=BROW_DOWN, head=(0, 20, 0))),
           (8, dict(loc='chest', shape='D', palm='down', fingers='fwd',
                    face=BROW_DOWN, head=(0, -20, 0))),
           (12, dict(loc='chest', shape='D', palm='down', fingers='fwd',
                     face=BROW_DOWN, head=(0, 16, 0))),
           (17, {})],

    'PLEASE': [(0, {}),                                 # flat hand circles the chest
               (5, dict(loc='heart', shape='FLAT', palm='back', fingers='up',
                        face=SMILE, offset=(0.0, 0.05, 0.0))),
               (10, dict(loc='heart', shape='FLAT', palm='back', fingers='up',
                         face=SMILE, offset=(-0.05, -0.02, 0.0))),
               (15, dict(loc='heart', shape='FLAT', palm='back', fingers='up',
                         face=SMILE, offset=(0.0, 0.05, 0.0))),
               (20, {})],

    'SORRY': [(0, {}),                                  # A-hand circles, head bowed
              (5, dict(loc='heart', shape='A', palm='back', fingers='up',
                       face=FROWN, head=(14, 0, 0))),
              (10, dict(loc='heart', shape='A', palm='back', fingers='up',
                        face=FROWN, head=(14, 0, 0), offset=(-0.05, 0.03, 0.0))),
              (15, dict(loc='heart', shape='A', palm='back', fingers='up',
                        face=FROWN, head=(14, 0, 0), offset=(0.0, -0.03, 0.0))),
              (20, {})],

    'NAME': [(0, {}),                                   # two U hands tap
             (5, dict(loc='chest', shape='U', palm='left', fingers='fwd',
                      loc2='chest', shape2='U', palm2='right', fingers2='fwd',
                      offset=(0.0, 0.035, 0.0))),
             (9, dict(loc='chest', shape='U', palm='left', fingers='fwd',
                      loc2='chest', shape2='U', palm2='right', fingers2='fwd')),
             (13, dict(loc='chest', shape='U', palm='left', fingers='fwd',
                       loc2='chest', shape2='U', palm2='right', fingers2='fwd',
                       offset=(0.0, 0.035, 0.0))),
             (18, {})],

    'LEARN': [(0, {}),                                  # take from palm to head
              (5, dict(loc='low', shape='CLAW', palm='down', fingers='fwd',
                       loc2='low', shape2='FLAT', palm2='up', fingers2='fwd')),
              (11, dict(loc='brow', shape='FIST', palm='down', fingers='back',
                        loc2='low', shape2='FLAT', palm2='up', fingers2='fwd')),
              (16, {})],

    'SIGN': [(0, {}),                                   # both index fingers circle
             (4, dict(loc='chest', shape='POINT', palm='back', fingers='up',
                      loc2='chest', shape2='POINT', palm2='back', fingers2='up')),
             (8, dict(loc='shoulder', shape='POINT', palm='back', fingers='up',
                      loc2='low', shape2='POINT', palm2='back', fingers2='up')),
             (12, dict(loc='low', shape='POINT', palm='back', fingers='up',
                       loc2='shoulder', shape2='POINT', palm2='back', fingers2='up')),
             (16, dict(loc='chest', shape='POINT', palm='back', fingers='up',
                       loc2='chest', shape2='POINT', palm2='back', fingers2='up')),
             (21, {})],

    'HELP': [(0, {}),                                   # fist lifted on flat palm
             (5, dict(loc='low', shape='A', palm='left', fingers='up',
                      loc2='low', shape2='FLAT', palm2='up', fingers2='fwd')),
             (11, dict(loc='chest', shape='A', palm='left', fingers='up',
                       loc2='chest', shape2='FLAT', palm2='up', fingers2='fwd')),
             (16, {})],

    'GOOD': [(0, {}),
             (4, dict(loc='chin', shape='FLAT', palm='back', fingers='up', face=SMILE)),
             (10, dict(loc='neutral', shape='FLAT', palm='up', fingers='fwd',
                       loc2='neutral', shape2='FLAT', palm2='up', fingers2='fwd',
                       face=SMILE)),
             (15, {})],

    'BAD': [(0, {}),
            (4, dict(loc='chin', shape='FLAT', palm='back', fingers='up', face=FROWN)),
            (10, dict(loc='out', shape='FLAT', palm='down', fingers='fwd',
                      face=FROWN, head=(8, 0, 0))),
            (15, {})],

    'LOVE': [(0, {}),                                   # arms crossed on the chest
             (6, dict(loc='across', shape='FIST', palm='back', fingers='left',
                      loc2='across', shape2='FIST', palm2='back', fingers2='right',
                      face=SMILE, torso=(6, 0, 0))),
             (14, dict(loc='across', shape='FIST', palm='back', fingers='left',
                       loc2='across', shape2='FIST', palm2='back', fingers2='right',
                       face=SMILE, torso=(6, 0, 0))),
             (19, {})],

    'YOU': [(0, {}),
            (4, dict(loc='far', shape='POINT', palm='down', fingers='fwd')),
            (9, dict(loc='far', shape='POINT', palm='down', fingers='fwd')),
            (13, {})],

    'ME': [(0, {}),
           (4, dict(loc='heart', shape='POINT', palm='left', fingers='back')),
           (9, dict(loc='heart', shape='POINT', palm='left', fingers='back')),
           (13, {})],

    'WHAT': [(0, {}),                                   # open hands, brows DOWN
             (5, dict(loc='low', shape='OPEN', palm='up', fingers='fwd',
                      loc2='low', shape2='OPEN', palm2='up', fingers2='fwd',
                      face=BROW_DOWN, head=(6, 0, 0))),
             (11, dict(loc='out', shape='OPEN', palm='up', fingers='fwd',
                       loc2='out', shape2='OPEN', palm2='up', fingers2='fwd',
                       face=BROW_DOWN, head=(6, 0, 0))),
             (16, {})],

    'WHERE': [(0, {}),                                  # index shakes side to side
              (4, dict(loc='chest', shape='POINT', palm='fwd', fingers='up',
                       face=BROW_DOWN, offset=(0.05, 0, 0))),
              (7, dict(loc='chest', shape='POINT', palm='fwd', fingers='up',
                       face=BROW_DOWN, offset=(-0.05, 0, 0))),
              (10, dict(loc='chest', shape='POINT', palm='fwd', fingers='up',
                        face=BROW_DOWN, offset=(0.05, 0, 0))),
              (13, dict(loc='chest', shape='POINT', palm='fwd', fingers='up',
                        face=BROW_DOWN, offset=(-0.05, 0, 0))),
              (18, {})],

    'DEAF': [(0, {}),                                   # ear then mouth
             (5, dict(loc='ear', shape='POINT', palm='back', fingers='up')),
             (11, dict(loc='chin', shape='POINT', palm='back', fingers='up')),
             (16, {})],

    'UNDERSTAND': [(0, {}),                             # S at the brow flicks to 1
                   (4, dict(loc='brow', shape='S', palm='back', fingers='up',
                            face=BROW_UP)),
                   (9, dict(loc='brow', shape='1', palm='back', fingers='up',
                            face=BROW_UP)),
                   (14, {})],

    'WELCOME': [(0, {}),                                # inviting sweep inward
                (5, dict(loc='wide', shape='FLAT', palm='up', fingers='fwd',
                         face=SMILE)),
                (12, dict(loc='chest', shape='FLAT', palm='up', fingers='left',
                          face=SMILE)),
                (17, {})],

    'WORLD': [(0, {}),                                  # W hands orbit each other
              (5, dict(loc='chest', shape='W', palm='back', fingers='up',
                       loc2='chest', shape2='W', palm2='back', fingers2='up',
                       offset=(0.0, 0.06, 0.0))),
              (11, dict(loc='chest', shape='W', palm='back', fingers='up',
                        loc2='chest', shape2='W', palm2='back', fingers2='up',
                        offset=(0.0, -0.05, 0.06))),
              (16, {})],

    'QUESTION': [(0, {}),                               # X hand + raised brows
                 (5, dict(loc='chest', shape='X', palm='fwd', fingers='up',
                          face=BROW_UP)),
                 (11, dict(loc='chest', shape='X', palm='fwd', fingers='up',
                           face=BROW_UP, offset=(0.0, -0.05, 0.0))),
                 (16, {})],
}


def word_sign(frames):
    return encode_hta([(f, *build_pose(**kw)) for f, kw in frames])


# ═══════════════════════════════════════════════════════════════════════════
def main(outdir):
    out = Path(outdir)
    out.mkdir(parents=True, exist_ok=True)
    for f in out.glob('*.hta'):
        try:
            f.unlink()
        except OSError:
            pass                    # read-only mount: files get overwritten instead

    manifest = {'rigVersion': RIG_VERSION, 'fps': FPS,
                'angleDivisor': ANGLE_DIVISOR, 'letters': {}, 'words': {}}
    total = 0

    for ch in list('abcdefghijklmnopqrstuvwxyz0123456789'):
        hta = letter_sign(ch)
        (out / f'@{ch}.hta').write_text(hta)
        manifest['letters'][ch] = {'id': f'@{ch}', 'bytes': len(hta)}
        total += len(hta)

    for name, shape, loc in (('space', 'REST', 'rest'), ('period', 'FIST', 'spell'),
                             ('question', '1', 'spell'), ('exclamation', '1', 'spell'),
                             ('at', 'C', 'spell')):
        kf = [(0, *build_pose()),
              (5, *build_pose(loc=loc, shape=shape)),
              (9, *build_pose(loc=loc, shape=shape)),
              (13, *build_pose())]
        hta = encode_hta(kf)
        (out / f'@{name}.hta').write_text(hta)
        manifest['letters'][name] = {'id': f'@{name}', 'bytes': len(hta)}
        total += len(hta)

    rest = encode_hta([(0, *build_pose()), (8, *build_pose())])
    (out / '@rest.hta').write_text(rest)
    manifest['letters']['rest'] = {'id': '@rest', 'bytes': len(rest)}

    for gloss, frames in WORD_SIGNS.items():
        hta = word_sign(frames)
        (out / f'{gloss}.hta').write_text(hta)
        manifest['words'][gloss] = {'bytes': len(hta),
                                    'durationMs': int(max(f for f, _ in frames) / FPS * 1000)}
        total += len(hta)

    (out / 'manifest.json').write_text(json.dumps(manifest, indent=2))
    nl, nw = len(manifest['letters']), len(manifest['words'])
    print(f'✓ {out}')
    print(f'  {nl} fingerspelling entries + {nw} word signs')
    print(f'  {total/1024:.1f} KB total · avg {total//(nl+nw)} bytes per sign')
    return 0


if __name__ == '__main__':
    ap = argparse.ArgumentParser()
    ap.add_argument('--out', default=str(Path(__file__).parent.parent / 'data/signs/ase'))
    sys.exit(main(ap.parse_args().out))
