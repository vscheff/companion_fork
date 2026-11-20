package companion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEngineRetainsCensusAfterFailedSession(t *testing.T) {
	for _, recovery := range []string{"receipt reimport", "full upload after pruning"} {
		t.Run(recovery, func(t *testing.T) {
			testCleanupAfterFailedSession(t, recovery)
		})
	}
}

func testCleanupAfterFailedSession(t *testing.T, recovery string) {
	t.Helper()
	clock := time.Unix(1794700000, 0)
	srv, serverURL, receipts := censusImportServer(t)
	e, path, content := newCleanupEngine(t, serverURL, &clock)
	ctx := context.Background()
	e.wowRunning = func() (bool, error) { return true, nil }

	e.RunOnce(ctx, false, false)
	baseline, lua := verifyCensusBundle(t, srv, 1, content, ModeFull, 0)
	if !strings.Contains(lua, "us:forever:realm:gone") {
		t.Fatal("the baseline is missing Character A")
	}
	receipts.Store("1", "imported")
	clock = clock.Add(receiptCheckEvery)
	e.RunOnce(ctx, false, false)
	assertCensusImportStatus(t, e, baseline, "imported")

	clock = clock.Add(time.Minute)
	failed := withCleanupCharacter(content, "failed-character", clock.Unix())
	writeCleanupFixture(t, path, failed, clock)
	e.RunOnce(ctx, false, false)
	delta, lua := verifyCensusBundle(t, srv, 2, failed, ModeSession, 1)
	if !strings.Contains(lua, "failed-character") {
		t.Fatal("the failed delta is missing Character B")
	}
	receipts.Store("2", "failed")
	clock = clock.Add(receiptCheckEvery)
	e.RunOnce(ctx, false, false)
	assertCensusImportStatus(t, e, delta, "failed")
	e.wowRunning = func() (bool, error) { return false, nil }
	e.RunOnce(ctx, false, false)
	assertCleanupBlocked(t, e, path, failed, CleanupWaitCoverage)
	e.wowRunning = func() (bool, error) { return true, nil }

	clock = clock.Add(time.Minute)
	latest := withCleanupCharacter(failed, "imported-character", clock.Unix())
	writeCleanupFixture(t, path, latest, clock)
	e.RunOnce(ctx, false, false)
	success, lua := verifyCensusBundle(t, srv, 3, latest, ModeSession, 1)
	if strings.Contains(lua, "failed-character") || !strings.Contains(lua, "imported-character") {
		t.Fatalf("the successful delta should contain Character C and omit Character B:\n%s", lua)
	}
	receipts.Store("3", "imported")
	clock = clock.Add(30 * 24 * time.Hour)
	e.RunOnce(ctx, false, false)
	assertCensusImportStatus(t, e, success, "imported")

	e = reloadCleanupEngine(t, e)
	e.wowRunning = func() (bool, error) { return false, nil }
	e.RunOnce(ctx, false, false)
	assertCleanupBlocked(t, e, path, latest, CleanupWaitCoverage)

	wantUploads := int32(3)
	if recovery == "receipt reimport" {
		receipts.Store("2", "imported")
	} else {
		pruneCensusReceipt(t, e, delta)
		e = reloadCleanupEngine(t, e)
		e.RunOnce(ctx, false, false)
		assertCleanupBlocked(t, e, path, latest, CleanupWaitCoverage)

		e.RunOnce(ctx, true, false)
		_, lua = verifyCensusBundle(t, srv, 4, latest, ModeFull, 0)
		if !strings.Contains(lua, "failed-character") || !strings.Contains(lua, "imported-character") ||
			strings.Contains(lua, `["market"]`) {
			t.Fatal("recovery did not send the complete census with market sharing disabled")
		}
		assertCleanupBlocked(t, e, path, latest, CleanupWaitImport)
		receipts.Store("4", "imported")
		wantUploads = 4
	}
	clock = clock.Add(receiptCheckEvery)
	e.RunOnce(ctx, false, false)
	e.RunOnce(ctx, false, false)
	cleaned := readCleanupFixture(t, path)
	f := e.Status().Files[0]
	if f.LastCleanup == nil || f.LastReceipt == nil || f.LastReceipt.Status != "imported" ||
		strings.Contains(cleaned, "failed-character") || strings.Contains(cleaned, "imported-character") {
		t.Fatalf("cleanup did not remove confirmed characters after recovery: %+v", f)
	}
	e = reloadCleanupEngine(t, e)
	e.RunOnce(ctx, false, false)
	if srv.puts.Load() != wantUploads || !e.Status().Files[0].Uploaded || readCleanupFixture(t, path) != cleaned {
		t.Fatal("cleanup did not carry forward the census checkpoint")
	}
	clock = clock.Add(time.Minute)
	changed := withCleanupCharacter(cleaned, "next-session", clock.Unix())
	writeCleanupFixture(t, path, changed, clock)
	e.RunOnce(ctx, false, false)
	_, lua = verifyCensusBundle(t, srv, int(wantUploads)+1, changed, ModeSession, 1)
	if !strings.Contains(lua, "next-session") || strings.Contains(lua, "failed-character") {
		t.Fatal("cleanup broke the incremental cursor")
	}
}

