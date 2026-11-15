package companion

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	cleanupDBName        = "WoWCensusDB"
	defaultCleanKeepDays = 14
	minCleanKeepDays     = 7
	maxCleanKeepDays     = 90
	defaultCleanAboveMB  = 2
	minCleanupSaving     = 64 << 10
	cleanupRecheckEvery  = 6 * time.Hour
	packedFormat         = "MLC1"
	packedLastSeenField  = 13
	packedDaysField      = 15
	packedFieldCount     = 16
)

var ErrNotWoWCensus = errors.New("the file has no WoWCensusDB table")

type CleanupResult struct {
	BytesBefore int64 `json:"bytesBefore"`
	BytesAfter  int64 `json:"bytesAfter"`
	KeepDays    int   `json:"keepDays"`
	Realms      int   `json:"realms"`
	Samples     int   `json:"samples"`
	Sweeps      int   `json:"sweeps"`
	SweepRows   int   `json:"sweepRows"`
	Characters  int   `json:"characters"`
	Days        int   `json:"days"`
}

func (r CleanupResult) Removed() bool {
	return r.Samples+r.Sweeps+r.SweepRows+r.Characters+r.Days > 0
}

func (r CleanupResult) Summary() string {
	var parts []string
	add := func(n int, one, many string) {
		if n == 1 {
			parts = append(parts, "1 "+one)
		} else if n > 1 {
			parts = append(parts, fmt.Sprintf("%d %s", n, many))
		}
	}
	add(r.Characters, "character", "characters")
	add(r.Sweeps, "old census sweep", "old census sweeps")
	add(r.SweepRows, "finished-sweep row", "finished-sweep rows")
	add(r.Samples, "/who sample", "/who samples")
	add(r.Days, "daily sighting", "daily sightings")
	if len(parts) == 0 {
		return "nothing"
	}
	return strings.Join(parts, ", ")
}

func CleanSavedVariables(src []byte, now time.Time, keepDays int) ([]byte, CleanupResult, error) {
	res := CleanupResult{BytesBefore: int64(len(src)), BytesAfter: int64(len(src)), KeepDays: keepDays}
	if keepDays < minCleanKeepDays {
		return src, res, fmt.Errorf("keep at least %d days", minCleanKeepDays)
	}
	assigns, err := parseSavedVariables(src)
	if err != nil {
		return src, res, fmt.Errorf("the file could not be read: %w", err)
	}
	var db *luaValue
	for _, a := range assigns {
		if a.Name == cleanupDBName {
			db = a.Value
		}
	}
	if db == nil || db.Kind != luaTable {
		return src, res, ErrNotWoWCensus
	}
	cutoff := float64(now.Unix() - int64(keepDays)*86400)
	cutoffDay := math.Floor(cutoff / 86400)
	c := &cleaner{src: src, cutoff: cutoff, cutoffDay: cutoffDay, res: &res}
	realms := db.Field("realms")
	if realms != nil && realms.Kind == luaTable {
		for _, re := range realms.Entries {
			pop := re.Value.Field("population")
			if pop == nil || pop.Kind != luaTable {
				continue
			}
			before := res
			c.samples(pop)
			c.sweeps(pop)
			c.characters(pop.Field("characters"))
			if err := c.packed(pop.Field("charactersPacked")); err != nil {
				return src, CleanupResult{BytesBefore: res.BytesBefore, BytesAfter: res.BytesBefore, KeepDays: keepDays},
					fmt.Errorf("%s: %w", re.KeyString(), err)
			}
			if res != before {
				res.Realms++
			}
		}
	}
	if len(c.edits) == 0 {
		return src, res, nil
	}
	out, err := applyLuaEdits(src, c.edits)
	if err != nil {
		return src, CleanupResult{BytesBefore: res.BytesBefore, BytesAfter: res.BytesBefore, KeepDays: keepDays}, err
	}
	if err := verifyCleaned(src, out, db); err != nil {
		return src, CleanupResult{BytesBefore: res.BytesBefore, BytesAfter: res.BytesBefore, KeepDays: keepDays}, err
	}
	res.BytesAfter = int64(len(out))
	return out, res, nil
}

type cleaner struct {
	src       []byte
	cutoff    float64
	cutoffDay float64
	res       *CleanupResult
	edits     []luaEdit
}

