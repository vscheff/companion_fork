package companion

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type luaKind int

const (
	luaNil luaKind = iota
	luaBool
	luaNumber
	luaString
	luaTable
)

type luaValue struct {
	Kind    luaKind
	Start   int
	End     int
	Str     string
	Num     float64
	Bool    bool
	Entries []*luaEntry
}

type luaEntry struct {
	Start int
	End   int
	Key   *luaValue
	Index int
	Value *luaValue
}

func (e *luaEntry) KeyString() string {
	if e.Key == nil {
		return ""
	}
	switch e.Key.Kind {
	case luaString:
		return e.Key.Str
	case luaNumber:
		return strconv.FormatFloat(e.Key.Num, 'f', -1, 64)
	}
	return ""
}

func (v *luaValue) Field(name string) *luaValue {
	if v == nil || v.Kind != luaTable {
		return nil
	}
	for _, e := range v.Entries {
		if e.Key != nil && e.Key.Kind == luaString && e.Key.Str == name {
			return e.Value
		}
	}
	return nil
}

func (v *luaValue) Number() float64 {
	if v == nil {
		return 0
	}
	switch v.Kind {
	case luaNumber:
		return v.Num
	case luaString:
		n, _ := strconv.ParseFloat(strings.TrimSpace(v.Str), 64)
		return n
	}
	return 0
}

func (v *luaValue) String() string {
	if v == nil || v.Kind != luaString {
		return ""
	}
	return v.Str
}

func (v *luaValue) Array() ([]*luaEntry, bool) {
	if v == nil || v.Kind != luaTable {
		return nil, false
	}
	var implicit, explicit []*luaEntry
	for _, e := range v.Entries {
		switch {
		case e.Key == nil:
			implicit = append(implicit, e)
		case e.Key.Kind == luaNumber && e.Key.Num >= 1 && e.Key.Num == math.Trunc(e.Key.Num) && e.Key.Num < 1<<31:
			explicit = append(explicit, e)
		default:
			return nil, false
		}
	}
	if len(implicit) > 0 && len(explicit) > 0 {
		return nil, false
	}
	if len(implicit) > 0 {
		return implicit, true
	}
	sort.SliceStable(explicit, func(i, j int) bool { return explicit[i].Key.Num < explicit[j].Key.Num })
	for i, e := range explicit {
		if int(e.Key.Num) != i+1 {
			return nil, false
		}
	}
	return explicit, true
}

// The outermost Lua table counts as level one.
const maxLuaTableDepth = 64

type luaParser struct {
	src []byte
	pos int
}

type luaAssignment struct {
	Name  string
	Value *luaValue
}

func parseSavedVariables(src []byte) ([]luaAssignment, error) {
	p := &luaParser{src: src}
	var out []luaAssignment
	for {
		p.skip()
		if p.pos >= len(p.src) {
			return out, nil
		}
		name := p.ident()
		if name == "" {
			return nil, p.errorf("expected a variable name")
		}
		p.skip()
		if !p.eat('=') {
			return nil, p.errorf("expected = after %s", name)
		}
		v, err := p.value(0)
		if err != nil {
			return nil, err
		}
		out = append(out, luaAssignment{Name: name, Value: v})
		p.skip()
		p.eat(';')
	}
}

func (p *luaParser) errorf(format string, args ...any) error {
	line := bytes.Count(p.src[:min(p.pos, len(p.src))], []byte{'\n'}) + 1
	return fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, args...))
}

func (p *luaParser) peek() byte {
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return 0
}

func (p *luaParser) eat(c byte) bool {
	if p.peek() == c && p.pos < len(p.src) {
		p.pos++
		return true
	}
	return false
}

func (p *luaParser) skip() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v':
			p.pos++
		case c == '-' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '-':
			p.pos += 2
			if level, ok := p.longOpen(); ok {
				p.longBody(level)
				continue
			}
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		case c == 0xEF && bytes.HasPrefix(p.src[p.pos:], []byte{0xEF, 0xBB, 0xBF}) && p.pos == 0:
			p.pos += 3
		default:
			return
		}
	}
}

func (p *luaParser) longOpen() (int, bool) {
	if p.peek() != '[' {
		return 0, false
	}
	i := p.pos + 1
	level := 0
	for i < len(p.src) && p.src[i] == '=' {
		level++
		i++
	}
	if i < len(p.src) && p.src[i] == '[' {
		p.pos = i + 1
		return level, true
	}
	return 0, false
}