func pruneCensusReceipt(t *testing.T, e *Engine, m Manifest) {
	t.Helper()
	cfg := e.Config()
	key := censusReceiptKey(m.SourceID, censusDestination(cfg.ServerURL, cfg.DeviceID), m.SHA256)
	e.state.mu.Lock()
	r, ok := e.state.Receipts[key]
	if !ok {
		e.state.mu.Unlock()
		t.Fatal("no historical receipt to prune")
	}
	r.At = time.Unix(1, 0)
	e.state.Receipts[key] = r
	for i := len(e.state.Receipts); i <= maxReceipts; i++ {
		e.state.Receipts[fmt.Sprintf("unrelated-%d", i)] = Receipt{At: e.now(), Status: "imported"}
	}
	e.state.pruneReceiptsLocked()
	_, kept := e.state.Receipts[key]
	e.state.mu.Unlock()
	if kept {
		t.Fatal("historical receipt was not pruned")
	}
	if err := e.state.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestEngineWaitsForConfirmedFullImport(t *testing.T) {
	for _, tc := range []struct {
		name         string
		response     any
		status, wait string
	}{
		{"stored", "stored", "stored", CleanupWaitImport},
		{"failed", "failed", "failed", CleanupWaitCoverage},
		{"unknown status", "unavailable", "unavailable", CleanupWaitCoverage},
		{"lookup error", http.StatusServiceUnavailable, "queued", CleanupWaitImport},
		{"missing receipt on server", nil, "unknown", CleanupWaitCoverage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := time.Unix(1794700000, 0)
			srv, serverURL, receipts := censusImportServer(t)
			e, path, content := newCleanupEngine(t, serverURL, &clock)
			if tc.response != nil {
				receipts.Store("1", tc.response)
			}
			ctx := context.Background()
			e.RunOnce(ctx, false, false)
			baseline, _ := verifyCensusBundle(t, srv, 1, content, ModeFull, 0)
			e.RunOnce(ctx, false, false)
			e = reloadCleanupEngine(t, e)
			e.RunOnce(ctx, false, false)
			assertCleanupBlocked(t, e, path, content, tc.wait)
			assertCensusImportStatus(t, e, baseline, tc.status)
		})
	}
}

