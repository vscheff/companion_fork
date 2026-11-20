package companion

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

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
