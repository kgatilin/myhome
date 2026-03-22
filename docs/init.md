# myhome — Personal Workspace Manager

A Go CLI tool for managing a git-tracked home folder across multiple machines and environments.

## Overview

`myhome` is a single tool that manages:
- **Repos** — git repositories cloned from a declarative manifest
- **Environments** — profiles (work/personal/full) controlling what gets installed where
- **Tools** — dev runtimes via mise (Go, Python, Node, etc.)
- **Packages** — system packages via brew (macOS) / apt (Linux)
- **Auth** — SSH keys and config, per-env, stored in KeePassXC vaults
- **Git identity** — per-directory git user/email config
- **Worktrees** — delegates to Worktrunk, adds repo-name resolution and cross-repo overview
- **Agent users** — OS-level sub-users with isolated homes, own vaults, read-only access to parent
- **Cleanup** — reports untracked large files, orphan worktrees, stale branches

## Requirements

### Go version

- Minimum: Go 1.25
- Target: Go 1.26 (latest stable)
- Current on this machine: Go 1.25.1

### Config file

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
  - path: dev/tools/myhome
    url: git@github.com:kgatilin/myhome.git
    env: base

  - path: work/uagent
    url: git@gitlab.iponweb.net:bidcore/uagent-space/uagent.git
    env: work
    worktrees:
      dir: .worktrees
      default_branch: main

  - path: dev/tools/go-arch-lint
    url: git@github.com:kgatilin/go-arch-lint.git
    env: personal

  # ... see full manifest in HOME_REORG_PROPOSAL.md

tools:
  base:
    go: "1.26"
  work:
    python: "3.11"
    terraform: "1.7"
    kubectl: "1.29"
  personal:
    node: "20"
    python: "3.12"

packages:
  base:
    brew: [git, gh, jq, yq, mise, worktrunk]
    apt: [git, gh, jq]
  work:
    brew: [vault, cdt]
    apt: [vault]
  personal:
    brew: [ffmpeg, imagemagick]
  full:
    brew_cask: [docker, cursor, keepassxc, google-chrome]

auth:
  github.com:
    key: id_personal
  gitlab.iponweb.net:
    key: id_work

agent_templates:
  claude-agent:
    template_repo: git@github.com:kgatilin/agent-template-claude.git
    service:
      command: "claude --config-dir ~/.claude"
      restart: always

users:
  agent:
    env: work
    template: claude-agent
  researcher:
    env: personal
    template: claude-agent
