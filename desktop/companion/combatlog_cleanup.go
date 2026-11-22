package companion

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func newestCombatLogs(files []FileStatus) map[string]bool {
	newest := map[string]FileStatus{}
	for _, f := range files {
		if f.Addon != combatLogKey {
			continue
		}
		dir := pathKey(filepath.Dir(f.Path))
		if cur, ok := newest[dir]; !ok || f.ModTime.After(cur.ModTime) {
			newest[dir] = f
		}
	}
	out := map[string]bool{}
	for _, f := range newest {
		out[pathKey(f.Path)] = true
	}
	return out
}

func (e *Engine) combatLogDeletable(cfg Config, fs FileStatus, now time.Time) bool {
	if fs.Addon != combatLogKey || !fs.Uploaded || fs.LastReceipt == nil || fs.SHA256 == "" {
		return false
	}
	if fs.LastReceipt.Status != "imported" || fs.LastReceipt.SHA256 != fs.SHA256 {
		return false
	}
	if now.Sub(fs.ModTime) < time.Duration(cfg.CleanKeepDays)*24*time.Hour {
		return false
	}
	seen, ok := e.state.SeenSig(fs.Path)
	if !ok || seen.SHA256 != fs.SHA256 {
		return false
	}
	sig, err := statSig(fs.Path)
	return err == nil && sig.Equal(seen.Sig)
}

func (e *Engine) pruneCombatLogs(cfg Config, files []FileStatus) []FileStatus {
	if !cfg.CleanCombatLogs || !cfg.ShareAddons[combatLogKey] {
		return files
	}
	now := e.now()
	current := newestCombatLogs(files)
	var doomed []int
	for i, f := range files {
		if !current[pathKey(f.Path)] && e.combatLogDeletable(cfg, f, now) {
			doomed = append(doomed, i)
		}
	}
	if len(doomed) == 0 {
		return files
	}
	if running, err := e.wowRunning(); err != nil || running {
		return files
	}
	gone := map[int]bool{}
	var freed int64
	var names []string
	for _, i := range doomed {
		f := files[i]
		if err := os.Remove(f.Path); err != nil {
			e.logf("warn", "Could not delete the old combat log %s: %v", filepath.Base(f.Path), err)
			continue
		}
		gone[i] = true
		freed += f.Bytes
		names = append(names, f.Edition+"/"+filepath.Base(f.Path))
	}
	if len(gone) == 0 {
		return files
	}
	e.logf("info", "Deleted %d old combat log(s) that wowcensus.io has imported, freeing %s: %s",
		len(gone), humanBytes(freed), strings.Join(names, ", "))
	kept := files[:0:0]
	for i, f := range files {
		if !gone[i] {
			kept = append(kept, f)
		}
	}
	return kept
}