func TestEngineChecksRowsSkippedByCensusTimestampCursor(t *testing.T) {
	for _, tc := range []struct {
		name  string
		age   time.Duration
		clean bool
	}{
		{"old omitted character blocks cleanup", 30 * 24 * time.Hour, false},
		{"recent omitted character survives cleanup", time.Hour, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := time.Unix(1794700000, 0)
			srv, serverURL, receipts := censusImportServer(t)
			e, path, content := newCleanupEngine(t, serverURL, &clock)
			e.wowRunning = func() (bool, error) { return true, nil }
			ctx := context.Background()
			receipts.Store("1", "imported")
			e.RunOnce(ctx, false, false)

			clock = clock.Add(time.Minute)
			changed := withCleanupCharacter(content, "skipped-by-cursor", clock.Add(-tc.age).Unix())
			changed = withCleanupCharacter(changed, "included-in-delta", clock.Unix())
			writeCleanupFixture(t, path, changed, clock)
			receipts.Store("2", "imported")
			e.RunOnce(ctx, false, false)
			_, lua := verifyCensusBundle(t, srv, 2, changed, ModeSession, 1)
			if strings.Contains(lua, "skipped-by-cursor") || !strings.Contains(lua, "included-in-delta") {
				t.Fatal("fixture did not exercise the timestamp filter")
			}

			e = reloadCleanupEngine(t, e)
			e.wowRunning = func() (bool, error) { return false, nil }
			e.RunOnce(ctx, false, false)
			if tc.clean {
				if e.Status().Files[0].LastCleanup == nil || !strings.Contains(readCleanupFixture(t, path), "skipped-by-cursor") {
					t.Fatal("cleanup should remove confirmed old records and keep the recent character")
				}
			} else {
				assertCleanupBlocked(t, e, path, changed, CleanupWaitCoverage)
				e.RunOnce(ctx, false, false)
				assertCleanupBlocked(t, e, path, changed, CleanupWaitCoverage)
				warnings := 0
				for _, entry := range e.Status().Log {
					if strings.Contains(entry.Message, "import coverage is incomplete") {
						warnings++
					}
				}
				if warnings != 1 {
					t.Fatalf("want one coverage warning, got %d", warnings)
				}
				e.RunOnce(ctx, true, false)
				_, lua = verifyCensusBundle(t, srv, 3, changed, ModeFull, 0)
				if !strings.Contains(lua, "skipped-by-cursor") {
					t.Fatal("the recovery snapshot omitted the backdated character")
				}
			}
		})
	}
}

func TestEngineVerifiesLegacyAndDestinationCoverage(t *testing.T) {
	for _, scenario := range []string{"legacy", "different server"} {
		t.Run(scenario, func(t *testing.T) {
			clock := time.Unix(1794700000, 0)
			srv, serverURL, receipts := censusImportServer(t)
			e, path, content := newCleanupEngine(t, serverURL, &clock)
			e.wowRunning = func() (bool, error) { return true, nil }
			ctx := context.Background()
			receipts.Store("1", "imported")
			e.RunOnce(ctx, false, false)
			baseline, _ := verifyCensusBundle(t, srv, 1, content, ModeFull, 0)
			assertCensusImportStatus(t, e, baseline, "imported")

			if scenario == "legacy" {
				cursor, _ := e.state.Cursor(baseline.SourceID)
				cursor.Census.Coverage = nil
				if err := e.state.SetCursor(baseline.SourceID, cursor); err != nil {
					t.Fatal(err)
				}
				e = reloadCleanupEngine(t, e)
				e.wowRunning = func() (bool, error) { return false, nil }
				e.RunOnce(ctx, false, false)
				assertCleanupBlocked(t, e, path, content, CleanupWaitCoverage)
				e.RunOnce(ctx, true, false)
				if e.Status().Files[0].LastCleanup == nil || readCleanupFixture(t, path) == content || srv.puts.Load() != 1 {
					t.Fatal("exactly matching full export should recover using its imported receipt")
				}
				return
			}

			clock = clock.Add(time.Minute)
			content = withCleanupCharacter(content, "waiting-at-original-server", clock.Unix())
			writeCleanupFixture(t, path, content, clock)
			receipts.Store("2", "queued")
			e.RunOnce(ctx, false, false)
			delta, _ := verifyCensusBundle(t, srv, 2, content, ModeSession, 1)
			cfg := e.Config()
			originalDestination := censusDestination(cfg.ServerURL, cfg.DeviceID)
			other, otherURL, otherReceipts := censusImportServer(t)
			otherReceipts.Store("1", "stored")
			otherReceipts.Store("2", "imported")
			cfg.ServerURL, cfg.AutoUpload = otherURL, false
			if err := e.UpdateConfig(cfg); err != nil {
				t.Fatal(err)
			}
			e.wowRunning = func() (bool, error) { return false, nil }
			e.RunOnce(ctx, false, false)
			assertCleanupBlocked(t, e, path, content, CleanupWaitCoverage)

			e.RunOnce(ctx, true, false)
			_, lua := verifyCensusBundle(t, other, 1, content, ModeFull, 0)
			if !strings.Contains(lua, "waiting-at-original-server") {
				t.Fatal("new destination did not receive a full snapshot")
			}
			clock = clock.Add(receiptCheckEvery)
			e.RunOnce(ctx, false, false)
			assertCleanupBlocked(t, e, path, content, CleanupWaitImport)
			old, ok := e.state.CensusReceipt(delta.SourceID, originalDestination, delta.SHA256)
			if !ok || old.Status != "queued" {
				t.Fatalf("receipt status crossed destinations: %+v", old)
			}
		})
	}
}

