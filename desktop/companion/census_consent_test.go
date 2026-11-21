package companion

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const consentCoreTime = 1790000100

func censusConsentFixture(coreTime int, newJournalatorRow bool) string {
	journalator := ""
	if newJournalatorRow {
		journalator = `New = {t = 1790000500, kind = "journalator-new-row"},`
	}
	return fmt.Sprintf(`
WoWCensusDB = {
	settings = {debug = true},
	minimap = {hide = false},
	populationStorageVersion = 2,
	leveling = {first = {level = 60}},
	realms = {
		Realm = {
			identity = {realmName = "census-realm"},
			population = {samples = {{t = %[1]d, count = 12}}},
		},
	},
	["mark\101t"] = {
		version = 3,
		game = "retail",
		region = "us",
		collectedAt = 1990000000,
		cursor = {journalator = "collector-cursor"},
		futureSource = {row = {t = 1990000000, data = "future-market-row"}},
		futureScalar = "future-market-scalar",
		journalator = {
			Invoices = {
				Old = {t = 1790000300, kind = "journalator-row"},
				%[2]s
			},
		},
		tsm = {Sale = {t = 1790000050, item = "tsm-row"}},
		auctionator = {Post = {t = 1790000040, key = "auctionator-row"}},
		dungeonJournal = {
			accountAt = 1790000030,
			account = {loot = "dungeon-account"},
			characters = {Character = {t = 1790000031, data = "dungeon-character"}},
			future = "dungeon-unknown",
		},
	},
	futureCensusField = {data = "future-census-row"},
}
NameplateInspectDB = {Player = {time = %[1]d, spec = "census-inspection"}}
FutureAddonDB = {data = "future-addon-row"}
`, coreTime, journalator)
}

func newCensusConsentEngine(t *testing.T, src string) (*Engine, *fakeServer, string) {
	t.Helper()
	root := t.TempDir()
	path := makeInstall(t, root, "_retail_", "A", src)
	fs := &fakeServer{token: "test-token"}
	server := httptest.NewServer(fs.handler(t))
	t.Cleanup(server.Close)
	e, err := NewEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e.discover = func([]string) []SavedVarFile { return Discover([]string{root}) }
	e.discoverEditions = func([]string) []string { return nil }
	cfg := e.Config()
	cfg.AutoUpload, cfg.ServerURL = true, server.URL
	if err := e.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return e, fs, path
}

func setCensusConsent(t *testing.T, e *Engine, shares map[string]bool) {
	t.Helper()
	cfg := e.Config()
	cfg.ShareAddons = shares
	if err := e.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

func verifyCensusBundle(t *testing.T, fs *fakeServer, number int, src, mode string, rows int) (Manifest, string) {
	t.Helper()
	_, _, bodies := fs.uploads()
	if len(bodies) != number {
		t.Fatalf("got %d uploads, want %d", len(bodies), number)
	}
	m, lua := bundleLua(t, bodies[number-1])
	fileSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(src)))
	uploadSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(lua)))
	if m.Kind != KindCensus || m.Mode != mode || m.Rows != rows || m.FileName != SavedVariablesName ||
		m.FileSHA256 != fileSHA || m.SHA256 != uploadSHA || m.FileSHA256 == m.SHA256 || m.Bytes != int64(len(lua)) {
		t.Fatalf("manifest does not describe the exported payload: %+v", m)
	}
	fs.mu.Lock()
	h := fs.headers[number-1]
	fs.mu.Unlock()
	if h.Get("X-WoWCensus-Sha256") != m.SHA256 || h.Get("X-WoWCensus-Lua-Bytes") != strconv.Itoa(len(lua)) ||
		h.Get("X-WoWCensus-Source-Id") != m.SourceID || h.Get("X-WoWCensus-Mode") != mode {
		t.Fatalf("headers do not match the manifest: %v", h)
	}
	if _, err := readCensusSave([]byte(lua)); err != nil {
		t.Fatalf("invalid exported Lua: %v\n%s", err, lua)
	}
	return m, lua
}

