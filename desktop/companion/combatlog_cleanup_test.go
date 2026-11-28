package companion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPruneCombatLogsAfterImport(t *testing.T) {
	clock := time.Now()
	srv := &chainServer{statuses: map[string]string{}}
	hs := httptest.NewServer(srv.handler())
	t.Cleanup(hs.Close)
	root := filepath.Join(t.TempDir(), "World of Warcraft")
	ed := filepath.Join(root, "_forever_")
	for _, d := range []string{filepath.Join(ed, "WTF", "Account", "ACC"), filepath.Join(ed, "Logs")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldLog := filepath.Join(ed, "Logs", "WoWCombatLog-110126_103937.txt")
	newestLog := filepath.Join(ed, "Logs", "WoWCombatLog-110626_103937.txt")
	for p, age := range map[string]time.Duration{oldLog: 20 * 24 * time.Hour, newestLog: 15 * 24 * time.Hour} {
		if err := os.WriteFile(p, []byte(sampleCombatLog), 0o644); err != nil {
			t.Fatal(err)
		}
		at := clock.Add(-age)
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
	e, err := NewEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	wowOpen := false
	e.now = func() time.Time { return clock }
	e.wowRunning = func() (bool, error) { return wowOpen, nil }
	e.discover = func([]string) []SavedVarFile { return nil }
	e.discoverEditions = func([]string) []string { return []string{ed} }
	cfg := e.Config()
	cfg.ServerURL, cfg.AutoUpload, cfg.CleanCombatLogs = hs.URL, true, true
	cfg.ShareAddons = map[string]bool{combatLogKey: true}
	if err := e.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	exists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}

	e.RunOnce(context.Background(), false, false)
	if n := len(srv.uploads()); n != 2 {
		t.Fatalf("uploads = %d, want 2", n)
	}
	clock = clock.Add(time.Hour)
	e.RunOnce(context.Background(), false, false)
	if !exists(oldLog) {
		t.Fatal("a queued log was deleted before wowcensus.io imported it")
	}

	srv.set("1", "imported")
	srv.set("2", "imported")
	wowOpen = true
	clock = clock.Add(time.Hour)
	e.RunOnce(context.Background(), false, false)
	if !exists(oldLog) {
		t.Fatal("a log was deleted while WoW was running")
	}

	wowOpen = false
	clock = clock.Add(time.Hour)
	e.RunOnce(context.Background(), false, false)
	if exists(oldLog) {
		t.Fatal("the imported old log was kept")
	}
	if !exists(newestLog) {
		t.Fatal("the newest log in the folder was deleted")
	}
	if got := e.Status().AddonFiles; len(got) != 1 || got[0].Path != newestLog {
		t.Fatalf("addon files = %+v", got)
	}
}

func TestPruneCombatLogsOffByDefault(t *testing.T) {
	if DefaultConfig().CleanCombatLogs {
		t.Fatal("combat log deletion must be opt-in")
	}
}

func TestPruneCombatLogsRequiresCompleteExport(t *testing.T) {
	cast := combatRedactionRecord("SPELL_CAST_START", redactionSource, redactionNPC, redactionSpell)
	raw := redactionHeader + redactionMap + cast
	want := redactionHeader + redactionMap + combatRedactionRecord("SPELL_CAST_START", redactedSource, redactionNPC, redactionSpell)
	ignored := "\n  \r\n" + redactionStamp + `COMBATANT_INFO,Player-9001-0AA10002,0,1,2,3` + "\n" +
		redactionStamp + `EMOTE,"Synthetic emote"` + "\n" +
		combatRedactionRecord("SPELL_CAST_START", redactionSource, redactionTarget, redactionSpell)
	badLayout := combatRedactionRecord("SPELL_CAST_START", redactionSource, redactionNPC, redactionSpell+",9")
	badNoMobLayout := combatRedactionRecord("SPELL_CAST_START", redactionSource, redactionTarget, redactionSpell+",9")
	unknownEvent := combatRedactionRecord("SPELL_UNREVIEWED", redactionSource, redactionNPC, redactionSpell)
	badQuote := combatRedactionRecord("SPELL_CAST_START", redactionSource, redactionNPC, `585,"Unclosed spell,0x2`)
	ambiguousStats := strings.Replace(redactionNPCStats, ",-9071.25,100.75,1429,1.25,7", ",1,1,1,1,1", 1)
	ambiguous := combatRedactionRecord("SPELL_DAMAGE", redactionSource, redactionNPC,
		redactionSpell+","+ambiguousStats+",40,0,2,0,0,0,nil,nil,nil")
	for _, tc := range []struct {
		name, raw, status string
		deleted           bool
	}{
		{"complete imported", raw, "imported", true},
		{"complete queued", raw, "queued", false},
		{"complete failed", raw, "failed", false},
		{"intentionally ignored records", raw + ignored, "imported", true},
		{"malformed line", raw + "truncated log record\n", "imported", false},
		{"malformed fields", raw + badQuote, "imported", false},
		{"unsupported layout", raw + badLayout, "imported", false},
		{"malformed non-mob record", raw + badNoMobLayout, "imported", false},
		{"unknown event", raw + unknownEvent, "imported", false},
		{"ambiguous advanced layout", raw + ambiguous, "imported", false},
		{"event before first header", cast + raw, "imported", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCombatLogCleanupHarness(t, tc.raw)
			h.scan()
			h.checkUpload(want)
			h.checkSource(tc.raw)
			h.checkReceipt("queued")

			h.restart()
			h.poll(tc.status)
			h.checkReceipt(tc.status)
			h.checkUpload(want)
			if tc.deleted {
				h.checkDeleted()
				return
			}
			h.checkSource(tc.raw)
		})
	}
}

