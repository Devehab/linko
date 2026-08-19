# linko for AI agents

`linko/SKILL.md` teaches a coding agent when to reach for linko and, more
importantly, how not to get it wrong: that `linko 3000` blocks forever without
`-d`, that `~/.linko/config.json` holds live credentials, that a published URL
is public and needs the user's say-so, and that exit code 0 is not proof the URL
answers.

It is plain Markdown with YAML frontmatter, so it works as a skill where skills
exist and as a context file everywhere else.

## Installing it

**Claude Code / Cowork** — copy the folder into the skills directory:

```bash
git clone https://github.com/Devehab/linko.git /tmp/linko
mkdir -p ~/.claude/skills
cp -r /tmp/linko/skills/linko ~/.claude/skills/
```

Project-scoped instead of global: `.claude/skills/linko/` inside the repo.

**Codex, Cursor, Windsurf, Aider, Continue** — point the agent's instruction
file at it, or paste it in:

```bash
curl -fsSL https://raw.githubusercontent.com/Devehab/linko/main/skills/linko/SKILL.md \
  >> AGENTS.md          # or CLAUDE.md, .cursorrules, .windsurfrules
```

**Anything else** — hand it the raw URL:

```
https://raw.githubusercontent.com/Devehab/linko/main/skills/linko/SKILL.md
```

## What it covers

- when linko is the right tool, and when it is not
- installing, and the Cloudflare token the user has to create themselves
- publishing without blocking the session
- reading the URL back without touching the tokens next to it
- verifying with a real request and a `cf-ray` header
- every failure this project has actually hit, with its cause
- cleaning up, so a public URL is not left running

## The rule that matters most

A published URL is reachable by anyone who has it, with nothing in front of it.
An agent should confirm what it is about to expose before exposing it — and
should never expose a database, an admin panel or a file server. That is stated
plainly at the top of the skill, not buried at the bottom.
