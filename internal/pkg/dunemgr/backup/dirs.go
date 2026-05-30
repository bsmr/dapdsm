package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// nameRE allows only safe backup-name characters. The name flows
// into both filesystem paths and a remote shell argument; this
// rejects shell-meta, path-walks, leading dashes, and whitespace.
var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// ValidateName rejects empty / shell-meta / path-walking names.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("backup name %q: must match [A-Za-z0-9][A-Za-z0-9_.-]{0,63}", name)
	}
	return nil
}

// LocalDir returns DataDir/<host>/<bg>/ (creating it).
func LocalDir(dataDir, host, bg string) (string, error) {
	if err := ValidateName(host); err != nil {
		return "", fmt.Errorf("host: %w", err)
	}
	if err := ValidateName(bg); err != nil {
		return "", fmt.Errorf("bg: %w", err)
	}
	dir := filepath.Join(dataDir, host, bg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// LocalPair returns the local .backup path under LocalDir. The
// .backup.yaml partner is LocalPair + ".yaml".
func LocalPair(dataDir, host, bg string, unixTS int64, name string) (string, error) {
	dir, err := LocalDir(dataDir, host, bg)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%d__%s.backup", unixTS, name)), nil
}

// RemotePair returns the on-host paths for both files.
func RemotePair(hostBackupDir, bg, name string) (backupPath, yamlPath string) {
	base := filepath.Join(hostBackupDir, bg, name)
	return base + ".backup", base + ".backup.yaml"
}
