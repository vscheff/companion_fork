package companion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	classHeader  = "11/22/2026 18:04:11.212+4  COMBAT_LOG_VERSION,22,ADVANCED_LOG_ENABLED,1,BUILD_VERSION,1.60.1,PROJECT_ID,18\n"
	classStamp   = "11/22/2026 18:04:12.000+4  "
	classNPC     = `Creature-0-1111-0-2414-618-0000AB1234,"Kobold Vermin",0xa48,0x0`
	classPlayerA = "Player-9001-0AA10001"
	classPlayerB = "Player-9001-0AA10002"
)

func classActor(guid, name string) string {
	return guid + `,"` + name + `",0x511,0x0`
}

func classCast(source, spell string) string {
	return classStamp + "SPELL_CAST_START," + source + "," + classNPC + "," + spell + "\n"
}

func classBuff(source, target, spell string) string {
	return classStamp + "SPELL_AURA_APPLIED," + source + "," + target + "," + spell + ",BUFF\n"
}

func filteredClassLog(t *testing.T, log string, census combatCensus) string {
	t.Helper()
	out, _, err := filterCombatLog(strings.NewReader(classHeader+log), census)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(string(out), "\x01\x02") {
		t.Fatalf("placeholder left in output:\n%q", out)
	}
	return string(out)
}

func TestCombatLogTagsClassFromCasts(t *testing.T) {
	a := classActor(classPlayerA, "Alpha-Gib-Normal-US")
	b := classActor(classPlayerB, "Beta-Gib-Normal-US")
	log := classCast(a, `1,"Unknown Strike",0x1`) +
		classBuff(a, b, `133,"Fireball",0x4`) +
		classCast(b, `585,"Smite",0x2`) +
		classBuff(b, b, `133,"Fireball",0x4`) +
		classStamp + "SPELL_CAST_SUCCESS," + b + "," + classNPC + `,591,"Smite",0x2,` + classNPC[:37] + ",0000000000000000,310,450,12,4,77,0,0,0,-1,0,0,0,-9071.25,100.75,1429,1.25,7\n"
	got := filteredClassLog(t, log, nil)
	if !strings.Contains(got, `SPELL_CAST_START,Player-0-00000008,"",0x511`) {
		t.Errorf("mage evidence on a dropped buff line should tag the earlier cast:\n%s", got)
	}
	if strings.Contains(got, "Player-0-00000005") || !strings.Contains(got, `SPELL_CAST_SUCCESS,Player-0-00000000,"",0x511`) {
		t.Errorf("player with priest and mage evidence should stay untagged:\n%s", got)
	}
}

func TestCombatLogTagsClassFromCensus(t *testing.T) {
	census := combatCensus{}
	census.addPacked("39:us:forever:camwowus2realm5pve:alpha gib"+strings.Repeat("0:", 9)+"7:PALADIN"+strings.Repeat("0:", 5), "")
	census.add("us:forever:camwowus2realm5pve:beta gib", "WARRIOR", "9002")
	census.add("us:forever:camwowus1realm3pve:beta gib", "ROGUE", "9003")
	census.add("us:retail:aegwynn:gamma", "HUNTER", "")
	census.add("us:forever:camwowus2realm5pve:delta gib", "PRIEST", "")
	census.add("us:forever:camwowus2realm5pve:eps gib", "WARLOCK", "9002")
	census.add("us:forever:camwowus1realm1pve:eps gib", "MAGE", "9001")
	cases := []struct{ name, spell, want string }{
		{"Alpha-Gib-Normal-US", `1,"Unknown Strike",0x1`, "Player-0-00000002"},
		{"Beta-Gib-Normal-US", `1,"Unknown Strike",0x1`, "Player-0-00000000"},
		{"Gamma-Aegwynn-US", `1,"Unknown Strike",0x1`, "Player-0-00000003"},
		{"Gamma-Proudmoore-US", `1,"Unknown Strike",0x1`, "Player-0-00000000"},
		{"Delta-Gib-Normal-US", `585,"Smite",0x2`, "Player-0-00000005"},
		{"Delta-Gib-Normal-US", `133,"Fireball",0x4`, "Player-0-00000000"},
		{"Eps-Gib-Normal-US", `1,"Unknown Strike",0x1`, "Player-0-00000008"},
	}
	for _, tc := range cases {
		t.Run(tc.name+tc.spell, func(t *testing.T) {
			got := filteredClassLog(t, classCast(classActor(classPlayerA, tc.name), tc.spell), census)
			if !strings.Contains(got, "SPELL_CAST_START,"+tc.want+`,"",0x511`) {
				t.Errorf("want %s:\n%s", tc.want, got)
			}
		})
	}
}

func TestLoadCombatCensusReadsBothLayouts(t *testing.T) {
	ed := filepath.Join(t.TempDir(), "_forever_")
	dir := filepath.Join(ed, "WTF", "Account", "ACC", "SavedVariables")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	packed := "39:us:forever:camwowus2realm5pve:alpha gib" + strings.Repeat("0:", 9) + "7:PALADIN" + strings.Repeat("0:", 5)
	src := `WoWCensusDB = {
["realms"] = {
["us:forever:CAMWOW US2 Realm 5 PvE-Alliance"] = {
["identity"] = { ["realmID"] = 6931 },
["population"] = {
["charactersPacked"] = { "MLC1", ` + luaQuote(packed) + ` },
["characters"] = {
["us:forever:camwowus2realm5pve:beta gib"] = { ["classFile"] = "DRUID" },
},
},
},
},
}
`
	if err := os.WriteFile(filepath.Join(dir, SavedVariablesName), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	census := loadCombatCensus(ed)
	if got := census.lookup("Alpha-Gib-Normal-US", ""); got != 2 {
		t.Errorf("packed paladin = %d", got)
	}
	if got := census.lookup("Beta-Gib-Normal-US", "6931"); got != 11 {
		t.Errorf("unpacked druid = %d", got)
	}
}
