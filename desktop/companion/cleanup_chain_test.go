package companion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type chainServer struct {
	mu       sync.Mutex
	next     int
	modes    []string
	statuses map[string]string
}

func (s *chainServer) set(id, status string) {
	s.mu.Lock()
	s.statuses[id] = status
	s.mu.Unlock()
}

func (s *chainServer) uploads() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.modes...)
}

func (s *chainServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/devices", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]string{"deviceId": "dev_1", "token": "tok"})
	})
	mux.HandleFunc("/v1/uploads", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		s.mu.Lock()
		s.next++
		id := strconv.Itoa(s.next)
		s.modes = append(s.modes, r.Header.Get("X-WoWCensus-Mode"))
		s.statuses[id] = "queued"
		s.mu.Unlock()
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(uploadResponse{Status: "queued", ReceiptID: id})
	})
	mux.HandleFunc("/v1/uploads/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		st, ok := s.statuses[strings.TrimPrefix(r.URL.Path, "/v1/uploads/")]
		s.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": st})
	})
	return mux
}

type chainHarness struct {
	t       *testing.T
	e       *Engine
	srv     *chainServer
	path    string
	content string
	clock   time.Time
	wowOpen atomic.Bool
	samples int
}

func newChainHarness(t *testing.T, autoClean bool) *chainHarness {
	t.Helper()
	h := &chainHarness{t: t, clock: time.Unix(1794700000, 0), srv: &chainServer{statuses: map[string]string{}}}
	root := t.TempDir()
	content := censusFixture(h.clock.Unix())
	var bulk strings.Builder
	for i := range 12000 {
		fmt.Fprintf(&bulk, "\t\t\t\t\t[\"us:forever:realm:old%05d\"] = {\n\t\t\t\t\t\t[\"lastSeen\"] = %d,\n\t\t\t\t\t\t[\"zone\"] = \"Elwynn Forest\",\n\t\t\t\t\t},\n", i, h.clock.Unix()-30*86400)
	}
	h.content = strings.Replace(content, "\t\t\t\t[\"characters\"] = {\n", "\t\t\t\t[\"characters\"] = {\n"+bulk.String(), 1)
	h.path = makeInstall(t, root, "_forever_", "ACC", h.content)
	h.settle()
	hs := httptest.NewServer(h.srv.handler())
	t.Cleanup(hs.Close)
	e, err := NewEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h.wowOpen.Store(true)
	e.now = func() time.Time { return h.clock }
	e.wowRunning = func() (bool, error) { return h.wowOpen.Load(), nil }
	e.discover = func([]string) []SavedVarFile { return Discover([]string{root}) }
	e.discoverEditions = func([]string) []string { return DiscoverEditions([]string{root}) }
	cfg := e.Config()
	cfg.ServerURL, cfg.AutoUpload, cfg.AutoClean, cfg.CleanAboveMB = hs.URL, true, autoClean, 1
	if err := e.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	h.e = e
	return h
}

func (h *chainHarness) settle() {
	h.t.Helper()
	old := h.clock.Add(-time.Minute)
	if err := os.Chtimes(h.path, old, old); err != nil {
		h.t.Fatal(err)
	}
}

func (h *chainHarness) run() FileStatus {
	h.t.Helper()
	h.e.RunOnce(context.Background(), false, false)
	return h.e.Status().Files[0]
}

func (h *chainHarness) play() {
	h.t.Helper()
	h.clock = h.clock.Add(time.Minute)
	h.samples++
	sample := fmt.Sprintf("\t\t\t\t[\"samples\"] = {\n\t\t\t\t\t{\n\t\t\t\t\t\t[\"t\"] = %d,\n\t\t\t\t\t\t[\"observed\"] = %d,\n\t\t\t\t\t},\n", h.clock.Unix(), h.samples)
	changed := strings.Replace(h.content, "\t\t\t\t[\"samples\"] = {\n", sample, 1)
	if changed == h.content {
		h.t.Fatal("fixture has no samples table to extend")
	}
	h.content = changed
	writeOld(h.t, h.path, changed)
	h.settle()
	h.run()
}

func (h *chainHarness) refresh() FileStatus {
	h.t.Helper()
	h.clock = h.clock.Add(receiptCheckEvery + time.Minute)
	h.run()
	return h.run()
}

func (h *chainHarness) closeWoW() FileStatus {
	h.t.Helper()
	h.wowOpen.Store(false)
	h.clock = h.clock.Add(cleanupRecheckEvery + time.Minute)
	return h.run()
}

