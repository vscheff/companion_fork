package companion

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func packedRecord(key string, lastSeen int64, days ...string) string {
	fields := []string{key, "Full-Realm", "Full", "Fu", "Ll", "Realm", "", "60", "Human", "Mage", "MAGE", "Elwynn",
		fmt.Sprint(lastSeen - 100), fmt.Sprint(lastSeen), "3", strings.Join(days, ";")}
	var b strings.Builder
	for _, f := range fields {
		fmt.Fprintf(&b, "%d:%s", len(f), f)
	}
	return b.String()
}

func censusFixture(now int64) string {
	day := now / 86400
	old := now - 40*86400
	recent := now - 3600
	return fmt.Sprintf(`
WoWCensusDB = {
	["minimap"] = {
		["minimapPos"] = 241.0065967515345,
		["hide"] = false,
	},
	["settings"] = {
		["populationRetentionDays"] = 35,
		["marketCollect"] = true,
	},
	["market"] = {
		["tsm"] = {
			["a"] = {
				["t"] = %[1]d,
			},
		},
	},
	["realms"] = {
		["us:forever:Realm-Alliance"] = {
			["identity"] = {
				["game"] = "forever",
			},
			["population"] = {
				["activeSweepID"] = "active-1",
				["census"] = {
					["sweepID"] = "active-1",
					["queue"] = {
						{
							["probeFrom"] = 55,
						}, -- [1]
					},
				},
				["samples"] = {
					{
						["t"] = %[1]d,
						["observed"] = 3,
					}, -- [1]
					{
						["t"] = %[2]d,
						["observed"] = 4,
					}, -- [2]
					{
						["t"] = %[1]d,
						["observed"] = 5,
					}, -- [3]
				},
				["sweeps"] = {
					["old-1"] = {
						["status"] = "complete",
						["label"] = "census forever",
						["startedAt"] = %[1]d,
						["completedAt"] = %[1]d,
						["queries"] = {
						},
						["observations"] = {
							["us:forever:realm:a"] = {
								1, -- [1]
								%[1]d, -- [2]
							},
						},
					},
					["done-2"] = {
						["status"] = "partial",
						["label"] = "census forever",
						["startedAt"] = %[2]d,
						["completedAt"] = %[2]d,
						["queries"] = {
							{
								["filter"] = "10-60",
							}, -- [1]
						},
						["observations"] = {
							["us:forever:realm:b"] = {
								1, -- [1]
								%[2]d, -- [2]
							},
							["us:forever:realm:c"] = {
								2, -- [1]
								%[2]d, -- [2]
							},
						},
					},
					["active-1"] = {
						["status"] = "active",
						["label"] = "census forever",
						["startedAt"] = %[3]d,
						["completedAt"] = 0,
						["observations"] = {
							["us:forever:realm:d"] = {
								1, -- [1]
								%[3]d, -- [2]
							},
						},
					},
				},
				["characters"] = {
					["us:forever:realm:gone"] = {
						["lastSeen"] = %[1]d,
						["days"] = {
							[%[4]d] = {
								1, -- [1]
							},
						},
					},
					["us:forever:realm:kept"] = {
						["lastSeen"] = %[3]d,
						["name"] = "Kept \"Quoted\" \\ Name",
						["days"] = {
							[%[4]d] = {
								1, -- [1]
							},
							[%[5]d] = {
								2, -- [1]
							},
						},
					},
				},
			},
		},
		["us:forever:Realm-Horde"] = {
			["population"] = {
				["charactersPacked"] = {
					"MLC1", -- [1]
					%[6]q, -- [2]
					%[7]q, -- [3]
				},
			},
		},
	},
}
`, old, recent, now-60, day-40, day, (packedRecord("old-a", old, fmt.Sprintf("%d,1,%d,%d", day-40, old, old)) + packedRecord("old-b", old)),
		(packedRecord("new-a", recent, fmt.Sprintf("%d,1,%d,%d", day-40, old, old), fmt.Sprintf("%d,2,%d,%d", day, recent, recent))))
}

func mustParseDB(t *testing.T, src []byte) *luaValue {
	t.Helper()
	as, err := parseSavedVariables(src)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, src)
	}
	for _, a := range as {
		if a.Name == cleanupDBName {
			return a.Value
		}
	}
	t.Fatal("no WoWCensusDB")
	return nil
}

