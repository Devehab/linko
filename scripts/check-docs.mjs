// The site is one hand-written HTML file per page, so a stray character in an
// inline <script> takes the whole page down silently: no language toggle, no
// theme switch, no copy buttons, and no error anywhere a visitor would see.
// This catches exactly that, plus the two markup slips that have bitten us.
import { readFileSync, writeFileSync, mkdtempSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const pages = ['docs/index.html', 'docs/guide.html'];
const tmp = mkdtempSync(join(tmpdir(), 'linko-docs-'));
let failed = 0;
const fail = (m) => { console.error(`  ✗ ${m}`); failed = 1; };

for (const page of pages) {
  console.log(page);
  const html = readFileSync(page, 'utf8');

  // 1 · every inline script must parse
  const scripts = [...html.matchAll(/<script(?![^>]*\bsrc=)[^>]*>([\s\S]*?)<\/script>/g)];
  if (!scripts.length) fail('no inline script found — did the page change shape?');
  scripts.forEach(([, body], i) => {
    const f = join(tmp, `s${i}.js`);
    writeFileSync(f, body);
    try {
      execFileSync(process.execPath, ['--check', f], { stdio: 'pipe' });
      console.log(`  ✓ script #${i} parses`);
    } catch (e) {
      fail(`script #${i} does not parse\n${e.stderr?.toString().trim()}`);
    }
  });

  // 2 · the controls the script binds to must exist
  for (const id of ['lang', 'theme']) {
    if (!html.includes(`id="${id}"`)) fail(`missing #${id}`);
  }
  for (const fn of ['function setLang', 'function setTheme']) {
    if (!html.includes(fn)) fail(`missing ${fn}`);
  }

  // 3 · the bilingual switch needs both halves of the rule
  if (!/\[data-ar\]\{display:none\}/.test(html)) fail('missing the [data-ar] hide rule');
  if (!/html\[lang="ar"\] \[data-en\]\{display:none\}/.test(html)) fail('missing the Arabic swap rule');

  // 4 · nothing may reference a brand asset that is not in the repo
  for (const [, href] of html.matchAll(/(?:src|href)="(brand\/[^"]+)"/g)) {
    try { readFileSync(join('docs', href)); } catch { fail(`missing asset ${href}`); }
  }
  console.log('  ✓ controls, language rules and brand assets present');
}
process.exit(failed);
