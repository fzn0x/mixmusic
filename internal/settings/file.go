package settings

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Decode reads the settings a settings.json holds, as MixMusic does at
// start and each time the file changes: keys it lacks take their defaults,
// and values out of range go back in range. A byte order mark is fine, and
// so is UTF-16 with one, which Windows PowerShell 5.1 writes. A file that
// doesn't read gives a *FileError, and no settings: none of it applies.
func Decode(data []byte) (Settings, error) {
	text, _ := Text(data)
	rest := bytes.TrimLeft(text, " \t\r\n")
	switch {
	case len(rest) == 0:
		return Settings{}, &FileError{Msg: "it is empty"}
	case rest[0] != '{':
		// As null, which encoding/json takes for no change at all.
		line, column := position(text, len(text)-len(rest))
		return Settings{}, &FileError{Line: line, Column: column, Msg: "it must hold one JSON object, from { to }"}
	}
	s := Default()
	if err := json.Unmarshal(text, &s); err != nil {
		// encoding/json goes on past a value of the wrong type, and fills
		// the other keys: nothing of it is kept.
		return Settings{}, fileError(text, err)
	}
	return s.Clean(), nil
}

// Text returns the text of a settings file as UTF-8, and whether it was
// UTF-8 or UTF-16: a byte order mark goes, and UTF-16 with one becomes
// UTF-8. Other text, as of the system's code page, stays as it is, and
// reads with replacement characters for the letters outside ASCII.
func Text(data []byte) ([]byte, bool) {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		data = data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return fromUTF16(data[2:], binary.LittleEndian)
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return fromUTF16(data[2:], binary.BigEndian)
	}
	return data, utf8.Valid(data)
}

// fromUTF16 is UTF-16 text as UTF-8, and whether it was whole: of an even
// length, without halves of surrogate pairs.
func fromUTF16(data []byte, order binary.ByteOrder) ([]byte, bool) {
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = order.Uint16(data[2*i:])
	}
	runes := utf16.Decode(units)
	whole := len(data)%2 == 0
	out := make([]byte, 0, len(runes))
	for _, r := range runes {
		if r == utf8.RuneError {
			whole = false
		}
		out = utf8.AppendRune(out, r)
	}
	return out, whole
}

// FileError is why a settings file doesn't read: where, when it's a
// mistake of JSON, or which key, when a value has the wrong type.
type FileError struct {
	// Line and Column are where the mistake is, from 1, in runes; 0 when
	// unknown.
	Line, Column int
	// Key is the key whose value has the wrong type, as "volume" or
	// "equalizer.gains.1".
	Key string
	// Msg says what is wrong.
	Msg string
}

func (e *FileError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Msg)
	}
	return e.Msg
}

// fileError explains err, which encoding/json gave for text.
func fileError(text []byte, err error) *FileError {
	var syntax *json.SyntaxError
	var kind *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return syntaxError(text, int(syntax.Offset), syntax)
	case errors.As(err, &kind) && kind.Field != "":
		// Its offset counts from the object Settings.UnmarshalJSON was
		// given, not from the file, and its text names the type that
		// method decodes into: neither is shown.
		return &FileError{Key: kind.Field, Msg: kind.Field + " should be " + kindName(kind.Type)}
	}
	return &FileError{Msg: strings.TrimPrefix(err.Error(), "json: ")}
}

// kindName names what a value of type t is written as, in JSON.
func kindName(t reflect.Type) string {
	if t == nil {
		return "something else"
	}
	switch t.Kind() {
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "a whole number"
	case reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.String:
		return "text in quotes"
	case reflect.Slice, reflect.Array:
		return "a list in [ ]"
	case reflect.Map, reflect.Struct:
		return "an object in { }"
	}
	return "something else"
}

