package companion

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

type table struct {
	keys []any
	vals map[any]any
}

func newTable() *table { return &table{vals: map[any]any{}} }

func normKey(k any) any {
	if f, ok := k.(float64); ok && f == math.Trunc(f) && math.Abs(f) < 1<<53 {
		return int64(f)
	}
	return k
}

func (t *table) Set(k, v any) {
	k = normKey(k)
	if _, ok := t.vals[k]; !ok {
		t.keys = append(t.keys, k)
	}
	t.vals[k] = v
}

func (t *table) Get(k any) any {
	if t == nil {
		return nil
	}
	return t.vals[normKey(k)]
}

func (t *table) Delete(k any) {
	k = normKey(k)
	if _, ok := t.vals[k]; !ok {
		return
	}
	delete(t.vals, k)
	for i, x := range t.keys {
		if x == k {
			t.keys = append(t.keys[:i], t.keys[i+1:]...)
			break
		}
	}
}

func (t *table) Len() int {
	if t == nil {
		return 0
	}
	return len(t.keys)
}

func (t *table) Append(v any) { t.Set(int64(t.Len()+1), v) }

func (t *table) Table(k any) *table {
	v, _ := t.Get(k).(*table)
	return v
}

func (t *table) Str(k any) string {
	v, _ := t.Get(k).(string)
	return v
}

func (t *table) Num(k any) float64 { return toNumber(t.Get(k)) }

func (t *table) Items() []any {
	if t == nil {
		return nil
	}
	out := make([]any, 0, len(t.keys))
	for _, k := range t.sortedKeys() {
		out = append(out, t.vals[k])
	}
	return out
}

func toNumber(v any) float64 {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case float64:
		return x
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n
	}
	return 0
}

func (t *table) sortedKeys() []any {
	keys := append([]any{}, t.keys...)
	rank := func(k any) int {
		switch k.(type) {
		case int64:
			return 0
		case float64:
			return 1
		case string:
			return 2
		}
		return 3
	}
	sort.SliceStable(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra < rb
		}
		switch x := a.(type) {
		case int64:
			return x < b.(int64)
		case float64:
			return x < b.(float64)
		case string:
			return x < b.(string)
		}
		return false
	})
	return keys
}

func (t *table) isArray() bool {
	for i, k := range t.sortedKeys() {
		if n, ok := k.(int64); !ok || n != int64(i+1) {
			return false
		}
	}
	return true
}

func luaValueToGo(src []byte, v *luaValue) any {
	if v == nil {
		return nil
	}
	switch v.Kind {
	case luaBool:
		return v.Bool
	case luaNumber:
		return normKey(v.Num)
	case luaString:
		return v.Str
	case luaTable:
		t := newTable()
		for _, e := range v.Entries {
			var k any
			if e.Key == nil {
				k = int64(e.Index)
			} else {
				k = luaValueToGo(src, e.Key)
			}
			if k == nil {
				continue
			}
			if val := luaValueToGo(src, e.Value); val != nil {
				t.Set(k, val)
			}
		}
		return t
	}
	return nil
}

func writeLuaNumber(b *strings.Builder, f float64) {
	switch {
	case math.IsNaN(f) || math.IsInf(f, 0):
		b.WriteString("0")
	case f == math.Trunc(f) && math.Abs(f) < 1<<53:
		b.WriteString(strconv.FormatInt(int64(f), 10))
	default:
		b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
	}
}

func writeLua(b *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("nil")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case float64:
		writeLuaNumber(b, x)
	case string:
		b.WriteString(luaQuote(x))
	case *table:
		if x.Len() == 0 {
			b.WriteString("{\n}")
			return
		}
		b.WriteString("{\n")
		array := x.isArray()
		for _, k := range x.sortedKeys() {
			if !array {
				b.WriteByte('[')
				writeLua(b, k)
				b.WriteString("] = ")
			}
			writeLua(b, x.vals[k])
			b.WriteString(",\n")
		}
		b.WriteByte('}')
	default:
		b.WriteString("nil")
	}
}

