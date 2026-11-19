package companion

import (
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
)

const inspectDBName = "NameplateInspectDB"

type Cursor struct {
	Buckets map[string]float64 `json:"buckets,omitempty"`
	Inspect float64            `json:"inspect,omitempty"`
	// The legacy "market" watermark is ignored because it proved no per-source coverage.
	MarketBySource map[string]float64 `json:"marketBySource,omitempty"`
	Marks          map[string]float64 `json:"marks,omitempty"`
	Hashes         map[string]string  `json:"hashes,omitempty"`
	Census         *censusCheckpoint  `json:"census,omitempty"`
}

func (c Cursor) clone() Cursor {
	c.Buckets = maps.Clone(c.Buckets)
	c.MarketBySource = maps.Clone(c.MarketBySource)
	c.Marks = maps.Clone(c.Marks)
	c.Hashes = maps.Clone(c.Hashes)
	if c.Census != nil {
		checkpoint := *c.Census
		c.Census = &checkpoint
	}
	return c
}

var stampKeys = map[string]bool{
	"t": true, "time": true, "lastScan": true, "lastSeen": true, "lastSellerSample": true,
	"startedAt": true, "completedAt": true, "finishedAt": true,
}

type censusSave struct {
	src      []byte
	db       *luaValue
	inspects *luaValue
}

func readCensusSave(src []byte) (*censusSave, error) {
	assigns, err := parseSavedVariables(src)
	if err != nil {
		return nil, err
	}
	s := &censusSave{src: src}
	for _, a := range assigns {
		switch a.Name {
		case cleanupDBName:
			s.db = a.Value
		case inspectDBName:
			s.inspects = a.Value
		}
	}
	if s.db == nil || s.db.Kind != luaTable {
		return nil, ErrNotWoWCensus
	}
	return s, nil
}

func newestStamp(v *luaValue) float64 {
	var best float64
	var walk func(*luaValue, int)
	walk = func(v *luaValue, depth int) {
		if v == nil || v.Kind != luaTable || depth > 12 {
			return
		}
		for _, e := range v.Entries {
			if e.Value == nil {
				continue
			}
			if e.Value.Kind == luaNumber && e.Key != nil && e.Key.Kind == luaString && stampKeys[e.Key.Str] {
				if e.Value.Num > best {
					best = e.Value.Num
				}
			} else if e.Value.Kind == luaTable {
				walk(e.Value, depth+1)
			}
		}
	}
	walk(v, 0)
	return best
}

func packedNewest(packed *luaValue) float64 {
	chunks, ok := packed.Array()
	if !ok || len(chunks) == 0 || chunks[0].Value.String() != packedFormat {
		return 0
	}
	var best float64
	for _, ch := range chunks[1:] {
		eachPacked(ch.Value.String(), func(fields []string) bool {
			if last, _ := strconv.ParseFloat(fields[packedLastSeenField], 64); last > best {
				best = last
			}
			return true
		})
	}
	return best
}

func eachPacked(chunk string, fn func(fields []string) bool) error {
	cursor := 0
	for cursor < len(chunk) {
		fields := make([]string, packedFieldCount)
		for i := range fields {
			v, next, err := readPackedField(chunk, cursor)
			if err != nil {
				return err
			}
			fields[i], cursor = v, next
		}
		if !fn(fields) {
			return nil
		}
	}
	return nil
}

func bucketNewest(bucket *luaValue) float64 {
	best := newestStamp(bucket)
	if p := packedNewest(bucket.Field("population").Field("charactersPacked")); p > best {
		best = p
	}
	return best
}

func entryTime(v *luaValue, names ...string) float64 {
	for _, n := range names {
		if t := v.Field(n).Number(); t > 0 {
			return t
		}
	}
	return 0
}

func (s *censusSave) marks() Cursor {
	c := Cursor{Buckets: map[string]float64{}}
	if realms := s.db.Field("realms"); realms != nil && realms.Kind == luaTable {
		for _, re := range realms.Entries {
			if re.Value != nil && re.Value.Kind == luaTable {
				c.Buckets[re.KeyString()] = bucketNewest(re.Value)
			}
		}
	}
	if s.inspects != nil && s.inspects.Kind == luaTable {
		for _, e := range s.inspects.Entries {
			if t := e.Value.Field("time").Number(); t > c.Inspect {
				c.Inspect = t
			}
		}
	}
	return c
}

type luaOut struct {
	src []byte
	b   strings.Builder
}

func (o *luaOut) raw(v *luaValue) { o.b.Write(o.src[v.Start:v.End]) }

func (o *luaOut) entry(e *luaEntry) {
	o.b.Write(o.src[e.Start:e.Value.End])
	o.b.WriteString(",\n")
}

