package container

import (
	"github.com/kgatilin/myhome/internal/config"
)

// ResolveAuth returns the mount flags and environment variable flags needed
// to inject a Claude auth profile into a container.
func ResolveAuth(profile config.AuthProfile, claudeConfigDir string, homeDir string) (mounts []string, envVars []string) {
	authFilePath := expandTilde(profile.AuthFile, homeDir)

	// Mount the auth file into the container at the same path.
	mounts = append(mounts, authFilePath+":"+authFilePath+":ro")

	// Mount the Claude config directory.
	configDir := expandTilde(claudeConfigDir, homeDir)
	mounts = append(mounts, configDir+":"+configDir)

	// Add environment variables from the profile.
	for k, v := range profile.Env {
		envVars = append(envVars, k+"="+v)
	}

	return mounts, envVars
}
