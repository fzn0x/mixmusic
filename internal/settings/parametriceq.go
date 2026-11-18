package settings

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ParametricEQ.txt, the file of a correction of headphones (#616), in
// Equalizer APO's format, which AutoEq writes for each model:
//
//	Preamp: -5.9 dB
//	Filter 1: ON LSC Fc 105 Hz Gain 0.6 dB Q 0.70
//	Filter 2: ON PK Fc 127 Hz Gain -3.6 dB Q 0.21
//
// It is read strictly: a line that isn't a preamp, a filter, a comment or
// blank, a filter MixMusic doesn't play, or a value out of its range stops
// the file, which changes nothing, and the error tells which line, and
// why.

// maxParametricEQ is the most bytes a ParametricEQ.txt has: twenty filters
// take some 1,200.
const maxParametricEQ = 64 << 10

// ParametricEQWhy is why a line of a ParametricEQ.txt doesn't read.
type ParametricEQWhy int

const (
	// NotParametricEQ is a line that is neither a preamp nor a filter,
	// as Equalizer APO's Channel: or Device:, which change what the
	// filters after them do.
	NotParametricEQ ParametricEQWhy = iota
	// UnknownFilter is a filter of a type MixMusic doesn't play: Value.
	UnknownFilter
	// MissingParameter is a filter without Value, Fc, Gain or Q, which
	// its type needs.
	MissingParameter
	// ParameterTwice is a filter that gives Value twice.
	ParameterTwice
	// OutOfRange is a Value, Preamp, Fc, Gain or Q, that isn't a number in
	// its range.
	OutOfRange
	// Bandwidth is a filter whose width is in octaves, BW Oct, not a Q.
	Bandwidth
	// LeftOver is a line with more than its filter: Value.
	LeftOver
)

// ParametricEQError is why a line of a ParametricEQ.txt doesn't read.
type ParametricEQError struct {
	// Line is the line's number, from 1, and Text the line, trimmed, cut
	// to 60 characters.
	Line int
	Text string
	Why  ParametricEQWhy
	// Value is the type, the parameter or the text Why is of.
	Value string
}

func (e *ParametricEQError) Error() string {
	var why string
	switch e.Why {
	case NotParametricEQ:
		why = "is neither a preamp nor a filter"
	case UnknownFilter:
		why = fmt.Sprintf("has a filter of type %s, which MixMusic doesn't play", e.Value)
	case MissingParameter:
		why = fmt.Sprintf("has a filter without its %s", e.Value)
	case ParameterTwice:
		why = fmt.Sprintf("gives %s twice", e.Value)
	case OutOfRange:
		lo, hi := ParameterRange(e.Value)
		why = fmt.Sprintf("has a %s that isn't a number from %s to %s", e.Value, lo, hi)
	case Bandwidth:
		why = "gives a bandwidth in octaves, not a Q"
	case LeftOver:
		why = fmt.Sprintf("has %q after its filter", e.Value)
	}
	return fmt.Sprintf("line %d, %q, %s", e.Line, e.Text, why)
}

// ParameterRange is the range of the parameter name, Preamp, Fc, Gain or
// Q, with its unit, as an error tells it.
func ParameterRange(name string) (lo, hi string) {
	switch name {
	case "Fc":
		return fmt.Sprint(MinFilterHz, " Hz"), fmt.Sprint(MaxFilterHz, " Hz")
	case "Q":
		return fmt.Sprint(MinFilterQ), fmt.Sprint(MaxFilterQ)
	}
	return fmt.Sprint(-MaxFilterGain, " dB"), fmt.Sprint(MaxFilterGain, " dB")
}

// TooManyFiltersError is a file of more filters than a correction has.
type TooManyFiltersError struct{ Count int }

func (e *TooManyFiltersError) Error() string {
	return fmt.Sprintf("the file holds %d filters, past the %d MixMusic plays", e.Count, MaxHeadphoneFilters)
}

var (
	// ErrNoFilters is a file that holds no filter that's on.
	ErrNoFilters = errors.New("the file holds no filters")
	// ErrParametricEQSize is a file too big to be a ParametricEQ.txt.
	ErrParametricEQSize = errors.New("the file is too big to be a ParametricEQ.txt")
)

// filterCommand is a filter's command: Filter, and its number, which
// means nothing.
var filterCommand = regexp.MustCompile(`(?i)^filter(\s+\d+)?$`)

// number is a number of a ParametricEQ.txt, once its decimal comma, if
// any, is a point.
const number = `([-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?)`

var (
	preampValue = regexp.MustCompile(`(?i)^` + number + `\s*(?:dB)?$`)
	// parameter is the next parameter of a filter, at the start of what's
	// left of it: Fc with its Hz, Gain with its dB, Q, or BW Oct.
	parameter = regexp.MustCompile(`(?i)^\s*(?:(fc)\s*` + number + `\s*hz|(gain)\s*` + number + `\s*db|(q)\s*` + number + `|(bw)\s+oct\s*` + number + `)(?:\s+|$)`)
	// thousands is a frequency of three decimals, as 1.000, which some
	// files write in thousands, and Equalizer APO reads so.
	thousands = regexp.MustCompile(`^\d+\.\d{3}$`)
)

// ParseParametricEQ reads a ParametricEQ.txt: its preamp, the sum of its
// Preamp lines, and the filters that are on, of the types PK, LSC, HSC,
// LP and HP, LPQ and HPQ being LP and HP, and PEQ PK, each with Fc in Hz,
// Gain in dB, but for LP and HP, and Q, which LP and HP may leave out,
// 0.707 then. Blank lines, comments, from a #, filters that are off and
// those of the type None are left out. What it returns has no ID, no name
// and no source.
func ParseParametricEQ(data []byte) (Headphones, error) {
	h := noHeadphones()
	if len(data) > maxParametricEQ {
		return h, ErrParametricEQSize
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // UTF-8's BOM
	count := 0
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		fail := func(why ParametricEQWhy, value string) (Headphones, error) {
			text := line
			if !utf8.ValidString(text) {
				text = strings.ToValidUTF8(text, "\uFFFD")
			}
			if r := []rune(text); len(r) > 60 {
				text = string(r[:59]) + "…"
			}
			return noHeadphones(), &ParametricEQError{Line: i + 1, Text: text, Why: why, Value: value}
		}
		command, rest, ok := strings.Cut(line, ":")
		command, rest = strings.TrimSpace(command), strings.TrimSpace(strings.ReplaceAll(rest, ",", "."))
		switch {
		case ok && strings.EqualFold(command, "Preamp"):
			m := preampValue.FindStringSubmatch(rest)
			if m == nil {
				return fail(OutOfRange, "Preamp")
			}
			v, err := strconv.ParseFloat(m[1], 64)
			if h.Preamp += v; err != nil || math.Abs(h.Preamp) > MaxFilterGain {
				return fail(OutOfRange, "Preamp")
			}
		case ok && filterCommand.MatchString(command):
			f, bad := parseFilter(rest)
			switch {
			case bad != nil:
				return fail(bad.Why, bad.Value)
			case f.Type == "":
				continue // off, or None
			}
			if count++; count <= MaxHeadphoneFilters {
				h.Filters = append(h.Filters, f)
			}
		default:
			return fail(NotParametricEQ, "")
		}
	}
	switch {
	case count > MaxHeadphoneFilters:
		return noHeadphones(), &TooManyFiltersError{Count: count}
	case count == 0:
		return noHeadphones(), ErrNoFilters
	}
	h.Preamp = math.Round(h.Preamp*1e6) / 1e6
	return h, nil
}

// filterTypesOf are the types of filters of a ParametricEQ.txt that a
// correction plays, as it keeps them.
var filterTypesOf = map[string]string{
	"PK": FilterPeak, "PEQ": FilterPeak,
	"LSC": FilterLowShelf, "HSC": FilterHighShelf,
	"LP": FilterLowPass, "LPQ": FilterLowPass,
	"HP": FilterHighPass, "HPQ": FilterHighPass,
}

// filterHead is what a filter starts with: ON or OFF, its type, and its
// parameters.
var filterHead = regexp.MustCompile(`^(\S+)(?:\s+(\S+))?(.*)$`)

// parseFilter reads what follows a filter's command, as "ON PK Fc 105 Hz
// Gain 0.6 dB Q 0.70": a filter with no type for one that's off, or of the
// type None, or why it doesn't read, without its line.
func parseFilter(rest string) (HeadphoneFilter, *ParametricEQError) {
	m := filterHead.FindStringSubmatch(rest)
	switch {
	case m == nil:
		return HeadphoneFilter{}, &ParametricEQError{Why: NotParametricEQ}
	case strings.EqualFold(m[1], "OFF"):
		return HeadphoneFilter{}, nil
	case !strings.EqualFold(m[1], "ON") || m[2] == "":
		return HeadphoneFilter{}, &ParametricEQError{Why: NotParametricEQ}
	case strings.EqualFold(m[2], "None"):
		return HeadphoneFilter{}, nil
	}
	t, ok := filterTypesOf[strings.ToUpper(m[2])]
	if !ok {
		return HeadphoneFilter{}, &ParametricEQError{Why: UnknownFilter, Value: m[2]}
	}
	values := map[string]float64{}
	for rest := strings.TrimSpace(m[3]); rest != ""; {
		p := parameter.FindStringSubmatch(rest)
		if p == nil {
			return HeadphoneFilter{}, &ParametricEQError{Why: LeftOver, Value: rest}
		}
		var key, text string
		switch {
		case p[1] != "":
			key, text = "Fc", p[2]
			if thousands.MatchString(text) {
				text = strings.Replace(text, ".", "", 1)
			}
		case p[3] != "":
			key, text = "Gain", p[4]
		case p[5] != "":
			key, text = "Q", p[6]
		default:
			return HeadphoneFilter{}, &ParametricEQError{Why: Bandwidth}
		}
		if _, twice := values[key]; twice {
			return HeadphoneFilter{}, &ParametricEQError{Why: ParameterTwice, Value: key}
		}
		v, err := strconv.ParseFloat(text, 64)
		if err != nil || !inRange(key, v) {
			return HeadphoneFilter{}, &ParametricEQError{Why: OutOfRange, Value: key}
		}
		values[key] = v
		rest = rest[len(p[0]):]
	}
	for _, key := range []string{"Fc", "Gain", "Q"} {
		if _, ok := values[key]; !ok && (key == "Fc" || takesGain(t)) {
			return HeadphoneFilter{}, &ParametricEQError{Why: MissingParameter, Value: key}
		}
	}
	f := HeadphoneFilter{Type: t, Fc: values["Fc"], Q: math.Sqrt2 / 2}
	if takesGain(t) {
		f.Gain = values["Gain"]
	}
	if q, ok := values["Q"]; ok {
		f.Q = q
	}
	return f, nil
}

// inRange reports whether v is in the range of the parameter key.
func inRange(key string, v float64) bool {
	switch key {
	case "Fc":
		return v >= MinFilterHz && v <= MaxFilterHz
	case "Q":
		return v >= MinFilterQ && v <= MaxFilterQ
	}
	return math.Abs(v) <= MaxFilterGain
}
