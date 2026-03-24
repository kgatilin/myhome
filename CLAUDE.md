# myhome — Personal Workspace Manager

## What is this

A Go CLI tool that manages a git-tracked home folder across multiple machines and environments.
See `docs/init.md` for full scope and requirements, `docs/implementation-plan.md` for the build plan.

## System Architecture: myhome + deskd

myhome is the **workspace manager** — it handles machine setup, repos, tools, containers, auth.
Agent orchestration is handled by a separate Rust project: **[deskd](https://github.com/kira-autonoma/deskd)**.

```
myhome (Go) — workspace                deskd (Rust) — agent runtime
├── repos, tools, packages             ├── agent lifecycle (create/send/list/rm)
├── containers (build, Dockerfile)      ├── prompt routing (promptlint)
├── auth profiles, SSH, git identity    ├── context pool + forks + queues
├── remote VPS management               ├── cost tracking + budgets
├── myhome init --env work              ├── task API (HTTP)
└── calls deskd for agent ops          └── session resume, stream-json parsing
```

### Data Access Model

Each agent has a **Desk** (scoped data access):
- **Physical data**: filesystem mounts, API tokens, credentials — ONLY own desk
- **Shared data**: `/shared/` — all agents read, only owner writes
- **Context**: own context RW, other agents' context via publish/subscribe/query
- **Credentials**: NEVER shared between agents

```
agents:
  dev:     desk=~/work/   creds=GitHub,GitLab  tools=Claude,MCP,archlint
  school:  desk=Gmail     creds=Google OAuth    tools=email,Telegram
  analyst: desk=web       creds=—               tools=search,Playwright
```

Context sharing between agents uses event queues:
- Agent publishes discoveries (visibility: all/specific/none)
- Other agents subscribe to event types (schedule, warnings, decisions)
- Queries: agent can request context from another, owner decides to share or deny

### Related Projects

| Project | Language | Role |
|---------|----------|------|
| [myhome](https://github.com/kgatilin/myhome) | Go | Workspace manager |
| [deskd](https://github.com/kira-autonoma/deskd) | Rust | Agent orchestration runtime |
| [promptlint](https://github.com/mikeshogin/promptlint) | Go | Prompt complexity analysis + routing |
| [archlint](https://github.com/mshogin/archlint) | Go | Architecture quality validation |

### Key Issues Tracking Agent Architecture

- [#44](https://github.com/kgatilin/myhome/issues/44) — Process-mode sub-agents (PR #45)
- [#46](https://github.com/kgatilin/myhome/issues/46) — --max-turns + cost tracking (PR #47)
- [#48](https://github.com/kgatilin/myhome/issues/48) — Hook/Plugin API (pre-route, post-validate)
- [#49](https://github.com/kgatilin/myhome/issues/49) — Prompt routing via promptlint
- [#50](https://github.com/kgatilin/myhome/issues/50) — Quality gate stage in workflows
- [#51](https://github.com/kgatilin/myhome/issues/51) — Cost tracking (costlint)
- [#52](https://github.com/kgatilin/myhome/issues/52) — Task creation HTTP API
- [#53](https://github.com/kgatilin/myhome/issues/53) — Context management + forks + queues
- [#55](https://github.com/kgatilin/myhome/issues/55) — TON Jetton tokens for agent economy

## Implementation

Follow `docs/implementation-plan.md` strictly — implement iteration by iteration, mark tasks as done.

### Architecture

- `cmd/myhome/main.go` — entrypoint, calls `internal/cmd.Execute()`
- `internal/cmd/` — cobra command definitions, thin wrappers that call into domain packages
- `internal/config/` — myhome.yml parsing, env resolution, state file
- `internal/platform/` — OS abstraction (macOS vs Linux): user creation, groups, ACLs, services, package manager
- `internal/repo/` — git repo management (clone, sync, add, rm)
- `internal/worktree/` — Worktrunk delegation + cross-repo overview
- `internal/tools/` — mise integration (generate .mise.toml, run mise install)
- `internal/packages/` — brew/apt package management
- `internal/auth/` — SSH config generation from myhome.yml
- `internal/identity/` — git identity config generation (per-directory includeIf)
- `internal/gitignore/` — .gitignore generation from static rules + repo paths
- `internal/cleanup/` — garbage detection (orphan worktrees, large files, empty dirs)
- `internal/archive/` — move to ~/archive/ + update .gitignore
- `internal/user/` — agent user lifecycle (OS user, template, vault, sync)
- `internal/service/` — launchd/systemd unit generation
- `internal/container/` — Docker container management (build, run, auth profiles, mounts)
- `internal/task/` — Task management (general tasks + dev run tasks with worktree/container orchestration)
- `internal/vault/` — KeePassXC vault management (create vault, import SSH keys, per-agent vaults)
- `internal/remote/` — Remote SSH + tmux session management
- `internal/schedule/` — Cron-based recurring tasks with template variables
- `internal/agent/` — Agent lifecycle management (create, start, stop, restart, state machine)
- `internal/daemon/` — Long-running supervisor process, gRPC API on unix socket, message routing
- `internal/adapter/` — InputSource interface for external event sources (CLI, webhooks, cron)

### Code Style

- Go 1.25+ (use current Go features, no legacy patterns)
- Use `errors.New` / `fmt.Errorf` with `%w` for wrapping
- Cobra for CLI, `gopkg.in/yaml.v3` for YAML
- No unnecessary abstractions — simple functions over interfaces unless polymorphism is needed
- Platform abstraction uses an interface because macOS and Linux have different implementations
- Shell out to external tools (git, mise, wt, brew, apt) via `os/exec` — don't reimplement their logic
- Commands are thin: parse flags → call domain function → print output
- All destructive operations require explicit confirmation (--apply, --force, or interactive prompt)
- Tests: table-driven, use `testing` stdlib. Mock filesystem and exec calls where needed.

### Config file

Single source of truth: `~/setup/myhome.yml` (see `docs/init.md` for full schema)

State file: `~/.myhome-state.yml` (gitignored, tracks current env and runtime state)

### Key Design Decisions

- **No chezmoi** — raw git for home folder, myhome generates .gitignore
- **mise** for dev runtimes — myhome generates .mise.toml, delegates install to mise
- **Worktrunk** for worktrees — myhome resolves repo names, delegates wt commands
- **myhome.yml is the source of truth** — .gitignore, .ssh/config, .mise.toml are generated artifacts
- **Container runtime agnostic** — auto-detects nerdctl/podman/docker, all use same OCI image format
- **Cleanup is report-only by default** — `--apply` flag for interactive confirmation
- **Agent users are OS-level** — real users with own homes, not containers

### Architecture Linting

This project uses [archlint](https://github.com/kgatilin/archlint) for architecture analysis.
An MCP server is available — when running in a container, `archlint serve` provides real-time feedback.

Available MCP tools (via `archlint serve`):
- `check_violations` — check for circular deps, high coupling, SOLID violations, god classes
- `analyze_file` — full file analysis (types, functions, dependencies, health score)
- `get_architecture` — get full architecture graph or filtered subset
- `get_dependencies` — dependency graph for a file or package
- `get_callgraph` — call graph from an entry point
- `analyze_change` — impact analysis of a file change

Key metrics: afferent/efferent coupling, instability, abstractness, SRP/DIP/ISP violations,
god classes, hub nodes, feature envy, shotgun surgery, cyclic dependencies.

Run `archlint collect . -l go -o architecture.yaml` to generate a full architecture snapshot.

### Architecture Validation

- Run `archlint check .` before committing to check for violations
- Run `archlint metrics .` for coupling analysis
- Key targets: no circular dependencies, minimize concrete config dependencies, keep packages cohesive
- archlint MCP server available in containers via `archlint serve`

### Container Config Constraints

- Container configs in myhome.yml must include `startup_commands` with a `{{.Prompt}}` template variable. The task runner renders this but does not add its own command. Example: `"exec claude --dangerously-skip-permissions --output-format text -p {{.Prompt}}"`
- `dependencies_go.txt` supports a `source:` prefix for packages that can't be `go install`ed (e.g. module path mismatches). Format: `source:github.com/user/repo cmd/tool` — triggers git clone + go build instead of go install. Use `go_deps_file:` in container config to install at build time (required for firewalled containers).

### Agent Runtime (deskd)

Agent orchestration is handled by a separate Rust project: [deskd](https://github.com/kira-autonoma/deskd).
myhome manages workspace (repos, tools, auth), deskd manages agents (lifecycle, routing, context, cost).
See `docs/deskd-integration.md` for full architecture, domain model, and data access patterns.

### Dependencies

- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing
- External: `git`, `mise`, `wt` (Worktrunk), `brew`/`apt`, `sysadminctl`/`useradd`

### Commit Style

- Short imperative subject line
- Group related changes into logical commits
- One iteration = one or more commits, each building toward iteration completion