func TestPruneCombatLogsChecksSourceWhenReusingReceipt(t *testing.T) {
	cast := combatRedactionRecord("SPELL_CAST_START", redactionSource, redactionNPC, redactionSpell)
	raw := redactionHeader + cast
	want := redactionHeader + combatRedactionRecord("SPELL_CAST_START", redactedSource, redactionNPC, redactionSpell)
	for _, tc := range []struct {
		name, updated string
		deleted       bool
	}{
		{"complete revision with same export", raw + "\n", true},
		{"incomplete revision with same export", raw + "truncated log record\n", false},
		{"same size and mtime with different bytes", strings.Replace(raw, "SPELL_CAST_START", "SPELL_UNREVIEWED", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCombatLogCleanupHarness(t, raw)
			h.scan()
			h.checkReceipt("queued")
			sig, err := statSig(h.path)
			if err != nil {
				t.Fatal(err)
			}
			h.writeLog(h.path, tc.updated, sig.ModTime)
			h.restart()
			h.poll("imported")
			h.checkReceipt("imported")
			h.checkUpload(want)
			if tc.deleted {
				h.checkDeleted()
				return
			}
			h.checkSource(tc.updated)
		})
	}
}

func TestPruneCombatLogsRetainsLegacyAndMismatchedReceipts(t *testing.T) {
	cast := combatRedactionRecord("SPELL_CAST_START", redactionSource, redactionNPC, redactionSpell)
	raw := redactionHeader + cast
	for _, tc := range []struct {
		name           string
		clearSourceSHA bool
		changeReceipt  func(*Receipt)
	}{
		{"legacy state", true, func(r *Receipt) { r.Destination = "" }},
		{"missing source evidence", true, nil},
		{"missing receipt destination", false, func(r *Receipt) { r.Destination = "" }},
		{"different upload payload", false, func(r *Receipt) { r.UploadSHA = hashText("different upload") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCombatLogCleanupHarness(t, raw)
			h.scan()
			r := h.checkReceipt("queued")
			if tc.clearSourceSHA {
				seen, ok := h.e.state.SeenSig(h.path)
				if !ok {
					t.Fatal("missing snapshot state")
				}
				if err := h.e.state.SetSeen(h.path, seen.Sig, seen.SHA256); err != nil {
					t.Fatal(err)
				}
			}
			if tc.changeReceipt != nil {
				tc.changeReceipt(&r)
				if err := h.e.state.PutReceipt(r); err != nil {
					t.Fatal(err)
				}
			}
			h.restart()
			h.poll("imported")
			h.checkReceipt("imported")
			h.checkSource(raw)
		})
	}
}

func TestPruneCombatLogsRequiresCurrentDestination(t *testing.T) {
	raw := redactionHeader + combatRedactionRecord("SPELL_CAST_START", redactionSource, redactionNPC, redactionSpell)
	h := newCombatLogCleanupHarness(t, raw)
	h.scan()
	h.checkReceipt("queued")
	original := h.e.Config()
	var checks atomic.Int32
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v1/uploads/1" {
			checks.Add(1)
			_ = json.NewEncoder(w).Encode(uploadResponse{Status: "imported"})
			return
		}
		t.Errorf("unexpected request to the other server: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	t.Cleanup(hs.Close)
	other := original
	other.ServerURL = hs.URL
	otherDevice := original
	otherDevice.DeviceID = "dev_other"
	for _, tc := range []struct {
		cfg     Config
		status  string
		wowOpen bool
	}{
		{other, "queued", false},
		{otherDevice, "queued", false},
		{original, "imported", true},
		{other, "imported", false},
		{otherDevice, "imported", false},
	} {
		if err := h.e.UpdateConfig(tc.cfg); err != nil {
			t.Fatal(err)
		}
		if err := h.e.setDevice(tc.cfg.ServerURL, tc.cfg.DeviceID, tc.cfg.DeviceToken); err != nil {
			t.Fatal(err)
		}
		h.e.wowRunning = func() (bool, error) { return tc.wowOpen, nil }
		h.poll("imported")
		h.checkReceipt(tc.status)
		h.checkSource(raw)
	}
	if got := checks.Load(); got != 0 {
		t.Fatalf("polled a receipt from the other server %d time(s)", got)
	}
}

