# myhome

A Go CLI tool for managing your entire home folder as a git-tracked, reproducible workspace.

One config file (`myhome.yml`) declares your repos, tools, packages, containers, SSH keys, and agent users. One command (`myhome init --env work`) sets up a new machine.

## What it does

- **Repos** — Declarative git repo management with worktree support (via [Worktrunk](https://worktrunk.dev))
- **Environments** — Profiles (`work`, `personal`, `full`) control what gets installed where
- **Tools** — Dev runtimes via [mise](https://mise.jdx.dev) (Go, Python, Node, etc.)
- **Packages** — System packages via brew (macOS) / apt (Linux)
- **Containers** — Build and run Docker/Podman/nerdctl containers with declarative config and Claude auth profiles
- **Tasks** — Lightweight task management + orchestrated dev runs (create worktree → launch Claude in container → track)
- **Secrets** — KeePassXC vault management: SSH keys in vault, per-agent vaults, SSH agent integration
- **Auth** — SSH config and git identity generation from config
- **Agent Users** — OS-level sub-users with isolated homes, own vaults, read-only access to your files
- **Remote** — SSH + tmux session management for running Claude on VPS
- **Scheduled Tasks** — Cron-based recurring prompts (e.g. auto-generate daily work blog from session history)
- **Cleanup** — Detect orphan worktrees, stale branches, large untracked files

## Install

```bash
go install github.com/kgatilin/myhome/cmd/myhome@latest
```

Or from source:

```bash
git clone https://github.com/kgatilin/myhome.git
cd myhome
go build -o ~/bin/myhome ./cmd/myhome/
```

## Quick Start

```bash
# Bootstrap a new machine
myhome init --env full

# See workspace status
myhome status

# Manage repos
myhome repo list
myhome repo sync
myhome repo add work/new-project git@github.com:user/repo.git --env work

# Worktrees (delegates to Worktrunk)
myhome repo uagent wt create TICKET-1234
myhome repo uagent wt list
myhome repo wt list                      # Cross-repo overview

# Dev runtimes
myhome tools list
myhome tools sync

# System packages
myhome packages list
myhome packages sync

# Containers
myhome container build claude-code
myhome container run claude-code --auth work
myhome container list

# Tasks
myhome task add "Review roadmap" --domain work
myhome task run uagent TICKET-1234 "Fix the crash" --auth work
myhome task list
myhome task log 1 -f
myhome task done 1

# Auth & identity
myhome auth generate                     # Generate ~/.ssh/config
myhome auth keys                         # List SSH keys

# Secrets vault
myhome vault init                        # Create KeePassXC vault + key file
myhome vault status                      # Check vault status
myhome vault ssh-add id_personal         # Import SSH key into vault
myhome vault ssh-agent                   # Enable SSH agent integration

# Cleanup
myhome cleanup                           # Report only
myhome cleanup --apply                   # Interactive confirmation

# Agent users
myhome user create agent --env work --template claude-agent
myhome user list
myhome user shell agent
myhome user sync agent

# Remote sessions (VPS)
myhome remote run vps-work uagent "Fix the bug" --auth work
myhome remote list vps-work
myhome remote attach vps-work uagent-fix-bug

# Scheduled tasks
myhome task schedule "Update work blog" --cron "0 18 * * 1-5" --container claude-code --auth work --workdir ~/work/blog
myhome task schedule list
```

## Configuration

Single source of truth: `~/setup/myhome.yml`

```yaml
envs:
  base:
    include: [base]
  work:
    include: [base, work]
  personal:
    include: [base, personal]
  full:
    include: [base, work, personal]

repos:
  - path: work/uagent
    url: git@gitlab.example.com:team/uagent.git
    env: work
    worktrees:
      dir: .worktrees
      default_branch: main

  - path: dev/tools/my-tool
    url: git@github.com:user/my-tool.git
    env: personal

tools:
  base:
    go: "1.26"
  work:
    python: "3.11"
  personal:
    node: "20"

packages:
  base:
    brew: [git, gh, jq, mise]

containers:
  claude-code:
    dockerfile: containers/claude-code/official
    image: claude-code-local:official
    firewall: true
    git_backup: true

container_runtime: auto    # nerdctl → podman → docker

claude:
  config_dir: ~/.claude
  auth_profiles:
    personal:
      auth_file: ~/.claude.json
    work:
      auth_file: ~/.claude-work.json

auth:
  github.com:
    key: id_personal
  gitlab.example.com:
    key: id_work
```

## Home Folder Layout

```
~/
├── setup/
│   └── myhome.yml              # Config
├── work/                        # Work projects
├── dev/                         # Personal dev projects
│   ├── tools/
│   ├── ai/
│   └── mcp/
├── life/                        # Personal non-code
│   ├── finance/
│   ├── travel/
│   └── family/
├── containers/                  # Dockerfiles per tool
│   ├── claude-code/
│   └── cursor/
├── tasks/                       # Git-tracked task files
│   ├── active/
│   └── done/
├── bin/                         # Personal scripts
├── .gitignore                   # Auto-generated by myhome
└── CLAUDE.md                    # Workspace map
```

## New Laptop Setup

```bash
# 1. Clone your home repo
git clone git@github.com:user/home.git ~

# 2. Bootstrap (installs mise → Go → myhome)
~/setup/bootstrap.sh

# 3. Initialize
myhome init --env full
```

## Platform Support

| Feature | macOS | Linux |
|---------|-------|-------|
| Packages | brew | apt |
| Container runtime | Docker / OrbStack / Podman | Docker / Podman / nerdctl |
| User management | sysadminctl | useradd |
| Services | launchd | systemd |

## Dependencies

| Tool | Purpose |
|------|---------|
| [mise](https://mise.jdx.dev) | Dev runtime management |
| [Worktrunk](https://worktrunk.dev) | Git worktree management |
| [KeePassXC](https://keepassxc.org) | Secrets & SSH key management |
| Docker / Podman / nerdctl | Container runtime |

## Status

Under active development. See [docs/implementation-plan.md](docs/implementation-plan.md) for progress.

## License

MIT
