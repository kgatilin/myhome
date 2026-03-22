package packages

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/kgatilin/myhome/internal/config"
	"github.com/kgatilin/myhome/internal/platform"
)

// PackageStatus describes whether a package is installed.
type PackageStatus struct {
	Name      string
	Expected  bool
	Installed bool
}

// List returns the status of expected vs installed packages.
func List(expected config.PackageSet, plat platform.Platform) ([]PackageStatus, error) {
	installed, err := plat.ListInstalledPackages()
	if err != nil {
		return nil, err
	}
	installedSet := make(map[string]bool, len(installed))
	for _, p := range installed {
		installedSet[p] = true
	}

	var packages []string
	if runtime.GOOS == "darwin" {
		packages = expected.Brew
	} else {
		packages = expected.Apt
	}

	var result []PackageStatus
	for _, p := range packages {
		result = append(result, PackageStatus{
			Name:      p,
			Expected:  true,
			Installed: installedSet[p],
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// Sync installs missing packages for the current platform.
func Sync(expected config.PackageSet, plat platform.Platform) error {
	installed, err := plat.ListInstalledPackages()
	if err != nil {
		return err
	}
	installedSet := make(map[string]bool, len(installed))
	for _, p := range installed {
		installedSet[p] = true
	}

	var packages []string
	if runtime.GOOS == "darwin" {
		packages = expected.Brew
	} else {
		packages = expected.Apt
	}

	var missing []string
	for _, p := range packages {
		if !installedSet[p] {
			missing = append(missing, p)
		}
	}

	if len(missing) > 0 {
		fmt.Printf("Installing %d packages: %s\n", len(missing), strings.Join(missing, ", "))
		if err := plat.InstallPackages(missing); err != nil {
			return err
		}
	}

	// Install cask packages on macOS
	if runtime.GOOS == "darwin" && len(expected.BrewCask) > 0 {
		if err := plat.InstallCaskPackages(expected.BrewCask); err != nil {
			return err
		}
	}

	return nil
}

// Dump captures currently installed packages into a PackageSet.
func Dump() (*config.PackageSet, error) {
	var pkgSet config.PackageSet
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("brew", "list", "--formula", "-1").Output()
		if err != nil {
			return nil, fmt.Errorf("brew list: %w", err)
		}
		pkgSet.Brew = splitLines(string(out))

		caskOut, err := exec.Command("brew", "list", "--cask", "-1").Output()
		if err == nil {
			pkgSet.BrewCask = splitLines(string(caskOut))
		}
	} else {
		out, err := exec.Command("dpkg-query", "-W", "-f", "${Package}\n").Output()
		if err != nil {
			return nil, fmt.Errorf("dpkg-query: %w", err)
		}
		pkgSet.Apt = splitLines(string(out))
	}
	return &pkgSet, nil
}

func splitLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	result := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}

// DumpToWriter prints the current packages in YAML format suitable for myhome.yml.
func DumpToWriter(w *os.File) error {
	pkgs, err := Dump()
	if err != nil {
		return err
	}
	if len(pkgs.Brew) > 0 {
		fmt.Fprintf(w, "  brew: [%s]\n", strings.Join(pkgs.Brew, ", "))
	}
	if len(pkgs.BrewCask) > 0 {
		fmt.Fprintf(w, "  brew_cask: [%s]\n", strings.Join(pkgs.BrewCask, ", "))
	}
	if len(pkgs.Apt) > 0 {
		fmt.Fprintf(w, "  apt: [%s]\n", strings.Join(pkgs.Apt, ", "))
	}
	return nil
}