func (p *luaParser) longBody(level int) (string, error) {
	closer := "]" + strings.Repeat("=", level) + "]"
	idx := bytes.Index(p.src[p.pos:], []byte(closer))
	if idx < 0 {
		p.pos = len(p.src)
		return "", p.errorf("unfinished long string")
	}
	s := string(p.src[p.pos : p.pos+idx])
	p.pos += idx + len(closer)
	if strings.HasPrefix(s, "\r\n") {
		s = s[2:]
	} else if strings.HasPrefix(s, "\n") {
		s = s[1:]
	}
	return s, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentChar(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }

func (p *luaParser) ident() string {
	start := p.pos
	if !isIdentStart(p.peek()) {
		return ""
	}
	for p.pos < len(p.src) && isIdentChar(p.src[p.pos]) {
		p.pos++
	}
	return string(p.src[start:p.pos])
}

func (p *luaParser) value(depth int) (*luaValue, error) {
	p.skip()
	start := p.pos
	c := p.peek()
	switch {
	case c == '{':
		return p.table(depth + 1)
	case c == '"' || c == '\'':
		s, err := p.quoted()
		if err != nil {
			return nil, err
		}
		return &luaValue{Kind: luaString, Str: s, Start: start, End: p.pos}, nil
	case c == '[':
		level, ok := p.longOpen()
		if !ok {
			return nil, p.errorf("unexpected [")
		}
		s, err := p.longBody(level)
		if err != nil {
			return nil, err
		}
		return &luaValue{Kind: luaString, Str: s, Start: start, End: p.pos}, nil
	case c == '-' || c == '.' || (c >= '0' && c <= '9'):
		return p.number()
	case isIdentStart(c):
		word := p.ident()
		switch word {
		case "true", "false":
			return &luaValue{Kind: luaBool, Bool: word == "true", Start: start, End: p.pos}, nil
		case "nil":
			return &luaValue{Kind: luaNil, Start: start, End: p.pos}, nil
		case "inf":
			return &luaValue{Kind: luaNumber, Num: math.Inf(1), Start: start, End: p.pos}, nil
		}
		return nil, p.errorf("unexpected word %q", word)
	}
	if p.pos >= len(p.src) {
		return nil, p.errorf("unexpected end of file")
	}
	return nil, p.errorf("unexpected character %q", c)
}

func (p *luaParser) number() (*luaValue, error) {
	start := p.pos
	if p.peek() == '-' {
		p.pos++
		p.skip()
	}
	numStart := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if (c >= '0' && c <= '9') || c == '.' || c == 'x' || c == 'X' || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') ||
			c == 'p' || c == 'P' || c == '#' || c == 'I' || c == 'N' || c == 'n' || c == 'i' {
			p.pos++
			continue
		}
		if (c == '+' || c == '-') && p.pos > numStart {
			prev := p.src[p.pos-1]
			if prev == 'e' || prev == 'E' || prev == 'p' || prev == 'P' {
				p.pos++
				continue
			}
		}
		break
	}
	text := string(p.src[numStart:p.pos])
	if text == "" {
		return nil, p.errorf("expected a number")
	}
	neg := p.src[start] == '-'
	n, err := parseLuaNumber(text)
	if err != nil {
		return nil, p.errorf("bad number %q", text)
	}
	if neg {
		n = -n
	}
	return &luaValue{Kind: luaNumber, Num: n, Start: start, End: p.pos}, nil
}

func parseLuaNumber(text string) (float64, error) {
	lower := strings.ToLower(text)
	switch {
	case strings.HasPrefix(lower, "1.#inf"), lower == "inf":
		return math.Inf(1), nil
	case strings.Contains(lower, "#"), lower == "nan":
		return 0, nil
	case strings.HasPrefix(lower, "0x"):
		if u, err := strconv.ParseUint(lower[2:], 16, 64); err == nil {
			return float64(u), nil
		}
		return strconv.ParseFloat(lower, 64)
	}
	return strconv.ParseFloat(text, 64)
}

func (p *luaParser) quoted() (string, error) {
	q := p.src[p.pos]
	p.pos++
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == q:
			p.pos++
			return b.String(), nil
		case c == '\n':
			return "", p.errorf("unfinished string")
		case c == '\\':
			p.pos++
			if p.pos >= len(p.src) {
				return "", p.errorf("unfinished string")
			}
			e := p.src[p.pos]
			p.pos++
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'a':
				b.WriteByte('\a')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'v':
				b.WriteByte('\v')
			case '\n':
				b.WriteByte('\n')
				if p.peek() == '\r' {
					p.pos++
				}
			case '\r':
				b.WriteByte('\n')
				if p.peek() == '\n' {
					p.pos++
				}
			case 'x':
				if p.pos+2 > len(p.src) {
					return "", p.errorf("bad \\x escape")
				}
				n, err := strconv.ParseUint(string(p.src[p.pos:p.pos+2]), 16, 8)
				if err != nil {
					return "", p.errorf("bad \\x escape")
				}
				b.WriteByte(byte(n))
				p.pos += 2
			case 'z':
				for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\n' || p.src[p.pos] == '\r' || p.src[p.pos] == '\t') {
					p.pos++
				}
			default:
				if e >= '0' && e <= '9' {
					n := int(e - '0')
					for i := 0; i < 2 && p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9'; i++ {
						n = n*10 + int(p.src[p.pos]-'0')
						p.pos++
					}
					if n > 255 {
						return "", p.errorf("bad decimal escape")
					}
					b.WriteByte(byte(n))
				} else {
					b.WriteByte(e)
				}
			}
		default:
			b.WriteByte(c)
			p.pos++
		}
	}
	return "", p.errorf("unfinished string")
}

