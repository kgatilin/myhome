# Implementation Plan

## Iteration 1: Foundation

Config parsing and platform abstraction — everything else depends on these.

### Tasks

- [x] `internal/config/config.go` — Define Go types for `myhome.yml` (envs, repos, tools, packages, auth, users, agent_templates, services, containers, claude, tasks)
- [x] `internal/config/loader.go` — Load and parse `myhome.yml`, resolve env includes, locate config file (look in `~/setup/myhome.yml`)
- [x] `internal/config/state.go` — State file (`~/.myhome-state.yml`): current env, last sync timestamps, registered users
- [x] `internal/platform/platform.go` — Interface for OS-specific operations (user creation, groups, ACLs, service management, package manager, home dir path)
- [x] `internal/platform/darwin.go` — macOS implementation (sysadminctl, dseditgroup, chmod +a, launchd, brew)
- [x] `internal/platform/linux.go` — Linux implementation (useradd, groupadd, setfacl, systemd, apt)
- [x] `internal/platform/detect.go` — Auto-detect platform at runtime
- [x] Tests for config parsing with sample myhome.yml
- [x] Tests for platform detection

### Acceptance Criteria

- `config.Load("path/to/myhome.yml")` returns fully parsed config
- `config.ResolveEnv("work")` returns merged list of repos/tools/packages for base+work
- `platform.Detect()` returns the correct platform implementation
- State file can be read/written

---

## Iteration 2: Core Features (independent, parallelizable)

These modules have no dependencies on each other, only on config + platform.

### Tasks

- [x] `internal/repo/repo.go` — List repos (with clone/dirty status), sync (clone missing), add (detect URL, update myhome.yml), rm (update myhome.yml)
- [x] `internal/gitignore/gitignore.go` — Generate `~/.gitignore` from static rules + dynamic repo paths from config. Write with header comment.
- [x] `internal/auth/auth.go` — Generate `~/.ssh/config` from auth section. List keys and their host mappings.
- [x] `internal/identity/identity.go` — Generate `.gitconfig` includes based on directory-to-identity mapping (personal default, work override for `~/work/`)
- [x] `internal/tools/tools.go` — Generate `~/.mise.toml` from merged tool specs per env. Shell out to `mise install`. List installed vs expected.
- [x] `internal/packages/packages.go` — Install packages via brew/apt per env. List installed vs expected. Dump current packages.
- [x] Update command files to call real implementations instead of stubs
- [x] Tests for each module

### Acceptance Criteria

- `myhome repo list` shows repos with clone status
- `myhome repo sync` clones missing repos
- `myhome repo add <path>` detects URL, adds to myhome.yml, regenerates .gitignore
- `myhome auth generate` writes ~/.ssh/config
- `myhome tools sync` generates .mise.toml and runs mise install
- `myhome packages sync` installs missing packages
- `.gitignore` is auto-regenerated on repo add/rm

---

## Iteration 3: Worktrees, Cleanup, Status

These depend on repo module from iteration 2.

### Tasks

- [x] `internal/worktree/worktree.go` — Resolve repo name to path from config. Delegate `wt` commands to Worktrunk inside repo dir. Cross-repo `wt list` (iterate all repos, collect worktree info).
- [x] `internal/cleanup/cleanup.go` — Scan for: orphan worktrees, stale branches, large untracked files (>10MB), empty dirs. Report mode (default) vs interactive apply mode.
- [x] `internal/archive/archive.go` — Move path to `~/archive/`, update .gitignore.
- [x] Update `status` command — show current env, dirty repos count, active worktrees count, disk usage summary
- [x] Handle `myhome repo <name> wt` dynamic subcommand routing (resolve repo name, delegate to worktree module)
- [x] Tests

### Acceptance Criteria

- `myhome repo uagent wt create TICKET-123` resolves path and delegates to wt
- `myhome repo wt list` shows worktrees across all repos
- `myhome cleanup` reports garbage without deleting
- `myhome cleanup --apply` prompts for confirmation per item
- `myhome status` shows meaningful overview
- `myhome archive <path>` moves folder and updates .gitignore

---

## Iteration 4: Init (orchestration)

Ties everything together.

### Tasks

- [x] Implement `myhome init --env <env>` — orchestrate in order:
  1. Load/create myhome.yml
  2. Save env to state file
  3. Generate .gitignore
  4. Generate .ssh/config
  5. Generate .gitconfig identity
  6. Generate .mise.toml + run mise install
  7. Install system packages
  8. Clone repos for env
- [x] Write `setup/bootstrap.sh` — installs mise → Go → myhome, then runs `myhome init`
- [x] Tests (integration-level)

