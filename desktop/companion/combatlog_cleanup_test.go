package companion

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
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
