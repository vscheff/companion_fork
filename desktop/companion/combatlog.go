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
	combatLogQuiet    = 10 * time.Minute
	maxCombatLogBytes = 2 << 30
	maxCombatFields   = 43
)

var combatLogNameRe = regexp.MustCompile(`(?i)^WoWCombatLog.*\.txt$`)

func discoverCombatLogs(edDir, edition string) []AddonFile {
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

func FilterCombatLog(r io.Reader) ([]byte, int, error) {
	return filterCombatLog(r, nil)
}

func filterCombatLog(r io.Reader, census combatCensus) ([]byte, int, error) {
	export, err := exportCombatLog(r, census)
	return export.data, export.events, err
}

type combatLogExport struct {
	data     []byte
	events   int
	complete bool
}

func exportCombatLog(r io.Reader, census combatCensus) (combatLogExport, error) {
	players := newCombatPlayers(census)
	br := bufio.NewReaderSize(r, 1<<20)
	var out bytes.Buffer
	var version, unsupported string
	events := 0
	complete := true
	for readErr := error(nil); readErr == nil; {
		var line string
		line, readErr = br.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return combatLogExport{}, readErr
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if strings.TrimSpace(strings.TrimPrefix(line, "\ufeff")) == "" {
			continue
		}
		stamp, payload, ok := strings.Cut(line, "  ")
		if !ok {
			complete = false
			continue
		}
		event, _, _ := strings.Cut(payload, ",")
		if event == "COMBAT_LOG_VERSION" {
			version = ""
		} else if version == "" {
			complete = false
			continue
		}
		stamp = strings.TrimPrefix(stamp, "\ufeff")
		var fields [maxCombatFields]string
		f := splitCombatFields(payload, fields[:0])
		result := combatRecordInvalid
		if f != nil && combatTimestampRe.MatchString(stamp) {
			if version != "" {
				players.observe(f)
			}
			result = filterCombatRecord(f, version, players)
		}
		switch result {
		case combatRecordIgnored:
			continue
		case combatRecordInvalid:
			complete = false
			if event == "COMBAT_LOG_VERSION" && len(f) > 1 && combatUint(f[1]) && combatAdvancedWidths(f[1]) == nil {
				unsupported = f[1]
			}
			continue
		}
		switch event {
		case "COMBAT_LOG_VERSION":
			version = f[1]
		case "MAP_CHANGE":
		default:
			events++
		}
		out.WriteString(stamp)
		out.WriteString("  ")
		out.WriteString(f[0])
		for _, field := range f[1:] {
			out.WriteByte(',')
			out.WriteString(field)
		}
		out.WriteByte('\n')
	}
	if events == 0 {
		if unsupported != "" {
			return combatLogExport{}, fmt.Errorf("combat log format %s is not supported yet; update the companion", unsupported)
		}
		return combatLogExport{}, nil
	}
	return combatLogExport{data: players.expand(out.Bytes()), events: events, complete: complete}, nil
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
	r := &io.LimitedReader{R: fh, N: before.Size}
	sourceHash := sha256.New()
	export, err := exportCombatLog(io.TeeReader(r, sourceHash), loadCombatCensus(f.Root))
	if err != nil {
		return nil, err
	}
	after, err := statSig(f.Path)
	if err != nil {
		return nil, err
	}
	if r.N != 0 || !after.Equal(before) {
		return nil, ErrUnstable
	}
	sum := sha256.Sum256(export.data)
	snap := &Snapshot{File: f, Data: export.data, SHA256: hex.EncodeToString(sum[:]), Sig: after, TakenAt: now.UTC()}
	if export.complete {
		snap.combatLogSourceSHA256 = hex.EncodeToString(sourceHash.Sum(nil))
	}
	return snap, nil
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