func (p *luaParser) table(depth int) (*luaValue, error) {
	if depth > maxLuaTableDepth {
		return nil, p.errorf("table nesting exceeds %d levels", maxLuaTableDepth)
	}
	t := &luaValue{Kind: luaTable, Start: p.pos}
	p.pos++
	index := 0
	for {
		p.skip()
		if p.eat('}') {
			t.End = p.pos
			return t, nil
		}
		if p.pos >= len(p.src) {
			return nil, p.errorf("unfinished table")
		}
		e := &luaEntry{Start: p.pos}
		switch c := p.peek(); {
		case c == '[' && !p.atLongString():
			p.pos++
			k, err := p.value(depth)
			if err != nil {
				return nil, err
			}
			p.skip()
			if !p.eat(']') {
				return nil, p.errorf("expected ]")
			}
			p.skip()
			if !p.eat('=') {
				return nil, p.errorf("expected =")
			}
			e.Key = k
		case isIdentStart(c) && p.identAssign():
			start := p.pos
			name := p.ident()
			e.Key = &luaValue{Kind: luaString, Str: name, Start: start, End: p.pos}
			p.skip()
			p.eat('=')
		}
		v, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		e.Value = v
		if e.Key == nil {
			index++
			e.Index = index
		}
		p.skip()
		if !p.eat(',') && !p.eat(';') {
			if p.peek() != '}' {
				return nil, p.errorf("expected , or }")
			}
		}
		e.End = p.pos
		t.Entries = append(t.Entries, e)
	}
}

func (p *luaParser) atLongString() bool {
	if p.peek() != '[' {
		return false
	}
	i := p.pos + 1
	for i < len(p.src) && p.src[i] == '=' {
		i++
	}
	return i < len(p.src) && p.src[i] == '['
}

func (p *luaParser) identAssign() bool {
	i := p.pos
	for i < len(p.src) && isIdentChar(p.src[i]) {
		i++
	}
	word := string(p.src[p.pos:i])
	if word == "true" || word == "false" || word == "nil" {
		return false
	}
	for i < len(p.src) && (p.src[i] == ' ' || p.src[i] == '\t' || p.src[i] == '\r' || p.src[i] == '\n') {
		i++
	}
	return i < len(p.src) && p.src[i] == '=' && (i+1 >= len(p.src) || p.src[i+1] != '=')
}

type luaEdit struct {
	Start int
	End   int
	Text  string
}

func entryRemoval(src []byte, e *luaEntry) luaEdit {
	start, end := e.Start, e.End
	for start > 0 && (src[start-1] == ' ' || src[start-1] == '\t') {
		start--
	}
	lineStart := start == 0 || src[start-1] == '\n'
	i := end
	for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
		i++
	}
	if i+1 < len(src) && src[i] == '-' && src[i+1] == '-' && !bytes.HasPrefix(src[i+2:], []byte("[")) {
		for i < len(src) && src[i] != '\n' {
			i++
		}
	}
	if lineStart {
		if i < len(src) && src[i] == '\r' {
			i++
		}
		if i < len(src) && src[i] == '\n' {
			i++
			end = i
		} else if i == len(src) {
			end = i
		}
	}
	if !lineStart {
		start = e.Start
	}
	return luaEdit{Start: start, End: end}
}

func applyLuaEdits(src []byte, edits []luaEdit) ([]byte, error) {
	sort.Slice(edits, func(i, j int) bool { return edits[i].Start < edits[j].Start })
	var out bytes.Buffer
	out.Grow(len(src))
	last := 0
	for _, ed := range edits {
		if ed.Start < last || ed.End < ed.Start || ed.End > len(src) {
			return nil, errors.New("overlapping cleanup edits")
		}
		out.Write(src[last:ed.Start])
		out.WriteString(ed.Text)
		last = ed.End
	}
	out.Write(src[last:])
	return out.Bytes(), nil
}

func luaQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			b.WriteString(`\"`)
		case c == '\\':
			b.WriteString(`\\`)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c < 32 || c == 127:
			fmt.Fprintf(&b, "\\%03d", c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