func (c *cleaner) remove(e *luaEntry) { c.edits = append(c.edits, entryRemoval(c.src, e)) }

func (c *cleaner) replace(v *luaValue, text string) {
	c.edits = append(c.edits, luaEdit{Start: v.Start, End: v.End, Text: text})
}

func (c *cleaner) removeItems(items []*luaEntry, drop []bool) {
	next := 0
	for i, e := range items {
		if drop[i] {
			c.remove(e)
			continue
		}
		next++
		if e.Key != nil && int(e.Key.Num) != next {
			c.replace(e.Key, strconv.Itoa(next))
		}
	}
}

func (c *cleaner) samples(pop *luaValue) {
	items, ok := pop.Field("samples").Array()
	if !ok || len(items) < 2 {
		return
	}
	drop := make([]bool, len(items))
	dropped := false
	for i, e := range items[:len(items)-1] {
		t := e.Value.Field("t").Number()
		if t > 0 && t < c.cutoff {
			drop[i], dropped = true, true
			c.res.Samples++
		}
	}
	if dropped {
		c.removeItems(items, drop)
	}
}

func (c *cleaner) sweeps(pop *luaValue) {
	sweeps := pop.Field("sweeps")
	if sweeps == nil || sweeps.Kind != luaTable {
		return
	}
	keep := map[string]bool{}
	if id := pop.Field("activeSweepID").String(); id != "" {
		keep[id] = true
	}
	if id := pop.Field("census").Field("sweepID").String(); id != "" {
		keep[id] = true
	}
	newestCensus, newestAt := "", -1.0
	for _, e := range sweeps.Entries {
		label := e.Value.Field("label").String()
		at := e.Value.Field("startedAt").Number()
		if strings.HasPrefix(label, "census ") && at > newestAt {
			newestCensus, newestAt = e.KeyString(), at
		}
	}
	for _, e := range sweeps.Entries {
		sw := e.Value
		if e.Key == nil || e.Key.Kind != luaString || sw == nil || sw.Kind != luaTable ||
			keep[e.KeyString()] || sw.Field("status").String() == "active" {
			continue
		}
		ended := sw.Field("completedAt").Number()
		if ended <= 0 {
			ended = sw.Field("startedAt").Number()
		}
		if ended > 0 && ended < c.cutoff && e.KeyString() != newestCensus {
			c.remove(e)
			c.res.Sweeps++
			continue
		}
		obs := sw.Field("observations")
		if obs != nil && obs.Kind == luaTable && len(obs.Entries) > 0 {
			c.replace(obs, "{\n}")
			c.res.SweepRows += len(obs.Entries)
		}
	}
}

func (c *cleaner) characters(chars *luaValue) {
	if chars == nil || chars.Kind != luaTable {
		return
	}
	for _, e := range chars.Entries {
		ch := e.Value
		if e.Key == nil || e.Key.Kind != luaString || ch == nil || ch.Kind != luaTable {
			continue
		}
		last := ch.Field("lastSeen").Number()
		if last > 0 && last < c.cutoff {
			c.remove(e)
			c.res.Characters++
			continue
		}
		days := ch.Field("days")
		if days == nil || days.Kind != luaTable {
			continue
		}
		for _, d := range days.Entries {
			if d.Key == nil || d.Key.Kind != luaNumber {
				continue
			}
			if day := d.Key.Num; day > 0 && day < c.cutoffDay {
				c.remove(d)
				c.res.Days++
			}
		}
	}
}

func (c *cleaner) packed(packed *luaValue) error {
	chunks, ok := packed.Array()
	if !ok || len(chunks) == 0 {
		return nil
	}
	if chunks[0].Value.Kind != luaString || chunks[0].Value.Str != packedFormat {
		return nil
	}
	for _, ch := range chunks[1:] {
		if ch.Value.Kind != luaString {
			return errors.New("packed characters are not text")
		}
		kept, changed, err := cleanPackedChunk(ch.Value.Str, c.cutoff, c.cutoffDay, c.res)
		if err != nil {
			return err
		}
		if changed {
			c.replace(ch.Value, luaQuote(kept))
		}
	}
	return nil
}