func luaAssign(b *strings.Builder, name string, v any) {
	b.WriteString(name)
	b.WriteString(" = ")
	writeLua(b, v)
	b.WriteString("\n")
}

var errCodec = errors.New("damaged data")

type cborReader struct {
	b   []byte
	pos int
}

func decodeCBOR(data []byte) (any, error) {
	r := &cborReader{b: data}
	v, err := r.value(0)
	if err != nil {
		return nil, err
	}
	if r.pos != len(data) {
		return nil, fmt.Errorf("%w: %d trailing bytes", errCodec, len(data)-r.pos)
	}
	return v, nil
}

func (r *cborReader) byte() (byte, error) {
	if r.pos >= len(r.b) {
		return 0, errCodec
	}
	c := r.b[r.pos]
	r.pos++
	return c, nil
}

func (r *cborReader) take(n uint64) ([]byte, error) {
	if n > uint64(len(r.b)-r.pos) {
		return nil, errCodec
	}
	s := r.b[r.pos : r.pos+int(n)]
	r.pos += int(n)
	return s, nil
}

func (r *cborReader) arg(info byte) (uint64, bool, error) {
	switch {
	case info < 24:
		return uint64(info), false, nil
	case info == 24:
		c, err := r.byte()
		return uint64(c), false, err
	case info == 25:
		s, err := r.take(2)
		if err != nil {
			return 0, false, err
		}
		return uint64(binary.BigEndian.Uint16(s)), false, nil
	case info == 26:
		s, err := r.take(4)
		if err != nil {
			return 0, false, err
		}
		return uint64(binary.BigEndian.Uint32(s)), false, nil
	case info == 27:
		s, err := r.take(8)
		if err != nil {
			return 0, false, err
		}
		return binary.BigEndian.Uint64(s), false, nil
	case info == 31:
		return 0, true, nil
	}
	return 0, false, errCodec
}

func (r *cborReader) isBreak() bool {
	if r.pos < len(r.b) && r.b[r.pos] == 0xFF {
		r.pos++
		return true
	}
	return false
}

func (r *cborReader) value(depth int) (any, error) {
	if depth > 64 {
		return nil, errCodec
	}
	c, err := r.byte()
	if err != nil {
		return nil, err
	}
	major, info := c>>5, c&31
	if major == 7 {
		switch info {
		case 20:
			return false, nil
		case 21:
			return true, nil
		case 22, 23:
			return nil, nil
		case 25:
			s, err := r.take(2)
			if err != nil {
				return nil, err
			}
			return halfFloat(binary.BigEndian.Uint16(s)), nil
		case 26:
			s, err := r.take(4)
			if err != nil {
				return nil, err
			}
			return float64(math.Float32frombits(binary.BigEndian.Uint32(s))), nil
		case 27:
			s, err := r.take(8)
			if err != nil {
				return nil, err
			}
			return math.Float64frombits(binary.BigEndian.Uint64(s)), nil
		}
		if info < 24 {
			return nil, nil
		}
		return nil, errCodec
	}
	n, indefinite, err := r.arg(info)
	if err != nil {
		return nil, err
	}
	switch major {
	case 0:
		return normKey(float64(n)), nil
	case 1:
		return normKey(-1 - float64(n)), nil
	case 2, 3:
		if !indefinite {
			s, err := r.take(n)
			return string(s), err
		}
		var sb strings.Builder
		for !r.isBreak() {
			part, err := r.value(depth + 1)
			if err != nil {
				return nil, err
			}
			s, ok := part.(string)
			if !ok {
				return nil, errCodec
			}
			sb.WriteString(s)
		}
		return sb.String(), nil
	case 4:
		t := newTable()
		for i := uint64(0); indefinite || i < n; i++ {
			if indefinite && r.isBreak() {
				break
			}
			v, err := r.value(depth + 1)
			if err != nil {
				return nil, err
			}
			t.Set(int64(i+1), v)
		}
		return t, nil
	case 5:
		t := newTable()
		for i := uint64(0); indefinite || i < n; i++ {
			if indefinite && r.isBreak() {
				break
			}
			k, err := r.value(depth + 1)
			if err != nil {
				return nil, err
			}
			v, err := r.value(depth + 1)
			if err != nil {
				return nil, err
			}
			if k != nil && v != nil {
				t.Set(k, v)
			}
		}
		return t, nil
	case 6:
		return r.value(depth + 1)
	}
	return nil, errCodec
}