// syntaxError explains a mistake of JSON that encoding/json found after
// reading off bytes of text: the byte text[off-1] doesn't belong where it
// is, or the text ends early. It tells which from the bytes, not from
// encoding/json's words, which a later version may change; those are only
// the last resort.
func syntaxError(text []byte, off int, err *json.SyntaxError) *FileError {
	at := min(max(off-1, 0), len(text))
	msg, back := "", 0
	if at < len(text) {
		msg, back = misplaced(text, at)
	}
	if msg == "" && off >= len(text) {
		at, msg = len(text), "the file ends before its last }"
	}
	if msg == "" {
		msg = strings.TrimPrefix(err.Error(), "invalid character ")
	}
	line, column := position(text, at-back)
	return &FileError{Line: line, Column: column, Msg: msg}
}

// misplaced says why the byte at text[at] doesn't belong where it is, or
// "" when it may; back is how many bytes before it the mistake starts.
func misplaced(text []byte, at int) (msg string, back int) {
	c := text[at]
	want, afterComma := expected(text[:at])
	if want == 's' {
		slashes := at - len(bytes.TrimRight(text[:at], `\`))
		switch {
		case slashes%2 == 1:
			return `a \ inside quotes is written \\`, 1
		case c == '\n' || c == '\r':
			return "a line break inside quotes", 0
		case c < 0x20:
			return "a control character inside quotes", 0
		}
		return "", 0
	}
	value := c == '"' || c == '{' || c == '[' || c == '-' || isWord(c)
	switch {
	case c == '/':
		return "JSON has no comments", 0
	case c == '\'':
		return "JSON takes double quotes", 0
	case want == 'k' && c == '}' && afterComma:
		return "a comma before } has nothing after it", 0
	case want == 'v' && c == ']' && afterComma:
		return "a comma before ] has nothing after it", 0
	case want == 'k' && isWord(c):
		return "a key takes double quotes", 0
	case want == 'v' && isWord(c) && (c < '0' || c > '9'):
		return "text takes double quotes", 0
	case want == 'v' && (c == '}' || c == ']' || c == ','):
		return "a value is missing before this", 0
	case want == ':':
		return "a colon is missing after the key", 0
	case want == ',' && value:
		return "a comma is missing before this", 0
	case want == 'w':
		return "text takes double quotes; only true, false and null go without", 0
	case want == 'e':
		return "there is more after the last }", 0
	}
	return "", 0
}

// expected walks text, the valid start of a JSON value, and tells what may
// come next: 'k' a key, or } after {; 'v' a value, or ] after [; ':' a
// colon; ',' a comma, or the end of the object or list; 'e' nothing, the
// value having ended; 's' more of a string; 'w' more of a word that
// isn't one yet, as tru. afterComma tells whether a comma came last.
func expected(text []byte) (want byte, afterComma bool) {
	var stack []byte
	want = 'v'
	ended := func() byte {
		if len(stack) == 0 {
			return 'e'
		}
		return ','
	}
	inString, escaped, isKey := false, false, false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
				if want = ended(); isKey {
					want = ':'
				}
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		afterComma = false
		switch c {
		case '"':
			inString, isKey = true, want == 'k'
		case '{', '[':
			stack = append(stack, c)
			want = 'k'
			if c == '[' {
				want = 'v'
			}
		case '}', ']':
			stack = stack[:max(len(stack)-1, 0)]
			want = ended()
		case ':':
			want = 'v'
		case ',':
			afterComma = true
			want = 'v'
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				want = 'k'
			}
		default:
			start := i
			for i+1 < len(text) && isWord(text[i+1]) {
				i++
			}
			word := string(text[start : i+1])
			want = ended()
			if i+1 == len(text) && !slices.Contains([]string{"true", "false", "null"}, word) && (c < '0' || c > '9') && c != '-' {
				want = 'w'
			}
		}
	}
	if inString {
		want = 's'
	}
	return want, afterComma
}

// isWord tells whether c may be part of a number, or of true, false or
// null, or of a word written without quotes.
func isWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '+' || c == '-' || c == '_'
}

// position is the line and column of text[at], from 1: the column counts
// runes.
func position(text []byte, at int) (line, column int) {
	before := text[:at]
	start := bytes.LastIndexByte(before, '\n') + 1
	return 1 + bytes.Count(before, []byte{'\n'}), 1 + utf8.RuneCount(before[start:])
}

// Report is what of a settings file MixMusic doesn't take as it is.
type Report struct {
	// NotUTF8 is set when its text was neither UTF-8 nor UTF-16: letters
	// outside ASCII read as replacement characters.
	NotUTF8 bool
	// Unknown are the keys no setting has, as the typo crosfade, which
	// MixMusic's next save drops.
	Unknown []string
	// Adjusted are the keys whose values went back in range, or to their
	// defaults.
	Adjusted []string
}

// Empty tells whether the report has nothing to tell.
func (r Report) Empty() bool { return !r.NotUTF8 && len(r.Unknown) == 0 && len(r.Adjusted) == 0 }

// Inspect tells what of a settings file, data, the settings s that Decode
// read of it don't take as it is. It compares only what the file sets:
// an object member by member, as "equalizer": {"enabled": true}, which
// takes the rest of the equalizer from the defaults; lists and other
// values whole.
func Inspect(data []byte, s Settings) Report {
	text, ok := Text(data)
	r := Report{NotUTF8: !ok}
	var file map[string]any
	if json.Unmarshal(text, &file) != nil {
		return r
	}
	var kept map[string]any
	if b, err := json.Marshal(s); err != nil || json.Unmarshal(b, &kept) != nil {
		return r
	}
	for key, v := range file {
		name, t, known := field(reflect.TypeFor[Settings](), key)
		switch {
		case !known && strings.EqualFold(key, "discordPresence"):
			// The key of before 0.5.6, which UnmarshalJSON reads.
		case !known:
			r.Unknown = append(r.Unknown, key)
		default:
			r.compare(name, v, kept[name], t)
		}
	}
	slices.Sort(r.Unknown)
	slices.Sort(r.Adjusted)
	return r
}

// compare adds to the report what of the file's value of path, file, the
// settings don't keep as it is, kept, of the type t.
func (r *Report) compare(path string, file, kept any, t reflect.Type) {
	if file == nil {
		return // null, which leaves the default
	}
	members, isObject := file.(map[string]any)
	keptMembers, keptObject := kept.(map[string]any)
	if !isObject || !keptObject {
		if !reflect.DeepEqual(file, kept) {
			r.Adjusted = append(r.Adjusted, path)
		}
		return
	}
	for key, v := range members {
		switch t.Kind() {
		case reflect.Struct:
			name, ft, known := field(t, key)
			if !known {
				r.Unknown = append(r.Unknown, path+"."+key)
				continue
			}
			r.compare(path+"."+name, v, keptMembers[name], ft)
		case reflect.Map:
			got, ok := keptMembers[key]
			if !ok {
				if v != nil {
					r.Adjusted = append(r.Adjusted, path+"."+key)
				}
				continue
			}
			r.compare(path+"."+key, v, got, t.Elem())
		default:
			r.Adjusted = append(r.Adjusted, path)
			return
		}
	}
}

// field finds the field of the struct type t that encoding/json decodes
// key into: by its name in JSON, exactly or else ignoring case.
func field(t reflect.Type, key string) (name string, ft reflect.Type, ok bool) {
	var folded reflect.StructField
	found := false
	for i := range t.NumField() {
		f := t.Field(i)
		tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if tag == "" || tag == "-" {
			continue
		}
		if tag == key {
			return tag, f.Type, true
		}
		if !found && strings.EqualFold(tag, key) {
			folded, found = f, true
		}
	}
	if !found {
		return "", nil, false
	}
	tag, _, _ := strings.Cut(folded.Tag.Get("json"), ",")
	return tag, folded.Type, true
}