func TestCleanSavedVariablesTrimsOnlyWhatTheServerHas(t *testing.T) {
	now := time.Unix(1794700000, 0)
	src := []byte(censusFixture(now.Unix()))
	out, res, err := CleanSavedVariables(src, now, 14)
	if err != nil {
		t.Fatal(err)
	}
	want := CleanupResult{BytesBefore: int64(len(src)), BytesAfter: int64(len(out)), KeepDays: 14, Realms: 2,
		Samples: 1, Sweeps: 1, SweepRows: 2, Characters: 3, Days: 2}
	if res != want {
		t.Fatalf("result %+v, want %+v", res, want)
	}
	if !res.Removed() || res.BytesAfter >= res.BytesBefore {
		t.Fatalf("nothing removed: %+v", res)
	}
	db := mustParseDB(t, out)
	orig := mustParseDB(t, src)
	for _, k := range []string{"minimap", "settings", "market"} {
		a, b := orig.Field(k), db.Field(k)
		if string(src[a.Start:a.End]) != string(out[b.Start:b.End]) {
			t.Fatalf("%s changed", k)
		}
	}
	pop := db.Field("realms").Field("us:forever:Realm-Alliance").Field("population")
	if pop.Field("census").Field("queue") == nil || pop.Field("activeSweepID").String() != "active-1" {
		t.Fatal("census state lost")
	}
	samples, ok := pop.Field("samples").Array()
	if !ok || len(samples) != 2 || samples[0].Value.Field("observed").Number() != 4 || samples[1].Value.Field("observed").Number() != 5 {
		t.Fatalf("samples wrong: %s", out)
	}
	sweeps := pop.Field("sweeps")
	if sweeps.Field("old-1") != nil {
		t.Fatal("old sweep kept")
	}
	if d := sweeps.Field("done-2"); d == nil || len(d.Field("observations").Entries) != 0 || len(d.Field("queries").Entries) != 1 {
		t.Fatal("finished sweep should keep its queries and lose its rows")
	}
	if a := sweeps.Field("active-1"); a == nil || len(a.Field("observations").Entries) != 1 {
		t.Fatal("active sweep changed")
	}
	chars := pop.Field("characters")
	if chars.Field("us:forever:realm:gone") != nil {
		t.Fatal("old character kept")
	}
	kept := chars.Field("us:forever:realm:kept")
	if kept == nil || len(kept.Field("days").Entries) != 1 || kept.Field("name").String() != `Kept "Quoted" \ Name` {
		t.Fatalf("kept character wrong: %s", out)
	}
	packed, _ := db.Field("realms").Field("us:forever:Realm-Horde").Field("population").Field("charactersPacked").Array()
	if len(packed) != 3 || packed[0].Value.Str != packedFormat || packed[1].Value.Str != "" {
		t.Fatalf("packed chunks wrong: %s", out)
	}
	if !strings.Contains(packed[2].Value.Str, "5:new-a") || strings.Contains(packed[2].Value.Str, fmt.Sprintf("%d,1,", now.Unix()/86400-40)) {
		t.Fatalf("packed record not trimmed: %q", packed[2].Value.Str)
	}

	again, res2, err := CleanSavedVariables(out, now, 14)
	if err != nil || res2.Removed() || !bytes.Equal(again, out) {
		t.Fatalf("second pass should be a no-op: %+v %v", res2, err)
	}
}

