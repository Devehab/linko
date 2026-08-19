'use strict';
/**
 * lexicon.js — the translation engine.
 *
 * Turns a sentence of written English into a sequence of ASL glosses. This is
 * deliberately rule-based, not neural: for sign language, retrieval from a
 * human-reviewed dictionary beats generation, and a rule engine is auditable by
 * the Deaf translators who have to sign off on the output.
 *
 * The pipeline mirrors what the traffic capture showed Hand Talk's backend
 * doing, including the two behaviours that are easy to miss:
 *
 *   • function words are DROPPED, not translated — they surface as an empty
 *     array in the response, because ASL has no articles or copula;
 *   • anything not in the dictionary is FINGERSPELLED letter by letter.
 */

// ── Words with no ASL equivalent. Dropping these is grammar, not an omission.
const FUNCTION_WORDS = new Set([
  'a', 'an', 'the', 'is', 'are', 'am', 'was', 'were', 'be', 'been', 'being',
  'of', 'to', 'at', 'by', 'for', 'with', 'from', 'as', 'into', 'onto',
  'do', 'does', 'did', 'has', 'have', 'had', 'that', 'this', 'these', 'those',
  'and', 'or', 'but', 'so', 'than', 'then', 'there', 'it', 'its',
]);

// ── Time adverbs move to the front of the clause in ASL.
const TIME_WORDS = new Set([
  'yesterday', 'today', 'tomorrow', 'now', 'later', 'before', 'after',
  'always', 'never', 'sometimes', 'often', 'soon', 'already', 'morning',
  'night', 'week', 'month', 'year',
]);

// ── wh- words move to the END of the clause and carry lowered brows.
const WH_WORDS = new Set(['what', 'where', 'when', 'who', 'why', 'how', 'which']);

/**
 * gloss -> the surface forms that map onto it.
 * Keeping the mapping in this direction makes it obvious which English words a
 * reviewer has actually signed off on for a given sign.
 */
const LEMMAS = {
  HELLO: ['hello', 'hi', 'hey', 'greetings'],
  'THANK-YOU': ['thanks', 'thank', 'thankyou', 'grateful', 'appreciate'],
  YES: ['yes', 'yeah', 'yep', 'correct', 'right', 'true', 'agree'],
  NO: ['no', 'not', 'nope', 'never', "don't", 'dont', "doesn't", 'doesnt',
    "isn't", 'isnt', "won't", 'wont', "can't", 'cant'],
  PLEASE: ['please', 'kindly'],
  SORRY: ['sorry', 'apologise', 'apologize', 'apology', 'regret'],
  NAME: ['name', 'named', 'called', 'names'],
  LEARN: ['learn', 'learns', 'learning', 'learned', 'study', 'studies'],
  SIGN: ['sign', 'signs', 'signing', 'signed', 'asl', 'signlanguage'],
  HELP: ['help', 'helps', 'helping', 'helped', 'assist', 'support'],
  GOOD: ['good', 'great', 'nice', 'fine', 'well', 'excellent', 'better', 'best'],
  BAD: ['bad', 'terrible', 'awful', 'poor', 'worse', 'worst', 'wrong'],
  LOVE: ['love', 'loves', 'loved', 'loving', 'adore'],
  YOU: ['you', 'your', 'yours', 'yourself', "you're", 'youre'],
  ME: ['i', 'me', 'my', 'mine', 'myself', "i'm", 'im'],
  WHAT: ['what', "what's", 'whats'],
  WHERE: ['where', "where's", 'wheres'],
  DEAF: ['deaf', 'deafness', 'hoh'],
  UNDERSTAND: ['understand', 'understands', 'understood', 'comprehend', 'realise',
    'realize', 'know', 'knows', 'knew'],
  WELCOME: ['welcome', 'welcomes', 'welcomed'],
  WORLD: ['world', 'global', 'worldwide', 'earth', 'everywhere'],
  QUESTION: ['question', 'questions', 'ask', 'asks', 'asked', 'wonder'],
};

const LOOKUP = new Map();
for (const [gloss, forms] of Object.entries(LEMMAS)) {
  for (const f of forms) LOOKUP.set(f, gloss);
}

// Very small suffix stripper. A real deployment would use a proper lemmatiser;
// this is honest about being a stub.
const SUFFIXES = ['ing', 'ed', 'es', 's', 'ly'];

