package companion

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
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

func (e *Engine) combatLogCleanupSource(cfg Config, fs FileStatus, now time.Time) (seenFile, bool) {
	if !cfg.CleanCombatLogs || !cfg.ShareAddons[combatLogKey] || cfg.ServerURL == "" || cfg.DeviceID == "" || cfg.DeviceToken == "" {
		return seenFile{}, false
	}
	if fs.Addon != combatLogKey || !fs.Uploaded || fs.SHA256 == "" || fs.SourceID != e.state.SourceID(fs.Path) {
		return seenFile{}, false
	}
	r, ok := e.state.Receipt(fs.SourceID, fs.SHA256)
	if !ok || r.ReceiptID == "" || r.Status != "imported" || r.SourceID != fs.SourceID ||
		r.SHA256 != fs.SHA256 || r.UploadSHA != fs.SHA256 || r.Destination != censusDestination(cfg.ServerURL, cfg.DeviceID) {
		return seenFile{}, false
	}
	seen, ok := e.state.SeenSig(fs.Path)
	if !ok || seen.CombatLogSourceSHA256 == "" || seen.SHA256 != fs.SHA256 ||
		seen.Sig.Size <= 0 || seen.Sig.Size > maxCombatLogBytes || now.Sub(seen.Sig.ModTime) < time.Duration(cfg.CleanKeepDays)*24*time.Hour {
		return seenFile{}, false
	}
	sig, err := statSig(fs.Path)
	return seen, err == nil && sig.Equal(seen.Sig)
}

func combatLogSourceMatches(path string, seen seenFile) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(f, seen.Sig.Size+1))
	return err == nil && n == seen.Sig.Size && hex.EncodeToString(hash.Sum(nil)) == seen.CombatLogSourceSHA256
}

func (e *Engine) pruneCombatLogs(cfg Config, files []FileStatus) []FileStatus {
	if !cfg.CleanCombatLogs || !cfg.ShareAddons[combatLogKey] {
		return files
	}
	now := e.now()
	current := newestCombatLogs(files)
	type candidate struct {
		index int
		seen  seenFile
	}
	var doomed []candidate
	for i, f := range files {
		if current[pathKey(f.Path)] {
			continue
		}
		if seen, ok := e.combatLogCleanupSource(cfg, f, now); ok {
			doomed = append(doomed, candidate{index: i, seen: seen})
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
	for _, d := range doomed {
		f := files[d.index]
		if !combatLogSourceMatches(f.Path, d.seen) {
			continue
		}
		if running, err := e.wowRunning(); err != nil || running {
			break
		}
		if seen, ok := e.combatLogCleanupSource(e.Config(), f, e.now()); !ok || seen != d.seen {
			continue
		}
		if err := os.Remove(f.Path); err != nil {
			e.logf("warn", "Could not delete the old combat log %s: %v", filepath.Base(f.Path), err)
			continue
		}
		gone[d.index] = true
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