type combatLogCleanupHarness struct {
	t      *testing.T
	appDir string
	ed     string
	path   string
	clock  time.Time
	e      *Engine
	server *fakeServer
	status atomic.Value
}

func newCombatLogCleanupHarness(t *testing.T, raw string) *combatLogCleanupHarness {
	t.Helper()
	h := &combatLogCleanupHarness{
		t:      t,
		appDir: t.TempDir(),
		ed:     filepath.Join(t.TempDir(), "_forever_"),
		clock:  time.Date(2026, 11, 22, 20, 0, 0, 0, time.UTC),
		server: &fakeServer{token: "test-token"},
	}
	logDir := filepath.Join(h.ed, "Logs")
	for _, dir := range []string{filepath.Join(h.ed, "WTF", "Account", "ACC"), logDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	h.path = filepath.Join(logDir, "WoWCombatLog-old.txt")
	h.writeLog(h.path, raw, h.clock.Add(-20*24*time.Hour))
	h.writeLog(filepath.Join(logDir, "WoWCombatLog-new.txt"), redactionHeader, h.clock.Add(-time.Hour))
	h.status.Store("queued")
	mux := http.NewServeMux()
	mux.Handle("/", h.server.handler(t))
	mux.HandleFunc("/v1/uploads/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+h.server.currentToken() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(uploadResponse{Status: h.status.Load().(string)})
	})
	hs := httptest.NewServer(mux)
	t.Cleanup(hs.Close)
	h.restart()
	cfg := h.e.Config()
	cfg.ServerURL, cfg.AutoUpload, cfg.CleanCombatLogs = hs.URL, true, true
	cfg.ShareAddons = map[string]bool{combatLogKey: true}
	if err := h.e.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h *combatLogCleanupHarness) writeLog(path, raw string, at time.Time) {
	h.t.Helper()
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		h.t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		h.t.Fatal(err)
	}
}

func (h *combatLogCleanupHarness) restart() {
	h.t.Helper()
	e, err := NewEngine(h.appDir)
	if err != nil {
		h.t.Fatal(err)
	}
	e.now = func() time.Time { return h.clock }
	e.wowRunning = func() (bool, error) { return false, nil }
	e.discover = func([]string) []SavedVarFile { return nil }
	e.discoverEditions = func([]string) []string { return []string{h.ed} }
	h.e = e
}

func (h *combatLogCleanupHarness) scan() {
	h.e.RunOnce(context.Background(), false, false)
}

func (h *combatLogCleanupHarness) poll(status string) {
	h.status.Store(status)
	h.clock = h.clock.Add(receiptCheckEvery)
	h.scan()
}

func (h *combatLogCleanupHarness) checkReceipt(status string) Receipt {
	h.t.Helper()
	r, ok := h.e.state.LatestReceipt(h.e.state.SourceID(h.path))
	if !ok || r.ReceiptID == "" || r.Status != status {
		h.t.Fatalf("receipt = %+v, want %s", r, status)
	}
	return r
}

func (h *combatLogCleanupHarness) checkUpload(want string) {
	h.t.Helper()
	_, _, uploads := h.server.uploads()
	if len(uploads) != 1 {
		h.t.Fatalf("uploads = %d, want one", len(uploads))
	}
	m, data := bundleLua(h.t, uploads[0])
	if m.Addon != combatLogKey || m.FileName != filepath.Base(h.path) || data != want {
		h.t.Fatalf("unexpected upload for %s/%s:\n%s\nwant:\n%s", m.Addon, m.FileName, data, want)
	}
}

func (h *combatLogCleanupHarness) checkSource(want string) {
	h.t.Helper()
	data, err := os.ReadFile(h.path)
	if err != nil || string(data) != want {
		h.t.Fatalf("original log was changed or removed: %v", err)
	}
}

func (h *combatLogCleanupHarness) checkDeleted() {
	h.t.Helper()
	if _, err := os.Stat(h.path); !os.IsNotExist(err) {
		h.t.Fatalf("expected the original log to be removed, stat returned %v", err)
	}
}
