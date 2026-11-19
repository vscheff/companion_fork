package companion

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"time"
)

type cleanCheck struct {
	sha string
	at  time.Time
}

const (
	CleanupWaitImport = "import"
	CleanupWaitWoW    = "wow"
)

func (e *Engine) maybeClean(cfg Config, fs *FileStatus, backupDir string) bool {
	if c, ok := e.state.LastCleanup(fs.SourceID); ok {
		fs.LastCleanup = &c
	}
	if !cfg.AutoClean || fs.SHA256 == "" || fs.LastError != "" || fs.Bytes < int64(cfg.CleanAboveMB)<<20 {
		return false
	}
	r, ok := e.state.CensusCleanupReceipt(fs.SourceID, fs.SHA256, censusDestination(cfg.ServerURL, cfg.DeviceID))
	if !ok || r.Status != "imported" || e.state.PendingReceipts(fs.SourceID) > 0 {
		fs.CleanupWait = CleanupWaitImport
		return false
	}
	now := e.now()
	key := pathKey(fs.Path)
	e.mu.Lock()
	checked, seen := e.cleanChecked[key]
	e.mu.Unlock()
	if seen && checked.sha == fs.SHA256 && now.Sub(checked.at) < cleanupRecheckEvery {
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
	backup, err := WriteBackup(backupDir, snap, fs.SourceID)
	if err != nil {
		e.logf("error", "Cleanup skipped for %s: the backup failed: %v", label, err)
		return false
	}
	if running, _ := e.wowRunning(); running {
		fs.CleanupWait = CleanupWaitWoW
		return true
	}
	if sig, err := statSig(fs.Path); err != nil || !sig.Equal(snap.Sig) {
		return true
	}
	perm := os.FileMode(0o644)
	if fi, err := os.Stat(fs.Path); err == nil {
		perm = fi.Mode().Perm()
	}
	if err := writeFileAtomic(fs.Path, out, perm); err != nil {
		e.logf("error", "Cleanup of %s failed: %v", label, err)
		return true
	}
	sum := sha256.Sum256(out)
	newSHA := hex.EncodeToString(sum[:])
	if sig, err := statSig(fs.Path); err == nil {
		_ = e.state.SetSeen(fs.Path, sig, newSHA)
		fs.Bytes, fs.ModTime = sig.Size, sig.ModTime
	}
	carried := r
	carried.SHA256, carried.At = newSHA, now.UTC()
	_ = e.state.PutReceipt(carried)
	if cur, ok := e.state.Cursor(fs.SourceID); ok && cur.Census != nil && cur.Census.FileSHA256 == snap.SHA256 {
		// Cleanup only removes core rows. The trimmed file has no new rows under this consent.
		check := *cur.Census
		check.FileSHA256, check.UploadSHA = newSHA, ""
		_ = e.state.PutCensusResult(fs.SourceID, check, cur, Receipt{})
	}
	rec := Cleanup{CleanupResult: res, At: now.UTC(), Backup: backup}
	_ = e.state.PutCleanup(fs.SourceID, rec)
	e.mu.Lock()
	e.cleanChecked[key] = cleanCheck{sha: newSHA, at: now}
	e.mu.Unlock()
	fs.SHA256, fs.Uploaded, fs.LastReceipt, fs.LastCleanup = newSHA, true, &carried, &rec
	e.logf("info", "Cleaned up %s: %s → %s. Removed %s that wowcensus.io already has, keeping the last %d days. Backup: %s",
		label, humanBytes(res.BytesBefore), humanBytes(res.BytesAfter), res.Summary(), res.KeepDays, backup)
	return true
}
