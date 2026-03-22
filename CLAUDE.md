# myhome — Personal Workspace Manager

## What is this

A Go CLI tool that manages a git-tracked home folder across multiple machines and environments.
See `docs/init.md` for full scope and requirements, `docs/implementation-plan.md` for the build plan.

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

### Dependencies

- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing
- External: `git`, `mise`, `wt` (Worktrunk), `brew`/`apt`, `sysadminctl`/`useradd`

### Commit Style

- Short imperative subject line
- Group related changes into logical commits
- One iteration = one or more commits, each building toward iteration completion
