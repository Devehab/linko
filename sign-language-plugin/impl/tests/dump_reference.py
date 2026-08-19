#!/usr/bin/env python3
"""
dump_reference.py — emits ground-truth poses for the decoder-equivalence test.

The compiler (Python) and the runtime decoder (JavaScript) are two independent
implementations of the same wire format. If they ever disagree about what a
payload means, every sign in the dictionary is silently wrong and nothing will
throw. So we sample every clip at several times, dump the resulting quaternions
and morph influences here, and let tests/test.js assert the browser decoder
reproduces them.
"""
import json
import math
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent / 'tools'))
from preview import sample_hta, hta_duration          # noqa: E402
from rig import BONE_NAMES, MORPHS                    # noqa: E402

SIGNS_DIR = Path(__file__).parent.parent / 'data/signs/ase'
SAMPLES = [0.0, 0.25, 0.5, 0.75, 1.0]


def main():
    out = {}
    for f in sorted(SIGNS_DIR.glob('*.hta')):
        hta = f.read_text()
        dur = hta_duration(hta)
        frames = []
        for s in SAMPLES:
            t = dur * s
            pose, morphs = sample_hta(hta, t)
            frames.append({
                't': round(t, 6),
                'bones': {b: [round(v, 6) for v in pose[b]] for b in sorted(pose)},
                'morphs': {m: round(v, 6) for m, v in sorted(morphs.items())},
            })
        out[f.stem] = {'duration': round(dur, 6), 'frames': frames}

    dest = Path(__file__).parent / 'reference_poses.json'
    dest.write_text(json.dumps({
        'boneNames': BONE_NAMES, 'morphNames': MORPHS,
        'samples': SAMPLES, 'signs': out,
    }))
    n = sum(len(v['frames']) for v in out.values())
    print(f'✓ {dest.name}: {len(out)} signs × {len(SAMPLES)} samples = {n} poses '
          f'({dest.stat().st_size / 1024:.0f} KB)')


if __name__ == '__main__':
    main()
