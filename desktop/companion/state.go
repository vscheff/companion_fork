package companion

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Receipt struct {
	SourceID    string    `json:"sourceId"`
	SHA256      string    `json:"sha256"`
	Status      string    `json:"status"`
	ReceiptID   string    `json:"receiptId,omitempty"`
	Duplicate   bool      `json:"duplicate,omitempty"`
	CheckedAt   time.Time `json:"checkedAt,omitempty"`
	Bytes       int64     `json:"bytes"`
	At          time.Time `json:"at"`
	UploadSHA   string    `json:"uploadSha,omitempty"`
	Mode        string    `json:"mode,omitempty"`
	Destination string    `json:"destination,omitempty"`
}

func receiptPending(status string) bool { return status == "queued" || status == "stored" }

type censusCheckpoint struct {
	FileSHA256  string          `json:"fileSha256"`
	Consent     marketConsent   `json:"consent"`
	Destination string          `json:"destination"`
	UploadSHA   string          `json:"uploadSha,omitempty"`
	Coverage    *censusCoverage `json:"coverage,omitempty"`
}

type censusCoverage struct {
	ImportedBaseline bool                 `json:"importedBaseline,omitempty"`
	Required         []censusImport       `json:"required"`
	Records          censusRecordCoverage `json:"records"`
}

type censusImport struct {
	UploadSHA string `json:"uploadSha"`
	ReceiptID string `json:"receiptId"`
	Mode      string `json:"mode"`
}

func censusDestination(serverURL, deviceID string) string {
	if serverURL == "" || deviceID == "" {
		return ""
	}
	return hashText(serverURL + "\x00" + deviceID)
}

func censusCheckpointFor(cfg Config, fileSHA string) censusCheckpoint {
	return censusCheckpoint{
		FileSHA256:  fileSHA,
		Consent:     censusMarketConsent(cfg.ShareAddons),
		Destination: censusDestination(cfg.ServerURL, cfg.DeviceID),
	}
}

type Failure struct {
	Attempts int       `json:"attempts"`
	LastErr  string    `json:"lastError"`
	NextTry  time.Time `json:"nextTry"`
}

type State struct {
	mu      sync.Mutex
	path    string
	saveErr error

	InstallSalt string              `json:"installSalt"`
	UpdateCheck time.Time           `json:"updateCheck"`
	UpdateSeen  string              `json:"updateSeen"`
	Receipts    map[string]Receipt  `json:"receipts"`
	Seen        map[string]seenFile `json:"seen"`
	Failures    map[string]Failure  `json:"failures"`
	Cleanups    map[string]Cleanup  `json:"cleanups,omitempty"`
	Cursors     map[string]Cursor   `json:"cursors,omitempty"`
}

type Cleanup struct {
	CleanupResult
	At     time.Time `json:"at"`
	Backup string    `json:"backup"`
}

type seenFile struct {
	Sig    FileSig `json:"sig"`
	SHA256 string  `json:"sha256"`
}

const maxReceipts = 2000

func LoadState(appDir string) (*State, error) {
	s := &State{path: filepath.Join(appDir, "state.json")}
	b, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(b, s); err != nil {

			_ = os.Rename(s.path, s.path+".corrupt-"+time.Now().Format("20060102-150405"))
			s = &State{path: s.path}
		}
	}
	if s.InstallSalt == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		s.InstallSalt = hex.EncodeToString(buf)
	}
	if s.Receipts == nil {
		s.Receipts = map[string]Receipt{}
	}
	if s.Seen == nil {
		s.Seen = map[string]seenFile{}
	}
	if s.Failures == nil {
		s.Failures = map[string]Failure{}
	}
	if s.Cleanups == nil {
		s.Cleanups = map[string]Cleanup{}
	}
	if s.Cursors == nil {
		s.Cursors = map[string]Cursor{}
	}
	return s, s.Save()
}

func (s *State) SourceID(path string) string {
	sum := sha256.Sum256([]byte(s.InstallSalt + "\x00" + pathKey(path)))
	return hex.EncodeToString(sum[:])
}