### Acceptance Criteria

- `myhome init --env work` performs full bootstrap for work environment
- `myhome init --env full` performs full bootstrap with all repos/tools/packages
- `bootstrap.sh` works on a clean macOS machine (manual verification)

---

## Iteration 5: Agent Users

OS-level user management with isolation.

### Tasks

- [x] `internal/user/user.go` — Create/remove OS users via platform abstraction. Manage shared group. Set ACLs for env-scoped dirs.
- [x] `internal/user/template.go` — Clone template repo into agent home. Create dedicated repo on git host. Init + push.
- [x] `internal/user/vault.go` — Generate SSH keypair for agent. Create agent vault.
- [x] `internal/service/service.go` — Generate launchd plist (macOS) or systemd unit (Linux) from template config. Enable/start/stop.
- [x] `internal/user/sync.go` — Push/pull agent's home repo.
- [x] Update user commands to call real implementations
- [x] Tests (may need mocking for OS-level operations)

### Acceptance Criteria

- `myhome user create agent --env work --template claude-agent` creates OS user with correct ACLs
- Agent user can read parent's work dirs, cannot write
- Agent service starts on boot
- `myhome user sync agent` pushes/pulls agent's home repo
- `myhome user list` shows users with env, template, service status
- `myhome user rm agent` cleans up user, home, service

---

## Iteration 6: Vault Management

KeePassXC vault setup, SSH key storage, agent vault creation.

### Tasks

- [x] `internal/vault/vault.go` — Create KeePassXC vault (shell out to `keepassxc-cli`). Generate key file in `~/.secrets/`. Check vault status (exists, locked).
- [x] `internal/vault/ssh.go` — Import SSH keys into vault via `keepassxc-cli`. Configure KeePassXC SSH agent integration.
- [x] `internal/vault/agent.go` — Create per-agent vault during `user create`. Store agent's SSH keypair in agent vault. Store agent key file in parent's `~/.secrets/`.
- [x] Add `vault` command group: `init`, `status`, `ssh-add`, `ssh-agent`
- [x] Integration with `myhome user create` — auto-create agent vault
- [x] Integration with `myhome init` — prompt to set up vault if not exists
- [x] Tests

### Acceptance Criteria

- `myhome vault init` creates `~/setup/vault.kdbx` + `~/.secrets/vault.key`
- `myhome vault status` shows vault location and whether KeePassXC is running
- `myhome vault ssh-add id_personal` imports key into vault
- `myhome user create agent` creates `/home/agent/vault.kdbx` with agent's SSH key
- Agent vault key files stored in `~/.secrets/<agent>-vault.key` (gitignored)

---

## Iteration 7: Containers

Docker container management — build, run, auth profiles.

### Tasks

- [x] `internal/container/container.go` — Parse container definitions from myhome.yml. Build images (shell out to `docker build`). Generate `docker run` commands from config (mounts, env vars, firewall caps, startup commands).
- [x] `internal/container/auth.go` — Resolve Claude auth profiles. Map auth profile to auth file + env vars. Mount correct auth file into container.
- [x] `internal/container/mounts.go` — Resolve mount paths from config. Handle `:ro` suffix. Auto-mount project dir as `/workspace`. Auto-mount MCP servers if configured.
- [x] `internal/container/backup.go` — Git backup before container run (rsync .git to ~/.git-backups/).
- [x] Add `container` command group: `build`, `run`, `list`, `shell` (wired to real implementations)
- [ ] Migrate existing Dockerfiles from `~/.claude-docker/` to `~/containers/claude-code/`
- [x] Tests

### Acceptance Criteria

- `myhome container build claude-code` builds image from `containers/claude-code/official/`
- `myhome container run claude-code --auth work` runs with correct auth file + env vars
- `myhome container run claude-code --auth vertex-work` sets Vertex env vars
- `myhome container run cursor` runs cursor container without auth profiles
- `myhome container list` shows defined containers with build status
- `myhome container shell claude-code` opens shell for debugging
- Adding a new container = folder in `~/containers/` + YAML block in `myhome.yml`
- Single `~/.claude/` config dir shared across all auth profiles

---

## Iteration 8: Task Management

Lightweight, git-tracked task system for both general tasks and dev run tasks (worktree + container orchestration).

### Tasks

- [x] `internal/task/task.go` — Task model (id, type, domain, description, status, timestamps). YAML serialization. ID auto-increment.
- [x] `internal/task/store.go` — File-based store in `~/tasks/`. Active tasks in `active/`, done in `done/`. Read/write YAML files.
- [x] `internal/task/run.go` — Orchestrate run tasks: create worktree (via git) → launch container → capture container ID → stream logs to `logs/<id>.log`. Background execution.
- [x] `internal/task/log.go` — Tail/stream log file for a run task.
- [x] Add `task` command group: `add`, `run`, `list`, `log`, `done`, `stop`, `rm` (wired to real implementations)
- [x] Tests

