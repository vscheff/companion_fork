package companion

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type ExtraAddon struct {
	Key   string `json:"key"`
	File  string `json:"file"`
	Label string `json:"label"`
}

var ExtraAddons = []ExtraAddon{
	{Key: "auctionator", File: "Auctionator.lua", Label: "Auctionator"},
	{Key: "journalator", File: "Journalator.lua", Label: "Journalator"},
	{Key: "tsm", File: "TradeSkillMaster.lua", Label: "TradeSkillMaster"},
	{Key: "dungeonjournal", File: "ForeverDungeonJournal.lua", Label: "Dungeon Journal"},
	{Key: combatLogKey, File: "WoWCombatLog.txt", Label: "Combat logs"},
}

func extraAddon(key string) (ExtraAddon, bool) {
	for _, a := range ExtraAddons {
		if a.Key == key {
			return a, true
		}
	}
	return ExtraAddon{}, false
}

type AddonFile struct {
	SavedVarFile
	Addon     string `json:"addon"`
	Realm     string `json:"realm,omitempty"`
	Character string `json:"character,omitempty"`
}

func (f AddonFile) Label() string {
	a, _ := extraAddon(f.Addon)
	label := a.Label + " / " + editionLabel(f.Edition)
	if f.Character != "" {
		label += " / " + f.Character
	}
	if f.Addon == combatLogKey {
		label += " / " + filepath.Base(f.Path)
	}
	return label
}

func regularFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular() && fi.Size() > 0
}