func (s *State) ClientID() string {
	sum := sha256.Sum256([]byte("client\x00" + s.InstallSalt))
	return hex.EncodeToString(sum[:16])
}

func (s *State) UpdateMarks() (time.Time, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.UpdateCheck, s.UpdateSeen
}

func (s *State) SetUpdateMarks(checked time.Time, seen string) error {
	s.mu.Lock()
	s.UpdateCheck, s.UpdateSeen = checked, seen
	s.mu.Unlock()
	return s.Save()
}

func receiptKey(sourceID, sha string) string { return sourceID + ":" + sha }

func (s *State) Receipt(sourceID, sha string) (Receipt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.Receipts[receiptKey(sourceID, sha)]
	return r, ok
}

func censusReceiptKey(sourceID, destination, uploadSHA string) string {
	return "census:" + destination + ":" + receiptKey(sourceID, uploadSHA)
}

func (s *State) CensusReceipt(sourceID, destination, uploadSHA string) (Receipt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.censusReceiptLocked(sourceID, destination, uploadSHA)
}

func (s *State) censusReceiptLocked(sourceID, destination, uploadSHA string) (Receipt, bool) {
	r, ok := s.Receipts[censusReceiptKey(sourceID, destination, uploadSHA)]
	return r, ok && r.SourceID == sourceID && r.Destination == destination && r.UploadSHA == uploadSHA && r.ReceiptID != ""
}

func (s *State) CensusResult(sourceID string, check censusCheckpoint) (Receipt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.Cursors[sourceID].Census
	if p == nil || p.FileSHA256 != check.FileSHA256 || p.Consent != check.Consent || p.Destination != check.Destination {
		return Receipt{}, false
	}
	if p.UploadSHA == "" {
		return Receipt{SourceID: sourceID, SHA256: p.FileSHA256, Status: "empty"}, true
	}
	r, ok := s.censusReceiptLocked(sourceID, p.Destination, p.UploadSHA)
	r.SHA256 = p.FileSHA256
	return r, ok
}

func (s *State) PutCensusResult(sourceID string, check censusCheckpoint, next Cursor, mode string, r Receipt, records censusRecordCoverage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ReceiptID != "" {
		check.Destination, check.UploadSHA = r.Destination, r.UploadSHA
	}
	check.Coverage = s.nextCensusCoverageLocked(sourceID, check.Destination, next.Census, mode, r, records)
	if r.ReceiptID != "" {
		r.SHA256 = check.FileSHA256
		s.Receipts[censusReceiptKey(sourceID, r.Destination, r.UploadSHA)] = r
		s.pruneReceiptsLocked()
	}
	next.Census = &check
	s.Cursors[sourceID] = next.clone()
	delete(s.Failures, receiptKey(sourceID, check.FileSHA256))
	return s.saveLocked()
}

func (s *State) nextCensusCoverageLocked(sourceID, destination string, prev *censusCheckpoint, mode string, r Receipt, records censusRecordCoverage) *censusCoverage {
	if destination == "" || r.ReceiptID != "" && (r.Mode != mode || r.SourceID != sourceID || r.UploadSHA == "") {
		return nil
	}
	ref := censusImport{UploadSHA: r.UploadSHA, ReceiptID: r.ReceiptID, Mode: r.Mode}
	if mode == ModeFull && r.ReceiptID != "" {
		return &censusCoverage{Required: []censusImport{ref}, Records: records}
	}
	if mode != ModeSession || !prev.hasCoverage(destination) {
		return nil
	}
	coverage := *prev.Coverage
	coverage.Records = records
	if r.ReceiptID == "" {
		return &coverage
	}
	required := coverage.Required
	for len(required) > 0 {
		prior, ok := s.censusImportLocked(sourceID, destination, required[0])
		if !ok || prior.Status != "imported" {
			break
		}
		coverage.ImportedBaseline = true
		required = required[1:]
	}
	if len(required) >= maxReceipts {
		return nil
	}
	coverage.Required = append(append([]censusImport(nil), required...), ref)
	return &coverage
}

