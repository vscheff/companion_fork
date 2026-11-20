package companion

import (
	"fmt"
	"math"
)

var censusMarketSources = [...]struct {
	field   string
	setting string
}{
	{"journalator", "journalator"},
	{"tsm", "tsm"},
	{"auctionator", "auctionator"},
	{"dungeonJournal", "dungeonjournal"},
}

type marketConsent uint8

func censusMarketConsent(shares map[string]bool) marketConsent {
	var consent marketConsent
	for i, source := range censusMarketSources {
		if shares[source.setting] {
			consent |= 1 << i
		}
	}
	return consent
}

func coreCensusField(key string) bool {
	switch key {
	case "realms", "settings", "minimap", "leveling", "populationStorageVersion":
		return true
	default:
		return false
	}
}

func buildCensusFull(src []byte, shares map[string]bool) (Delta, error) {
	s, err := readCensusSave(src)
	if err != nil {
		return Delta{}, err
	}
	next := s.marks()
	o := luaOut{src: src}
	o.b.WriteString(cleanupDBName + " = {\n")
	for _, e := range s.db.Entries {
		if coreCensusField(e.KeyString()) {
			o.entry(e)
		}
	}
	o.market(s.db.Field("market"), shares, &next)
	o.b.WriteString("}\n")
	if s.inspects != nil && s.inspects.Kind == luaTable {
		o.b.WriteString(inspectDBName + " = ")
		o.raw(s.inspects)
		o.b.WriteByte('\n')
	}
	out := []byte(o.b.String())
	if _, err := readCensusSave(out); err != nil {
		return Delta{}, fmt.Errorf("the full export could not be read back: %w", err)
	}
	return Delta{Lua: out, Next: next}, nil
}

func (o *luaOut) market(m *luaValue, shares map[string]bool, next *Cursor) int {
	if m == nil || m.Kind != luaTable {
		return 0
	}
	part := luaOut{src: o.src}
	rows := 0
	for _, source := range censusMarketSources {
		v := m.Field(source.field)
		if !shares[source.setting] || v == nil || v.Kind != luaTable {
			continue
		}
		prev := next.MarketBySource[source.field]
		latest := prev
		keep := func(t float64) bool {
			if !(t > prev) || math.IsInf(t, 0) {
				return false
			}
			latest = max(latest, t)
			return true
		}
		byTime := func(row *luaValue) bool { return keep(row.Field("t").Number()) }
		n := part.marketSource(source.field, v, byTime, keep)
		if n > 0 {
			if next.MarketBySource == nil {
				next.MarketBySource = map[string]float64{}
			}
			next.MarketBySource[source.field] = latest
			rows += n
		}
	}
	if rows == 0 {
		return 0
	}
	o.key("market")
	o.b.WriteString("{\n")
	for _, e := range m.Entries {
		switch e.KeyString() {
		case "game", "region":
			if e.Value.Kind == luaString {
				o.entry(e)
			}
		case "version":
			if e.Value.Kind == luaNumber {
				o.entry(e)
			}
		}
	}
	o.b.WriteString(part.b.String())
	o.b.WriteString("},\n")
	return rows
}

func (o *luaOut) marketSource(name string, v *luaValue, byTime func(*luaValue) bool, keep func(float64) bool) int {
	part := luaOut{src: o.src}
	rows := 0
	switch name {
	case "tsm", "auctionator":
		return o.filteredMap(name, v, byTime)
	case "journalator":
		for _, e := range v.Entries {
			if e.Key != nil && e.Key.Kind == luaString {
				rows += part.filteredMap(e.Key.Str, e.Value, byTime)
			}
		}
	case "dungeonJournal":
		account, at := v.Field("account"), v.Field("accountAt")
		if account != nil && account.Kind == luaTable && keep(at.Number()) {
			part.key("account")
			part.raw(account)
			part.b.WriteString(",\n")
			part.key("accountAt")
			part.raw(at)
			part.b.WriteString(",\n")
			rows++
		}
		rows += part.filteredMap("characters", v.Field("characters"), byTime)
	}
	if rows > 0 {
		o.key(name)
		o.b.WriteString("{\n")
		o.b.WriteString(part.b.String())
		o.b.WriteString("},\n")
	}
	return rows
}
