"""
kinematics.py — two-bone analytic IK for the arms, plus euler helpers.

Sign languages describe a sign by its *location* ("at the chin", "in neutral
space in front of the chest"), not by joint angles. Authoring signs as euler
triples therefore fights the notation. This module lets build_signs.py say
"put the right wrist here, with the palm facing there" and solves the shoulder,
elbow and wrist rotations that achieve it.

Bind pose has no rotations, so every bone's parent frame is world-aligned,
which keeps the conversion from world-space directions to local euler angles
straightforward.
"""
import math

import numpy as np

from rig import BONE_OFFSET, world_rest

REST = world_rest()

L_UPPER = abs(BONE_OFFSET['forearmL'][0])     # shoulder -> elbow
L_FORE = abs(BONE_OFFSET['handL'][0])         # elbow -> wrist


def _norm(v):
    v = np.asarray(v, dtype=np.float64)
    n = np.linalg.norm(v)
    return v / n if n > 1e-12 else np.array([1.0, 0.0, 0.0])


def axis_angle(axis, angle):
    a = _norm(axis)
    c, s = math.cos(angle), math.sin(angle)
    x, y, z = a
    return np.array([
        [c + x * x * (1 - c),     x * y * (1 - c) - z * s, x * z * (1 - c) + y * s],
        [y * x * (1 - c) + z * s, c + y * y * (1 - c),     y * z * (1 - c) - x * s],
        [z * x * (1 - c) - y * s, z * y * (1 - c) + x * s, c + z * z * (1 - c)],
    ])


def rot_between(a, b):
    """Minimal rotation matrix taking unit vector a onto unit vector b."""
    a, b = _norm(a), _norm(b)
    d = float(np.clip(np.dot(a, b), -1.0, 1.0))
    if d > 1 - 1e-9:
        return np.eye(3)
    if d < -1 + 1e-9:
        perp = np.array([1.0, 0.0, 0.0])
        if abs(a[0]) > 0.9:
            perp = np.array([0.0, 1.0, 0.0])
        return axis_angle(np.cross(a, perp), math.pi)
    return axis_angle(np.cross(a, b), math.acos(d))


def to_euler_xyz(R):
    """Decompose R = Rx(x) @ Ry(y) @ Rz(z) into degrees."""
    sy = float(np.clip(R[0, 2], -1.0, 1.0))
    y = math.asin(sy)
    if abs(sy) < 0.9999:
        x = math.atan2(-R[1, 2], R[2, 2])
        z = math.atan2(-R[0, 1], R[0, 0])
    else:
        x = math.atan2(R[2, 1], R[1, 1])
        z = 0.0
    return tuple(math.degrees(v) for v in (x, y, z))


DIRS = {
    'fwd': (0, 0, 1), 'back': (0, 0, -1), 'up': (0, 1, 0), 'down': (0, -1, 0),
    'left': (1, 0, 0), 'right': (-1, 0, 0),
}


def d(v):
    """Accept either a named direction or a raw 3-vector."""
    return _norm(DIRS[v] if isinstance(v, str) else v)


def hand_frame(side, fingers, palm):
    """World rotation for the hand: `fingers` is where the hand axis points,
    `palm` is where the palm faces. Bind pose: axis = ±X, palm normal = -Y."""
    sx = 1.0 if side == 'L' else -1.0
    f = d(fingers)
    p = d(palm)
    p = _norm(p - f * float(np.dot(p, f)))            # orthogonalise
    if np.linalg.norm(p) < 1e-6:
        p = _norm(np.cross(f, [0, 1, 0]))
    t = np.cross(f, p)
    target = np.column_stack([f, p, t])

    bf = np.array([sx, 0.0, 0.0])
    bp = np.array([0.0, -1.0, 0.0])
    bt = np.cross(bf, bp)
    bind = np.column_stack([bf, bp, bt])
    return target @ bind.T


def solve_arm(side, target, pole=None, palm='fwd', fingers=None, shoulder_lift=0.0):
    """Solve one arm from a wrist position plus hand orientation.

    side           'L' or 'R'
    target         world-space wrist position
    pole           direction the elbow should point (default: down and back)
    palm           where the palm faces  ('fwd', 'down', ... or a vector)
    fingers        where the fingers point; default = continue the forearm
    shoulder_lift  extra shrug about Z, degrees

    Returns {bone: (rx, ry, rz) degrees}.
    """
    sx = 1.0 if side == 'L' else -1.0
    rest_dir = np.array([sx, 0.0, 0.0])
    S = np.array(REST[f'upperArm{side}'], dtype=np.float64)

    sh_rot = np.eye(3)
    if abs(shoulder_lift) > 1e-6:
        sh_rot = axis_angle([0, 0, 1], math.radians(shoulder_lift * sx))
        anchor = np.array(REST[f'shoulder{side}'], dtype=np.float64)
        S = anchor + sh_rot @ (S - anchor)

    T = np.asarray(target, dtype=np.float64)
    to_t = T - S
    dist = float(np.linalg.norm(to_t))
    reach = L_UPPER + L_FORE
    dist = max(min(dist, reach * 0.995), abs(L_UPPER - L_FORE) + 1e-4)
    dir_t = _norm(to_t)

    cos_a = (L_UPPER ** 2 + dist ** 2 - L_FORE ** 2) / (2 * L_UPPER * dist)
    a = math.acos(float(np.clip(cos_a, -1.0, 1.0)))

    if pole is None:
        pole = np.array([-sx * 0.35, -0.72, -0.60])   # elbow hangs down, back, out
    pole = _norm(pole)
    swing_axis = np.cross(dir_t, pole)
    if np.linalg.norm(swing_axis) < 1e-6:
        swing_axis = np.cross(dir_t, [0.0, 0.0, 1.0])
    u = _norm(axis_angle(swing_axis, -a) @ dir_t)
    E = S + u * L_UPPER
    v = _norm(T - E)

    R_upper_world = rot_between(rest_dir, u)
    R_fore_world = rot_between(rest_dir, v)
    R_fore_local = R_upper_world.T @ R_fore_world

    R_hand_world = hand_frame(side, fingers if fingers is not None else v, palm)
    R_hand_local = R_fore_world.T @ R_hand_world

    return {
        f'shoulder{side}': (0.0, 0.0, shoulder_lift * sx),
        f'upperArm{side}': to_euler_xyz(sh_rot.T @ R_upper_world),
        f'forearm{side}': to_euler_xyz(R_fore_local),
        f'hand{side}': to_euler_xyz(R_hand_local),
    }


def wrist_of(side, pose):
    """Forward-kinematics check: where does this pose actually put the wrist?"""
    import preview
    av_pose = dict(pose)
    chain = ['root', 'hips', 'spine', 'chest', f'shoulder{side}',
             f'upperArm{side}', f'forearm{side}', f'hand{side}']
    M = np.eye(4)
    for b in chain:
        m = np.eye(4)
        m[:3, 3] = BONE_OFFSET[b]
        r = av_pose.get(b)
        if r and any(abs(x) > 1e-9 for x in r):
            m[:3, :3] = preview.euler_xyz(*[math.radians(v) for v in r])
        M = M @ m
    return M[:3, 3]