func verifyCensusMarket(t *testing.T, lua string, shares map[string]bool) {
	t.Helper()
	s, err := readCensusSave([]byte(lua))
	if err != nil {
		t.Fatal(err)
	}
	market := s.db.Field("market")
	wantMarket := false
	for _, source := range []struct {
		field, setting string
		markers        []string
	}{
		{"journalator", "journalator", []string{"journalator-row"}},
		{"tsm", "tsm", []string{"tsm-row"}},
		{"auctionator", "auctionator", []string{"auctionator-row"}},
		{"dungeonJournal", "dungeonjournal", []string{"dungeon-account", "dungeon-character"}},
	} {
		want := shares[source.setting]
		wantMarket = wantMarket || want
		if got := market.Field(source.field) != nil; got != want {
			t.Errorf("market.%s present = %t, want %t", source.field, got, want)
		}
		for _, marker := range source.markers {
			if got := strings.Contains(lua, marker); got != want {
				t.Errorf("%q present = %t, want %t", marker, got, want)
			}
		}
	}
	if !wantMarket && market != nil {
		t.Error("core-only payload still contains the market table")
	}
	for _, gone := range []string{
		"cursor", "collectedAt", "future-market-row", "future-market-scalar", "future-census-row", "future-addon-row", "dungeon-unknown",
	} {
		if strings.Contains(lua, gone) {
			t.Errorf("export contains %q", gone)
		}
	}
	if market != nil && (market.Field("version").Number() != 3 || market.Field("game").String() != "retail" ||
		market.Field("region").String() != "us") {
		t.Error("market is missing supported metadata")
	}
}

func TestCensusConsentMatrix(t *testing.T) {
	cases := []struct {
		name   string
		shares map[string]bool
		rows   int
	}{
		{"disabled", map[string]bool{"journalator": false, "tsm": false, "auctionator": false, "dungeonjournal": false}, 0},
		{"journalator", map[string]bool{"journalator": true}, 1},
		{"tsm", map[string]bool{"tsm": true}, 1},
		{"auctionator", map[string]bool{"auctionator": true}, 1},
		{"dungeon journal", map[string]bool{"dungeonjournal": true}, 2},
		{"selected", map[string]bool{"journalator": true, "dungeonjournal": true, "futureSource": true}, 3},
		{"enabled", map[string]bool{"journalator": true, "tsm": true, "auctionator": true, "dungeonjournal": true}, 5},
		{"missing", nil, 0},
	}
	src := censusConsentFixture(consentCoreTime, false)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, fs, path := newCensusConsentEngine(t, src)
			setCensusConsent(t, e, tc.shares)
			e.RunOnce(context.Background(), true, false)
			_, lua := verifyCensusBundle(t, fs, 1, src, ModeFull, 0)
			verifyCensusMarket(t, lua, tc.shares)
			for _, want := range []string{"census-realm", "census-inspection", "settings", "minimap", "populationStorageVersion", "leveling", "count = 12"} {
				if !strings.Contains(lua, want) {
					t.Errorf("full upload omitted %q", want)
				}
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != src {
				t.Fatalf("export changed the local save: %v", err)
			}

			d, err := BuildCensusDelta([]byte(src), Cursor{}, tc.shares)
			if err != nil || d.Rows != 2+tc.rows {
				t.Fatalf("delta rows = %d, want %d, err = %v", d.Rows, 2+tc.rows, err)
			}
			verifyCensusMarket(t, string(d.Lua), tc.shares)
			if again, err := BuildCensusDelta([]byte(src), d.Next, tc.shares); err != nil || again.Rows != 0 || again.Lua != nil {
				t.Fatalf("unchanged delta should be empty: rows %d, err %v", again.Rows, err)
			}
		})
	}
}

