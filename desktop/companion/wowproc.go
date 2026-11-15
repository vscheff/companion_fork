package companion

import (
	"path/filepath"
	"regexp"
	"strings"
)

var wowProcessRe = regexp.MustCompile(`^wow(classic)?[a-z]?(-64)?(\.exe)?$`)

func isWoWProcess(name string) bool {
	base := strings.ToLower(filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), `\`, "/")))
	if wowProcessRe.MatchString(base) {
		return true
	}
	return strings.HasPrefix(base, "world of warcraft") && !strings.Contains(base, "launcher")
}

func WoWRunning() (bool, error) {
	names, err := processNames()
	if err != nil {
		return true, err
	}
	for _, n := range names {
		if isWoWProcess(n) {
			return true, nil
		}
	}
	return false, nil
}
