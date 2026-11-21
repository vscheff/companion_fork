package companion

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type LogEntry struct {
	Seq     int64     `json:"seq"`
	At      time.Time `json:"at"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type FileStatus struct {
	SavedVarFile
	SourceID      string     `json:"sourceId"`
	Bytes         int64      `json:"bytes"`
	ModTime       time.Time  `json:"modTime"`
	SHA256        string     `json:"sha256"`
	OverThreshold bool       `json:"overThreshold"`
	Uploaded      bool       `json:"uploaded"`
	LastReceipt   *Receipt   `json:"lastReceipt,omitempty"`
	NextRetry     *time.Time `json:"nextRetry,omitempty"`
	LastError     string     `json:"lastError,omitempty"`
	LastCleanup   *Cleanup   `json:"lastCleanup,omitempty"`
	CleanupWait   string     `json:"cleanupWait,omitempty"`
	Addon         string     `json:"addon,omitempty"`
	Character     string     `json:"character,omitempty"`
}

type Status struct {
	Running    bool         `json:"running"`
	Busy       string       `json:"busy,omitempty"`
	LastScan   time.Time    `json:"lastScan"`
	AutoUpload bool         `json:"autoUpload"`
	Files      []FileStatus `json:"files"`
	AddonFiles []FileStatus `json:"addonFiles"`
	Addon      AddonStatus  `json:"addon"`
	Update     UpdateStatus `json:"update"`
	Log        []LogEntry   `json:"log"`
}

type MaintenanceReport struct {
	GeneratedAt time.Time    `json:"generatedAt"`
	ThresholdMB int          `json:"thresholdMB"`
	Files       []FileStatus `json:"files"`
	BackupDir   string       `json:"backupDir"`
	BackupCount int          `json:"backupCount"`
	BackupBytes int64        `json:"backupBytes"`
	BackupCapMB int          `json:"backupCapMB"`
	BackupKeep  int          `json:"backupKeep"`
	Notes       []string     `json:"notes"`
}

type action int

const (
	actScan action = iota
	actUploadNow
	actBackupNow
)

type Engine struct {
	appDir string
	state  *State

	mu       sync.Mutex
	cfg      Config
	status   Status
	onChange func(Status)
	logSeq   int64

	actions chan action

	now              func() time.Time
	newClient        func(serverURL, token string) *Client
	discover         func(roots []string) []SavedVarFile
	discoverEditions func(roots []string) []string
	wowRunning       func() (bool, error)
	httpClient       *http.Client
	cleanChecked     map[string]cleanCheck

	update        UpdateInfo
	updateData    []byte
	onUpdateFound func(version string)

	addonLatest   AddonRelease
	addonLatestAt time.Time
	addonErr      string

	uploadPause time.Time
}

var (
	errUploadPaused          = errors.New("waiting for the server's upload limit")
	errUploadSettingsChanged = errors.New("upload settings changed")
)

const maxLog = 200

func NewEngine(appDir string) (*Engine, error) {
	if appDir == "" {
		d, err := DefaultAppDir()
		if err != nil {
			return nil, err
		}
		appDir = d
	}
	cfg, cfgErr := LoadConfig(appDir)
	st, err := LoadState(appDir)
	if err != nil {
		return nil, err
	}
	e := &Engine{
		appDir: appDir, state: st, cfg: cfg,
		actions:          make(chan action, 8),
		now:              time.Now,
		newClient:        NewClient,
		discover:         DiscoverWithDefaults,
		discoverEditions: DiscoverEditionsWithDefaults,
		wowRunning:       WoWRunning,
		httpClient:       &http.Client{Timeout: addonDownloadLimit},
		cleanChecked:     map[string]cleanCheck{},
	}
	e.status.Files = []FileStatus{}
	e.status.AddonFiles = []FileStatus{}
	e.status.AutoUpload = cfg.AutoUpload
	e.status.Addon.Installs = []AddonInstall{}
	e.status.Update.Current = AppVersion
	e.status.Log = []LogEntry{}
	if cfgErr != nil {
		e.logf("error", "%v (defaults loaded)", cfgErr)
	}
	return e, nil
}

func (e *Engine) AppDir() string { return e.appDir }

func (e *Engine) OnChange(fn func(Status)) {
	e.mu.Lock()
	e.onChange = fn
	e.mu.Unlock()
}

func (e *Engine) Config() Config {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := e.cfg
	c.WoWRoots = append([]string{}, e.cfg.WoWRoots...)
	c.ShareAddons = make(map[string]bool, len(e.cfg.ShareAddons))
	for k, v := range e.cfg.ShareAddons {
		c.ShareAddons[k] = v
	}
	return c
}

func (e *Engine) UpdateConfig(c Config) error {
	c.Normalize()
	if err := e.saveSettings(c); err != nil {
		return err
	}
	e.logf("info", "Settings saved (automatic uploads %s, automatic cleanup %s)", onOff(c.AutoUpload), onOff(c.AutoClean))
	e.Trigger(actScan)
	return nil
}

func (e *Engine) saveSettings(c Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	old := e.cfg
	c.DeviceID, c.DeviceToken = old.DeviceID, old.DeviceToken
	if c.ServerURL != old.ServerURL {
		c.DeviceID, c.DeviceToken = "", ""
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if c.StartAtLogin != old.StartAtLogin {
		if err := SetStartAtLogin(c.StartAtLogin); err != nil {
			return fmt.Errorf("start at login: %w", err)
		}
	}
	if err := SaveConfig(e.appDir, c); err != nil {
		return err
	}
	if c.ServerURL != old.ServerURL {
		_ = e.state.ClearFailures()
	}
	e.cfg = c
	e.status.AutoUpload = c.AutoUpload
	if c.AutoClean != old.AutoClean || c.CleanKeepDays != old.CleanKeepDays {
		e.cleanChecked = map[string]cleanCheck{}
	}
	return nil
}

func (e *Engine) setDevice(serverURL, id, token string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := e.cfg
	if c.ServerURL != serverURL {
		return errUploadSettingsChanged
	}
	c.DeviceID, c.DeviceToken = id, token
	if err := SaveConfig(e.appDir, c); err != nil {
		return err
	}
	e.cfg = c
	return nil
}

func (e *Engine) ensureDevice(ctx context.Context, serverURL string) (string, string, error) {
	cfg := e.Config()
	if cfg.ServerURL != serverURL {
		return "", "", errUploadSettingsChanged
	}
	if cfg.DeviceToken != "" {
		return cfg.DeviceID, cfg.DeviceToken, nil
	}
	c := e.newClient(cfg.ServerURL, "")
	c.ClientID = e.state.ClientID()
	id, token, err := c.Register(ctx)
	if err != nil {
		return "", "", fmt.Errorf("registering this computer: %w", err)
	}
	if err := e.setDevice(serverURL, id, token); err != nil {
		return "", "", err
	}
	e.logf("info", "Registered this computer with %s (%s)", cfg.ServerURL, id)
	return id, token, nil
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.status
	s.Files = publicStatuses(e.status.Files)
	s.AddonFiles = publicStatuses(e.status.AddonFiles)
	s.Log = append([]LogEntry{}, e.status.Log...)
	return s
}

func publicStatuses(in []FileStatus) []FileStatus {
	out := make([]FileStatus, len(in))
	for i, f := range in {
		f.SavedVarFile = f.SavedVarFile.Public()
		f.LastError = HideAccountFolders(f.LastError)
		out[i] = f
	}
	return out
}

func (e *Engine) Trigger(a action) {
	select {
	case e.actions <- a:
	default:
	}
}

func (e *Engine) ScanNow()   { e.Trigger(actScan) }
func (e *Engine) UploadNow() { e.Trigger(actUploadNow) }
func (e *Engine) BackupNow() { e.Trigger(actBackupNow) }

func (e *Engine) Run(ctx context.Context) {
	e.setRunning(true)
	defer e.setRunning(false)
	e.logf("info", "Watcher started (polling every %ds)", e.Config().PollSeconds)
	e.CheckForUpdate(ctx)
	e.reconcile(ctx, actScan)
	for {
		interval := time.Duration(e.Config().PollSeconds) * time.Second
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case a := <-e.actions:
			timer.Stop()
			e.reconcile(ctx, a)
		case <-timer.C:
			e.maybeCheckUpdate(ctx)
			e.reconcile(ctx, actScan)
		}
	}
}

func (e *Engine) RunOnce(ctx context.Context, upload, backup bool) {
	switch {
	case upload:
		e.reconcile(ctx, actUploadNow)
	case backup:
		e.reconcile(ctx, actBackupNow)
	default:
		e.reconcile(ctx, actScan)
	}
}

func (e *Engine) reconcile(ctx context.Context, a action) {
	cfg := e.Config()
	forceUpload := a == actUploadNow
	forceBackup := a == actBackupNow
	switch a {
	case actUploadNow:
		e.setBusy("Uploading")
		if cfg.ServerURL == "" {
			e.logf("error", "Upload now needs a server URL in Settings")
			forceUpload = false
		}
	case actBackupNow:
		e.setBusy("Backing up")
	default:
		e.setBusy("Scanning")
	}
	defer e.setBusy("")

	files := e.discover(cfg.WoWRoots)
	now := e.now()
	statuses := make([]FileStatus, 0, len(files))
	backupDir := cfg.ResolvedBackupDir(e.appDir)
	backedUp := false

	for _, f := range files {
		if ctx.Err() != nil {
			return
		}
		fs := FileStatus{SavedVarFile: f, SourceID: e.state.SourceID(f.Path)}
		sig, err := statSig(f.Path)
		if err != nil {
			fs.LastError = err.Error()
			statuses = append(statuses, fs)
			continue
		}
		fs.Bytes, fs.ModTime = sig.Size, sig.ModTime
		fs.OverThreshold = sig.Size > int64(cfg.SizeThresholdMB)<<20
		seen, known := e.state.SeenSig(f.Path)
		changed := !known || !seen.Sig.Equal(sig)
		if !changed {
			fs.SHA256 = seen.SHA256
		}
		uploadCfg := e.Config()
		_, checked := e.state.CensusResult(fs.SourceID, censusCheckpointFor(uploadCfg, fs.SHA256))
		_, inBackoff := e.state.Backoff(fs.SourceID, fs.SHA256, now)
		rebase := forceUpload && e.censusNeedsFull(uploadCfg, fs.SourceID)
		wantUpload := (uploadCfg.AutoUpload || forceUpload) && uploadCfg.ServerURL != "" &&
			(!checked || rebase) && (forceUpload || !inBackoff)
		wantBackup := forceBackup || (cfg.AutoBackup && changed)

		if changed || wantUpload || wantBackup {
			snap, err := TakeSnapshot(f, now)
			switch {
			case errors.Is(err, ErrUnstable):
				if forceUpload || forceBackup {
					e.logf("warn", "%s (%s) is still being written; will retry", f.Edition, f.AccountLabel())
				}
				fs.LastError = "waiting for the file to settle"
				statuses = append(statuses, fs)
				continue
			case err != nil:
				fs.LastError = err.Error()
				e.logf("error", "Read %s: %v", f.Path, err)
				statuses = append(statuses, fs)
				continue
			}
			fs.SHA256 = snap.SHA256
			if changed {
				_ = e.state.SetSeen(f.Path, snap.Sig, snap.SHA256)
				if known {
					e.logf("info", "New save detected: %s / %s (%s)", f.Edition, f.AccountLabel(), humanBytes(sig.Size))
				}
			}
			if wantBackup {
				if p, err := WriteBackup(backupDir, snap, fs.SourceID); err != nil {
					e.logf("error", "Backup failed: %v", err)
				} else {
					backedUp = true
					e.logf("info", "Backup written: %s", p)
				}
			}
			_, inBackoff = e.state.Backoff(fs.SourceID, snap.SHA256, now)
			if wantUpload && (forceUpload || !inBackoff) {
				e.uploadCensus(ctx, snap, fs.SourceID, forceUpload)
			}
		}

		uploadCfg = e.Config()
		if r, ok := e.state.CensusResult(fs.SourceID, censusCheckpointFor(uploadCfg, fs.SHA256)); ok {
			fs.Uploaded = true
			fs.LastReceipt = &r
		} else if r, ok := e.state.LatestReceipt(fs.SourceID); ok {
			fs.LastReceipt = &r
		}
		if fail, inBackoff := e.state.Backoff(fs.SourceID, fs.SHA256, e.now()); inBackoff {
			t := fail.NextTry
			fs.NextRetry = &t
			fs.LastError = fail.LastErr
		}
		if e.maybeClean(uploadCfg, &fs, backupDir) {
			backedUp = true
		}
		statuses = append(statuses, fs)
	}

	if backedUp || forceBackup {
		if n, err := RotateBackups(backupDir, cfg.BackupKeep, int64(cfg.BackupCapMB)<<20); err != nil {
			e.logf("error", "Backup rotation: %v", err)
		} else if n > 0 {
			e.logf("info", "Removed %d old backup(s)", n)
		}
	}
	if forceBackup && len(files) == 0 {
		e.logf("warn", "No WoWCensus.lua found to back up")
	}

	cfg = e.Config()
	addonFiles := e.reconcileAddons(ctx, cfg, forceUpload)

	if (cfg.AutoUpload || cfg.AutoClean) && cfg.DeviceToken != "" {
		e.refreshReceipts(ctx, cfg, statuses, addonFiles)
	}
	addon := e.addonStatus(cfg)

	e.mu.Lock()
	e.status.Files = statuses
	e.status.AddonFiles = addonFiles
	e.status.Addon = addon
	e.status.LastScan = e.now().UTC()
	e.status.AutoUpload = cfg.AutoUpload
	e.mu.Unlock()
	e.notify()
}

func (e *Engine) send(ctx context.Context, cfg Config, m Manifest, data []byte, label string) (Receipt, error) {
	e.mu.Lock()
	paused := e.now().Before(e.uploadPause)
	e.mu.Unlock()
	if paused {
		return Receipt{}, errUploadPaused
	}
	if !e.uploadAllowed(cfg, m) {
		return Receipt{}, errUploadSettingsChanged
	}
	id, token, err := e.ensureDevice(ctx, cfg.ServerURL)
	if err != nil {
		return Receipt{}, err
	}
	destination := censusDestination(cfg.ServerURL, id)
	if m.Kind == KindCensus {
		if r, ok := e.state.CensusReceipt(m.SourceID, destination, m.SHA256); ok {
			r.Duplicate = true
			return r, nil
		}
	}
	b, err := BuildBundleData(m, data)
	if err != nil {
		return Receipt{}, fmt.Errorf("bundle failed: %w", err)
	}
	what := "the whole export"
	if m.Mode == ModeSession {
		what = fmt.Sprintf("%d new rows", m.Rows)
	}
	e.logf("info", "Uploading %s: %s (%s zipped)", label, what, humanBytes(int64(len(b.Zip))))
	c := e.newClient(cfg.ServerURL, token)
	c.ClientID = e.state.ClientID()
	if !e.uploadAllowed(cfg, m) {
		return Receipt{}, errUploadSettingsChanged
	}
	r, err := c.Upload(ctx, b)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized {
			_ = e.setDevice(cfg.ServerURL, "", "")
		}
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
			wait := max(apiErr.RetryAfter, time.Minute)
			e.mu.Lock()
			e.uploadPause = e.now().Add(wait)
			e.mu.Unlock()
			e.logf("info", "The server asked to slow down; the remaining uploads continue in about %d minute(s).", int(wait.Round(time.Minute)/time.Minute))
			return Receipt{}, errUploadPaused
		}
		return Receipt{}, err
	}
	if m.Kind == KindCensus {
		r.Destination = destination
	}
	return r, nil
}

func (e *Engine) uploadAllowed(cfg Config, m Manifest) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cfg.ServerURL != e.cfg.ServerURL {
		return false
	}
	if m.Kind == KindAddon {
		return e.cfg.ShareAddons[m.Addon]
	}
	return censusMarketConsent(cfg.ShareAddons) == censusMarketConsent(e.cfg.ShareAddons)
}

func (e *Engine) uploadFailed(sourceID, fileSHA, label string, err error) {
	if errors.Is(err, errUploadPaused) || errors.Is(err, errUploadSettingsChanged) {
		return
	}
	_ = e.state.RecordFailure(sourceID, fileSHA, err, e.now())
	e.logf("error", "Upload of %s failed: %v", label, err)
}

func (e *Engine) accept(r Receipt, fileSHA, label string) {
	r.SHA256 = fileSHA
	_ = e.state.PutReceipt(r)
	e.logReceipt(r, label)
}

func (e *Engine) logReceipt(r Receipt, label string) {
	if r.Duplicate {
		e.logf("info", "%s: server already has this version", label)
	} else {
		e.logf("info", "%s uploaded. It will appear on wowcensus.io after the next import.", label)
	}
}

func (e *Engine) coverWithLatest(sourceID, fileSHA string) bool {
	prev, ok := e.state.LatestReceipt(sourceID)
	if !ok {
		return false
	}
	prev.SHA256, prev.At, prev.Duplicate = fileSHA, e.now().UTC(), true
	_ = e.state.PutReceipt(prev)
	return true
}

func (e *Engine) censusNeedsFull(cfg Config, sourceID string) bool {
	cur, ok := e.state.Cursor(sourceID)
	if !ok {
		return true
	}
	destination := censusDestination(cfg.ServerURL, cfg.DeviceID)
	if cur.Census != nil && cur.Census.Destination != destination {
		return true
	}
	return cfg.AutoClean && e.state.CensusNeedsBaseline(sourceID, destination)
}

func (e *Engine) uploadCensus(ctx context.Context, snap *Snapshot, sourceID string, forceUpload bool) {
	cfg := e.Config()
	if cfg.ServerURL == "" || !(cfg.AutoUpload || forceUpload) {
		return
	}
	label := snap.File.Edition + " / " + snap.File.AccountLabel()
	check := censusCheckpointFor(cfg, snap.SHA256)
	full := e.censusNeedsFull(cfg, sourceID)
	if _, ok := e.state.CensusResult(sourceID, check); ok && !(forceUpload && full) {
		return
	}
	m := NewManifest(snap, sourceID)
	cur, hasCursor := e.state.Cursor(sourceID)
	var d Delta
	var err error
	if hasCursor && !full {
		d, err = BuildCensusDelta(snap.Data, cur, cfg.ShareAddons)
		m.Mode, m.Rows = ModeSession, d.Rows
	} else {
		d, err = buildCensusFull(snap.Data, cfg.ShareAddons)
	}
	if err != nil {
		e.uploadFailed(sourceID, snap.SHA256, label, fmt.Errorf("could not build a census export: %w", err))
		return
	}
	var r Receipt
	if len(d.Lua) > 0 {
		m.SetData(d.Lua)
		r, err = e.send(ctx, cfg, m, d.Lua, label)
		if err != nil {
			e.uploadFailed(sourceID, snap.SHA256, label, err)
			return
		}
	}
	if err := e.state.PutCensusResult(sourceID, check, d.Next, m.Mode, r); err != nil {
		e.logf("error", "Could not save upload state for %s: %v", label, err)
		return
	}
	if r.ReceiptID == "" {
		e.logf("info", "%s: nothing new to upload", label)
	} else {
		e.logReceipt(r, label)
	}
}

func (e *Engine) accountID(f SavedVarFile) string {
	return e.state.SourceID(filepath.Join(f.Root, "WTF", "Account", f.Account))
}

func (e *Engine) reconcileAddons(ctx context.Context, cfg Config, forceUpload bool) []FileStatus {
	statuses := []FileStatus{}
	if !cfg.SharesAddons() {
		return statuses
	}
	files := DiscoverAddonFiles(e.discoverEditions(cfg.WoWRoots), cfg.ShareAddons)
	now := e.now()
	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		fs := FileStatus{SavedVarFile: f.SavedVarFile, SourceID: e.state.SourceID(f.Path), Addon: f.Addon, Character: f.Character}
		sig, err := statSig(f.Path)
		if err != nil {
			fs.LastError = err.Error()
			statuses = append(statuses, fs)
			continue
		}
		fs.Bytes, fs.ModTime = sig.Size, sig.ModTime
		seen, known := e.state.SeenSig(f.Path)
		changed := !known || !seen.Sig.Equal(sig)
		if !changed {
			fs.SHA256 = seen.SHA256
		}
		_, hasReceipt := e.state.Receipt(fs.SourceID, fs.SHA256)
		_, inBackoff := e.state.Backoff(fs.SourceID, fs.SHA256, now)
		want := (cfg.AutoUpload || forceUpload) && cfg.ServerURL != "" && (changed || !hasReceipt) && (forceUpload || !inBackoff)
		if want {
			snap, err := TakeSnapshot(f.SavedVarFile, now)
			switch {
			case errors.Is(err, ErrUnstable):
				fs.LastError = "waiting for the file to settle"
			case err != nil:
				fs.LastError = err.Error()
			default:
				fs.SHA256 = snap.SHA256
				if changed {
					_ = e.state.SetSeen(f.Path, snap.Sig, snap.SHA256)
				}
				_, hasReceipt = e.state.Receipt(fs.SourceID, snap.SHA256)
				_, inBackoff = e.state.Backoff(fs.SourceID, snap.SHA256, now)
				if !hasReceipt && (forceUpload || !inBackoff) {
					e.uploadAddon(ctx, f, snap, fs.SourceID, forceUpload)
				}
			}
		}
		if r, ok := e.state.Receipt(fs.SourceID, fs.SHA256); ok {
			fs.Uploaded = true
			fs.LastReceipt = &r
		} else if r, ok := e.state.LatestReceipt(fs.SourceID); ok {
			fs.LastReceipt = &r
		}
		if fail, inBackoff := e.state.Backoff(fs.SourceID, fs.SHA256, e.now()); inBackoff {
			t := fail.NextTry
			fs.NextRetry = &t
			fs.LastError = fail.LastErr
		}
		statuses = append(statuses, fs)
	}
	return statuses
}

func (e *Engine) uploadAddon(ctx context.Context, f AddonFile, snap *Snapshot, sourceID string, forceUpload bool) {
	cfg := e.Config()
	if !cfg.ShareAddons[f.Addon] || cfg.ServerURL == "" || !(cfg.AutoUpload || forceUpload) {
		return
	}
	label := f.Label()
	cur, hasCursor := e.state.Cursor(sourceID)
	d, err := BuildAddonUpload(f, snap.Data, cur)
	if err != nil {
		_ = e.state.RecordFailure(sourceID, snap.SHA256, err, e.now())
		e.logf("error", "%s: %v", label, err)
		return
	}
	if d.Lua == nil {
		_ = e.state.SetCursor(sourceID, d.Next)
		if !e.coverWithLatest(sourceID, snap.SHA256) {
			_ = e.state.PutReceipt(Receipt{SourceID: sourceID, SHA256: snap.SHA256, Status: "empty", At: e.now().UTC()})
		}
		return
	}
	a, _ := extraAddon(f.Addon)
	m := NewManifest(snap, sourceID)
	m.Kind, m.Addon, m.FileName, m.Rows = KindAddon, f.Addon, a.File, d.Rows
	m.Realm, m.Character, m.AccountID = f.Realm, f.Character, e.accountID(f.SavedVarFile)
	if hasCursor {
		m.Mode = ModeSession
	}
	m.SetData(d.Lua)
	r, err := e.send(ctx, cfg, m, d.Lua, label)
	if err != nil {
		e.uploadFailed(sourceID, snap.SHA256, label, err)
		return
	}
	e.accept(r, snap.SHA256, label)
	_ = e.state.SetCursor(sourceID, d.Next)
}

func (e *Engine) Report() MaintenanceReport {
	cfg := e.Config()
	st := e.Status()
	dir := cfg.ResolvedBackupDir(e.appDir)
	rep := MaintenanceReport{
		GeneratedAt: e.now().UTC(), ThresholdMB: cfg.SizeThresholdMB, Files: st.Files,
		BackupDir: dir, BackupCapMB: cfg.BackupCapMB, BackupKeep: cfg.BackupKeep,
	}
	if bs, err := ListBackups(dir); err == nil {
		rep.BackupCount = len(bs)
		for _, b := range bs {
			rep.BackupBytes += b.Bytes
		}
	}
	over := 0
	for _, f := range st.Files {
		if f.OverThreshold {
			over++
		}
	}
	if over > 0 {
		rep.Notes = append(rep.Notes, fmt.Sprintf(
			"%d file(s) exceed %d MB. Large SavedVariables slow WoW's /reload and logout. "+
				"Turn on automatic cleanup in Settings to trim data wowcensus.io already has; it only runs while WoW is closed.",
			over, cfg.SizeThresholdMB))
	}
	rep.Notes = append(rep.Notes,
		"Upload receipts say \"queued\": stored and waiting for the ingest worker, not yet imported into the database.",
		"Backups are companion-made gzip copies, rotated by generation count and total size.")
	if cfg.AutoClean {
		rep.Notes = append(rep.Notes, fmt.Sprintf(
			"Automatic cleanup is on: saves above %d MB that wowcensus.io has imported are trimmed to the last %d days while WoW is closed, after a backup.",
			cfg.CleanAboveMB, cfg.CleanKeepDays))
	} else {
		rep.Notes = append(rep.Notes, "Automatic cleanup is off, so your WoW files are never modified.")
	}
	return rep
}

func (e *Engine) setRunning(b bool) {
	e.mu.Lock()
	e.status.Running = b
	e.mu.Unlock()
	e.notify()
}

func (e *Engine) setBusy(s string) {
	e.mu.Lock()
	e.status.Busy = s
	e.mu.Unlock()
	e.notify()
}

func (e *Engine) logf(level, format string, args ...any) {
	e.mu.Lock()
	e.logSeq++
	e.status.Log = append(e.status.Log, LogEntry{Seq: e.logSeq, At: e.now().UTC(), Level: level, Message: HideAccountFolders(fmt.Sprintf(format, args...))})
	if len(e.status.Log) > maxLog {
		e.status.Log = e.status.Log[len(e.status.Log)-maxLog:]
	}
	e.mu.Unlock()
	e.notify()
}

func (e *Engine) notify() {
	e.mu.Lock()
	fn := e.onChange
	e.mu.Unlock()
	if fn != nil {
		fn(e.Status())
	}
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

const receiptCheckEvery = 10 * time.Minute

const receiptChecksPerScan = 5

func (e *Engine) refreshReceipts(ctx context.Context, cfg Config, groups ...[]FileStatus) {
	client := e.newClient(cfg.ServerURL, cfg.DeviceToken)
	checks := e.state.ReceiptsToCheck(e.now(), receiptCheckEvery, receiptChecksPerScan)
	updated := map[string]string{}
	for _, c := range checks {
		status, err := client.ReceiptStatus(ctx, c.ReceiptID)
		if err != nil {
			continue
		}
		_ = e.state.SetReceiptStatus(c.ReceiptID, status, e.now().UTC())
		updated[c.ReceiptID] = status
	}
	if len(updated) == 0 {
		return
	}
	for _, files := range groups {
		for i := range files {
			r := files[i].LastReceipt
			if r == nil || updated[r.ReceiptID] == "" {
				continue
			}
			fresh := *r
			fresh.Status, fresh.CheckedAt = updated[r.ReceiptID], e.now().UTC()
			files[i].LastReceipt = &fresh
			if fresh.Status == "imported" && files[i].Addon == "" && e.state.PendingReceipts(files[i].SourceID) == 0 {
				e.logf("info", "%s / %s is now on wowcensus.io", files[i].Edition, files[i].AccountLabel())
			}
		}
	}
}

func (e *Engine) addonStatus(cfg Config) AddonStatus {
	installs := AddonInstalls(e.discoverEditions(cfg.WoWRoots))
	e.mu.Lock()
	defer e.mu.Unlock()
	return AddonStatus{Latest: e.addonLatest.Version, Installs: installs, Error: e.addonErr}
}

func (e *Engine) latestAddon(ctx context.Context, force bool) (AddonRelease, error) {
	e.mu.Lock()
	cached, at := e.addonLatest, e.addonLatestAt
	e.mu.Unlock()
	if !force && cached.ZipURL != "" && e.now().Sub(at) < addonLatestMaxAge {
		return cached, nil
	}
	rel, err := FetchLatestAddon(ctx, e.httpClient)
	e.mu.Lock()
	if err != nil {
		e.addonErr = "Couldn't check GitHub for the add-on right now."
	} else {
		e.addonLatest, e.addonLatestAt, e.addonErr = rel, e.now(), ""
	}
	e.mu.Unlock()
	return rel, err
}

func (e *Engine) CheckAddon(ctx context.Context) AddonStatus {
	_, _ = e.latestAddon(ctx, false)
	st := e.addonStatus(e.Config())
	e.mu.Lock()
	e.status.Addon = st
	e.mu.Unlock()
	e.notify()
	return st
}

func (e *Engine) InstallAddon(ctx context.Context) (string, error) {
	editions := e.discoverEditions(e.Config().WoWRoots)
	if len(editions) == 0 {
		return "", errors.New("no World of Warcraft installation found; add your WoW folder in Settings")
	}
	rel, err := e.latestAddon(ctx, true)
	if err != nil {
		return "", fmt.Errorf("couldn't find the add-on download: %w", err)
	}
	data, err := DownloadAddon(ctx, e.httpClient, rel)
	if err != nil {
		return "", err
	}
	var done, failed []string
	for _, ed := range editions {
		label := editionLabel(editionName(ed))
		if err := InstallAddonFiles(ed, data); err != nil {
			failed = append(failed, label)
			e.logf("error", "Add-on install for %s failed: %v", label, err)
			continue
		}
		done = append(done, label)
	}
	e.CheckAddon(ctx)
	if len(done) == 0 {
		return "", fmt.Errorf("the add-on could not be installed (%s). Close World of Warcraft and try again", strings.Join(failed, ", "))
	}
	msg := fmt.Sprintf("Installed WoWCensus %s for %s.", rel.Version, strings.Join(done, ", "))
	if len(failed) > 0 {
		msg += fmt.Sprintf(" Couldn't install for %s; close World of Warcraft and try again.", strings.Join(failed, ", "))
	}
	e.logf("info", "%s Restart World of Warcraft (or /reload) to load it.", msg)
	return msg, nil
}

func editionLabel(key string) string {
	switch key {
	case "retail":
		return "Retail"
	case "classic_era":
		return "Classic Era"
	case "anniversary":
		return "TBC Anniversary"
	case "classic":
		return "MoP Classic"
	case "forever":
		return "Forever"
	case "classic_beta":
		return "Classic Beta"
	case "ptr", "xptr":
		return "PTR"
	case "beta":
		return "Beta"
	}
	return strings.ReplaceAll(key, "_", " ")
}

func (e *Engine) OnUpdateFound(fn func(version string)) {
	e.mu.Lock()
	e.onUpdateFound = fn
	e.mu.Unlock()
}

func (e *Engine) maybeCheckUpdate(ctx context.Context) {
	checked, _ := e.state.UpdateMarks()
	if e.now().Sub(checked) < updateCheckEvery {
		return
	}
	e.CheckForUpdate(ctx)
}

func (e *Engine) CheckForUpdateIfOlder(ctx context.Context, age time.Duration) {
	e.mu.Lock()
	last := e.status.Update.CheckedAt
	e.mu.Unlock()
	if !last.IsZero() && e.now().Sub(last) < age {
		return
	}
	e.CheckForUpdate(ctx)
}

func (e *Engine) CheckForUpdate(ctx context.Context) UpdateStatus {
	info, err := FetchLatestCompanion(ctx, e.httpClient)
	if err != nil {
		e.mu.Lock()
		e.status.Update.Current = AppVersion
		e.status.Update.CheckedAt = e.now()
		e.status.Update.CheckError = err.Error()
		st := e.status.Update
		e.mu.Unlock()
		e.logf("error", "Update check failed: %v", err)
		e.notify()
		return st
	}
	_, seen := e.state.UpdateMarks()
	e.mu.Lock()
	previous := e.status.Update
	st := UpdateStatus{Current: AppVersion, Installing: previous.Installing, CheckedAt: e.now()}
	newVersion := ""
	prepare := false
	if NewerVersion(info.Version, AppVersion) {
		e.update = info
		st.Available, st.Version = true, info.Version
		if previous.Version == info.Version {
			st.Ready = previous.Ready
			st.Downloading = previous.Downloading
			st.Error = previous.Error
		} else {
			e.updateData = nil
		}
		prepare = !st.Ready && !st.Downloading && !st.Installing && st.Error == ""
		if prepare {
			st.Downloading = true
		}
		if info.Version != seen {
			newVersion = info.Version
		}
	}
	e.status.Update = st
	fn := e.onUpdateFound
	e.mu.Unlock()
	if newVersion != "" {
		seen = newVersion
		e.logf("info", "WoWCensus Companion %s is available.", newVersion)
		if fn != nil {
			fn(newVersion)
		}
	}
	_ = e.state.SetUpdateMarks(e.now(), seen)
	e.notify()
	if prepare {
		go e.downloadUpdate(ctx, info)
	}
	return st
}

func (e *Engine) PrepareUpdate(ctx context.Context) error {
	e.mu.Lock()
	info := e.update
	if !e.status.Update.Available || info.URL == "" {
		e.mu.Unlock()
		return errors.New("no update is available")
	}
	if e.status.Update.Ready || e.status.Update.Downloading || e.status.Update.Installing {
		e.mu.Unlock()
		return nil
	}
	e.status.Update.Downloading, e.status.Update.Error = true, ""
	e.mu.Unlock()
	e.notify()
	return e.downloadUpdate(ctx, info)
}

func (e *Engine) downloadUpdate(ctx context.Context, info UpdateInfo) error {
	e.logf("info", "Downloading WoWCensus Companion %s…", info.Version)
	data, err := DownloadUpdate(ctx, e.httpClient, info)
	e.mu.Lock()
	if e.update.Version == info.Version {
		e.status.Update.Downloading = false
		if err == nil {
			e.updateData = data
			e.status.Update.Ready = true
		} else {
			e.status.Update.Error = err.Error()
		}
	}
	e.mu.Unlock()
	if err != nil {
		e.logf("error", "Update download failed: %v", err)
	} else {
		e.logf("info", "WoWCensus Companion %s is ready to install.", info.Version)
	}
	e.notify()
	return err
}

func (e *Engine) InstallUpdate(ctx context.Context) (Relaunch, error) {
	e.mu.Lock()
	info := e.update
	data := e.updateData
	if e.status.Update.Installing {
		e.mu.Unlock()
		return Relaunch{}, errors.New("an update is already being installed")
	}
	e.status.Update.Installing, e.status.Update.Error = true, ""
	e.mu.Unlock()
	e.notify()
	fail := func(err error) (Relaunch, error) {
		e.mu.Lock()
		e.status.Update.Installing, e.status.Update.Error = false, err.Error()
		e.mu.Unlock()
		e.logf("error", "Update failed: %v", err)
		return Relaunch{}, err
	}
	if !NewerVersion(info.Version, AppVersion) || len(data) == 0 {
		return fail(errors.New("the update is not ready yet"))
	}
	rl, err := ApplyUpdate(data)
	if err != nil {
		return fail(err)
	}
	e.logf("info", "Updated to %s. Restarting…", info.Version)
	return rl, nil
}