func TestCensusConsentTransitions(t *testing.T) {
	src := censusConsentFixture(consentCoreTime, false)
	e, fs, path := newCensusConsentEngine(t, src)
	ctx := context.Background()
	e.RunOnce(ctx, false, false)
	full, lua := verifyCensusBundle(t, fs, 1, src, ModeFull, 0)
	verifyCensusMarket(t, lua, nil)

	journalator := map[string]bool{"journalator": true}
	setCensusConsent(t, e, journalator)
	e.RunOnce(ctx, false, false)
	journal, lua := verifyCensusBundle(t, fs, 2, src, ModeSession, 1)
	verifyCensusMarket(t, lua, journalator)

	restarted, err := NewEngine(e.AppDir())
	if err != nil {
		t.Fatal(err)
	}
	restarted.discover, restarted.discoverEditions = e.discover, e.discoverEditions
	e = restarted
	e.RunOnce(ctx, false, false)
	tsm := map[string]bool{"tsm": true}
	setCensusConsent(t, e, map[string]bool{"journalator": true, "tsm": true})
	e.RunOnce(ctx, false, false)
	trade, lua := verifyCensusBundle(t, fs, 3, src, ModeSession, 1)
	verifyCensusMarket(t, lua, tsm)
	cfg := e.Config()
	destination := censusDestination(cfg.ServerURL, cfg.DeviceID)
	ids := map[string]bool{}
	for _, m := range []Manifest{full, journal, trade} {
		r, ok := e.state.CensusReceipt(m.SourceID, destination, m.SHA256)
		if !ok || ids[r.ReceiptID] || r.SHA256 != m.FileSHA256 || r.UploadSHA != m.SHA256 {
			t.Fatalf("receipt should identify this exact export: %+v for %+v", r, m)
		}
		ids[r.ReceiptID] = true
	}

	setCensusConsent(t, e, tsm)
	e.RunOnce(ctx, false, false)
	r := e.Status().Files[0].LastReceipt
	if fs.puts.Load() != 3 || r == nil || r.Status != "empty" || r.ReceiptID != "" {
		t.Fatalf("revocation without new rows should complete without an upload or imported receipt: %+v", r)
	}
	newSave := censusConsentFixture(consentCoreTime+100, true)
	writeOld(t, path, newSave)
	e.RunOnce(ctx, false, false)
	_, lua = verifyCensusBundle(t, fs, 4, newSave, ModeSession, 2)
	verifyCensusMarket(t, lua, nil)
	if strings.Contains(lua, "journalator-new-row") {
		t.Fatal("a withheld row was uploaded")
	}
	setCensusConsent(t, e, journalator)
	e.RunOnce(ctx, false, false)
	restored, lua := verifyCensusBundle(t, fs, 5, newSave, ModeSession, 1)
	if !strings.Contains(lua, "journalator-new-row") || strings.Contains(lua, `"journalator-row"`) {
		t.Fatalf("restored consent should send only the withheld Journalator row:\n%s", lua)
	}
	r = e.Status().Files[0].LastReceipt
	if r == nil {
		t.Fatal("restored consent upload has no receipt")
	}
	if err := e.state.SetReceiptStatus(r.ReceiptID, "imported", time.Now()); err != nil {
		t.Fatal(err)
	}
	if e.state.CensusCleanupReady(restored.SourceID, restored.FileSHA256, destination) {
		t.Fatal("an imported market-only delta made the original save eligible for cleanup")
	}
	setCensusConsent(t, e, nil)
	e.RunOnce(ctx, false, false)
	setCensusConsent(t, e, journalator)
	e.RunOnce(ctx, true, false)
	if fs.puts.Load() != 5 {
		t.Fatalf("restoring consent sent already covered rows: %d uploads", fs.puts.Load())
	}
}

func TestCensusConsentExportFailureRetries(t *testing.T) {
	src := censusConsentFixture(consentCoreTime, false)
	updated := censusConsentFixture(consentCoreTime+100, true)
	for _, tc := range []struct {
		name, invalid string
		withCursor    bool
	}{
		{"malformed full save", "WoWCensusDB = {\n" + src, false},
		{"malformed session save", "WoWCensusDB = {\n" + updated, true},
		{"damaged packed delta", strings.Replace(updated, "population = {", `population = {charactersPacked = {"MLC1", "5:abc"},`, 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, fs, path := newCensusConsentEngine(t, src)
			ctx := context.Background()
			var priorPuts int32
			if tc.withCursor {
				e.RunOnce(ctx, false, false)
				priorPuts = 1
			}
			writeOld(t, path, tc.invalid)
			e.RunOnce(ctx, false, false)
			e.RunOnce(ctx, false, false)
			f := e.Status().Files[0]
			fail, waiting := e.state.Backoff(f.SourceID, f.SHA256, e.now())
			if f.Uploaded || !waiting || f.NextRetry == nil || !strings.Contains(f.LastError, "could not build a census export") ||
				fail.Attempts != 1 || fs.puts.Load() != priorPuts {
				t.Fatalf("failed filtering must stay local and enter backoff: %+v, %+v, %d PUTs", f, fail, fs.puts.Load())
			}
			e.RunOnce(ctx, true, false)
			fail, _ = e.state.Backoff(f.SourceID, f.SHA256, e.now())
			if fail.Attempts != 2 || fs.puts.Load() != priorPuts {
				t.Fatalf("retry bypassed the filter: %+v, %d PUTs", fail, fs.puts.Load())
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != tc.invalid {
				t.Fatalf("a failed export changed the save: %v", err)
			}
			writeOld(t, path, updated)
			e.RunOnce(ctx, false, false)
			mode, rows := ModeFull, 0
			if tc.withCursor {
				mode, rows = ModeSession, 2
			}
			_, lua := verifyCensusBundle(t, fs, int(priorPuts)+1, updated, mode, rows)
			verifyCensusMarket(t, lua, nil)
		})
	}
}

func TestCensusConsentLegacyState(t *testing.T) {
	src := censusConsentFixture(consentCoreTime, false)
	e, fs, path := newCensusConsentEngine(t, src)
	sourceID := e.state.SourceID(path)
	fileSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(src)))
	old := map[string]any{
		"installSalt": e.state.InstallSalt,
		"receipts": map[string]Receipt{
			receiptKey(sourceID, fileSHA): {SourceID: sourceID, SHA256: fileSHA, UploadSHA: fileSHA, ReceiptID: "legacy", Status: "imported", Mode: ModeFull},
		},
		"cursors": map[string]any{sourceID: map[string]any{
			"buckets": map[string]int{"Realm": consentCoreTime},
			"inspect": consentCoreTime,
			"market":  1990000000,
		}},
	}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.state.path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	e.state, err = LoadState(e.AppDir())
	if err != nil {
		t.Fatal(err)
	}
	tsm := map[string]bool{"tsm": true}
	setCensusConsent(t, e, tsm)
	e.RunOnce(context.Background(), false, false)
	_, lua := verifyCensusBundle(t, fs, 1, src, ModeSession, 1)
	verifyCensusMarket(t, lua, tsm)
	r := e.Status().Files[0].LastReceipt
	if r == nil || r.ReceiptID == "legacy" || r.Status != "queued" {
		t.Fatalf("old file receipt was reused for a new payload: %+v", r)
	}
}

