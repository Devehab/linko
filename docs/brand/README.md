# linko brand assets

Two links of a chain, interlocked — the oldest and most legible symbol for a
hyperlink, drawn properly: a true over/under weave, one break per link, a
consistent gap, and perfect 180° rotational symmetry. The wordmark is monoline
geometry on the same 10.5-unit stroke, so the **o** rhymes with the links.

Every file here is generated geometry — pure paths, no masks, no embedded fonts,
no external references. That is what lets one file serve as a favicon at 16 px, a
nav mark, a README header and a 1200×630 social card without a second version
drifting out of sync.

| File | Use |
| --- | --- |
| `logotype.svg` · `.png` | **The default.** Mark + wordmark. Use this wherever there is room. |
| `logotype-adaptive.svg` | Same lockup, but the word inherits `currentColor` — for inlining into a themed page. |
| `logo.svg` | The mark alone. Only where the name is already present. |
| `logo-mono.svg` | The mark in `currentColor`. |
| `wordmark.svg` | The word alone. |
| `icon.svg` · `apple-touch-icon.png` | App icon, on the dark background. |
| `favicon.svg` | Tighter margins, for 16–32 px. |
| `og.png` | Social preview card, 1200×630. |

## Colour

The gradient runs `#FFB457 → #F6821F → #DE4E08` on the 135° diagonal. `#F6821F`
alone is correct wherever a single flat colour is needed. On a dark surface use
`#0B0D12` behind it; the lockup also holds up on white unmodified.

## Rules

Prefer the full lockup — the mark alone reads as a generic link icon without the
name beside it. Leave clear space equal to one link's width on every side. The
mark stays legible down to 16 px; below that it should not be used. Never rebuild
the wordmark in a system font, recolour the gradient, rotate the mark off its 45°
axis, or pull the two links apart.