func (o *luaOut) key(k string) {
	o.b.WriteByte('[')
	o.b.WriteString(luaQuote(k))
	o.b.WriteString("] = ")
}

func (o *luaOut) keyed(e *luaEntry) {
	if e.Key == nil {
		return
	}
	if e.Key.Kind == luaString {
		o.key(e.Key.Str)
		return
	}
	o.b.WriteByte('[')
	o.raw(e.Key)
	o.b.WriteString("] = ")
}

func isScalar(v *luaValue) bool { return v != nil && v.Kind != luaTable }

func (o *luaOut) scalars(v *luaValue) {
	for _, e := range v.Entries {
		if e.Key != nil && isScalar(e.Value) {
			o.entry(e)
		}
	}
}

func (o *luaOut) filteredArray(name string, v *luaValue, keep func(*luaValue) bool) int {
	items, ok := v.Array()
	if !ok {
		return 0
	}
	n := 0
	for _, e := range items {
		if !keep(e.Value) {
			continue
		}
		if n == 0 {
			o.key(name)
			o.b.WriteString("{\n")
		}
		o.raw(e.Value)
		o.b.WriteString(",\n")
		n++
	}
	if n > 0 {
		o.b.WriteString("},\n")
	}
	return n
}

func (o *luaOut) filteredMap(name string, v *luaValue, keep func(*luaValue) bool) int {
	if v == nil || v.Kind != luaTable {
		return 0
	}
	n := 0
	for _, e := range v.Entries {
		if e.Key == nil || !keep(e.Value) {
			continue
		}
		if n == 0 {
			o.key(name)
			o.b.WriteString("{\n")
		}
		o.entry(e)
		n++
	}
	if n > 0 {
		o.b.WriteString("},\n")
	}
	return n
}

func dayLastSeen(d *luaValue) float64 {
	if t := d.Field("lastSeen").Number(); t > 0 {
		return t
	}
	if items, ok := d.Array(); ok && len(items) >= 3 {
		return items[2].Value.Number()
	}
	return 0
}

func (o *luaOut) characters(chars *luaValue, since float64) int {
	if chars == nil || chars.Kind != luaTable {
		return 0
	}
	n := 0
	for _, e := range chars.Entries {
		ch := e.Value
		if e.Key == nil || ch == nil || ch.Kind != luaTable || ch.Field("lastSeen").Number() <= since {
			continue
		}
		if n == 0 {
			o.key("characters")
			o.b.WriteString("{\n")
		}
		o.keyed(e)
		o.b.WriteString("{\n")
		for _, f := range ch.Entries {
			if f.KeyString() != "days" {
				o.entry(f)
			}
		}
		o.filteredMap("days", ch.Field("days"), func(d *luaValue) bool { return dayLastSeen(d) > since })
		o.b.WriteString("},\n")
		n++
	}
	if n > 0 {
		o.b.WriteString("},\n")
	}
	return n
}