```

### Commands

#### Bootstrap

```
myhome init --env <env>
```

Full machine bootstrap:
1. Verify/install mise
2. Generate `~/.mise.toml` from merged tool specs for the env
3. Run `mise install`
4. Install system packages (brew/apt) for the env
5. Clone all repos matching the env
6. Generate `~/.ssh/config` from auth section
7. Generate `~/.gitignore` from repo paths
8. Generate `~/.gitconfig` identity includes
9. Store current env in `~/.myhome-state.yml` for subsequent commands

#### Repos

```
myhome repo list                         # List repos for current env with clone/dirty status
myhome repo sync                         # Clone missing repos for current env
myhome repo add <path> [url] --env <e>   # Add repo to manifest + regenerate .gitignore
myhome repo rm <path>                    # Remove from manifest + regenerate .gitignore
```

- `repo add` auto-detects URL from existing git remote if not specified
- `repo add` / `repo rm` regenerate `.gitignore` automatically

#### Worktrees

```
myhome repo <name> wt create <branch>   # Delegates to: cd <repo-path> && wt switch --create <branch>
myhome repo <name> wt list              # Delegates to: cd <repo-path> && wt list
myhome repo <name> wt rm <branch>       # Delegates to: cd <repo-path> && wt remove <branch>
myhome repo wt list                     # Cross-repo: shows all worktrees across all repos
```

- `<name>` is resolved from `myhome.yml` repos (last path segment by default, e.g. `work/uagent` → `uagent`)
- Name conflicts (same basename in different envs): use full path or env prefix

#### Tools & Packages

```
myhome tools list                        # Installed vs expected runtimes (from mise)
myhome tools sync                        # Generate .mise.toml + run mise install
myhome packages list                     # Installed vs expected system packages
myhome packages sync                     # Install missing (brew install / apt install)
myhome packages dump                     # Snapshot current packages into myhome.yml
```

#### Auth

```
myhome auth generate                     # Generate ~/.ssh/config from myhome.yml auth section
myhome auth keys                         # List keys and which hosts/envs they serve
```

#### Status & Cleanup

```
myhome status                            # Current env, dirty repos, active worktrees, disk usage
myhome cleanup                           # Report: orphan worktrees, stale branches, large untracked files, empty dirs
myhome cleanup --apply                   # Interactively confirm each cleanup action
myhome archive <path>                    # Move to ~/archive/, update .gitignore
```

#### Agent Users

```
myhome user create <name> --env <e> --template <t>   # Create OS user, clone template, setup vault, ACLs, service
myhome user list                                      # List users with env, template, service status, sync status
myhome user rm <name>                                 # Remove user, home, service
myhome user shell <name>                              # sudo -u <name> -i
myhome user sync <name>                               # Push/pull agent's home repo
myhome user sync --all                                # Sync all agent users
```

Agent user creation flow:
1. Create OS user (`sysadminctl` on macOS, `useradd` on Linux)
2. Create shared group, add both users
3. Set read-only ACLs for dirs matching the env (`chmod +a` / `setfacl`)
4. Clone template repo into agent's home
5. Create dedicated repo on the env's git host (github/gitlab)
6. Init git in agent home, push to dedicated repo
7. Generate SSH keypair, store in agent's vault
8. Generate service (launchd plist / systemd unit), enable + start

### .gitignore Generation

`myhome` generates `~/.gitignore` from:
- Static rules (macOS system dirs, caches, secrets, tool state)
- Dynamic rules from `myhome.yml` repos (each repo path → gitignore entry)

Regenerated on: `myhome init`, `myhome repo add`, `myhome repo rm`

Header: `# Auto-generated by myhome — do not edit manually`

### State File

`~/.myhome-state.yml` (gitignored) tracks:
- Current env
- Last sync timestamps
- Registered agent users

### Platform Support

| Feature | macOS | Linux |
|---------|-------|-------|
| Packages | brew / brew cask | apt |
| User creation | sysadminctl | useradd |
| Groups | dseditgroup | groupadd / usermod |
| ACLs | chmod +a | setfacl |
| Services | launchd (plist) | systemd (unit) |
| Home dir | /Users/<name>/ | /home/<name>/ |

### Dependencies

| Tool | Purpose | Install |
|------|---------|---------|
| mise | Dev runtime management | `curl https://mise.jdx.dev/install.sh \| sh` |
| Worktrunk | Git worktree management | `brew install worktrunk` / `cargo install worktrunk` |
| KeePassXC | Secret/key management | `brew install --cask keepassxc` |

### Non-Goals (out of scope)

- Agent-to-user communication protocols (agents write logs, that's it for now)
- CI/CD integration
- Cloud infrastructure management
- Dotfile templating (raw git, no chezmoi)

### Project Structure (planned)

```
myhome/
├── cmd/
│   └── myhome/
│       └── main.go
├── internal/
│   ├── config/          # myhome.yml parsing
│   ├── repo/            # Repo management (clone, sync, add, rm)
│   ├── worktree/        # Worktrunk delegation + cross-repo overview
│   ├── tools/           # mise integration
│   ├── packages/        # brew/apt abstraction
│   ├── auth/            # SSH config generation
│   ├── identity/        # Git identity config generation
│   ├── user/            # Agent user management (OS-level)
│   ├── service/         # launchd/systemd generation
│   ├── gitignore/       # .gitignore generation
│   ├── cleanup/         # Garbage detection + interactive cleanup
│   └── platform/        # macOS vs Linux abstraction
├── docs/
│   └── init.md          # This file
├── setup/
│   └── bootstrap.sh     # Minimal bootstrap: mise → Go → myhome
├── go.mod
├── go.sum
├── CLAUDE.md
└── README.md
```
