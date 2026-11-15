//go:build !windows

package companion

import (
	"os/exec"
	"strings"
)

func processNames() ([]string, error) {
	out, err := exec.Command("ps", "-A", "-o", "comm=").Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}
