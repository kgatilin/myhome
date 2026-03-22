package user

import (
	"fmt"
	"os/user"
	"slices"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/platform"
	"github.com/kgatilin/myhome/internal/service"
)

const sharedGroup = "myhome-agents"

// UserInfo holds display information about an agent user.
type UserInfo struct {
	Name     string
	Env      string
	Template string
	Running  bool
	HomeDir  string
}

// Create creates an agent user with env-scoped access.
//
// Steps:
//  1. Create OS user via platform
//  2. Create shared group "myhome-agents"
//  3. Add both parent user and agent to the group
//  4. Set read-only ACLs on env-scoped directories
//  5. Clone template repo into agent's home (best-effort)
//  6. Generate SSH keypair (best-effort)
//  7. Init agent home as git repo (best-effort)
//  8. Install and start service
//  9. Register user in state file
func Create(name string, userCfg config.User, cfg *config.Config, plat platform.Platform, homeDir string) error {
	// Step 1: Create OS user.
	if err := plat.CreateUser(name); err != nil {
		return fmt.Errorf("create user %s: %w", name, err)
	}
	fmt.Printf("Created user %s\n", name)

	// Step 2: Create shared group.
	if err := plat.CreateGroup(sharedGroup); err != nil {
		// Group may already exist — log but continue.
		fmt.Printf("Note: create group %s: %v\n", sharedGroup, err)
	}

	// Step 3: Add both parent user and agent to the group.
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}
	if err := plat.AddUserToGroup(currentUser.Username, sharedGroup); err != nil {
		fmt.Printf("Note: add %s to group: %v\n", currentUser.Username, err)
	}
	if err := plat.AddUserToGroup(name, sharedGroup); err != nil {
		fmt.Printf("Note: add %s to group: %v\n", name, err)
	}

	// Step 4: Set read-only ACLs on env-scoped directories.
	if userCfg.Env != "" {
		resolved, err := cfg.ResolveEnv(userCfg.Env)
		if err != nil {
			fmt.Printf("Warning: resolve env %s for ACLs: %v\n", userCfg.Env, err)
		} else {
			for _, repo := range resolved.Repos {
				repoPath := homeDir + "/" + repo.Path
				if err := plat.SetReadOnlyACL(name, repoPath); err != nil {
					fmt.Printf("Warning: set ACL on %s: %v\n", repoPath, err)
				}
			}
		}
	}

	agentHome := plat.UserHome(name)

	// Step 5: Clone template repo (best-effort).
	if userCfg.Template != "" {
		if tmpl, ok := cfg.AgentTemplates[userCfg.Template]; ok {
			if err := CloneTemplate(tmpl.TemplateRepo, agentHome); err != nil {
				fmt.Printf("Warning: clone template: %v\n", err)
			}
		} else {
			fmt.Printf("Warning: unknown template %q\n", userCfg.Template)
		}
	}

	// Step 6: Generate SSH keypair (best-effort).
	if err := GenerateSSHKeypair(agentHome, name); err != nil {
		fmt.Printf("Warning: generate SSH keypair: %v\n", err)
	}

	// Step 7: Init agent home as git repo (best-effort).
	if err := InitAgentRepo(agentHome, name); err != nil {
		fmt.Printf("Warning: init agent repo: %v\n", err)
	}

	// Step 8: Install and start service.
	if userCfg.Template != "" {
		if tmpl, ok := cfg.AgentTemplates[userCfg.Template]; ok && tmpl.Service.Command != "" {
			svcName := "myhome-" + name
			if err := service.Install(svcName, tmpl.Service, name, plat); err != nil {
				return fmt.Errorf("install service for %s: %w", name, err)
			}
			fmt.Printf("Service %s installed and started\n", svcName)
		}
	}

	// Step 9: Register user in state file.
	if err := registerUser(name); err != nil {
		return fmt.Errorf("register user in state: %w", err)
	}
	fmt.Printf("Agent user %s created successfully\n", name)
	return nil
}

// Remove removes an agent user, service, and home directory.
func Remove(name string, plat platform.Platform, homeDir string) error {
	// Stop and remove service.
	svcName := "myhome-" + name
	service.Remove(svcName, plat)

	// Remove OS user and home directory.
	if err := plat.RemoveUser(name, true); err != nil {
		return fmt.Errorf("remove user %s: %w", name, err)
	}
	fmt.Printf("Removed user %s\n", name)

	// Unregister from state.
	if err := unregisterUser(name); err != nil {
		return fmt.Errorf("unregister user from state: %w", err)
	}
	return nil
}

// List returns info about registered agent users.
func List(cfg *config.Config, state *config.State, plat platform.Platform) ([]UserInfo, error) {
	var users []UserInfo
	for _, name := range state.Users {
		info := UserInfo{
			Name:    name,
			HomeDir: plat.UserHome(name),
		}
		// Look up user config for env/template.
		if userCfg, ok := cfg.Users[name]; ok {
			info.Env = userCfg.Env
			info.Template = userCfg.Template
		}
		// Check service status.
		svcName := "myhome-" + name
		running, err := plat.ServiceStatus(svcName)
		if err == nil {
			info.Running = running
		}
		users = append(users, info)
	}
	return users, nil
}

// registerUser adds a user to the state file.
func registerUser(name string) error {
	statePath, err := config.DefaultStatePath()
	if err != nil {
		return err
	}
	state, err := config.LoadState(statePath)
	if err != nil {
		return err
	}
	if !slices.Contains(state.Users, name) {
		state.Users = append(state.Users, name)
	}
	return state.Save(statePath)
}

// unregisterUser removes a user from the state file.
func unregisterUser(name string) error {
	statePath, err := config.DefaultStatePath()
	if err != nil {
		return err
	}
	state, err := config.LoadState(statePath)
	if err != nil {
		return err
	}
	state.Users = slices.DeleteFunc(state.Users, func(u string) bool {
		return u == name
	})
	return state.Save(statePath)
}
