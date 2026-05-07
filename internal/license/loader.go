package license

import (
	"fmt"
	"os"
	"path/filepath"
)

// Default search paths for the license file, tried in order.
var defaultPaths = []string{
	"", // $LATTICE_LICENSE_PATH
	"", // $LATTICE_LICENSE (env var containing JWT directly)
	"", // computed below
}

func init() {
	home, _ := os.UserHomeDir()

	defaultPaths[0] = os.Getenv("LATTICE_LICENSE_PATH")
	defaultPaths[1] = os.Getenv("LATTICE_LICENSE")
	if home != "" {
		defaultPaths[2] = filepath.Join(home, ".lattice", "license.lic")
	}
	defaultPaths = append(defaultPaths, "/var/lib/lattice/license.lic")
}

// ResolvePath finds the first available license file path.
// Returns the path and whether a license was found.
// For $LATTICE_LICENSE (env var containing the JWT directly), returns a marker.
func ResolvePath() (path string, fromEnv bool, found bool) {
	for _, p := range defaultPaths {
		if p == "" {
			continue
		}
		// Check if it's the $LATTICE_LICENSE env var (JWT string, not a path)
		if p == os.Getenv("LATTICE_LICENSE") {
			// Check it looks like a JWT
			if len(p) > 100 && p != os.Getenv("LATTICE_LICENSE_PATH") {
				return p, true, true
			}
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, false, true
		}
	}
	return "", false, false
}

// ReadFile reads the license file from the given path.
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read license file: %w", err)
	}
	return string(data), nil
}
