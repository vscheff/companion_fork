package companion

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func auctionLogFixture(rows ...string) string {
	return fmt.Sprintf(`
WoWCensusDB = {
	realms = {
		Realm = {
			identity = {realmName = "census-realm"},
			population = {samples = {{t = %d, count = 12}}},
		},
	},
	auctionLog = {
		["classic:Player-4395-0ABC1234"] = {
			identity = {game = "classic", region = "us", realmName = "Whitemane"},
			char = "Sellchar",
			realm = "Whitemane",
			faction = "Horde",
			unknownTable = {data = "auction-unknown-table"},
			rows = {
				%s
			},
		},
		["classic:Player-4395-0DEF5678"] = {
			char = "Idlechar",
			rows = {},
		},
	},
}
`, consentCoreTime, strings.Join(rows, "\n\t\t\t\t"))
}

const (
	auctionPost = `["post|1790000200|Linen Cloth|20|5000|nil"] = {kind = "post", t = 1790000200, name = "Linen Cloth", itemID = 2589, count = 20, buyout = 5000, runTime = 2},`
	auctionSold = `["sold|1790000900|Linen Cloth|nil|nil|nil"] = {kind = "sold", t = 1790000900, name = "Linen Cloth"},`
	auctionMail = `["mail|1790001000|Linen Cloth|20|nil|nil"] = {kind = "mail", t = 1790001000, invoiceType = "seller", name = "Linen Cloth", count = 20, bid = 5000, daysLeft = 29.5},`
)

func TestCensusFullExportIncludesAuctionLog(t *testing.T) {
	src := auctionLogFixture(auctionPost, auctionSold)
	d, err := buildCensusFull([]byte(src), nil)
	if err != nil {
		t.Fatal(err)
	}
	lua := string(d.Lua)
	for _, want := range []string{"auctionLog", "classic:Player-4395-0ABC1234", "Sellchar", "Whitemane", "Horde", `realmName = "Whitemane"`,
		"Linen Cloth", "itemID = 2589", `kind = "sold"`} {
		if !strings.Contains(lua, want) {
			t.Fatalf("full export is missing %q:\n%s", want, lua)
		}
	}
	for _, unwanted := range []string{"auction-unknown-table", "Idlechar"} {
		if strings.Contains(lua, unwanted) {
			t.Fatalf("full export should not contain %q:\n%s", unwanted, lua)
		}
	}
	if d.Next.AuctionLog != 1790000900 {
		t.Fatalf("want the auction cursor at the newest row, got %v", d.Next.AuctionLog)
	}
}

func TestCensusSessionExportSendsOnlyNewAuctionRows(t *testing.T) {
	first, err := buildCensusFull([]byte(auctionLogFixture(auctionPost)), nil)
	if err != nil {
		t.Fatal(err)
	}
	same, err := BuildCensusDelta([]byte(auctionLogFixture(auctionPost)), first.Next, nil)
	if err != nil {
		t.Fatal(err)
	}
	if same.Rows != 0 || len(same.Lua) != 0 {
		t.Fatalf("an unchanged auction log should not upload, got %d rows:\n%s", same.Rows, same.Lua)
	}
	d, err := BuildCensusDelta([]byte(auctionLogFixture(auctionPost, auctionSold, auctionMail)), first.Next, nil)
	if err != nil {
		t.Fatal(err)
	}
	lua := string(d.Lua)
	if d.Rows != 2 || !strings.Contains(lua, `kind = "sold"`) || !strings.Contains(lua, `kind = "mail"`) || strings.Contains(lua, `kind = "post"`) {
		t.Fatalf("want only the sold and mail rows, got %d rows:\n%s", d.Rows, lua)
	}
	if !strings.Contains(lua, "Sellchar") || !strings.Contains(lua, `realmName = "Whitemane"`) {
		t.Fatalf("session rows lost their character and realm:\n%s", lua)
	}
	if d.Next.AuctionLog != 1790001000 {
		t.Fatalf("want the auction cursor at the newest row, got %v", d.Next.AuctionLog)
	}
}

func TestEngineUploadsAuctionOnlySession(t *testing.T) {
	src := auctionLogFixture(auctionPost)
	e, fs, path := newCensusConsentEngine(t, src)
	ctx := context.Background()
	e.RunOnce(ctx, false, false)
	_, lua := verifyCensusBundle(t, fs, 1, src, ModeFull, 0)
	if !strings.Contains(lua, `kind = "post"`) {
		t.Fatalf("the first upload is missing the auction log:\n%s", lua)
	}
	changed := auctionLogFixture(auctionPost, auctionSold)
	writeOld(t, path, changed)
	e.RunOnce(ctx, false, false)
	_, lua = verifyCensusBundle(t, fs, 2, changed, ModeSession, 1)
	if !strings.Contains(lua, `kind = "sold"`) || strings.Contains(lua, `kind = "post"`) {
		t.Fatalf("the session upload should carry only the new sale:\n%s", lua)
	}
}
