# flywheel-long-running-research

> Agents learn daily. Vault notes go stale. Flywheel closes the loop.

A Go CLI that syncs tagged learnings from agent daily logs into your Obsidian vault, with semantic matching via QMD.

## How It Works

Agents mark learnings in their daily logs:

```markdown
[LEARNING] topic | the actual learning
[UPDATE] topic | something that changed
[STALE] topic | this note is outdated
```

Flywheel reads those, uses QMD to semantically match topics to existing vault notes, and creates/updates accordingly.

## Install

```bash
go install github.com/cyperx84/flywheel-long-running-research/cmd/flywheel@latest
```

## Commands

### `flywheel sync`

```bash
flywheel sync                          # today's logs, all agents
flywheel sync --since 2026-03-15       # date range
flywheel sync --agent builder          # single agent
flywheel sync --dry-run                # preview only
flywheel sync --json                   # machine-readable output
```

Uses QMD for semantic matching when available. Falls back to topic ID matching.

### `flywheel freshness`

```bash
flywheel freshness                     # notes stale 30+ days
flywheel freshness --days 60           # custom threshold
flywheel freshness --json
```

### `flywheel verify`

```bash
flywheel verify "ollama-setup"         # mark as current
flywheel verify --all                  # verify everything
```

## Matching

1. **QMD semantic** (preferred): queries the vault's QMD index to find the best matching note by topic + content
2. **Topic ID fallback**: converts topic to kebab-case slug (e.g., `ollama-setup` → `ollama-setup.md`)

## Config

`~/.config/flywheel/config.json`:

```json
{
  "agents": ["builder", "researcher", "ops"],
  "workspace": "~/.openclaw/agents",
  "vault": "~/path/to/obsidian/vault",
  "freshness_days": 30,
  "qmd_bin": "~/.bun/bin/qmd"
}
```

## Dependencies

- `obsidian-cli` for vault writes
- `qmd` for semantic matching (optional — falls back to topic ID)

## Cron

A daily cron job runs `flywheel sync` at 8am Brisbane time and posts results to Discord.

## License

MIT
