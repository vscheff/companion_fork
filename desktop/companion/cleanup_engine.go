package companion

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"slices"
	"time"
)

type cleanCheck struct {
	sha             string
	at              time.Time
	coverageBlocked bool
}

const (
	CleanupWaitImport   = "import"
	CleanupWaitCoverage = "coverage"
	CleanupWaitWoW      = "wow"
)

func (e *Engine) maybeClean(cfg Config, fs *FileStatus, backupDir string) bool {
	if c, ok := e.state.LastCleanup(fs.SourceID); ok {
		fs.LastCleanup = &c
	}
	if !cfg.AutoClean || fs.SHA256 == "" || fs.LastError != "" || fs.Bytes < int64(cfg.CleanAboveMB)<<20 {
		return false
	}
	destination := censusDestination(cfg.ServerURL, cfg.DeviceID)
	r, records, wait := e.state.CensusCleanupReceipt(fs.SourceID, fs.SHA256, destination)
	if wait != "" {
		if wait == CleanupWaitCoverage {
			e.waitForCleanupCoverage(fs)
		} else {
			fs.CleanupWait = wait
		}
		return false
	}
	now := e.now()
	key := pathKey(fs.Path)
	e.mu.Lock()
	checked, previouslyChecked := e.cleanChecked[key]
	e.mu.Unlock()
	if previouslyChecked && checked.sha == fs.SHA256 && now.Sub(checked.at) < cleanupRecheckEvery && !(checked.coverageBlocked && records.Complete) {
		if checked.coverageBlocked {
			e.waitForCleanupCoverage(fs)
		}
		return false
	}
	if running, _ := e.wowRunning(); running {
		fs.CleanupWait = CleanupWaitWoW
		return false
	}
	snap, err := TakeSnapshot(fs.SavedVarFile, now)
	if err != nil || snap.SHA256 != fs.SHA256 {
		return false
	}
	out, res, err := CleanSavedVariables(snap.Data, now, cfg.CleanKeepDays)
	e.mu.Lock()
	e.cleanChecked[key] = cleanCheck{sha: fs.SHA256, at: now}
	e.mu.Unlock()
	label := fs.Edition + " / " + fs.AccountLabel()
	if err != nil {
		e.logf("warn", "Cleanup skipped for %s: %v", label, err)
		return false
	}
	if res.BytesBefore-res.BytesAfter < minCleanupSaving {
		return false
	}
	if !records.coversCleanup(snap.Data, out) {
		e.mu.Lock()
		e.cleanChecked[key] = cleanCheck{sha: fs.SHA256, at: now, coverageBlocked: true}
		e.mu.Unlock()
		e.waitForCleanupCoverage(fs)
		return false
	}
	backup, err := WriteBackup(backupDir, snap, fs.SourceID)
	if err != nil {
		e.logf("error", "Cleanup skipped for %s: the backup failed: %v", label, err)
		return false
	}
	if running, _ := e.wowRunning(); running {
		fs.CleanupWait = CleanupWaitWoW
		return true
	}
	e.mu.Lock()
	current := e.cfg
	if !current.AutoClean || current.ServerURL != cfg.ServerURL || current.DeviceID != cfg.DeviceID ||
		current.CleanKeepDays != cfg.CleanKeepDays || current.CleanAboveMB != cfg.CleanAboveMB {
		e.mu.Unlock()
		return true
	}
	if sig, err := statSig(fs.Path); err != nil || !sig.Equal(snap.Sig) {
		e.mu.Unlock()
		return true
	}
	perm := os.FileMode(0o644)
	if fi, err := os.Stat(fs.Path); err == nil {
		perm = fi.Mode().Perm()
	}
	err = writeFileAtomic(fs.Path, out, perm)
	e.mu.Unlock()
	if err != nil {
		e.logf("error", "Cleanup of %s failed: %v", label, err)
		return true
	}
	sum := sha256.Sum256(out)
	newSHA := hex.EncodeToString(sum[:])
	seen, statErr := statSig(fs.Path)
	if statErr == nil {
		fs.Bytes, fs.ModTime = seen.Size, seen.ModTime
	}
	carried := r
	carried.SHA256, carried.At = newSHA, now.UTC()
	rec := Cleanup{CleanupResult: res, At: now.UTC(), Backup: backup}
	if err := e.state.PutCensusCleanup(fs.SourceID, destination, snap, newSHA, seen, rec); err != nil {
		e.logf("error", "Could not save cleanup state for %s: %v", label, err)
	}
	e.mu.Lock()
	e.cleanChecked[key] = cleanCheck{sha: newSHA, at: now}
	e.mu.Unlock()
	fs.SHA256, fs.Uploaded, fs.LastReceipt, fs.LastCleanup = newSHA, true, &carried, &rec
	e.logf("info", "Cleaned up %s: %s → %s. Removed %s that wowcensus.io already has, keeping the last %d days. Backup: %s",
		label, humanBytes(res.BytesBefore), humanBytes(res.BytesAfter), res.Summary(), res.KeepDays, backup)
	return true
}

func (e *Engine) waitForCleanupCoverage(fs *FileStatus) {
	fs.CleanupWait = CleanupWaitCoverage
	e.mu.Lock()
	alreadyWaiting := slices.ContainsFunc(e.status.Files, func(f FileStatus) bool {
		return f.SourceID == fs.SourceID && f.CleanupWait == CleanupWaitCoverage
	})
	e.mu.Unlock()
	if !alreadyWaiting {
		e.logf("warn", "Cleanup paused for %s / %s because import coverage is incomplete. Upload now can send a full census snapshot.",
			fs.Edition, fs.AccountLabel())
	}
}
