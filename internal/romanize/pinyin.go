package romanize

import (
	"bytes"
	"compress/flate"
	_ "embed"
	"encoding/binary"
	"errors"
	"io"
	"slices"
	"sync"
)

// Chinese is written in Hanyu Pinyin with its tone marks, a syllable a
// character, as lyrics are sung: "我爱你" is "wǒ ài nǐ". A character of
// several readings is read as the word it is in reads it (银行, yín háng;
// 行走, xíng zǒu), else by its commonest reading, as pinyin.bin has them
// (tools/pinyindata). Traditional characters are read as their simplified
// forms are in words, and by Unihan's reading for Taiwan, where it is
// another, in lyrics in traditional Chinese, "zh-Hant".

//go:embed pinyin.bin
var pinyinData []byte

// pinyinTable is pinyin.bin read.
type pinyinTable struct {
	syllables []string
	// chars are the characters with a reading, in order, and reads the
	// reading of each, an index of syllables.
	chars []rune
	reads []uint16
	// taiwan are the readings of traditional Chinese that are others.
	taiwan map[rune]uint16
	// simplified are the simplified forms of traditional characters.
	simplified map[rune]rune
	// words are the readings of the words read otherwise than their
	// characters, by their simplified text, and longest the most
	// characters of one.
	words   map[string][]uint16
	longest int
}

var (
	pinyinOnce sync.Once
	pinyin     *pinyinTable
)

// loadPinyin reads pinyin.bin, the first time Chinese is romanized.
func loadPinyin() *pinyinTable {
	pinyinOnce.Do(func() {
		t, err := readPinyin(pinyinData)
		if err != nil {
			panic("romanize: pinyin.bin: " + err.Error())
		}
		pinyin = t
	})
	return pinyin
}

// readPinyin reads the tables of pinyin.bin, as tools/pinyindata writes
// them.
func readPinyin(data []byte) (*pinyinTable, error) {
	raw, err := io.ReadAll(flate.NewReader(bytes.NewReader(data)))
	if err != nil {
		return nil, err
	}
	at, short := 0, false
	next := func() int {
		v, n := binary.Uvarint(raw[at:])
		if n <= 0 {
			short = true
			return 0
		}
		at += n
		return int(v)
	}
	t := &pinyinTable{taiwan: map[rune]uint16{}, simplified: map[rune]rune{}, words: map[string][]uint16{}}
	t.syllables = make([]string, next())
	for i := range t.syllables {
		n := next()
		if at+n > len(raw) {
			return nil, errors.New("cut short")
		}
		t.syllables[i] = string(raw[at : at+n])
		at += n
	}
	n := next()
	t.chars, t.reads = make([]rune, n), make([]uint16, n)
	last := rune(0)
	for i := range n {
		last += rune(next())
		t.chars[i], t.reads[i] = last, uint16(next())
	}
	last = 0
	for range next() {
		last += rune(next())
		t.taiwan[last] = uint16(next())
	}
	last = 0
	for range next() {
		last += rune(next())
		t.simplified[last] = rune(next())
	}
	for range next() {
		n := next()
		word := make([]rune, n)
		for i := range word {
			word[i] = rune(next())
		}
		read := make([]uint16, n)
		for i := range read {
			read[i] = uint16(next())
		}
		t.words[string(word)] = read
		t.longest = max(t.longest, n)
	}
	if short {
		return nil, errors.New("cut short")
	}
	for _, i := range t.reads {
		if int(i) >= len(t.syllables) {
			return nil, errors.New("a reading out of range")
		}
	}
	return t, nil
}

// reading returns the reading of the character r, "" when it has none.
func (t *pinyinTable) reading(r rune, taiwan bool) string {
	if taiwan {
		if i, ok := t.taiwan[r]; ok {
			return t.syllables[i]
		}
	}
	if i, ok := slices.BinarySearch(t.chars, r); ok {
		return t.syllables[t.reads[i]]
	}
	return ""
}

// word returns the reading of the longest word read otherwise than its
// characters that runes start with, of two characters at least; nil when
// none.
func (t *pinyinTable) word(runes []rune) []uint16 {
	key := make([]rune, 0, t.longest)
	for _, r := range runes[:min(len(runes), t.longest)] {
		if s, ok := t.simplified[r]; ok {
			r = s
		}
		key = append(key, r)
	}
	for n := len(key); n >= 2; n-- {
		if read, ok := t.words[string(key[:n])]; ok {
			return read
		}
	}
	return nil
}

func init() { register(han, chinese, "zh zh-Hant") }

// chinese romanizes a run of Chinese characters, a syllable apart.
func chinese(run, lang string) []piece {
	t := loadPinyin()
	taiwan := lang == "zh-Hant"
	runes := []rune(run)
	var pieces []piece
	at := 0
	var word []uint16
	for i, r := range runes {
		if len(word) == 0 {
			word = t.word(runes[i:])
		}
		latin := ""
		if len(word) > 0 {
			latin, word = t.syllables[word[0]], word[1:]
		} else {
			latin = t.reading(r, taiwan)
		}
		if latin == "" {
			latin = plain(r)
		}
		if i > 0 {
			latin = " " + latin
		}
		at += len(string(r))
		pieces = append(pieces, piece{at, latin})
	}
	return pieces
}