func (h *chainHarness) wantModes(want ...string) {
	h.t.Helper()
	if got := h.srv.uploads(); strings.Join(got, ",") != strings.Join(want, ",") {
		h.t.Fatalf("want uploads %v, got %v", want, got)
	}
}

func (h *chainHarness) wantNoCleanup(f FileStatus) {
	h.t.Helper()
	data, err := os.ReadFile(h.path)
	if err != nil {
		h.t.Fatal(err)
	}
	if f.LastCleanup != nil || string(data) != h.content || f.CleanupWait != CleanupWaitImport {
		h.t.Fatalf("cleanup should wait for the import, got wait %q, cleanup %+v", f.CleanupWait, f.LastCleanup)
	}
}

func TestCleanupWaitsAfterFailedSessionImport(t *testing.T) {
	h := newChainHarness(t, true)
	h.run()
	h.srv.set("1", "imported")
	h.play()
	h.srv.set("2", "failed")
	h.refresh()
	h.wantNoCleanup(h.closeWoW())

	h.wowOpen.Store(true)
	h.play()
	h.wantModes(ModeFull, ModeSession, ModeFull)
	h.wantNoCleanup(h.closeWoW())

	h.srv.set("3", "imported")
	h.refresh()
	if f := h.closeWoW(); f.LastCleanup == nil {
		t.Fatalf("cleanup never ran after the new full upload was imported (wait %q)", f.CleanupWait)
	}
}

func TestCleanupWaitsForEarlierPendingSession(t *testing.T) {
	h := newChainHarness(t, true)
	h.run()
	h.srv.set("1", "imported")
	h.play()
	h.play()
	h.wantModes(ModeFull, ModeSession, ModeSession)
	h.srv.set("3", "imported")
	h.refresh()
	h.wantNoCleanup(h.closeWoW())

	h.srv.set("2", "imported")
	h.refresh()
	if f := h.closeWoW(); f.LastCleanup == nil {
		t.Fatalf("cleanup never ran after every session was imported (wait %q)", f.CleanupWait)
	}
}

func TestCleanupKeepsImportedChainAfterReceiptsArePruned(t *testing.T) {
	h := newChainHarness(t, true)
	h.run()
	h.srv.set("1", "imported")
	h.play()
	h.srv.set("2", "imported")
	h.refresh()
	h.e.state.mu.Lock()
	for k, r := range h.e.state.Receipts {
		if r.Status == "imported" {
			delete(h.e.state.Receipts, k)
		}
	}
	h.e.state.mu.Unlock()
	if f := h.closeWoW(); f.LastCleanup == nil {
		t.Fatalf("pruning imported receipts blocked cleanup (wait %q)", f.CleanupWait)
	}
}

func TestLegacyCheckpointSendsFullUploadOnlyForCleanup(t *testing.T) {
	for _, autoClean := range []bool{true, false} {
		t.Run(fmt.Sprintf("autoClean=%v", autoClean), func(t *testing.T) {
			h := newChainHarness(t, autoClean)
			h.run()
			h.srv.set("1", "imported")
			h.refresh()
			sourceID := h.e.state.SourceID(h.path)
			cur, ok := h.e.state.Cursor(sourceID)
			if !ok || cur.Census == nil || !cur.Census.Baseline {
				t.Fatalf("a full upload should start the import chain: %+v", cur.Census)
			}
			cur.Census.Baseline, cur.Census.Chain = false, nil
			if err := h.e.state.SetCursor(sourceID, cur); err != nil {
				t.Fatal(err)
			}
			if autoClean {
				h.wantNoCleanup(h.closeWoW())
				h.wowOpen.Store(true)
			}
			h.play()
			want := ModeSession
			if autoClean {
				want = ModeFull
			}
			h.wantModes(ModeFull, want)
			h.play()
			h.wantModes(ModeFull, want, ModeSession)
		})
	}
}

func TestForcedUploadRestartsBrokenChain(t *testing.T) {
	h := newChainHarness(t, true)
	h.run()
	h.srv.set("1", "imported")
	h.play()
	h.srv.set("2", "failed")
	h.refresh()
	h.e.RunOnce(context.Background(), true, false)
	h.wantModes(ModeFull, ModeSession, ModeFull)
	h.e.RunOnce(context.Background(), true, false)
	h.wantModes(ModeFull, ModeSession, ModeFull)
}