func TestCleanSavedVariablesRenumbersExplicitArrays(t *testing.T) {
	now := time.Unix(1794700000, 0)
	old, recent := now.Unix()-30*86400, now.Unix()-60
	src := []byte(fmt.Sprintf("WoWCensusDB = {\n[\"realms\"] = {\n[\"r\"] = {\n[\"population\"] = {\n[\"samples\"] = {\n"+
		"[1] = {\n[\"t\"] = %d,\n},\n[2] = {\n[\"t\"] = %d,\n},\n[3] = {\n[\"t\"] = %d,\n},\n[4] = {\n[\"t\"] = %d,\n},\n},\n},\n},\n},\n}\n",
		old, recent, old, recent))
	out, res, err := CleanSavedVariables(src, now, 7)
	if err != nil || res.Samples != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	items, ok := mustParseDB(t, out).Field("realms").Field("r").Field("population").Field("samples").Array()
	if !ok || len(items) != 2 || items[0].Key.Num != 1 || items[1].Key.Num != 2 {
		t.Fatalf("explicit array not renumbered:\n%s", out)
	}
	mixed := []byte(fmt.Sprintf("WoWCensusDB = {\n[\"realms\"] = {\n[\"r\"] = {\n[\"population\"] = {\n[\"samples\"] = {\n"+
		"{\n[\"t\"] = %d,\n},\n[5] = {\n[\"t\"] = %d,\n},\n{\n[\"t\"] = %d,\n},\n},\n},\n},\n},\n}\n", old, old, recent))
	if out, res, err := CleanSavedVariables(mixed, now, 7); err != nil || res.Samples != 0 || !bytes.Equal(out, mixed) {
		t.Fatalf("mixed array should be left alone: %+v %v", res, err)
	}
}

func TestCleanSavedVariablesRefusesWhatItCannotRead(t *testing.T) {
	now := time.Unix(1794700000, 0)
	cases := map[string]string{
		"other addon": "MarketLensDB = {\n[\"realms\"] = {\n},\n}\n",
		"truncated":   "WoWCensusDB = {\n[\"realms\"] = {\n",
		"damaged packed": "WoWCensusDB = {\n[\"realms\"] = {\n[\"r\"] = {\n[\"population\"] = {\n[\"charactersPacked\"] = {\n" +
			"\"MLC1\",\n\"5:abc\",\n},\n},\n},\n},\n}\n",
	}
	for name, src := range cases {
		out, res, err := CleanSavedVariables([]byte(src), now, 14)
		if err == nil || res.Removed() || string(out) != src {
			t.Errorf("%s: want a refusal and the input back, got %+v %v", name, res, err)
		}
	}
	if _, _, err := CleanSavedVariables([]byte(censusFixture(now.Unix())), now, 3); err == nil {
		t.Error("a keep window under 7 days should be refused")
	}
}

func TestLuaQuoteRoundTrips(t *testing.T) {
	s := "a\"b\\c\nd\re\x00f\x01g\x7fh 한국어 ü"
	v, err := (&luaParser{src: []byte(luaQuote(s))}).value()
	if err != nil || v.Str != s {
		t.Fatalf("round trip %q -> %q (%v)", s, v.Str, err)
	}
}

func TestIsWoWProcess(t *testing.T) {
	for name, want := range map[string]bool{
		"Wow.exe": true, "WowClassic.exe": true, "WowClassicT.exe": true, "WowT.exe": true, "WowB.exe": true,
		"Wow-64.exe": true, "/Applications/World of Warcraft/_retail_/World of Warcraft.app/Contents/MacOS/World of Warcraft": true,
		"World of Warcraft Classic": true, "WowheadClient.exe": false, "WowVoiceProxy.exe": false,
		"World of Warcraft Launcher.exe": false, "Battle.net.exe": false, "wowcensus-companion.exe": false,
	} {
		if got := isWoWProcess(name); got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

type importServer struct {
	puts   atomic.Int32
	mu     sync.Mutex
	status string
}

func (s *importServer) setStatus(v string) {
	s.mu.Lock()
	s.status = v
	s.mu.Unlock()
}

func (s *importServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/devices", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]string{"deviceId": "dev_1", "token": "tok"})
	})
	mux.HandleFunc("/v1/uploads", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		s.puts.Add(1)
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(uploadResponse{Status: "queued", ReceiptID: "00000000-0000-0000-0000-000000000009"})
	})
	mux.HandleFunc("/v1/uploads/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		st := s.status
		s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]string{"status": st})
	})
	return mux
}