func (p *censusCheckpoint) hasCoverage(destination string) bool {
	if destination == "" || p == nil || p.FileSHA256 == "" || p.Destination != destination || !p.Coverage.valid() {
		return false
	}
	required := p.Coverage.Required
	return p.UploadSHA == "" || p.UploadSHA == required[len(required)-1].UploadSHA
}

func (c *censusCoverage) valid() bool {
	if c == nil || len(c.Required) == 0 || len(c.Required) > maxReceipts {
		return false
	}
	for i, ref := range c.Required {
		mode := ModeSession
		if i == 0 && !c.ImportedBaseline {
			mode = ModeFull
		}
		if ref.Mode != mode || ref.ReceiptID == "" || ref.UploadSHA == "" {
			return false
		}
	}
	return true
}

func (s *State) censusImportLocked(sourceID, destination string, ref censusImport) (Receipt, bool) {
	r, ok := s.censusReceiptLocked(sourceID, destination, ref.UploadSHA)
	return r, ok && r.ReceiptID == ref.ReceiptID && r.Mode == ref.Mode
}

func (s *State) CensusCleanupReceipt(sourceID, fileSHA, destination string) (Receipt, censusRecordCoverage, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.Cursors[sourceID].Census
	if s.saveErr != nil || !p.hasCoverage(destination) {
		return Receipt{}, censusRecordCoverage{}, CleanupWaitCoverage
	}
	wait := ""
	var latest Receipt
	for _, ref := range p.Coverage.Required {
		r, ok := s.censusImportLocked(sourceID, destination, ref)
		if !ok {
			return Receipt{}, censusRecordCoverage{}, CleanupWaitCoverage
		}
		switch {
		case r.Status == "imported":
		case receiptPending(r.Status):
			wait = CleanupWaitImport
		default:
			return Receipt{}, censusRecordCoverage{}, CleanupWaitCoverage
		}
		latest = r
	}
	if p.FileSHA256 != fileSHA {
		return Receipt{}, p.Coverage.Records, CleanupWaitImport
	}
	latest.SHA256 = fileSHA
	return latest, p.Coverage.Records, wait
}

func (s *State) LatestReceipt(sourceID string) (Receipt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var best Receipt
	ok := false
	for _, r := range s.Receipts {
		if r.SourceID == sourceID && (!ok || r.At.After(best.At)) {
			best, ok = r, true
		}
	}
	return best, ok
}

func (s *State) PutReceipt(r Receipt) error {
	s.mu.Lock()
	s.Receipts[receiptKey(r.SourceID, r.SHA256)] = r
	delete(s.Failures, receiptKey(r.SourceID, r.SHA256))
	if len(s.Receipts) > maxReceipts {
		s.pruneReceiptsLocked()
	}
	s.mu.Unlock()
	return s.Save()
}

func (s *State) pruneReceiptsLocked() {
	var oldestK string
	var oldest time.Time
	for len(s.Receipts) > maxReceipts {
		oldestK = ""
		for k, r := range s.Receipts {
			if oldestK == "" || r.At.Before(oldest) {
				oldestK, oldest = k, r.At
			}
		}
		delete(s.Receipts, oldestK)
	}
}

func (s *State) Backoff(sourceID, sha string, now time.Time) (Failure, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.Failures[receiptKey(sourceID, sha)]
	return f, ok && now.Before(f.NextTry)
}

func (s *State) RecordFailure(sourceID, sha string, err error, now time.Time) error {
	s.mu.Lock()
	k := receiptKey(sourceID, sha)
	f := s.Failures[k]
	f.Attempts++
	f.LastErr = err.Error()
	delay := min(30*time.Second<<min(f.Attempts-1, 7), time.Hour)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.RetryAfter > delay {
		delay = min(apiErr.RetryAfter, 24*time.Hour)
	}
	f.NextTry = now.Add(delay)
	s.Failures[k] = f
	s.mu.Unlock()
	return s.Save()
}

func (s *State) ClearFailures() error {
	s.mu.Lock()
	s.Failures = map[string]Failure{}
	s.mu.Unlock()
	return s.Save()
}