func halfFloat(h uint16) float64 {
	exp := int(h>>10) & 0x1f
	mant := float64(h & 0x3ff)
	var v float64
	switch exp {
	case 0:
		v = mant * math.Pow(2, -24)
	case 31:
		v = math.Inf(1)
	default:
		v = (1 + mant/1024) * math.Pow(2, float64(exp-15))
	}
	if h&0x8000 != 0 {
		v = -v
	}
	return v
}

const printAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789()"

func decodeForPrint(s string) ([]byte, error) {
	var idx [256]int16
	for i := range idx {
		idx[i] = -1
	}
	for i := 0; i < len(printAlphabet); i++ {
		idx[printAlphabet[i]] = int16(i)
	}
	s = strings.TrimSpace(s)
	out := make([]byte, 0, len(s)*3/4+3)
	var acc uint32
	bits := 0
	for i := 0; i < len(s); i++ {
		v := idx[s[i]]
		if v < 0 {
			return nil, fmt.Errorf("%w: unexpected character in encoded data", errCodec)
		}
		acc |= uint32(v) << bits
		bits += 6
		for bits >= 8 {
			out = append(out, byte(acc))
			acc >>= 8
			bits -= 8
		}
	}
	return out, nil
}

func inflateRaw(data []byte, limit int64) ([]byte, error) {
	r := flate.NewReader(bytes.NewReader(data))
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(out)) > limit {
		return nil, fmt.Errorf("%w: data expands beyond %d MB", errCodec, limit>>20)
	}
	return out, nil
}

type libSerializeReader struct {
	b       []byte
	pos     int
	strings []string
	tables  []*table
}

