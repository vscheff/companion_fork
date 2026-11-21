package companion

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	combatLogKey      = "combatlog"
	combatLogEdition  = "forever"
	combatLogQuiet    = 10 * time.Minute
	maxCombatLogBytes = 2 << 30
)

var (
	combatLogNameRe   = regexp.MustCompile(`(?i)^WoWCombatLog.*\.txt$`)
	combatPlayerRe    = regexp.MustCompile(`Player-\d+-[0-9A-Fa-f]+`)
	combatEventPrefix = []string{"SWING_", "SPELL_", "RANGE_", "DAMAGE_", "UNIT_DIED", "UNIT_DESTROYED", "PARTY_KILL"}
)

const anonymousPlayer = "Player-0-00000000"

func discoverCombatLogs(edDir, edition string) []AddonFile {
	if edition != combatLogEdition {
		return nil
	}
	dir := filepath.Join(edDir, "Logs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []AddonFile
	for _, e := range entries {
		if e.IsDir() || !combatLogNameRe.MatchString(e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if !regularFile(p) {
			continue
		}
		f := AddonFile{SavedVarFile: SavedVarFile{Root: edDir, Edition: edition}, Addon: combatLogKey}
		f.Path = p
		out = append(out, f)
	}
	return out
}

func splitCombatFields(s string) []string {
	var out []string
	start, quoted := 0, false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

func combatEventKept(ev string) bool {
	for _, p := range combatEventPrefix {
		if strings.HasPrefix(ev, p) {
			return true
		}
	}
	return false
}

func playerOwned(guid string) bool {
	return strings.HasPrefix(guid, "Player-") || strings.HasPrefix(guid, "Pet-")
}

func filterCombatLine(line string) (string, bool) {
	stamp, rest, ok := strings.Cut(line, "  ")
	if !ok {
		return "", false
	}
	f := splitCombatFields(rest)
	switch f[0] {
	case "COMBAT_LOG_VERSION", "MAP_CHANGE":
		return line, true
	}
	if len(f) < 9 || !combatEventKept(f[0]) {
		return "", false
	}
	if !strings.HasPrefix(f[1], "Creature-") && !strings.HasPrefix(f[5], "Creature-") {
		return "", false
	}
	for _, i := range []int{1, 5} {
		if playerOwned(f[i]) {
			f[i+1] = `""`
		}
	}
	for i := range f {
		if strings.Contains(f[i], "Player-") {
			f[i] = combatPlayerRe.ReplaceAllString(f[i], anonymousPlayer)
		}
	}
	return stamp + "  " + strings.Join(f, ","), true
}

func FilterCombatLog(r io.Reader) ([]byte, int, error) {
	br := bufio.NewReaderSize(r, 1<<20)
	var out bytes.Buffer
	advanced, events := false, 0
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			if kept, ok := filterCombatLine(line); ok {
				if strings.Contains(kept, "  COMBAT_LOG_VERSION,") {
					f := splitCombatFields(strings.SplitN(kept, "  ", 2)[1])
					if len(f) > 3 && f[2] == "ADVANCED_LOG_ENABLED" && f[3] == "1" {
						advanced = true
					}
				} else if !strings.Contains(kept, "  MAP_CHANGE,") {
					events++
				}
				out.WriteString(kept)
				out.WriteByte('\n')
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}
	}
	if !advanced || events == 0 {
		return nil, 0, nil
	}
	return out.Bytes(), events, nil
}

func TakeCombatLogSnapshot(f SavedVarFile, now time.Time) (*Snapshot, error) {
	before, err := statSig(f.Path)
	if err != nil {
		return nil, err
	}
	if before.Size == 0 || now.Sub(before.ModTime) < combatLogQuiet {
		return nil, ErrUnstable
	}
	if before.Size > maxCombatLogBytes {
		return nil, fmt.Errorf("%s is %d MB, above the %d MB limit for combat logs", filepath.Base(f.Path), before.Size>>20, maxCombatLogBytes>>20)
	}
	fh, err := os.Open(f.Path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	data, _, err := FilterCombatLog(io.LimitReader(fh, before.Size))
	if err != nil {
		return nil, err
	}
	after, err := statSig(f.Path)
	if err != nil {
		return nil, err
	}
	if !after.Equal(before) {
		return nil, ErrUnstable
	}
	sum := sha256.Sum256(data)
	return &Snapshot{File: f, Data: data, SHA256: hex.EncodeToString(sum[:]), Sig: after, TakenAt: now.UTC()}, nil
}

func combatLogUpload(src []byte, since Cursor) (Delta, error) {
	next := newCursor(since)
	if len(src) == 0 {
		return Delta{Next: next}, nil
	}
	digest := hashText(string(src))
	if next.Hashes["file"] == digest {
		return Delta{Next: next}, nil
	}
	next.Hashes["file"] = digest
	events := bytes.Count(src, []byte("\n"))
	return Delta{Lua: src, Rows: events, Next: next}, nil
}