func TestEngineCleansOnlyAfterImportWhileWoWIsClosed(t *testing.T) {
	clock := time.Unix(1794700000, 0)
	root := t.TempDir()
	content := censusFixture(clock.Unix())
	var bulk strings.Builder
	for i := 0; i < 12000; i++ {
		fmt.Fprintf(&bulk, "\t\t\t\t\t[\"us:forever:realm:old%05d\"] = {\n\t\t\t\t\t\t[\"lastSeen\"] = %d,\n\t\t\t\t\t\t[\"zone\"] = \"Elwynn Forest\",\n\t\t\t\t\t},\n", i, clock.Unix()-30*86400)
	}
	content = strings.Replace(content, "\t\t\t\t[\"characters\"] = {\n", "\t\t\t\t[\"characters\"] = {\n"+bulk.String(), 1)
	p := makeInstall(t, root, "_forever_", "ACC", content)
	old := clock.Add(-time.Minute)
	_ = os.Chtimes(p, old, old)

	srv := &importServer{status: "queued"}
	hs := httptest.NewServer(srv.handler())
	defer hs.Close()
	appDir := t.TempDir()
	e, err := NewEngine(appDir)
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
	if e.Config().CleanKeepDays != defaultCleanKeepDays {
		t.Fatalf("keep days default %d", e.Config().CleanKeepDays)
	}
	ctx := context.Background()
	original, _ := os.ReadFile(p)
	if len(original) < 1<<20 {
		t.Fatalf("fixture too small: %d", len(original))
	}

	e.RunOnce(ctx, false, false)
	f := e.Status().Files[0]
	if srv.puts.Load() != 1 || f.CleanupWait != CleanupWaitImport {
		t.Fatalf("want one upload and a wait for import, got %d puts, %+v", srv.puts.Load(), f)
	}

	srv.setStatus("imported")
	clock = clock.Add(receiptCheckEvery + time.Minute)
	e.RunOnce(ctx, false, false)
	e.RunOnce(ctx, false, false)
	f = e.Status().Files[0]
	if f.CleanupWait != CleanupWaitWoW || f.LastCleanup != nil {
		t.Fatalf("must wait while WoW runs: %+v", f)
	}
	if now, _ := os.ReadFile(p); !bytes.Equal(now, original) {
		t.Fatal("file changed while WoW was running")
	}

	wowOpen.Store(false)
	e.RunOnce(ctx, false, false)
	f = e.Status().Files[0]
	cleaned, _ := os.ReadFile(p)
	if f.LastCleanup == nil || bytes.Equal(cleaned, original) || int64(len(cleaned)) != f.LastCleanup.BytesAfter {
		t.Fatalf("not cleaned: %+v\n%v", f, e.Status().Log)
	}
	if f.LastCleanup.Characters != 12003 || f.LastCleanup.SweepRows != 2 || !f.Uploaded || f.LastReceipt.Status != "imported" {
		t.Fatalf("cleanup record %+v receipt %+v", f.LastCleanup, f.LastReceipt)
	}
	gz, err := os.Open(f.LastCleanup.Backup)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(gz)
	if err != nil {
		t.Fatal(err)
	}
	backed, _ := io.ReadAll(zr)
	gz.Close()
	if !bytes.Equal(backed, original) {
		t.Fatal("backup is not the original file")
	}
	found := false
	for _, l := range e.Status().Log {
		if strings.HasPrefix(l.Message, "Cleaned up forever / ACC:") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no cleanup log line: %+v", e.Status().Log)
	}

	clock = clock.Add(time.Hour)
	e.RunOnce(ctx, false, false)
	e2, _ := NewEngine(appDir)
	e2.now, e2.wowRunning, e2.discover, e2.discoverEditions = e.now, e.wowRunning, e.discover, e.discoverEditions
	e2.RunOnce(ctx, false, false)
	if srv.puts.Load() != 1 {
		t.Fatalf("the cleaned file was uploaded again: %d puts", srv.puts.Load())
	}
	if after, _ := os.ReadFile(p); !bytes.Equal(after, cleaned) {
		t.Fatal("second pass changed the file again")
	}
	if f2 := e2.Status().Files[0]; f2.LastCleanup == nil || f2.LastCleanup.BytesBefore != int64(len(original)) {
		t.Fatalf("cleanup record not persisted: %+v", f2)
	}

	off := e2.Config()
	off.AutoClean = false
	_ = e2.UpdateConfig(off)
	writeOld(t, p, content)
	e2.RunOnce(ctx, false, false)
	if after, _ := os.ReadFile(p); string(after) != content {
		t.Fatal("cleanup ran while the setting was off")
	}
}
