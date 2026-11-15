package companion

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func sessionRecord(key string, lastSeen int, days ...string) string {
	fields := []string{key, "Full", "Name", "First", "", "Realm", "Guild", "60", "Orc", "Warrior", "WARRIOR", "Zone",
		"1790000000", strconv.Itoa(lastSeen), "3", strings.Join(days, ";")}
	var b strings.Builder
	for _, f := range fields {
		b.WriteString(strconv.Itoa(len(f)) + ":" + f)
	}
	return b.String()
}

func sessionFixture(stage int) string {
	recs := sessionRecord("us:retail:realm:old", 1790000100, "20700,1,1790000100,1790000100")
	if stage >= 2 {
		recs += sessionRecord("us:retail:realm:new", 1790000500, "20700,2,1790000400,1790000500")
	}
	var b strings.Builder
	b.WriteString("WoWCensusDB = {\n[\"settings\"] = {\n[\"debug\"] = false,\n},\n[\"realms\"] = {\n")
	b.WriteString("[\"us:retail:Quiet-Horde\"] = {\n[\"identity\"] = {\n[\"realmName\"] = \"Quiet\",\n},\n[\"population\"] = {\n[\"samples\"] = {\n{\n[\"t\"] = 1790000050,\n},\n},\n},\n},\n")
	b.WriteString("[\"us:retail:Busy-Alliance\"] = {\n[\"identity\"] = {\n[\"realmName\"] = \"Busy\",\n},\n[\"population\"] = {\n")
	b.WriteString("[\"lastScan\"] = " + map[bool]string{true: "1790000500", false: "1790000100"}[stage >= 2] + ",\n")
	b.WriteString("[\"census\"] = {\n[\"queue\"] = {\n\"a\",\n},\n},\n")
	b.WriteString("[\"samples\"] = {\n{\n[\"t\"] = 1790000100,\n},\n")
	if stage >= 2 {
		b.WriteString("{\n[\"t\"] = 1790000500,\n},\n")
	}
	b.WriteString("},\n[\"charactersPacked\"] = {\n\"MLC1\",\n" + luaQuote(recs) + ",\n},\n")
	b.WriteString("[\"sweeps\"] = {\n[\"1-1\"] = {\n[\"label\"] = \"census\",\n[\"queries\"] = {\n{\n[\"t\"] = 1790000100,\n},\n")
	if stage >= 2 {
		b.WriteString("{\n[\"t\"] = 1790000450,\n},\n")
	}
	b.WriteString("},\n[\"observations\"] = {\n[\"us:retail:realm:old\"] = {\n10,\n1790000100,\n},\n")
	if stage >= 2 {
		b.WriteString("[\"us:retail:realm:new\"] = {\n11,\n1790000450,\n},\n")
	}
	b.WriteString("},\n},\n},\n},\n},\n},\n")
	b.WriteString("[\"market\"] = {\n[\"game\"] = \"retail\",\n[\"tsm\"] = {\n[\"a\"] = {\n[\"t\"] = 1790000100,\n},\n")
	if stage >= 2 {
		b.WriteString("[\"b\"] = {\n[\"t\"] = 1790000500,\n},\n")
	}
	b.WriteString("},\n},\n}\n")
	b.WriteString("NameplateInspectDB = {\n[\"Player-1-01\"] = {\n[\"time\"] = 1790000100,\n},\n")
	if stage >= 2 {
		b.WriteString("[\"Player-1-02\"] = {\n[\"time\"] = 1790000500,\n},\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func TestCensusDeltaKeepsOnlyNewRows(t *testing.T) {
	first := []byte(sessionFixture(1))
	marks, err := CensusMarks(first)
	if err != nil {
		t.Fatal(err)
	}
	if marks.Buckets["us:retail:Busy-Alliance"] != 1790000100 || marks.Inspect != 1790000100 || marks.Market != 1790000100 {
		t.Fatalf("marks %+v", marks)
	}
	if d, err := BuildCensusDelta(first, marks); err != nil || d.Lua != nil {
		t.Fatalf("an unchanged save should have nothing new: %v %q", err, d.Lua)
	}

	d, err := BuildCensusDelta([]byte(sessionFixture(2)), marks)
	if err != nil {
		t.Fatal(err)
	}
	lua := string(d.Lua)
	for _, want := range []string{"us:retail:realm:new", "1790000450", "Player-1-02", "[\"b\"]", "realmName", "[\"lastScan\"] = 1790000500", "[\"debug\"] = false"} {
		if !strings.Contains(lua, want) {
			t.Errorf("delta is missing %q:\n%s", want, lua)
		}
	}
	for _, gone := range []string{"Quiet-Horde", "us:retail:realm:old", "Player-1-01", "[\"a\"]", "queue"} {
		if strings.Contains(lua, gone) {
			t.Errorf("delta should not repeat %q:\n%s", gone, lua)
		}
	}
	if d.Rows != 6 {
		t.Errorf("want 6 rows (sample, character, query, observation, tsm, inspect), got %d", d.Rows)
	}
	if d.Next.Buckets["us:retail:Busy-Alliance"] != 1790000500 || d.Next.Inspect != 1790000500 || d.Next.Market != 1790000500 {
		t.Errorf("cursor did not advance: %+v", d.Next)
	}
	if _, err := readCensusSave(d.Lua); err != nil {
		t.Fatal(err)
	}
	again, err := BuildCensusDelta([]byte(sessionFixture(2)), d.Next)
	if err != nil || again.Lua != nil {
		t.Fatalf("the same save twice should send nothing the second time: %v %q", err, again.Lua)
	}
}

func TestCensusDeltaSendsNewRealmsWhole(t *testing.T) {
	d, err := BuildCensusDelta([]byte(sessionFixture(2)), Cursor{Buckets: map[string]float64{"us:retail:Busy-Alliance": 1790000500}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(d.Lua), "Quiet-Horde") || strings.Contains(string(d.Lua), "us:retail:realm:new") {
		t.Fatalf("want only the realm the cursor has never seen:\n%s", d.Lua)
	}
}

func encodeForPrint(b []byte) string {
	var out strings.Builder
	var acc uint32
	bits := 0
	for _, c := range b {
		acc |= uint32(c) << bits
		bits += 8
		for bits >= 6 {
			out.WriteByte(printAlphabet[acc&63])
			acc >>= 6
			bits -= 6
		}
	}
	if bits > 0 {
		out.WriteByte(printAlphabet[acc&63])
	}
	return out.String()
}

func lsString(s string) []byte {
	if len(s) < 16 {
		return append([]byte{byte(len(s)<<4 | 2)}, s...)
	}
	return append([]byte{14 << 3, byte(len(s))}, s...)
}

func journalatorStore(t *testing.T, when uint32, buyer string) string {
	t.Helper()
	var b bytes.Buffer
	b.WriteByte(1)
	b.WriteByte(1<<4 | 1<<2 | 2)
	b.Write(lsString("Invoices"))
	b.WriteByte(1<<4 | 2<<2 | 2)
	b.WriteByte(5<<4 | 1<<2 | 2)
	b.Write(lsString("time"))
	b.WriteByte(5 << 3)
	_ = binary.Write(&b, binary.BigEndian, when)
	b.Write(lsString("playerName"))
	b.Write(lsString(buyer))
	b.Write(lsString("itemLink"))
	b.Write(lsString("|cff1eff00|Hitem:15268::Player-1-0ABC:22|h[Axe]|h|r"))
	b.Write(lsString("count"))
	b.Write([]byte{(300&15)<<4 | 4, 300 >> 4})
	b.Write(lsString("source"))
	b.WriteByte(1<<4 | 1<<2 | 2)
	b.Write(lsString("character"))
	b.Write(lsString("Me"))
	var z bytes.Buffer
	w, _ := flate.NewWriter(&z, flate.BestCompression)
	w.Write(b.Bytes())
	w.Close()
	return encodeForPrint(z.Bytes())
}

func TestJournalatorUploadIsScrubbedAndIncremental(t *testing.T) {
	save := func(stores ...string) []byte {
		var b strings.Builder
		b.WriteString("JOURNALATOR_ARCHIVE = {\n[\"stores\"] = {\n[\"SometimesLocked\"] = {\n")
		for i, s := range stores {
			b.WriteString("[\"Logs-" + strconv.Itoa(i) + "\"] = {\n[\"data\"] = \"" + s + "\",\n},\n")
		}
		b.WriteString("},\n},\n}\n")
		return []byte(b.String())
	}
	f := AddonFile{Addon: "journalator"}
	one := journalatorStore(t, 1790000100, "Buyerperson")
	d, err := BuildAddonUpload(f, save(one), Cursor{})
	if err != nil {
		t.Fatal(err)
	}
	lua := string(d.Lua)
	if d.Rows != 1 || !strings.Contains(lua, "JournalatorEntries") || !strings.Contains(lua, `"item:15268:::22"`) ||
		!strings.Contains(lua, "[\"count\"] = 300") || !strings.Contains(lua, `"Me"`) {
		t.Fatalf("unexpected upload (%d rows):\n%s", d.Rows, lua)
	}
	for _, gone := range []string{"Buyerperson", "playerName", "Player-1", "itemLink"} {
		if strings.Contains(lua, gone) {
			t.Fatalf("upload still contains %q:\n%s", gone, lua)
		}
	}
	if again, _ := BuildAddonUpload(f, save(one), d.Next); again.Lua != nil {
		t.Fatalf("unchanged archive uploaded again:\n%s", again.Lua)
	}
	two := journalatorStore(t, 1790000900, "Otherbuyer")
	d2, err := BuildAddonUpload(f, save(one, two), d.Next)
	if err != nil || d2.Rows != 1 || !strings.Contains(string(d2.Lua), "1790000900") {
		t.Fatalf("want only the new entry: %v\n%s", err, d2.Lua)
	}
}

func auctionatorBlob(days map[string]uint16) string {
	var b bytes.Buffer
	b.Write([]byte{0xA1, 0x43, '1', '2', '3', 0xA2, 0x41, 'm', 0x19, 0x01, 0xF4, 0x41, 'h', 0xA0 | byte(len(days))})
	for _, k := range []string{"2460", "2461", "2462"} {
		if v, ok := days[k]; ok {
			b.WriteByte(0x44)
			b.WriteString(k)
			b.WriteByte(0x19)
			_ = binary.Write(&b, binary.BigEndian, v)
		}
	}
	return b.String()
}

func TestAuctionatorPricesAreDecodedAndIncremental(t *testing.T) {
	save := func(blob string) []byte {
		return []byte("AUCTIONATOR_PRICE_DATABASE = {\n[\"__dbversion\"] = 9,\n[\"Stormrage\"] = " + luaQuote(blob) +
			",\n[\"Empty\"] = " + luaQuote("\xa1\x47version\x02") + ",\n}\nAUCTIONATOR_POSTING_HISTORY = {\n[\"__dbversion\"] = 1,\n[\"123\"] = {\n{\n[\"price\"] = 5,\n[\"time\"] = 1790000100,\n},\n},\n}\n")
	}
	f := AddonFile{Addon: "auctionator"}
	d, err := BuildAddonUpload(f, save(auctionatorBlob(map[string]uint16{"2460": 400, "2461": 500})), Cursor{})
	if err != nil {
		t.Fatal(err)
	}
	lua := string(d.Lua)
	if !strings.Contains(lua, "[2460] = 400") || !strings.Contains(lua, "[\"m\"] = 500") || !strings.Contains(lua, "AuctionatorPostings") || strings.Contains(lua, "Empty") {
		t.Fatalf("unexpected first upload:\n%s", lua)
	}
	d2, err := BuildAddonUpload(f, save(auctionatorBlob(map[string]uint16{"2460": 400, "2461": 500, "2462": 450})), d.Next)
	if err != nil {
		t.Fatal(err)
	}
	lua = string(d2.Lua)
	if strings.Contains(lua, "[2460]") || !strings.Contains(lua, "[2461] = 500") || !strings.Contains(lua, "[2462] = 450") || strings.Contains(lua, "AuctionatorPostings") {
		t.Fatalf("second upload should hold the last known day onward and no repeated postings:\n%s", lua)
	}
}

func TestTSMUploadDropsOtherPlayers(t *testing.T) {
	src := []byte("TradeSkillMasterDB = {\n[\"r@Stormrage@internalData@csvSales\"] = \"itemString,stackSize,quantity,price,otherPlayer,player,time,source\\ni:123,1,2,500,Buyerperson,Mychar,1790000100,Auction\",\n" +
		"[\"r@Stormrage@internalData@csvIncome\"] = \"type,amount,otherPlayer,player,time\\nMoney Transfer,100,Friend,Mychar,1790000100\",\n}\n")
	d, err := BuildAddonUpload(AddonFile{Addon: "tsm"}, src, Cursor{})
	if err != nil {
		t.Fatal(err)
	}
	lua := string(d.Lua)
	if d.Rows != 1 || strings.Contains(lua, "Buyerperson") || strings.Contains(lua, "Friend") || !strings.Contains(lua, "[\"itemID\"] = 123") || !strings.Contains(lua, `"Mychar"`) {
		t.Fatalf("unexpected TSM upload:\n%s", lua)
	}
}

func TestAddonSharingIsOptIn(t *testing.T) {
	root := t.TempDir()
	makeInstall(t, root, "_retail_", "A", censusLua(1))
	sv := filepath.Join(root, "World of Warcraft", "_retail_", "WTF", "Account", "A", "SavedVariables")
	writeOld(t, filepath.Join(sv, "TradeSkillMaster.lua"), "TradeSkillMasterDB = {\n[\"r@Stormrage@internalData@csvSales\"] = \"itemString,stackSize,quantity,price,otherPlayer,player,time,source\\ni:123,1,2,500,Buyer,Me,1790000100,Auction\",\n}\n")
	writeOld(t, filepath.Join(sv, "Auctionator.lua"), "AUCTIONATOR_POSTING_HISTORY = {\n}\n")
	fs := &fakeServer{token: "test-token"}
	hs := httptest.NewServer(fs.handler(t))
	defer hs.Close()
	srv := hs.URL
	e, err := NewEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e.discover = func([]string) []SavedVarFile { return Discover([]string{root}) }
	e.discoverEditions = func([]string) []string { return DiscoverEditions([]string{root}) }
	cfg := e.Config()
	cfg.ServerURL, cfg.AutoUpload = srv, true
	if err := e.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	e.RunOnce(ctx, false, false)
	if _, kinds, _ := fs.uploads(); len(kinds) != 1 || kinds[0] != KindCensus {
		t.Fatalf("other add-ons were uploaded without opting in: %v", kinds)
	}
	cfg = e.Config()
	cfg.ShareAddons = map[string]bool{"tsm": true, "auctionator": true, "bogus": true}
	if err := e.UpdateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if e.Config().ShareAddons["bogus"] {
		t.Fatal("unknown add-on kept in settings")
	}
	e.RunOnce(ctx, false, false)
	_, kinds, bodies := fs.uploads()
	if len(kinds) != 2 || kinds[1] != KindAddon {
		t.Fatalf("want the census upload plus one TSM upload (Auctionator has nothing), got %v", kinds)
	}
	m, lua := bundleLua(t, bodies[1])
	if m.Addon != "tsm" || m.FileName != "TradeSkillMaster.lua" || len(m.AccountID) != 64 || strings.Contains(lua, "Buyer") {
		t.Fatalf("bad add-on bundle %+v\n%s", m, lua)
	}
	st := e.Status()
	if len(st.AddonFiles) != 2 {
		t.Fatalf("want both add-on files in the status, got %+v", st.AddonFiles)
	}
	for _, f := range st.AddonFiles {
		if !f.Uploaded {
			t.Fatalf("%s should count as handled: %+v", f.Addon, f)
		}
	}
	e.RunOnce(ctx, false, false)
	if _, kinds, _ := fs.uploads(); len(kinds) != 2 {
		t.Fatalf("unchanged add-on files uploaded again: %v", kinds)
	}
}
