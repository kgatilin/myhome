package platform

// Platform abstracts OS-specific operations for macOS and Linux.
type Platform interface {
	// OS returns the platform name ("darwin" or "linux").
	OS() string

	// HomeDir returns the base home directory path (e.g., /Users or /home).
	HomeDir() string

	// UserHome returns the home directory for a specific user.
	UserHome(username string) string

	// CreateUser creates an OS-level user account.
	CreateUser(username string) error

	// RemoveUser removes an OS-level user account and optionally their home directory.
	RemoveUser(username string, removeHome bool) error

	// CreateGroup creates a system group.
	CreateGroup(group string) error

	// AddUserToGroup adds a user to a group.
	AddUserToGroup(username, group string) error

	// SetReadOnlyACL sets read-only ACL for a user on a directory.
	SetReadOnlyACL(username, path string) error

	// PackageManager returns the name of the system package manager ("brew" or "apt").
	PackageManager() string

	// InstallPackages installs system packages using the platform's package manager.
	InstallPackages(packages []string) error

	// InstallCaskPackages installs GUI/cask packages (macOS only, no-op on Linux).
	InstallCaskPackages(packages []string) error

	// ListInstalledPackages returns currently installed packages from the system package manager.
	ListInstalledPackages() ([]string, error)

	// ServiceInstall installs a service (launchd plist or systemd unit).
	ServiceInstall(name, command, username string, restart bool) error

	// ServiceStart starts a service.
	ServiceStart(name string) error

	// ServiceStop stops a service.
	ServiceStop(name string) error

	// ServiceStatus returns whether a service is running.
	ServiceStatus(name string) (bool, error)
}