func (s *State) SeenSig(path string) (seenFile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.Seen[pathKey(path)]
	return v, ok
}

func (s *State) SetSeen(path string, sig FileSig, sha string) error {
	s.mu.Lock()
	s.Seen[pathKey(path)] = seenFile{Sig: sig, SHA256: sha}
	s.mu.Unlock()
	return s.Save()
}

func (s *State) LastCleanup(sourceID string) (Cleanup, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.Cleanups[sourceID]
	return c, ok
}

func (s *State) PutCensusCleanup(sourceID, destination string, snap *Snapshot, afterSHA string, seen FileSig, c Cleanup) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Cleanups[sourceID] = c
	if seen != (FileSig{}) {
		s.Seen[pathKey(snap.File.Path)] = seenFile{Sig: seen, SHA256: afterSHA}
	}
	cur := s.Cursors[sourceID]
	if p := cur.Census; p.hasCoverage(destination) && p.FileSHA256 == snap.SHA256 {
		check := *p
		check.FileSHA256 = afterSHA
		cur.Census = &check
		s.Cursors[sourceID] = cur
		delete(s.Failures, receiptKey(sourceID, afterSHA))
	}
	return s.saveLocked()
}

func (s *State) Cursor(sourceID string) (Cursor, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.Cursors[sourceID]
	return c.clone(), ok
}

func (s *State) SetCursor(sourceID string, c Cursor) error {
	s.mu.Lock()
	s.Cursors[sourceID] = c.clone()
	s.mu.Unlock()
	return s.Save()
}

func (s *State) PendingReceipts(sourceID, destination string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := map[string]bool{}
	for _, r := range s.Receipts {
		if r.SourceID == sourceID && r.Destination == destination && r.ReceiptID != "" && receiptPending(r.Status) {
			ids[r.ReceiptID] = true
		}
	}
	return len(ids)
}

type receiptCheck struct {
	ReceiptID string
	CheckedAt time.Time
	Coverage  bool
}

func (s *State) ReceiptsToCheck(destination string, checkAddons bool, now time.Time, every time.Duration, limit int) []receiptCheck {
	s.mu.Lock()
	defer s.mu.Unlock()
	required := map[string]bool{}
	for sourceID, cur := range s.Cursors {
		p := cur.Census
		if !p.hasCoverage(destination) {
			continue
		}
		for _, ref := range p.Coverage.Required {
			required[censusReceiptKey(sourceID, destination, ref.UploadSHA)] = true
		}
	}
	byID := map[string]receiptCheck{}
	for key, r := range s.Receipts {
		if (r.Destination != "" && r.Destination != destination) || (r.Destination == "" && !checkAddons) {
			continue
		}
		if r.ReceiptID == "" || r.Status == "imported" ||
			(!receiptPending(r.Status) && !required[key]) || now.Sub(r.CheckedAt) < every {
			continue
		}
		if c, ok := byID[r.ReceiptID]; !ok || r.CheckedAt.Before(c.CheckedAt) {
			byID[r.ReceiptID] = receiptCheck{ReceiptID: r.ReceiptID, CheckedAt: r.CheckedAt, Coverage: required[key]}
		}
	}
	out := make([]receiptCheck, 0, len(byID))
	for _, c := range byID {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CheckedAt.Equal(out[j].CheckedAt) {
			return out[i].CheckedAt.Before(out[j].CheckedAt)
		}
		return out[i].ReceiptID < out[j].ReceiptID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *State) SetReceiptStatus(destination, receiptID, status string, checked time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, r := range s.Receipts {
		if r.ReceiptID == receiptID && (r.Destination == "" || r.Destination == destination) {
			if status != "" {
				r.Status = status
			}
			r.CheckedAt = checked
			s.Receipts[k] = r
		}
	}
	return s.saveLocked()
}

func (s *State) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *State) saveLocked() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err == nil {
		err = writeFileAtomic(s.path, b, 0o600)
	}
	s.saveErr = err
	return err
}