func TestCensusSessionCoverageChecksRemovedDaysAndObservations(t *testing.T) {
	now := time.Unix(1794700000, 0)
	old := now.Add(-30 * 24 * time.Hour).Unix()
	day := old / 86400
	nextSeen := now.Add(time.Minute).Unix()
	packedDay := fmt.Sprintf("%d,1,%d,%d", day, old, old)
	for _, tc := range []struct {
		name, baseline, changed string
	}{
		{"character day",
			fmt.Sprintf(`characters = {Mage = {lastSeen = %d}},`, now.Unix()),
			fmt.Sprintf(`characters = {Mage = {lastSeen = %d, days = {[%d] = {1,%d,%d}}}},`, nextSeen, day, old, old)},
		{"packed character day",
			fmt.Sprintf(`charactersPacked = {"MLC1", %s},`, luaQuote(packedRecord("Mage", now.Unix()))),
			fmt.Sprintf(`charactersPacked = {"MLC1", %s},`, luaQuote(packedRecord("Mage", nextSeen, packedDay)))},
		{"completed sweep observations",
			fmt.Sprintf(`sweeps = {Sweep = {startedAt = %d, status = "complete"}},`, now.Unix()),
			fmt.Sprintf(`sweeps = {Sweep = {startedAt = %d, status = "complete", observations = {Mage = {observedAt = %d}}}},`, now.Unix(), old)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial := []byte(fmt.Sprintf("WoWCensusDB = {realms = {Realm = {population = {%s samples = {{t = %d}}}}}}", tc.baseline, now.Unix()))
			changed := []byte(fmt.Sprintf("WoWCensusDB = {realms = {Realm = {population = {%s samples = {{t = %d}}}}}}", tc.changed, nextSeen))
			full, err := buildCensusFull(initial, nil)
			if err != nil {
				t.Fatal(err)
			}
			delta, err := BuildCensusDelta(changed, full.Next, nil)
			if err != nil || len(delta.Lua) == 0 {
				t.Fatalf("expected a session export, got %v", err)
			}
			prior := censusRecordsForUpload(initial, full.Lua, true, true, censusRecordCoverage{})
			proof := censusRecordsForUpload(changed, delta.Lua, false, true, prior)
			cleaned, result, err := CleanSavedVariables(changed, now.Add(time.Minute), defaultCleanKeepDays)
			if err != nil || !result.Removed() {
				t.Fatalf("expected deletions in the cleanup plan, got %+v (%v)", result, err)
			}
			if proof.coversCleanup(changed, cleaned) {
				t.Fatal("the proposed deletion includes records omitted by the session")
			}
		})
	}
}

