# deskd — Agent Runtime (Rust)

## What is deskd

A separate Rust project that handles agent orchestration: spawning, routing, context management, and cost tracking. myhome manages the workspace (repos, tools, auth), deskd manages the agents.

**Repo**: https://github.com/kira-autonoma/deskd

## Architecture Split

```
myhome (Go) — workspace manager        deskd (Rust) — agent runtime
├── Repos (clone, sync, worktrees)      ├── Agent lifecycle (create/send/list/rm)
├── Tools (mise, go, node, python)      ├── Prompt routing (promptlint)
├── Packages (brew, apt)                ├── Context pool + forks + queues
├── Auth (SSH, git identity)            ├── Cost tracking + budgets
├── Containers (build, run)             ├── Task API (HTTP/gRPC)
├── Vault (KeePassXC)                   └── Economy (TON payments, future)
├── Remote (SSH + tmux)
└── Schedule (cron tasks)
```

myhome calls deskd for agent operations. deskd doesn't know about repos or packages.

## How They Work Together

```bash
myhome init --env work          # sets up machine: repos, tools, packages, auth
myhome container build claude   # builds Docker image

deskd agent create dev \        # creates agent with scoped access
  --workdir ~/work/uagent \
  --model claude-sonnet-4-20250514

deskd agent send dev "fix the auth bug"   # agent works, cost tracked
deskd agent stats dev                      # shows turns, cost, session info
```

## Domain Model

### Agents (top-level, life/work domains)

| Agent   | Scope                      | What it does                            |
|---------|----------------------------|-----------------------------------------|
| dev     | repos, CI/CD, code         | Software development                    |
| school  | Gmail, Telegram channel    | School notifications + follow-ups       |
| analyst | web access, Playwright     | Research, comparisons, price tracking   |
| manager | calendar, tasks, people    | Coordination, reminders, collaboration  |
| home    | booking sites, monitoring  | Reservations, monitoring                |
| collab  | shared repos, channels     | Shared work with collaborators          |

### Data Access Model

```
Filesystem:
  /shared/              ← all agents read (preferences, contacts, calendar)
  /desks/dev/           ← only dev agent
  /desks/school/        ← only school agent
  /desks/home/          ← only home agent

Context:
  /context/dev/         ← dev agent's discoveries, decisions
  /context/school/      ← school agent's knowledge
  /context/shared/      ← cross-agent facts ("family travels to Karlsruhe on Easter")
```

**Access rules:**
1. Physical data (files, APIs): ONLY own desk
2. /shared/: all read, only owner writes
3. Own context: full RW
4. Other agent's context: only if published or answered a query
5. Credentials: NEVER shared between agents
6. Urgent events: bypass — all agents receive

### Context Queue (event-driven coordination)

```
Agent A: "found JWT bug in auth module" → publishes to queue
Agent B: subscribed to Agent A → receives update → knows about JWT

No direct agent-to-agent calls. Loose coupling via events.
```

### Prompt Routing (promptlint integration)

```
Request arrives → promptlint analyze → complexity?
  ├── high → Claude Opus subprocess (subscription)
  ├── medium → Claude Sonnet subprocess
  ├── low → Gemini Flash / cheap model (API key)
  └── trivial → template response, no LLM
```

No API proxying — routes to different subprocesses. Each subprocess manages its own auth.

## Related Issues

| Issue | What |
|-------|------|
| [#44](https://github.com/kgatilin/myhome/issues/44) | Process-mode sub-agents |
| [#46](https://github.com/kgatilin/myhome/issues/46) | --max-turns + cost tracking |
| [#48](https://github.com/kgatilin/myhome/issues/48) | Hook/plugin API |
| [#49](https://github.com/kgatilin/myhome/issues/49) | Prompt routing |
| [#50](https://github.com/kgatilin/myhome/issues/50) | Quality gates |
| [#51](https://github.com/kgatilin/myhome/issues/51) | Cost tracking (costlint) |
| [#52](https://github.com/kgatilin/myhome/issues/52) | Task creation API |
| [#53](https://github.com/kgatilin/myhome/issues/53) | Context management |
| [#55](https://github.com/kgatilin/myhome/issues/55) | TON agent economy |

## deskd CLI Reference

```bash
deskd agent create <name> [--prompt "..."] [--model ...] [--workdir ...] [--max-turns N]
deskd agent send <name> "message" [--max-turns N]
deskd agent list
deskd agent stats <name>
deskd agent rm <name>
```

State stored in `~/.deskd/agents/*.yaml`. Cost and session tracked per agent.