function lemmatise(word) {
  if (LOOKUP.has(word)) return LOOKUP.get(word);
  for (const suf of SUFFIXES) {
    if (word.length > suf.length + 2 && word.endsWith(suf)) {
      const stem = word.slice(0, -suf.length);
      if (LOOKUP.has(stem)) return LOOKUP.get(stem);
      if (LOOKUP.has(stem + 'e')) return LOOKUP.get(stem + 'e');
    }
  }
  return null;
}

// ── text normalisation ─────────────────────────────────────────────────────
function normalise(text) {
  return String(text)
    .replace(/&nbsp;/g, ' ')
    .replace(/[‘’]/g, "'")
    .replace(/[“”]/g, '"')
    .replace(/\t+/g, ' ')
    .replace(/ +/g, ' ')
    .replace(/\n+/g, '\n')
    .trim();
}

// Sentence splitting that does not trip over common abbreviations.
const ABBREV = /\b(mr|mrs|ms|dr|prof|st|vs|etc|inc|ltd|jr|sr|e\.g|i\.e|u\.s)\.$/i;

function splitSentences(text) {
  const out = [];
  let buf = '';
  for (const tok of text.split(/(\s+)/)) {
    buf += tok;
    if (/[.!?]["')\]]?\s*$/.test(tok) && !ABBREV.test(tok.trim())) {
      if (buf.trim()) out.push(buf.trim());
      buf = '';
    }
  }
  if (buf.trim()) out.push(buf.trim());
  return out.length ? out : [text];
}

const FINGERSPELLABLE = /^[a-z0-9]$/;
const PUNCT_SIGN = { '.': '@period', '?': '@question', '!': '@exclamation', '@': '@at' };

function fingerspell(word) {
  const ids = [];
  for (const ch of word.toLowerCase()) {
    if (FINGERSPELLABLE.test(ch)) ids.push('@' + ch);
    else if (PUNCT_SIGN[ch]) ids.push(PUNCT_SIGN[ch]);
    // everything else is silently skipped, exactly like the reference plugin
  }
  return ids;
}

/**
 * Translate one sentence into an array of glosses.
 * Each gloss is { type, gloss?, ids?, source } — the caller decides how to
 * serialise it.
 */
function translateSentence(sentence) {
  const raw = sentence.toLowerCase().split(/\s+/).filter(Boolean);
  const words = raw.map((w) => ({
    surface: w,
    clean: w.replace(/^[^a-z0-9@']+|[^a-z0-9@']+$/g, ''),
  })).filter((w) => w.clean);

  const time = [];
  const wh = [];
  const body = [];

  for (const w of words) {
    if (TIME_WORDS.has(w.clean)) { time.push(w); continue; }
    if (WH_WORDS.has(w.clean)) { wh.push(w); continue; }
    body.push(w);
  }

  // ASL clause order: TIME → TOPIC/COMMENT → WH
  const ordered = [...time, ...body, ...wh];
  const glosses = [];

  for (const w of ordered) {
    if (FUNCTION_WORDS.has(w.clean)) {
      glosses.push({ type: 'dropped', source: w.surface, ids: [] });
      continue;
    }
    const g = lemmatise(w.clean);
    if (g) {
      glosses.push({ type: 'sign', gloss: g, source: w.surface, ids: [g] });
    } else {
      glosses.push({ type: 'fingerspell', source: w.surface, ids: fingerspell(w.clean) });
    }
  }

  // Non-manual marker: a wh- word lowers the brows across the whole clause;
  // a sentence ending in "?" without a wh- word raises them (yes/no question).
  let nmm = null;
  if (wh.length) nmm = 'brow-down';
  else if (/\?\s*$/.test(sentence)) nmm = 'brow-up';

  return { glosses, nmm };
}

function translate(text, { maxTextSize = 800 } = {}) {
  const clean = normalise(text);
  if (!clean) return { sentences: [], truncated: false };
  const truncated = clean.length > maxTextSize;
  const body = truncated ? clean.slice(0, maxTextSize) : clean;

  const sentences = splitSentences(body).map((s) => ({
    source: s,
    ...translateSentence(s),
  }));
  return { sentences, truncated };
}

/**
 * Serialise into the wire shape: sentences[] -> glosses[] -> ids[].
 * A dropped word becomes an empty array — that is meaningful, not noise: it
 * tells the client the word was consumed and intentionally not signed.
 */
function toWire(result) {
  return result.sentences.map((s) => s.glosses.map((g) => g.ids));
}

module.exports = {
  translate, toWire, fingerspell, normalise, splitSentences,
  LEMMAS, FUNCTION_WORDS, WH_WORDS, TIME_WORDS,
  GLOSSES: Object.keys(LEMMAS),
};