func DiscoverAddonFiles(editionDirs []string, enabled map[string]bool) []AddonFile {
	var out []AddonFile
	seen := map[string]bool{}
	add := func(f AddonFile) {
		if k := pathKey(f.Path); !seen[k] {
			seen[k] = true
			out = append(out, f)
		}
	}
	for _, ed := range editionDirs {
		accountsDir := filepath.Join(ed, "WTF", "Account")
		accounts, err := os.ReadDir(accountsDir)
		if err != nil {
			continue
		}
		edition := editionName(ed)
		if enabled[combatLogKey] {
			for _, f := range discoverCombatLogs(ed, edition) {
				add(f)
			}
		}
		n := 0
		for _, acct := range accounts {
			if !isAccountDir(acct) {
				continue
			}
			n++
			acctDir := filepath.Join(accountsDir, acct.Name())
			base := SavedVarFile{Root: ed, Edition: edition, Account: acct.Name(), AccountNumber: n}
			for _, a := range ExtraAddons {
				if !enabled[a.Key] || a.Key == combatLogKey {
					continue
				}
				if p := filepath.Join(acctDir, "SavedVariables", a.File); regularFile(p) {
					f := AddonFile{SavedVarFile: base, Addon: a.Key}
					f.Path = p
					add(f)
				}
			}
			if !enabled["dungeonjournal"] {
				continue
			}
			realms, _ := os.ReadDir(acctDir)
			for _, realm := range realms {
				if !realm.IsDir() || strings.EqualFold(realm.Name(), "SavedVariables") {
					continue
				}
				chars, _ := os.ReadDir(filepath.Join(acctDir, realm.Name()))
				if len(chars) > 200 {
					chars = chars[:200]
				}
				for _, ch := range chars {
					if !ch.IsDir() {
						continue
					}
					p := filepath.Join(acctDir, realm.Name(), ch.Name(), "SavedVariables", "ForeverDungeonJournal.lua")
					if regularFile(p) {
						f := AddonFile{SavedVarFile: base, Addon: "dungeonjournal", Realm: realm.Name(), Character: ch.Name()}
						f.Path = p
						add(f)
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

var privateKeys = map[string]bool{
	"playerName": true, "player": true, "recipient": true, "sender": true, "subject": true, "text": true,
	"crafterName": true, "crafterNote": true, "customerNote": true, "otherPlayer": true,
	"members": true, "party": true, "group": true, "players": true, "roster": true, "buyer": true, "seller": true,
}

var (
	itemStringRe = regexp.MustCompile(`item:[^|]*`)
	playerGUIDRe = regexp.MustCompile(`Player-\d+-[0-9A-Fa-f]+`)
	trailingRe   = regexp.MustCompile(`:+$`)
)

func cleanItemLink(link string) (string, int64) {
	s := itemStringRe.FindString(link)
	if s == "" {
		return "", 0
	}
	s = trailingRe.ReplaceAllString(playerGUIDRe.ReplaceAllString(s, ""), "")
	id, _ := strconv.ParseInt(strings.SplitN(strings.TrimPrefix(s, "item:"), ":", 2)[0], 10, 64)
	return s, id
}

func scrub(v any, depth int) any {
	t, ok := v.(*table)
	if !ok {
		if s, isStr := v.(string); isStr && playerGUIDRe.MatchString(s) {
			return playerGUIDRe.ReplaceAllString(s, "")
		}
		return v
	}
	if depth > 16 {
		return nil
	}
	out := newTable()
	for _, k := range t.keys {
		x := t.vals[k]
		ks, isStr := k.(string)
		switch {
		case isStr && privateKeys[ks]:
		case isStr && ks == "guid":
		case isStr && strings.HasSuffix(ks, "Link"):
			if s, ok := x.(string); ok {
				prefix := strings.TrimSuffix(ks, "Link")
				if item, id := cleanItemLink(s); item != "" {
					out.Set(prefix+"String", item)
					out.Set(prefix+"ID", id)
				}
			}
		default:
			if y := scrub(x, depth+1); y != nil {
				out.Set(k, y)
			}
		}
	}
	return out
}

func readAssignments(src []byte) (map[string]*luaValue, error) {
	assigns, err := parseSavedVariables(src)
	if err != nil {
		return nil, err
	}
	out := map[string]*luaValue{}
	for _, a := range assigns {
		out[a.Name] = a.Value
	}
	return out, nil
}

func hashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:12])
}

func newCursor(since Cursor) Cursor {
	next := since.clone()
	if next.Marks == nil {
		next.Marks = map[string]float64{}
	}
	if next.Hashes == nil {
		next.Hashes = map[string]string{}
	}
	return next
}

const maxDecodedBytes = 256 << 20

func BuildAddonUpload(f AddonFile, src []byte, since Cursor) (Delta, error) {
	if f.Addon == combatLogKey {
		return combatLogUpload(src, since)
	}
	vars, err := readAssignments(src)
	if err != nil {
		return Delta{}, fmt.Errorf("the file could not be read: %w", err)
	}
	switch f.Addon {
	case "auctionator":
		return auctionatorUpload(src, vars, since)
	case "journalator":
		return journalatorUpload(src, vars, since)
	case "tsm":
		return tsmUpload(vars, since)
	case "dungeonjournal":
		return dungeonJournalUpload(src, vars, f, since)
	}
	return Delta{}, fmt.Errorf("unknown add-on %q", f.Addon)
}

func finish(b *strings.Builder, rows int, next Cursor) Delta {
	if rows == 0 {
		return Delta{Next: next}
	}
	return Delta{Lua: []byte(b.String()), Rows: rows, Next: next}
}

func dayKeyed(t *table) bool {
	if t.Len() == 0 {
		return false
	}
	for _, k := range t.keys {
		switch x := k.(type) {
		case int64:
		case string:
			if _, err := strconv.ParseInt(x, 10, 64); err != nil {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func dayNumber(k any) int64 {
	switch x := k.(type) {
	case int64:
		return x
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	return 0
}

func priceItemsSince(realm *table, fromDay int64) (*table, int64, int) {
	out := newTable()
	var newest int64
	rows := 0
	for _, item := range realm.keys {
		rec, ok := realm.vals[item].(*table)
		if !ok {
			continue
		}
		kept := newTable()
		days := 0
		for _, field := range rec.keys {
			x := rec.vals[field]
			sub, isTable := x.(*table)
			if !isTable || !dayKeyed(sub) {
				continue
			}
			ft := newTable()
			for _, d := range sub.keys {
				day := dayNumber(d)
				if day > newest {
					newest = day
				}
				if day >= fromDay {
					ft.Set(day, sub.vals[d])
				}
			}
			if ft.Len() > 0 {
				kept.Set(field, ft)
				days += ft.Len()
			}
		}
		if days == 0 && fromDay > 0 {
			continue
		}
		for _, field := range rec.keys {
			if _, isTable := rec.vals[field].(*table); !isTable {
				kept.Set(field, rec.vals[field])
			}
		}
		out.Set(item, kept)
		rows++
	}
	return out, newest, rows
}

func auctionatorUpload(src []byte, vars map[string]*luaValue, since Cursor) (Delta, error) {
	next := newCursor(since)
	var b strings.Builder
	rows := 0
	prices := newTable()
	if db := vars["AUCTIONATOR_PRICE_DATABASE"]; db != nil && db.Kind == luaTable {
		for _, e := range db.Entries {
			realmName := e.KeyString()
			if e.Key == nil || e.Key.Kind != luaString || strings.HasPrefix(realmName, "__") {
				continue
			}
			var realm *table
			var digest string
			switch e.Value.Kind {
			case luaString:
				digest = hashText(e.Value.Str)
				if next.Hashes["price:"+realmName] == digest {
					continue
				}
				v, err := decodeCBOR([]byte(e.Value.Str))
				if err != nil {
					return Delta{}, fmt.Errorf("Auctionator prices for %s: %w", realmName, err)
				}
				realm, _ = v.(*table)
			case luaTable:
				digest = hashText(string(src[e.Value.Start:e.Value.End]))
				if next.Hashes["price:"+realmName] == digest {
					continue
				}
				realm, _ = luaValueToGo(src, e.Value).(*table)
			}
			if realm == nil {
				continue
			}
			realm.Delete("version")
			fromDay := int64(since.Marks["price:"+realmName])
			items, newest, n := priceItemsSince(realm, fromDay)
			next.Hashes["price:"+realmName] = digest
			if float64(newest) > next.Marks["price:"+realmName] {
				next.Marks["price:"+realmName] = float64(newest)
			}
			if n > 0 {
				prices.Set(realmName, items)
				rows += n
			}
		}
	}
	if prices.Len() > 0 {
		luaAssign(&b, "AuctionatorPrices", prices)
	}
	posts := newTable()
	if hist := vars["AUCTIONATOR_POSTING_HISTORY"]; hist != nil && hist.Kind == luaTable {
		after := since.Marks["posting"]
		for _, e := range hist.Entries {
			key := e.KeyString()
			if e.Key == nil || strings.HasPrefix(key, "__") {
				continue
			}
			list, _ := luaValueToGo(src, e.Value).(*table)
			kept := newTable()
			for _, p := range list.Items() {
				pt, ok := p.(*table)
				if !ok {
					continue
				}
				t := pt.Num("time")
				if t > next.Marks["posting"] {
					next.Marks["posting"] = t
				}
				if t > after {
					kept.Append(pt)
				}
			}
			if kept.Len() > 0 {
				posts.Set(key, kept)
				rows += kept.Len()
			}
		}
	}
	if posts.Len() > 0 {
		luaAssign(&b, "AuctionatorPostings", posts)
	}
	return finish(&b, rows, next), nil
}

func journalatorStores(src []byte, vars map[string]*luaValue) (map[string]string, map[string]*luaValue) {
	encoded := map[string]string{}
	plain := map[string]*luaValue{}
	stores := vars["JOURNALATOR_ARCHIVE"].Field("stores")
	if stores == nil || stores.Kind != luaTable {
		return encoded, plain
	}
	for _, group := range stores.Entries {
		if group.Value == nil || group.Value.Kind != luaTable {
			continue
		}
		for _, st := range group.Value.Entries {
			id := group.KeyString() + "/" + st.KeyString()
			if data := st.Value.Field("data"); data != nil && data.Kind == luaString {
				encoded[id] = data.Str
			} else if st.Value != nil && st.Value.Kind == luaTable {
				plain[id] = st.Value
			}
		}
	}
	return encoded, plain
}

func decodeJournalatorStore(data string) (*table, error) {
	raw, err := decodeForPrint(data)
	if err != nil {
		return nil, err
	}
	inflated, err := inflateRaw(raw, maxDecodedBytes)
	if err != nil {
		return nil, err
	}
	objs, err := decodeLibSerialize(inflated)
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, errCodec
	}
	t, ok := objs[0].(*table)
	if !ok {
		return nil, errCodec
	}
	return t, nil
}

func journalatorUpload(src []byte, vars map[string]*luaValue, since Cursor) (Delta, error) {
	next := newCursor(since)
	after := since.Marks["time"]
	sections := newTable()
	rows := 0
	take := func(sec any, list *table) {
		name, ok := sec.(string)
		if !ok || list == nil {
			return
		}
		for _, item := range list.Items() {
			e, ok := item.(*table)
			if !ok {
				continue
			}
			t := e.Num("time")
			if t > next.Marks["time"] {
				next.Marks["time"] = t
			}
			if t <= after {
				continue
			}
			clean, _ := scrub(e, 0).(*table)
			if clean == nil {
				continue
			}
			dest := sections.Table(name)
			if dest == nil {
				dest = newTable()
				sections.Set(name, dest)
			}
			dest.Append(clean)
			rows++
		}
	}
	encoded, plain := journalatorStores(src, vars)
	ids := make([]string, 0, len(encoded))
	for id := range encoded {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		digest := hashText(encoded[id])
		if next.Hashes[id] == digest {
			continue
		}
		store, err := decodeJournalatorStore(encoded[id])
		if err != nil {
			return Delta{}, fmt.Errorf("Journalator archive %s: %w", id, err)
		}
		for _, sec := range store.keys {
			list, _ := store.vals[sec].(*table)
			take(sec, list)
		}
		next.Hashes[id] = digest
	}
	for _, v := range plain {
		if t, ok := luaValueToGo(src, v).(*table); ok {
			for _, sec := range t.keys {
				list, _ := t.vals[sec].(*table)
				take(sec, list)
			}
		}
	}
	if logs := vars["JOURNALATOR_LOGS"]; logs != nil && logs.Kind == luaTable {
		if t, ok := luaValueToGo(src, logs).(*table); ok {
			for _, sec := range t.keys {
				list, _ := t.vals[sec].(*table)
				take(sec, list)
			}
		}
	}
	var b strings.Builder
	if sections.Len() > 0 {
		luaAssign(&b, "JournalatorEntries", sections)
	}
	return finish(&b, rows, next), nil
}

var tsmKinds = map[string]string{"Sales": "sale", "Buys": "buy", "Expired": "expired", "Cancelled": "cancelled"}

var tsmKeyRe = regexp.MustCompile(`^r@(.+)@internalData@csv([A-Za-z]+)$`)

func tsmUpload(vars map[string]*luaValue, since Cursor) (Delta, error) {
	next := newCursor(since)
	db := vars["TradeSkillMasterDB"]
	if db == nil || db.Kind != luaTable {
		return Delta{Next: next}, nil
	}
	rowsOut := newTable()
	for _, e := range db.Entries {
		m := tsmKeyRe.FindStringSubmatch(e.KeyString())
		if m == nil || tsmKinds[m[2]] == "" || e.Value.Kind != luaString {
			continue
		}
		realm, kind := m[1], tsmKinds[m[2]]
		mark := "csv:" + realm + ":" + kind
		after := since.Marks[mark]
		lines := strings.Split(e.Value.Str, "\n")
		if len(lines) < 2 {
			continue
		}
		header := strings.Split(strings.TrimSpace(lines[0]), ",")
		for _, line := range lines[1:] {
			line = strings.TrimRight(line, "\r")
			if line == "" {
				continue
			}
			fields := strings.Split(line, ",")
			rec := map[string]string{}
			for i, name := range header {
				if i < len(fields) {
					rec[name] = fields[i]
				}
			}
			t, err := strconv.ParseFloat(rec["time"], 64)
			if err != nil || t <= 0 {
				continue
			}
			if t > next.Marks[mark] {
				next.Marks[mark] = t
			}
			if t <= after {
				continue
			}
			row := newTable()
			row.Set("kind", kind)
			row.Set("realm", realm)
			row.Set("t", t)
			if s := rec["itemString"]; s != "" {
				row.Set("item", s)
				if id, err := strconv.ParseInt(strings.SplitN(strings.TrimPrefix(s, "i:"), ":", 2)[0], 10, 64); err == nil && strings.HasPrefix(s, "i:") {
					row.Set("itemID", id)
				}
			}
			for src, dst := range map[string]string{"stackSize": "stack", "quantity": "qty", "price": "price"} {
				if n, err := strconv.ParseFloat(rec[src], 64); err == nil {
					row.Set(dst, n)
				}
			}
			if s := rec["player"]; s != "" {
				row.Set("char", s)
			}
			if s := rec["source"]; s != "" {
				row.Set("source", s)
			}
			rowsOut.Append(row)
		}
	}
	var b strings.Builder
	if rowsOut.Len() > 0 {
		luaAssign(&b, "TSMAccounting", rowsOut)
	}
	return finish(&b, rowsOut.Len(), next), nil
}

func dungeonJournalUpload(src []byte, vars map[string]*luaValue, f AddonFile, since Cursor) (Delta, error) {
	next := newCursor(since)
	name := "ForeverDungeonJournalDB"
	if f.Character != "" {
		name = "ForeverDungeonJournalCharacterDB"
	}
	v := vars[name]
	if v == nil || v.Kind != luaTable {
		return Delta{Next: next}, nil
	}
	digest := hashText(string(src[v.Start:v.End]))
	if next.Hashes["file"] == digest {
		return Delta{Next: next}, nil
	}
	next.Hashes["file"] = digest
	data, _ := scrub(luaValueToGo(src, v), 0).(*table)
	if data == nil {
		return Delta{Next: next}, nil
	}
	out := newTable()
	if f.Character != "" {
		out.Set("realm", f.Realm)
		out.Set("character", f.Character)
		out.Set("characterData", data)
	} else {
		out.Set("account", data)
	}
	var b strings.Builder
	luaAssign(&b, "DungeonJournal", out)
	return finish(&b, 1, next), nil
}
