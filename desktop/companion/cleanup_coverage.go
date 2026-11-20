package companion

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"slices"
	"strings"
)

type censusRecordCoverage struct {
	Complete bool   `json:"complete,omitempty"`
	Hashes   string `json:"hashes,omitempty"`
}

type cleanupRecordHash [sha256.Size]byte
type cleanupRecords map[cleanupRecordHash]int

const maxCleanupRecords = 128 * 1024

func censusRecordsForUpload(src, payload []byte, full, track bool, prior censusRecordCoverage) censusRecordCoverage {
	proof := censusRecordCoverage{Complete: full}
	if !track {
		return proof
	}
	current := readCleanupRecords(src)
	if current == nil {
		return proof
	}
	if !full {
		previous := prior.records()
		sent := readCleanupRecords(payload)
		proof.Complete = true
		for record, count := range current {
			covered := max(previous[record], sent[record])
			if covered < count {
				proof.Complete = false
				current[record] = covered
			}
		}
	}
	hashes := make([]cleanupRecordHash, 0, len(current))
	for record, count := range current {
		for range count {
			hashes = append(hashes, record)
		}
	}
	slices.SortFunc(hashes, func(a, b cleanupRecordHash) int { return slices.Compare(a[:], b[:]) })
	data := make([]byte, 0, len(hashes)*sha256.Size)
	for _, record := range hashes {
		data = append(data, record[:]...)
	}
	proof.Hashes = base64.StdEncoding.EncodeToString(data)
	return proof
}

func (p censusRecordCoverage) records() cleanupRecords {
	if len(p.Hashes) > base64.StdEncoding.EncodedLen(maxCleanupRecords*sha256.Size) {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(p.Hashes)
	if err != nil || len(data)%sha256.Size != 0 {
		return nil
	}
	records := make(cleanupRecords)
	for len(data) > 0 {
		records[cleanupRecordHash(data[:sha256.Size])]++
		data = data[sha256.Size:]
	}
	return records
}

func (p censusRecordCoverage) coversCleanup(before, after []byte) bool {
	if p.Complete {
		return true
	}
	original, remaining := readCleanupRecords(before), readCleanupRecords(after)
	if original == nil || remaining == nil {
		return false
	}
	confirmed := p.records()
	for record, count := range original {
		if remaining[record] < count && confirmed[record] < count {
			return false
		}
	}
	return true
}

func readCleanupRecords(src []byte) cleanupRecords {
	if len(src) == 0 {
		return nil
	}
	save, err := readCensusSave(src)
	if err != nil {
		return nil
	}
	records := make(cleanupRecords)
	count := 0
	add := func(parts ...string) {
		count++
		if count > maxCleanupRecords {
			return
		}
		var data []byte
		for _, part := range parts {
			data = binary.LittleEndian.AppendUint64(data, uint64(len(part)))
			data = append(data, part...)
		}
		records[sha256.Sum256(data)]++
	}
	value := func(v *luaValue) string {
		return string(src[v.Start:v.End])
	}
	key := func(e *luaEntry) string {
		switch {
		case e.Key == nil:
			return ""
		case e.Key.Kind == luaString:
			return luaQuote(e.Key.Str)
		default:
			return value(e.Key)
		}
	}
	realms := save.db.Field("realms")
	if realms == nil || realms.Kind != luaTable {
		return records
	}
	for _, realm := range realms.Entries {
		pop := realm.Value.Field("population")
		id := key(realm)
		for _, field := range []string{"samples", "sweeps", "characters"} {
			table := pop.Field(field)
			if table == nil || table.Kind != luaTable {
				continue
			}
			rows := table.Entries
			if field == "samples" {
				var ok bool
				rows, ok = table.Array()
				if !ok {
					continue
				}
			}
			for _, row := range rows {
				if count >= maxCleanupRecords {
					return nil
				}
				if field == "samples" {
					add(id, field, value(row.Value))
					continue
				}
				rowKey := key(row)
				if row.Value.Kind != luaTable {
					add(id, field, rowKey, value(row.Value))
					continue
				}
				metadata := []string{id, field, rowKey, "metadata"}
				for _, part := range row.Value.Entries {
					name := part.KeyString()
					if part.Value.Kind != luaTable || !(field == "characters" && name == "days" ||
						field == "sweeps" && (name == "observations" || name == "queries")) {
						metadata = append(metadata, key(part), value(part.Value))
						continue
					}
					children := part.Value.Entries
					if name == "queries" {
						var ok bool
						children, ok = part.Value.Array()
						if !ok {
							metadata = append(metadata, key(part), value(part.Value))
							continue
						}
					}
					for _, child := range children {
						childKey := key(child)
						if name == "queries" {
							childKey = ""
						}
						add(id, field, rowKey, name, childKey, value(child.Value))
					}
				}
				add(metadata...)
			}
		}
		if packed, ok := pop.Field("charactersPacked").Array(); ok && len(packed) > 0 && packed[0].Value.String() == packedFormat {
			for _, chunk := range packed[1:] {
				if chunk.Value.Kind != luaString {
					return nil
				}
				if err := eachPacked(chunk.Value.Str, func(fields []string) bool {
					add(append([]string{id, "packed"}, fields[:packedDaysField]...)...)
					for day := range strings.SplitSeq(fields[packedDaysField], ";") {
						if day != "" {
							add(id, "packedDay", fields[0], day)
						}
					}
					return count <= maxCleanupRecords
				}); err != nil {
					return nil
				}
			}
		}
		if count > maxCleanupRecords {
			return nil
		}
	}
	return records
}
