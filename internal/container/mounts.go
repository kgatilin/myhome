package container

import (
	"strings"
)

// ResolveMounts expands mount specifications into -v flags for the container runtime.
// Each mount can be a path (optionally with :ro suffix). Tilde (~) is expanded to homeDir.
func ResolveMounts(mounts []string, homeDir string) []string {
	var flags []string
	for _, m := range mounts {
		flags = append(flags, resolveMount(m, homeDir))
	}
	return flags
}

// resolveMount converts a single mount spec into a -v flag value.
// Examples:
//
//	~/.ssh:ro     -> /home/user/.ssh:/home/user/.ssh:ro
//	~/.gitconfig  -> /home/user/.gitconfig:/home/user/.gitconfig
func resolveMount(mount string, homeDir string) string {
	readOnly := false
	spec := mount
	if strings.HasSuffix(spec, ":ro") {
		readOnly = true
		spec = strings.TrimSuffix(spec, ":ro")
	}

	hostPath := expandTilde(spec, homeDir)

	// Mount to the same path inside the container.
	result := hostPath + ":" + hostPath
	if readOnly {
		result += ":ro"
	}
	return result
}

// expandTilde replaces a leading ~ with homeDir.
func expandTilde(path string, homeDir string) string {
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return homeDir + path[1:]
	}
	return path
}