### Acceptance Criteria

- `myhome task add "Review roadmap" --domain work` creates a YAML file in `tasks/active/`
- `myhome task run uagent TICKET-1234 "Fix crash" --auth work` creates worktree + launches container + creates task
- `myhome task list` shows both general and run tasks with status
- `myhome task log 1` streams Claude's output for a run task
- `myhome task done 1` moves task to `tasks/done/`, optionally removes worktree
- `myhome task stop 1` kills the container
- Tasks are git-tracked (just YAML files in `~/tasks/`)
- Logs are gitignored (can be large)

---

## Iteration 9: Remote Sessions

SSH + tmux remote session management for running Claude on VPS.

### Tasks

- [x] `internal/remote/remote.go` — SSH into host, manage tmux sessions. Run commands inside tmux. List/attach/stop sessions.
- [x] Add `remotes:` section to config parser (host, home path, env)
- [x] Add `remote` command group: `run`, `list`, `attach`, `stop`
- [x] Integrate with task system — `myhome task run` gains `--remote <host>` flag
- [x] Tests (mock SSH/tmux commands)

### Commands

```
myhome remote init <host> --env <env>                # Full VPS bootstrap from laptop
myhome remote run <host> <repo> <prompt> [--auth]    # SSH → tmux → cd repo → claude -p
myhome remote list <host>                            # List tmux sessions on host
myhome remote attach <host> <session>                # SSH -t → tmux attach
myhome remote stop <host> <session>                  # SSH → tmux kill-session
```

### Remote Init Flow

`myhome remote init user@vps --env work` does:
1. `ssh-copy-id` — push your SSH key to VPS (GitHub auth works via forwarded key)
2. SSH in, `git clone git@github.com:user/home.git ~`
3. `scp ~/.secrets/vault.key` to VPS `~/.secrets/vault.key`
4. SSH in, run `~/setup/bootstrap.sh` (installs mise → Go → myhome)
5. SSH in, run `myhome init --env work` (extracts SSH keys from vault, sets up everything)

### Configuration

```yaml
remotes:
  vps-work:
    host: user@work-vps.example.com
    home: ~/
    env: work
  vps-personal:
    host: user@personal-vps.example.com
    home: ~/
    env: personal
```

### Acceptance Criteria

- `myhome remote run vps-work uagent "Fix bug"` SSHs in, creates tmux session, launches Claude
- `myhome remote list vps-work` shows active tmux sessions
- `myhome remote attach vps-work uagent-fix-bug` attaches to the session
- `myhome task run uagent TICKET-1234 "Fix crash" --remote vps-work` creates task + runs remotely
- Sessions persist after SSH disconnect (tmux)

---

## Iteration 10: Scheduled Tasks & Auto-Blog

Cron-based recurring tasks with template variables. Primary use case: auto-generated blog from Claude session history.

### Tasks

- [x] `internal/schedule/schedule.go` — Parse schedule definitions from myhome.yml. Resolve template variables ({date}, {year}, {week}). Generate launchd plists (macOS) or cron entries (Linux).
- [x] Add `schedules:` section to config parser
- [x] Add `task schedule` subcommands: schedule with prompt/cron/container/auth/workdir, list, rm
- [x] Template variable resolution: `{date}` → `2026-03-22`, `{year}` → `2026`, `{week}` → `12`, `{domain}` → work/personal
- [x] Integration: scheduled task triggers `myhome task run` with resolved prompt
- [x] Tests

### Acceptance Criteria

- `myhome task schedule "Update blog" --cron "0 18 * * 1-5" --container claude-code --workdir ~/work/blog` creates launchd plist / cron entry
- `myhome task schedule list` shows scheduled tasks with next run time
- `myhome task schedule rm <id>` removes the schedule
- Template variables resolve correctly at execution time
- Auto-blog: daily digest appears in `work/blog/{date}.md` after scheduled run
- Scheduled tasks show up in `myhome task list` when running

---

## Iteration 11: Polish

- [x] Zsh completions (`myhome completion zsh`)
- [x] Repo name tab-completion (dynamic from myhome.yml)
- [x] `myhome packages dump` — snapshot current brew/apt packages into myhome.yml
- [x] Error messages and help text refinement
- [x] Edge cases: name conflicts (same repo basename in different envs), missing dependencies (mise/wt not installed)
- [x] Integration tests with a sample myhome.yml