func packedDelta(packed *luaValue, since float64) ([]string, int, error) {
	chunks, ok := packed.Array()
	if !ok || len(chunks) == 0 || chunks[0].Value.String() != packedFormat {
		return nil, 0, nil
	}
	var out []string
	var cur strings.Builder
	count := 0
	for _, ch := range chunks[1:] {
		if ch.Value.Kind != luaString {
			return nil, 0, errors.New("packed characters are not text")
		}
		err := eachPacked(ch.Value.Str, func(fields []string) bool {
			last, _ := strconv.ParseFloat(fields[packedLastSeenField], 64)
			if last <= since {
				return true
			}
			var days []string
			for _, tok := range strings.Split(fields[packedDaysField], ";") {
				parts := strings.Split(tok, ",")
				if len(parts) != 4 {
					continue
				}
				if dl, _ := strconv.ParseFloat(parts[3], 64); dl > since {
					days = append(days, tok)
				}
			}
			fields[packedDaysField] = strings.Join(days, ";")
			for _, f := range fields {
				cur.WriteString(strconv.Itoa(len(f)))
				cur.WriteByte(':')
				cur.WriteString(f)
			}
			count++
			if count%256 == 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			return true
		})
		if err != nil {
			return nil, 0, err
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out, count, nil
}

func observationTime(v *luaValue) float64 {
	if t := v.Field("observedAt").Number(); t > 0 {
		return t
	}
	if items, ok := v.Array(); ok && len(items) >= 2 {
		return items[1].Value.Number()
	}
	return 0
}

func (o *luaOut) sweeps(sweeps *luaValue, since float64) int {
	if sweeps == nil || sweeps.Kind != luaTable {
		return 0
	}
	rows, n := 0, 0
	for _, e := range sweeps.Entries {
		sw := e.Value
		if e.Key == nil || sw == nil || sw.Kind != luaTable {
			continue
		}
		var part luaOut
		part.src = o.src
		part.scalars(sw)
		q := part.filteredArray("queries", sw.Field("queries"), func(v *luaValue) bool { return v.Field("t").Number() > since })
		ob := part.filteredMap("observations", sw.Field("observations"), func(v *luaValue) bool { return observationTime(v) > since })
		if q+ob == 0 {
			continue
		}
		if n == 0 {
			o.key("sweeps")
			o.b.WriteString("{\n")
		}
		o.keyed(e)
		o.b.WriteString("{\n")
		o.b.WriteString(part.b.String())
		o.b.WriteString("},\n")
		rows += q + ob
		n++
	}
	if n > 0 {
		o.b.WriteString("},\n")
	}
	return rows
}

func (o *luaOut) population(pop *luaValue, since float64) (int, error) {
	o.key("population")
	o.b.WriteString("{\n")
	o.scalars(pop)
	rows := o.filteredArray("samples", pop.Field("samples"), func(v *luaValue) bool { return v.Field("t").Number() > since })
	rows += o.filteredArray("specSamples", pop.Field("specSamples"), func(v *luaValue) bool { return entryTime(v, "t", "time") > since })
	rows += o.filteredMap("inspectBuilds", pop.Field("inspectBuilds"), func(v *luaValue) bool { return v.Field("time").Number() > since })
	rows += o.characters(pop.Field("characters"), since)
	packed, packedRows, err := packedDelta(pop.Field("charactersPacked"), since)
	if err != nil {
		return 0, err
	}
	if len(packed) > 0 {
		o.key("charactersPacked")
		o.b.WriteString("{\n\"MLC1\",\n")
		for _, c := range packed {
			o.b.WriteString(luaQuote(c))
			o.b.WriteString(",\n")
		}
		o.b.WriteString("},\n")
		rows += packedRows
	}
	rows += o.sweeps(pop.Field("sweeps"), since)
	o.b.WriteString("},\n")
	return rows, nil
}

func (o *luaOut) bucket(bucket *luaValue, since float64) (int, error) {
	o.b.WriteString("{\n")
	rows := 0
	for _, e := range bucket.Entries {
		switch {
		case e.KeyString() == "identity":
			o.entry(e)
		case e.KeyString() == "population" && e.Value.Kind == luaTable:
			n, err := o.population(e.Value, since)
			if err != nil {
				return 0, err
			}
			rows += n
		case e.Key != nil && isScalar(e.Value):
			o.entry(e)
		}
	}
	o.b.WriteString("}")
	return rows, nil
}

type Delta struct {
	Lua  []byte
	Rows int
	Next Cursor
}

func BuildCensusDelta(src []byte, since Cursor, shares map[string]bool) (Delta, error) {
	s, err := readCensusSave(src)
	if err != nil {
		return Delta{}, err
	}
	now := s.marks()
	next := since.clone()
	if next.Buckets == nil {
		next.Buckets = map[string]float64{}
	}
	for k, v := range now.Buckets {
		next.Buckets[k] = max(next.Buckets[k], v)
	}
	next.Inspect = max(next.Inspect, now.Inspect)

	o := &luaOut{src: src}
	o.b.WriteString(cleanupDBName + " = {\n")
	for _, e := range s.db.Entries {
		if k := e.KeyString(); k != "realms" && coreCensusField(k) {
			o.entry(e)
		}
	}
	rows := 0
	o.key("realms")
	o.b.WriteString("{\n")
	if realms := s.db.Field("realms"); realms != nil && realms.Kind == luaTable {
		for _, re := range realms.Entries {
			k := re.KeyString()
			prev := since.Buckets[k]
			if re.Value == nil || re.Value.Kind != luaTable || now.Buckets[k] <= prev {
				continue
			}
			var part luaOut
			part.src = src
			n, err := part.bucket(re.Value, prev)
			if err != nil {
				return Delta{}, err
			}
			if n == 0 {
				continue
			}
			o.keyed(re)
			o.b.WriteString(part.b.String())
			o.b.WriteString(",\n")
			rows += n
		}
	}
	o.b.WriteString("},\n")
	rows += o.market(s.db.Field("market"), shares, &next)
	o.b.WriteString("}\n")
	if s.inspects != nil && s.inspects.Kind == luaTable {
		o.b.WriteString(inspectDBName + " = {\n")
		for _, e := range s.inspects.Entries {
			if e.Key != nil && e.Value.Field("time").Number() > since.Inspect {
				o.entry(e)
				rows++
			}
		}
		o.b.WriteString("}\n")
	}
	if rows == 0 {
		return Delta{Next: next}, nil
	}
	out := []byte(o.b.String())
	if _, err := readCensusSave(out); err != nil {
		return Delta{}, fmt.Errorf("the session file could not be read back: %w", err)
	}
	return Delta{Lua: out, Rows: rows, Next: next}, nil
}