func readPackedField(chunk string, cursor int) (string, int, error) {
	colon := strings.IndexByte(chunk[cursor:], ':')
	if colon < 0 {
		return "", 0, errors.New("packed characters are truncated")
	}
	size, err := strconv.Atoi(chunk[cursor : cursor+colon])
	start := cursor + colon + 1
	if err != nil || size < 0 || start+size > len(chunk) {
		return "", 0, errors.New("packed characters are damaged")
	}
	return chunk[start : start+size], start + size, nil
}

func cleanPackedChunk(chunk string, cutoff, cutoffDay float64, res *CleanupResult) (string, bool, error) {
	var out strings.Builder
	changed := false
	cursor := 0
	for cursor < len(chunk) {
		recStart := cursor
		fields := make([]string, packedFieldCount)
		for i := range fields {
			v, next, err := readPackedField(chunk, cursor)
			if err != nil {
				return "", false, err
			}
			fields[i], cursor = v, next
		}
		last, _ := strconv.ParseFloat(fields[packedLastSeenField], 64)
		if last > 0 && last < cutoff {
			res.Characters++
			changed = true
			continue
		}
		var days []string
		dropped := 0
		for _, tok := range strings.Split(fields[packedDaysField], ";") {
			if tok == "" {
				continue
			}
			parts := strings.SplitN(tok, ",", 2)
			day, err := strconv.ParseFloat(parts[0], 64)
			if err != nil {
				return "", false, errors.New("packed sighting days are damaged")
			}
			if day > 0 && day < cutoffDay {
				dropped++
				continue
			}
			days = append(days, tok)
		}
		if dropped == 0 {
			out.WriteString(chunk[recStart:cursor])
			continue
		}
		res.Days += dropped
		changed = true
		fields[packedDaysField] = strings.Join(days, ";")
		for _, f := range fields {
			out.WriteString(strconv.Itoa(len(f)))
			out.WriteByte(':')
			out.WriteString(f)
		}
	}
	return out.String(), changed, nil
}

func verifyCleaned(before, after []byte, oldDB *luaValue) error {
	assigns, err := parseSavedVariables(after)
	if err != nil {
		return fmt.Errorf("cleanup produced an unreadable file: %w", err)
	}
	var db *luaValue
	for _, a := range assigns {
		if a.Name == cleanupDBName {
			db = a.Value
		}
	}
	if db == nil || db.Kind != luaTable {
		return errors.New("cleanup lost the WoWCensusDB table")
	}
	if len(db.Entries) != len(oldDB.Entries) {
		return errors.New("cleanup changed the top level of the file")
	}
	for i, e := range oldDB.Entries {
		k := e.KeyString()
		if db.Entries[i].KeyString() != k {
			return errors.New("cleanup changed the top level of the file")
		}
		if k == "realms" {
			continue
		}
		if string(before[e.Value.Start:e.Value.End]) != string(after[db.Entries[i].Value.Start:db.Entries[i].Value.End]) {
			return fmt.Errorf("cleanup changed %q, which it must keep", k)
		}
	}
	oldRealms, newRealms := oldDB.Field("realms"), db.Field("realms")
	if oldRealms != nil {
		if newRealms == nil || len(newRealms.Entries) != len(oldRealms.Entries) {
			return errors.New("cleanup lost a realm")
		}
		for i, re := range oldRealms.Entries {
			nr := newRealms.Entries[i]
			if nr.KeyString() != re.KeyString() {
				return errors.New("cleanup lost a realm")
			}
			for _, e := range re.Value.Entries {
				k := e.KeyString()
				if k == "population" {
					continue
				}
				nv := nr.Value.Field(k)
				if nv == nil || string(before[e.Value.Start:e.Value.End]) != string(after[nv.Start:nv.End]) {
					return fmt.Errorf("cleanup changed %s of a realm, which it must keep", k)
				}
			}
			op, np := re.Value.Field("population"), nr.Value.Field("population")
			if op == nil {
				continue
			}
			if np == nil {
				return errors.New("cleanup lost a realm's population")
			}
			for _, e := range op.Entries {
				k := e.KeyString()
				switch k {
				case "samples", "sweeps", "characters", "charactersPacked":
					continue
				}
				nv := np.Field(k)
				if nv == nil || string(before[e.Value.Start:e.Value.End]) != string(after[nv.Start:nv.End]) {
					return fmt.Errorf("cleanup changed population.%s, which it must keep", k)
				}
			}
		}
	}
	return nil
}
