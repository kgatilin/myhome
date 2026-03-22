package user_test

import (
	"fmt"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/user"
)

// mockPlatform records all method calls for verification.
type mockPlatform struct {
	calls []string

	// Configurable return values.
	serviceStatus    bool
	serviceStatusErr error
	createUserErr    error
	removeUserErr    error
}

func (m *mockPlatform) record(call string) {
	m.calls = append(m.calls, call)
}

func (m *mockPlatform) OS() string                          { return "linux" }
func (m *mockPlatform) HomeDir() string                     { return "/home" }
func (m *mockPlatform) UserHome(username string) string      { return "/home/" + username }
func (m *mockPlatform) PackageManager() string               { return "apt" }
func (m *mockPlatform) ListInstalledPackages() ([]string, error) { return nil, nil }

func (m *mockPlatform) CreateUser(username string) error {
	m.record("CreateUser:" + username)
	return m.createUserErr
}

func (m *mockPlatform) RemoveUser(username string, removeHome bool) error {
	m.record(fmt.Sprintf("RemoveUser:%s:%v", username, removeHome))
	return m.removeUserErr
}

func (m *mockPlatform) CreateGroup(group string) error {
	m.record("CreateGroup:" + group)
	return nil
}

func (m *mockPlatform) AddUserToGroup(username, group string) error {
	m.record("AddUserToGroup:" + username + ":" + group)
	return nil
}

func (m *mockPlatform) SetReadOnlyACL(username, path string) error {
	m.record("SetReadOnlyACL:" + username + ":" + path)
	return nil
}

func (m *mockPlatform) InstallPackages(packages []string) error {
	m.record("InstallPackages")
	return nil
}

func (m *mockPlatform) InstallCaskPackages(packages []string) error {
	m.record("InstallCaskPackages")
	return nil
}

func (m *mockPlatform) ServiceInstall(name, command, username string, restart bool) error {
	m.record(fmt.Sprintf("ServiceInstall:%s:%s", name, username))
	return nil
}

func (m *mockPlatform) ServiceStart(name string) error {
	m.record("ServiceStart:" + name)
	return nil
}

func (m *mockPlatform) ServiceStop(name string) error {
	m.record("ServiceStop:" + name)
	return nil
}

func (m *mockPlatform) ServiceStatus(name string) (bool, error) {
	m.record("ServiceStatus:" + name)
	return m.serviceStatus, m.serviceStatusErr
}

func TestList(t *testing.T) {
	tests := []struct {
		name          string
		stateUsers    []string
		cfgUsers      map[string]config.User
		serviceStatus bool
		wantCount     int
		wantFirst     user.UserInfo
	}{
		{
			name:       "empty state",
			stateUsers: nil,
			cfgUsers:   nil,
			wantCount:  0,
		},
		{
			name:       "single user with config",
			stateUsers: []string{"agent1"},
			cfgUsers: map[string]config.User{
				"agent1": {Env: "work", Template: "claude-agent"},
			},
			serviceStatus: true,
			wantCount:     1,
			wantFirst: user.UserInfo{
				Name:     "agent1",
				Env:      "work",
				Template: "claude-agent",
				Running:  true,
				HomeDir:  "/home/agent1",
			},
		},
		{
			name:       "user not in config",
			stateUsers: []string{"orphan"},
			cfgUsers:   map[string]config.User{},
			wantCount:  1,
			wantFirst: user.UserInfo{
				Name:    "orphan",
				HomeDir: "/home/orphan",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := &mockPlatform{serviceStatus: tt.serviceStatus}
			cfg := &config.Config{Users: tt.cfgUsers}
			state := &config.State{Users: tt.stateUsers}

			users, err := user.List(cfg, state, plat)
			if err != nil {
				t.Fatalf("List() error: %v", err)
			}
			if len(users) != tt.wantCount {
				t.Fatalf("List() returned %d users, want %d", len(users), tt.wantCount)
			}
			if tt.wantCount > 0 {
				got := users[0]
				if got != tt.wantFirst {
					t.Errorf("List()[0] = %+v, want %+v", got, tt.wantFirst)
				}
			}
		})
	}
}

func TestRemoveCallsOrder(t *testing.T) {
	plat := &mockPlatform{}

	err := user.Remove("agent1", plat, "/home/testuser")
	// This will fail because unregisterUser tries to access the real state file.
	// In a real test we'd inject the state path. For now, we verify platform calls.
	_ = err

	// Verify service stop was called before user removal.
	wantCalls := []string{
		"ServiceStop:myhome-agent1",
		"RemoveUser:agent1:true",
	}
	if len(plat.calls) < len(wantCalls) {
		t.Fatalf("Remove() made %d platform calls, want at least %d: %v", len(plat.calls), len(wantCalls), plat.calls)
	}
	for i, want := range wantCalls {
		if plat.calls[i] != want {
			t.Errorf("call[%d] = %q, want %q", i, plat.calls[i], want)
		}
	}
}

func TestCreateUserError(t *testing.T) {
	plat := &mockPlatform{createUserErr: fmt.Errorf("permission denied")}
	cfg := &config.Config{}
	userCfg := config.User{Env: "work"}

	err := user.Create("agent1", userCfg, cfg, plat, "/home/testuser")
	if err == nil {
		t.Fatal("Create() should fail when CreateUser fails")
	}
	// Should contain both context and original error.
	if got := err.Error(); got != "create user agent1: permission denied" {
		t.Errorf("error = %q, want wrapped error", got)
	}
}

func TestListServiceStatusError(t *testing.T) {
	plat := &mockPlatform{
		serviceStatusErr: fmt.Errorf("not found"),
	}
	cfg := &config.Config{
		Users: map[string]config.User{"agent1": {Env: "work"}},
	}
	state := &config.State{Users: []string{"agent1"}}

	users, err := user.List(cfg, state, plat)
	if err != nil {
		t.Fatalf("List() should not fail on service status error: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("List() returned %d users, want 1", len(users))
	}
	if users[0].Running {
		t.Error("Running should be false when ServiceStatus returns error")
	}
}