func TestCensusConsentRevokedBeforeDispatch(t *testing.T) {
	src := censusConsentFixture(consentCoreTime, false)
	e, fs, _ := newCensusConsentEngine(t, src)
	setCensusConsent(t, e, map[string]bool{"journalator": true})
	revoked := false
	e.newClient = func(serverURL, token string) *Client {
		if token != "" && !revoked {
			revoked = true
			setCensusConsent(t, e, nil)
		}
		return NewClient(serverURL, token)
	}
	e.RunOnce(context.Background(), false, false)
	f := e.Status().Files[0]
	if !revoked || fs.puts.Load() != 0 || f.Uploaded || f.NextRetry != nil {
		t.Fatalf("stale consent reached dispatch or was treated as a failure: %+v, %d PUTs", f, fs.puts.Load())
	}
	e.RunOnce(context.Background(), false, false)
	_, lua := verifyCensusBundle(t, fs, 1, src, ModeFull, 0)
	verifyCensusMarket(t, lua, nil)
}

func TestAddonSharingRevokedBeforeDispatch(t *testing.T) {
	e, fs, path := newCensusConsentEngine(t, censusLua(1))
	ctx := context.Background()
	e.RunOnce(ctx, false, false)
	f := e.discover(nil)[0]
	e.discoverEditions = func([]string) []string { return []string{f.Root} }
	writeOld(t, filepath.Join(filepath.Dir(path), "TradeSkillMaster.lua"), `TradeSkillMasterDB = {
		["r@Stormrage@internalData@csvSales"] = "itemString,stackSize,quantity,price,player,time,source\ni:123,1,2,500,Me,1790000100,Auction",
	}`)
	tsm := map[string]bool{"tsm": true}
	setCensusConsent(t, e, tsm)
	revoked := false
	e.newClient = func(serverURL, token string) *Client {
		if !revoked {
			revoked = true
			setCensusConsent(t, e, nil)
		}
		return NewClient(serverURL, token)
	}
	e.RunOnce(ctx, false, false)
	if files := e.Status().AddonFiles; !revoked || fs.puts.Load() != 1 || len(files) != 1 || files[0].Uploaded {
		t.Fatalf("revoked add-on reached dispatch: %+v, %d PUTs", files, fs.puts.Load())
	}
	setCensusConsent(t, e, tsm)
	e.RunOnce(ctx, false, false)
	if _, kinds, _ := fs.uploads(); len(kinds) != 2 || kinds[1] != KindAddon {
		t.Fatalf("cancelled add-on export should be eligible after opting back in: %v", kinds)
	}
}

func TestCensusReceiptReuseUsesPayloadHash(t *testing.T) {
	src := censusConsentFixture(consentCoreTime, false)
	e, fs, path := newCensusConsentEngine(t, src)
	ctx := context.Background()
	e.RunOnce(ctx, false, false)
	m, _ := verifyCensusBundle(t, fs, 1, src, ModeFull, 0)
	e.state.mu.Lock()
	delete(e.state.Cursors, m.SourceID)
	e.state.mu.Unlock()
	writeOld(t, path, strings.Replace(src, "collector-cursor", "changed-local-cursor", 1))
	e.RunOnce(ctx, false, false)
	r := e.Status().Files[0].LastReceipt
	if fs.puts.Load() != 1 || r == nil || !r.Duplicate || r.UploadSHA != m.SHA256 || r.SHA256 == m.FileSHA256 {
		t.Fatalf("identical exports should reuse only the payload receipt: %+v, %d PUTs", r, fs.puts.Load())
	}
}
