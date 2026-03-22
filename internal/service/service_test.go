package service_test

import (
	"fmt"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/service"
)

type mockPlatform struct {
	calls            []string
	installErr       error
	startErr         error
	stopErr          error
	statusResult     bool
	statusErr        error
}

func (m *mockPlatform) record(call string) {
	m.calls = append(m.calls, call)
}

func (m *mockPlatform) OS() string                                      { return "linux" }
func (m *mockPlatform) HomeDir() string                                 { return "/home" }
func (m *mockPlatform) UserHome(username string) string                 { return "/home/" + username }
func (m *mockPlatform) PackageManager() string                          { return "apt" }
func (m *mockPlatform) CreateUser(username string) error                { return nil }
func (m *mockPlatform) RemoveUser(username string, removeHome bool) error { return nil }
func (m *mockPlatform) CreateGroup(group string) error                  { return nil }
func (m *mockPlatform) AddUserToGroup(username, group string) error     { return nil }
func (m *mockPlatform) SetReadOnlyACL(username, path string) error      { return nil }
func (m *mockPlatform) InstallPackages(packages []string) error         { return nil }
func (m *mockPlatform) InstallCaskPackages(packages []string) error     { return nil }
func (m *mockPlatform) ListInstalledPackages() ([]string, error)        { return nil, nil }

func (m *mockPlatform) ServiceInstall(name, command, username string, restart bool) error {
	m.record(fmt.Sprintf("ServiceInstall:%s:%s:%s:%v", name, command, username, restart))
	return m.installErr
}

func (m *mockPlatform) ServiceStart(name string) error {
	m.record("ServiceStart:" + name)
	return m.startErr
}

func (m *mockPlatform) ServiceStop(name string) error {
	m.record("ServiceStop:" + name)
	return m.stopErr
}

func (m *mockPlatform) ServiceStatus(name string) (bool, error) {
	m.record("ServiceStatus:" + name)
	return m.statusResult, m.statusErr
}

func TestInstall(t *testing.T) {
	tests := []struct {
		name       string
		svcCfg     config.ServiceConfig
		installErr error
		startErr   error
		wantErr    bool
		wantCalls  []string
	}{
		{
			name:   "success with restart always",
			svcCfg: config.ServiceConfig{Command: "claude run", Restart: "always"},
			wantCalls: []string{
				"ServiceInstall:test-svc:claude run:agent1:true",
				"ServiceStart:test-svc",
			},
		},
		{
			name:   "success without restart",
			svcCfg: config.ServiceConfig{Command: "claude run", Restart: "never"},
			wantCalls: []string{
				"ServiceInstall:test-svc:claude run:agent1:false",
				"ServiceStart:test-svc",
			},
		},
		{
			name:       "install fails",
			svcCfg:     config.ServiceConfig{Command: "claude run"},
			installErr: fmt.Errorf("disk full"),
			wantErr:    true,
		},
		{
			name:     "start fails",
			svcCfg:   config.ServiceConfig{Command: "claude run"},
			startErr: fmt.Errorf("already running"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := &mockPlatform{
				installErr: tt.installErr,
				startErr:   tt.startErr,
			}

			err := service.Install("test-svc", tt.svcCfg, "agent1", plat)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Install() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(plat.calls) != len(tt.wantCalls) {
					t.Fatalf("calls = %v, want %v", plat.calls, tt.wantCalls)
				}
				for i, want := range tt.wantCalls {
					if plat.calls[i] != want {
						t.Errorf("call[%d] = %q, want %q", i, plat.calls[i], want)
					}
				}
			}
		})
	}
}

func TestRemove(t *testing.T) {
	plat := &mockPlatform{}

	service.Remove("test-svc", plat)

	if len(plat.calls) != 1 || plat.calls[0] != "ServiceStop:test-svc" {
		t.Errorf("Remove() calls = %v, want [ServiceStop:test-svc]", plat.calls)
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		name         string
		statusResult bool
		statusErr    error
		wantRunning  bool
		wantErr      bool
	}{
		{
			name:         "running",
			statusResult: true,
			wantRunning:  true,
		},
		{
			name:         "not running",
			statusResult: false,
			wantRunning:  false,
		},
		{
			name:      "error",
			statusErr: fmt.Errorf("not found"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := &mockPlatform{
				statusResult: tt.statusResult,
				statusErr:    tt.statusErr,
			}

			running, err := service.Status("test-svc", plat)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Status() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && running != tt.wantRunning {
				t.Errorf("Status() = %v, want %v", running, tt.wantRunning)
			}
		})
	}
}
