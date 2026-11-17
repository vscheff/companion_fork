package companion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHideAccountFolders(t *testing.T) {
	cases := map[string]string{
		`C:\Games\World of Warcraft\_retail_\WTF\Account\123456789#1\SavedVariables\WoWCensus.lua`: `C:\Games\World of Warcraft\_retail_\WTF\Account\…\SavedVariables\WoWCensus.lua`,
		"/home/a/wow/_classic_/WTF/Account/MYNAME/SavedVariables/WoWCensus.lua":                    "/home/a/wow/_classic_/WTF/Account/…/SavedVariables/WoWCensus.lua",
		`open C:\wow\_retail_\wtf\account\987#2\SavedVariables\x.lua: access denied`:               `open C:\wow\_retail_\wtf\account\…\SavedVariables\x.lua: access denied`,
		"nothing to hide": "nothing to hide",
	}
	for in, want := range cases {
		if got := HideAccountFolders(in); got != want {
			t.Errorf("HideAccountFolders(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAccountsAreNumberedNotNamed(t *testing.T) {
	root := t.TempDir()
	ed := filepath.Join(root, "_retail_")
	for _, acct := range []string{"111111111#1", "111111111#2", "SavedVariables"} {
		dir := filepath.Join(ed, "WTF", "Account", acct, "SavedVariables")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if acct != "111111111#1" {
			if err := os.WriteFile(filepath.Join(dir, SavedVariablesName), []byte("WoWCensusDB = {}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	files := savedVarsInEdition(ed)
	if len(files) != 1 {
		t.Fatalf("found %d files, want 1", len(files))
	}
	if files[0].AccountNumber != 2 || files[0].AccountLabel() != "Account 2" {
		t.Fatalf("got number %d label %q", files[0].AccountNumber, files[0].AccountLabel())
	}
	pub := PublicFiles(files)[0]
	if strings.Contains(pub.Path, "111111111") {
		t.Fatalf("public path still has the account folder: %s", pub.Path)
	}
	if files[0].Path == pub.Path {
		t.Fatal("Public changed the original file")
	}
}
