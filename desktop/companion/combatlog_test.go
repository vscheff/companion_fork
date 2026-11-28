package companion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleCombatLog = `11/21/2026 10:39:37.968-6  COMBAT_LOG_VERSION,22,ADVANCED_LOG_ENABLED,1,BUILD_VERSION,1.60.1,PROJECT_ID,2
11/21/2026 10:39:38.001-6  MAP_CHANGE,1429,"Elwynn Forest",-10,0,-9,0
11/21/2026 10:39:40.100-6  SWING_DAMAGE,Creature-0-6823-0-24349-474-000181C935,"Defias Rogue Wizard",0xa48,0x0,Player-6931-02B38E72,"Benedict-Spellclicker",0x511,0x0,Creature-0-6823-0-24349-474-000181C935,0000000000000000,120,120,0,0,0,35,0,0,0,0,0,0,-9000,100,1429,0,7,9,9,-1,1,0,0,0,nil,nil,nil
11/21/2026 10:39:41.200-6  SPELL_DAMAGE,Player-6931-02B38E72,"Benedict-Spellclicker",0x511,0x0,Creature-0-6823-0-24349-474-000181C935,"Defias Rogue Wizard",0xa48,0x0,585,"Smite",0x2,Creature-0-6823-0-24349-474-000181C935,0000000000000000,80,120,0,0,0,35,0,0,0,0,0,0,-9000,100,1429,0,7,40,40,-1,2,0,0,0,nil,nil,nil
11/21/2026 10:39:42.300-6  SPELL_CAST_SUCCESS,Player-6931-02B38E72,"Benedict-Spellclicker",0x511,0x0,Player-6931-0A11AA11,"Other-Player",0x512,0x0,17,"Power Word: Shield",0x2,Player-6931-02B38E72,0000000000000000,100,100,0,0,0,0,0,0,0,0,0,0,0,0,1429,0,7
11/21/2026 10:39:43.400-6  SPELL_DAMAGE,Pet-0-6823-0-24349-416-0100ABCDEF,"Krakzazt",0x1111,0x0,Creature-0-6823-0-24349-474-000181C935,"Defias Rogue Wizard",0xa48,0x0,3110,"Firebolt",0x4,Pet-0-6823-0-24349-416-0100ABCDEF,Player-6931-0A11AA11,50,50,0,0,0,0,0,0,0,0,0,0,0,0,1429,0,5,12,12,-1,4,0,0,0,nil,nil,nil
11/21/2026 10:39:44.500-6  COMBATANT_INFO,Player-6931-02B38E72,0,1,2,3
11/21/2026 10:39:45.600-6  EMOTE,Creature-0-6823-0-24349-474-000181C935,"Defias Rogue Wizard",0000000000000000,nil,"Defias Rogue Wizard attacks Benedict!"
11/21/2026 10:39:46.700-6  UNIT_DIED,0000000000000000,nil,0x80000000,0x80000000,Creature-0-6823-0-24349-474-000181C935,"Defias Rogue Wizard",0xa48,0x0,0
`

func TestFilterCombatLogDropsPlayers(t *testing.T) {
	out, events, err := FilterCombatLog(strings.NewReader(sampleCombatLog))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, leak := range []string{"Benedict", "Spellclicker", "Other-Player", "Krakzazt", "02B38E72", "0A11AA11", "COMBATANT_INFO", "EMOTE", "Power Word: Shield"} {
		if strings.Contains(got, leak) {
			t.Errorf("filtered log still contains %q:\n%s", leak, got)
		}
	}
	for _, keep := range []string{"COMBAT_LOG_VERSION,22,ADVANCED_LOG_ENABLED,1", `MAP_CHANGE,1429,"Elwynn Forest"`, `"Defias Rogue Wizard"`, `"Smite"`, "UNIT_DIED", "Player-0-00000005,\"\",0x511"} {
		if !strings.Contains(got, keep) {
			t.Errorf("filtered log lost %q:\n%s", keep, got)
		}
	}
	if events != 4 {
		t.Errorf("events = %d, want 4", events)
	}
}

func TestFilterCombatLogNeedsAdvancedLogging(t *testing.T) {
	plain := strings.Replace(sampleCombatLog, "ADVANCED_LOG_ENABLED,1", "ADVANCED_LOG_ENABLED,0", 1)
	out, events, err := FilterCombatLog(strings.NewReader(plain))
	if err != nil || out != nil || events != 0 {
		t.Fatalf("got %d bytes, %d events, %v; want nothing", len(out), events, err)
	}
}

func TestCombatLogDiscoveryAndUpload(t *testing.T) {
	root := filepath.Join(t.TempDir(), "World of Warcraft")
	forever := filepath.Join(root, "_forever_")
	retail := filepath.Join(root, "_retail_")
	for _, ed := range []string{forever, retail} {
		if err := os.MkdirAll(filepath.Join(ed, "WTF", "Account", "ACCOUNT1"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(ed, "Logs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ed, "Logs", "WoWCombatLog-112126_103937.txt"), []byte(sampleCombatLog), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(forever, "Logs", "gx.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DiscoverAddonFiles([]string{forever, retail}, map[string]bool{"journalator": true}); len(got) != 0 {
		t.Fatalf("combat logs found without the setting: %v", got)
	}
	files := DiscoverAddonFiles([]string{forever, retail}, map[string]bool{combatLogKey: true})
	if len(files) != 2 || files[0].Edition != "forever" || files[1].Edition != "retail" || filepath.Base(files[0].Path) != "WoWCombatLog-112126_103937.txt" {
		t.Fatalf("files = %+v", files)
	}
	if _, err := TakeCombatLogSnapshot(files[0].SavedVarFile, time.Now()); err != ErrUnstable {
		t.Fatalf("fresh log: err = %v, want ErrUnstable", err)
	}
	snap, err := TakeCombatLogSnapshot(files[0].SavedVarFile, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(snap.Data), "Benedict") {
		t.Fatal("snapshot holds a player name")
	}
	d, err := BuildAddonUpload(files[0], snap.Data, Cursor{})
	if err != nil || d.Lua == nil || d.Rows == 0 {
		t.Fatalf("first upload: %d bytes, %d rows, %v", len(d.Lua), d.Rows, err)
	}
	again, err := BuildAddonUpload(files[0], snap.Data, d.Next)
	if err != nil || again.Lua != nil {
		t.Fatalf("unchanged log uploaded again: %d bytes, %v", len(again.Lua), err)
	}
}