func TestEngineWaitsAfterFailedImportStateWrite(t *testing.T) {
	clock := time.Unix(1794700000, 0)
	srv, serverURL, receipts := censusImportServer(t)
	e, path, content := newCleanupEngine(t, serverURL, &clock)
	ctx := context.Background()
	receipts.Store("1", "queued")
	e.RunOnce(ctx, false, false)
	baseline, _ := verifyCensusBundle(t, srv, 1, content, ModeFull, 0)
	receipts.Store("1", "imported")
	clock = clock.Add(receiptCheckEvery)
	e.state.path = t.TempDir()
	e.RunOnce(ctx, false, false)
	e.RunOnce(ctx, false, false)
	assertCleanupBlocked(t, e, path, content, CleanupWaitCoverage)

	e = reloadCleanupEngine(t, e)
	assertCensusImportStatus(t, e, baseline, "queued")
	e.RunOnce(ctx, false, false)
	e.RunOnce(ctx, false, false)
	if e.Status().Files[0].LastCleanup == nil || readCleanupFixture(t, path) == content {
		t.Fatal("cleanup did not resume after the import status was saved")
	}
}

func assertCleanupBlocked(t *testing.T, e *Engine, path, original, wait string) {
	t.Helper()
	f := e.Status().Files[0]
	if f.LastCleanup != nil || f.CleanupWait != wait || readCleanupFixture(t, path) != original {
		t.Fatalf("want intact save waiting for %q, got %+v", wait, f)
	}
	backups, err := ListBackups(e.Config().ResolvedBackupDir(e.AppDir()))
	if err != nil || len(backups) != 0 {
		t.Fatalf("unexpected cleanup backups: %v (%v)", backups, err)
	}
}

func censusImportServer(t *testing.T) (*fakeServer, string, *sync.Map) {
	t.Helper()
	server := &fakeServer{token: "cleanup-token"}
	var receipts sync.Map
	mux := http.NewServeMux()
	uploads := server.handler(t)
	mux.Handle("/v1/devices", uploads)
	mux.Handle("/v1/uploads", uploads)
	mux.HandleFunc("/v1/uploads/", func(w http.ResponseWriter, r *http.Request) {
		if status, ok := receipts.Load(strings.TrimPrefix(r.URL.Path, "/v1/uploads/")); ok {
			if code, ok := status.(int); ok {
				w.WriteHeader(code)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"status": status.(string)})
			return
		}
		http.NotFound(w, r)
	})
	hs := httptest.NewServer(mux)
	t.Cleanup(hs.Close)
	return server, hs.URL, &receipts
}

func newCleanupEngine(t *testing.T, serverURL string, clock *time.Time) (*Engine, string, string) {
	t.Helper()
	root := t.TempDir()
	content := censusFixture(clock.Unix())
	var bulk strings.Builder
	for i := range 12000 {
		fmt.Fprintf(&bulk, "[\"us:forever:realm:old%05d\"] = {lastSeen = %d, zone = \"%s\"},\n",
			i, clock.Unix()-30*86400, strings.Repeat("Elwynn Forest ", 3))
	}
	content = strings.Replace(content, "[\"characters\"] = {\n", "[\"characters\"] = {\n"+bulk.String(), 1)
	path := makeInstall(t, root, "_forever_", "ACC", content)
	writeCleanupFixture(t, path, content, *clock)
	e, err := NewEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e.now = func() time.Time { return *clock }
	e.wowRunning = func() (bool, error) { return false, nil }
	e.discover = func([]string) []SavedVarFile { return Discover([]string{root}) }
	e.discoverEditions = func([]string) []string { return nil }
	cfg := e.Config()
	cfg.ServerURL, cfg.AutoUpload, cfg.AutoClean, cfg.CleanAboveMB = serverURL, true, true, 1
	if err := e.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return e, path, content
}

func withCleanupCharacter(src, name string, seen int64) string {
	const start = "[\"characters\"] = {\n"
	return strings.Replace(src, start, start+fmt.Sprintf("[%q] = {lastSeen = %d},\n", name, seen), 1)
}

func writeCleanupFixture(t *testing.T, path, content string, clock time.Time) {
	t.Helper()
	writeOld(t, path, content)
	settled := clock.Add(-time.Minute)
	if err := os.Chtimes(path, settled, settled); err != nil {
		t.Fatal(err)
	}
}

func readCleanupFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func reloadCleanupEngine(t *testing.T, e *Engine) *Engine {
	t.Helper()
	reloaded, err := NewEngine(e.AppDir())
	if err != nil {
		t.Fatal(err)
	}
	reloaded.now, reloaded.wowRunning = e.now, e.wowRunning
	reloaded.discover, reloaded.discoverEditions = e.discover, e.discoverEditions
	return reloaded
}

func assertCensusImportStatus(t *testing.T, e *Engine, m Manifest, want string) {
	t.Helper()
	cfg := e.Config()
	r, ok := e.state.CensusReceipt(m.SourceID, censusDestination(cfg.ServerURL, cfg.DeviceID), m.SHA256)
	if !ok || r.Status != want {
		t.Fatalf("receipt for %s upload: got %+v, want %s", m.Mode, r, want)
	}
}

func TestEngineCleansAfterImportedSessionUpload(t *testing.T) {
	clock := time.Unix(1794700000, 0)
	root := t.TempDir()
	content := censusFixture(clock.Unix())
	var bulk strings.Builder
	for i := 0; i < 12000; i++ {
		fmt.Fprintf(&bulk, "\t\t\t\t\t[\"us:forever:realm:old%05d\"] = {\n\t\t\t\t\t\t[\"lastSeen\"] = %d,\n\t\t\t\t\t\t[\"zone\"] = \"Elwynn Forest\",\n\t\t\t\t\t},\n", i, clock.Unix()-30*86400)
	}
	content = strings.Replace(content, "\t\t\t\t[\"characters\"] = {\n", "\t\t\t\t[\"characters\"] = {\n"+bulk.String(), 1)
	p := makeInstall(t, root, "_forever_", "ACC", content)
	settle := func() {
		t.Helper()
		old := clock.Add(-time.Minute)
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	settle()

	srv := &importServer{status: "queued"}
	hs := httptest.NewServer(srv.handler())
	defer hs.Close()
	e, err := NewEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wowOpen atomic.Bool
	wowOpen.Store(true)
	e.now = func() time.Time { return clock }
	e.wowRunning = func() (bool, error) { return wowOpen.Load(), nil }
	e.discover = func([]string) []SavedVarFile { return Discover([]string{root}) }
	e.discoverEditions = func([]string) []string { return DiscoverEditions([]string{root}) }
	cfg := e.Config()
	cfg.ServerURL, cfg.AutoUpload, cfg.AutoClean, cfg.CleanAboveMB = hs.URL, true, true, 1
	if err := e.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	e.RunOnce(ctx, false, false)
	if srv.puts.Load() != 1 {
		t.Fatalf("want the first full upload, got %d puts", srv.puts.Load())
	}

	clock = clock.Add(time.Minute)
	newSample := fmt.Sprintf("\t\t\t\t[\"samples\"] = {\n\t\t\t\t\t{\n\t\t\t\t\t\t[\"t\"] = %d,\n\t\t\t\t\t\t[\"observed\"] = 9,\n\t\t\t\t\t},\n", clock.Unix())
	changed := strings.Replace(content, "\t\t\t\t[\"samples\"] = {\n", newSample, 1)
	if changed == content {
		t.Fatal("fixture has no samples table to extend")
	}
	writeOld(t, p, changed)
	settle()
	e.RunOnce(ctx, false, false)
	f := e.Status().Files[0]
	if srv.puts.Load() != 2 || f.LastReceipt == nil || f.LastReceipt.Mode != ModeSession {
		t.Fatalf("want a second, session-mode upload, got %d puts, receipt %+v\n%v", srv.puts.Load(), f.LastReceipt, e.Status().Log)
	}

	srv.setStatus("imported")
	clock = clock.Add(receiptCheckEvery + time.Minute)
	e.RunOnce(ctx, false, false)
	e.RunOnce(ctx, false, false)

	wowOpen.Store(false)
	e.RunOnce(ctx, false, false)
	f = e.Status().Files[0]
	if f.LastCleanup == nil {
		t.Fatalf("cleanup never ran after the session upload was imported (cleanup wait %q, receipt %+v)", f.CleanupWait, f.LastReceipt)
	}
}