func decodeLibSerialize(data []byte) ([]any, error) {
	r := &libSerializeReader{b: data}
	if len(data) == 0 {
		return nil, errCodec
	}
	if data[0] > 1 {
		return nil, fmt.Errorf("%w: unknown serializer version %d", errCodec, data[0])
	}
	r.pos = 1
	var out []any
	for r.pos < len(r.b) {
		v, err := r.object(0)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *libSerializeReader) byte() (byte, error) {
	if r.pos >= len(r.b) {
		return 0, errCodec
	}
	c := r.b[r.pos]
	r.pos++
	return c, nil
}

func (r *libSerializeReader) uint(n int) (uint64, error) {
	if r.pos+n > len(r.b) {
		return 0, errCodec
	}
	var v uint64
	for i := 0; i < n; i++ {
		v = v<<8 | uint64(r.b[r.pos+i])
	}
	r.pos += n
	return v, nil
}

func (r *libSerializeReader) str(n uint64) (string, error) {
	if n > uint64(len(r.b)-r.pos) {
		return "", errCodec
	}
	s := string(r.b[r.pos : r.pos+int(n)])
	r.pos += int(n)
	if len(s) > 2 {
		r.strings = append(r.strings, s)
	}
	return s, nil
}

func (r *libSerializeReader) fill(t *table, arrayCount, mapCount uint64, depth int) error {
	for i := uint64(0); i < arrayCount; i++ {
		v, err := r.object(depth + 1)
		if err != nil {
			return err
		}
		if v != nil {
			t.Set(int64(i+1), v)
		}
	}
	for i := uint64(0); i < mapCount; i++ {
		k, err := r.object(depth + 1)
		if err != nil {
			return err
		}
		v, err := r.object(depth + 1)
		if err != nil {
			return err
		}
		if k != nil && v != nil {
			t.Set(k, v)
		}
	}
	return nil
}

func (r *libSerializeReader) newTable() *table {
	t := newTable()
	r.tables = append(r.tables, t)
	return t
}

var libSerializeWidths = [...]int{0, 2, 2, 3, 3, 4, 4, 7, 7}

func (r *libSerializeReader) object(depth int) (any, error) {
	if depth > 64 {
		return nil, errCodec
	}
	c, err := r.byte()
	if err != nil {
		return nil, err
	}
	switch {
	case c&1 == 1:
		return int64(c >> 1), nil
	case c&3 == 2:
		count := uint64(c >> 4)
		switch (c & 0x0C) >> 2 {
		case 0:
			return r.str(count)
		case 1:
			t := r.newTable()
			return t, r.fill(t, 0, count, depth)
		case 2:
			t := r.newTable()
			return t, r.fill(t, count, 0, depth)
		default:
			t := r.newTable()
			return t, r.fill(t, count&3+1, count>>2+1, depth)
		}
	case c&7 == 4:
		next, err := r.byte()
		if err != nil {
			return nil, err
		}
		v := int64(c>>4) | int64(next)<<4
		if c&8 != 0 {
			v = -v
		}
		return v, nil
	}
	kind := int(c >> 3)
	switch {
	case kind == 0:
		return nil, nil
	case kind >= 1 && kind <= 8:
		v, err := r.uint(libSerializeWidths[kind])
		if err != nil {
			return nil, err
		}
		if kind%2 == 0 {
			return normKey(-float64(v)), nil
		}
		return normKey(float64(v)), nil
	case kind == 9:
		v, err := r.uint(8)
		if err != nil {
			return nil, err
		}
		return normKey(math.Float64frombits(v)), nil
	case kind == 10 || kind == 11:
		n, err := r.byte()
		if err != nil {
			return nil, err
		}
		if r.pos+int(n) > len(r.b) {
			return nil, errCodec
		}
		f, err := strconv.ParseFloat(string(r.b[r.pos:r.pos+int(n)]), 64)
		r.pos += int(n)
		if err != nil {
			return nil, errCodec
		}
		if kind == 11 {
			f = -f
		}
		return normKey(f), nil
	case kind == 12:
		return true, nil
	case kind == 13:
		return false, nil
	case kind >= 14 && kind <= 16:
		n, err := r.uint(kind - 13)
		if err != nil {
			return nil, err
		}
		return r.str(n)
	case kind >= 17 && kind <= 19:
		n, err := r.uint(kind - 16)
		if err != nil {
			return nil, err
		}
		t := r.newTable()
		return t, r.fill(t, 0, n, depth)
	case kind >= 20 && kind <= 22:
		n, err := r.uint(kind - 19)
		if err != nil {
			return nil, err
		}
		t := r.newTable()
		return t, r.fill(t, n, 0, depth)
	case kind >= 23 && kind <= 25:
		a, err := r.uint(kind - 22)
		if err != nil {
			return nil, err
		}
		m, err := r.uint(kind - 22)
		if err != nil {
			return nil, err
		}
		t := r.newTable()
		return t, r.fill(t, a, m, depth)
	case kind >= 26 && kind <= 28:
		n, err := r.uint(kind - 25)
		if err != nil {
			return nil, err
		}
		if n < 1 || n > uint64(len(r.strings)) {
			return nil, errCodec
		}
		return r.strings[n-1], nil
	case kind >= 29 && kind <= 31:
		n, err := r.uint(kind - 28)
		if err != nil {
			return nil, err
		}
		if n < 1 || n > uint64(len(r.tables)) {
			return nil, errCodec
		}
		return r.tables[n-1], nil
	}
	return nil, errCodec
}
