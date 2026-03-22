# Implementation Plan

## Iteration 1: Foundation

Config parsing and platform abstraction — everything else depends on these.

### Tasks

- [ ] `internal/config/config.go` — Define Go types for `myhome.yml` (envs, repos, tools, packages, auth, users, agent_templates, services)
- [ ] `internal/config/loader.go` — Load and parse `myhome.yml`, resolve env includes, locate config file (look in `~/setup/myhome.yml`)
- [ ] `internal/config/state.go` — State file (`~/.myhome-state.yml`): current env, last sync timestamps, registered users
- [ ] `internal/platform/platform.go` — Interface for OS-specific operations (user creation, groups, ACLs, service management, package manager, home dir path)
- [ ] `internal/platform/darwin.go` — macOS implementation (sysadminctl, dseditgroup, chmod +a, launchd, brew)
- [ ] `internal/platform/linux.go` — Linux implementation (useradd, groupadd, setfacl, systemd, apt)
- [ ] `internal/platform/detect.go` — Auto-detect platform at runtime
- [ ] Tests for config parsing with sample myhome.yml
- [ ] Tests for platform detection

### Acceptance Criteria

- `config.Load("path/to/myhome.yml")` returns fully parsed config
- `config.ResolveEnv("work")` returns merged list of repos/tools/packages for base+work
- `platform.Detect()` returns the correct platform implementation
- State file can be read/written

---

## Iteration 2: Core Features (independent, parallelizable)

These modules have no dependencies on each other, only on config + platform.

### Tasks

- [ ] `internal/repo/repo.go` — List repos (with clone/dirty status), sync (clone missing), add (detect URL, update myhome.yml), rm (update myhome.yml)
- [ ] `internal/gitignore/gitignore.go` — Generate `~/.gitignore` from static rules + dynamic repo paths from config. Write with header comment.
- [ ] `internal/auth/auth.go` — Generate `~/.ssh/config` from auth section. List keys and their host mappings.
- [ ] `internal/identity/identity.go` — Generate `.gitconfig` includes based on directory-to-identity mapping (personal default, work override for `~/work/`)
- [ ] `internal/tools/tools.go` — Generate `~/.mise.toml` from merged tool specs per env. Shell out to `mise install`. List installed vs expected.
- [ ] `internal/packages/packages.go` — Install packages via brew/apt per env. List installed vs expected. Dump current packages.
- [ ] Update command files to call real implementations instead of stubs
- [ ] Tests for each module

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

- [ ] `internal/worktree/worktree.go` — Resolve repo name to path from config. Delegate `wt` commands to Worktrunk inside repo dir. Cross-repo `wt list` (iterate all repos, collect worktree info).
- [ ] `internal/cleanup/cleanup.go` — Scan for: orphan worktrees, stale branches, large untracked files (>10MB), empty dirs. Report mode (default) vs interactive apply mode.
- [ ] `internal/archive/archive.go` — Move path to `~/archive/`, update .gitignore.
- [ ] Update `status` command — show current env, dirty repos count, active worktrees count, disk usage summary
- [ ] Handle `myhome repo <name> wt` dynamic subcommand routing (resolve repo name, delegate to worktree module)
- [ ] Tests

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

- [ ] Implement `myhome init --env <env>` — orchestrate in order:
  1. Load/create myhome.yml
  2. Save env to state file
  3. Generate .gitignore
  4. Generate .ssh/config
  5. Generate .gitconfig identity
  6. Generate .mise.toml + run mise install
  7. Install system packages
  8. Clone repos for env
- [ ] Write `setup/bootstrap.sh` — installs mise → Go → myhome, then runs `myhome init`
- [ ] Tests (integration-level)

### Acceptance Criteria

- `myhome init --env work` performs full bootstrap for work environment
- `myhome init --env full` performs full bootstrap with all repos/tools/packages
- `bootstrap.sh` works on a clean macOS machine (manual verification)

---

## Iteration 5: Agent Users

OS-level user management with isolation.

### Tasks

- [ ] `internal/user/user.go` — Create/remove OS users via platform abstraction. Manage shared group. Set ACLs for env-scoped dirs.
- [ ] `internal/user/template.go` — Clone template repo into agent home. Create dedicated repo on git host. Init + push.
- [ ] `internal/user/vault.go` — Generate SSH keypair for agent. Create agent vault.
- [ ] `internal/service/service.go` — Generate launchd plist (macOS) or systemd unit (Linux) from template config. Enable/start/stop.
- [ ] `internal/user/sync.go` — Push/pull agent's home repo.
- [ ] Update user commands to call real implementations
- [ ] Tests (may need mocking for OS-level operations)

### Acceptance Criteria

- `myhome user create agent --env work --template claude-agent` creates OS user with correct ACLs
- Agent user can read parent's work dirs, cannot write
- Agent service starts on boot
- `myhome user sync agent` pushes/pulls agent's home repo
- `myhome user list` shows users with env, template, service status
- `myhome user rm agent` cleans up user, home, service

---

## Iteration 6: Polish

- [ ] Zsh completions (`myhome completion zsh`)
- [ ] Repo name tab-completion (dynamic from myhome.yml)
- [ ] `myhome packages dump` — snapshot current brew/apt packages into myhome.yml
- [ ] Error messages and help text refinement
- [ ] Edge cases: name conflicts (same repo basename in different envs), missing dependencies (mise/wt not installed)
- [ ] Integration tests with a sample myhome.yml
