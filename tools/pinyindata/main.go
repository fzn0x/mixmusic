// Command pinyindata makes internal/romanize/pinyin.bin, the readings in
// Hanyu Pinyin that romanize reads Chinese by, from data of permissive
// licences only:
//
//   - the reading of each character: the first of mozillazg's
//     pinyin-data's kMandarin_8105.txt, the commonest reading of the 8105
//     characters of the Table of General Standard Chinese Characters, then
//     its kMandarin_overwrite.txt, then Unihan's kMandarin, as pinyin-data
//     has them (MIT, and the Unicode License); Unihan's second kMandarin,
//     where it has one, is the reading of traditional characters;
//   - the simplified form of each traditional character, Unihan's
//     kSimplifiedVariant (Unicode_Variants.txt of Unihan.zip);
//   - the readings of words, mozillazg's phrase-pinyin-data's pinyin.txt
//     (MIT), but not its large_pinyin.txt, which holds CC-CEDICT's.
//
// Only the words whose reading isn't that of their characters one by one
// are kept, as only they change how a line reads: 银行, yínháng, but not
// 快乐, kuàilè. The file is compressed with DEFLATE.
//
//	go run ./tools/pinyindata -pinyin-data pinyin-data -phrases phrase-pinyin-data/pinyin.txt -unihan Unihan_Variants.txt
package main

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

func main() {
	pinyinData := flag.String("pinyin-data", "", "the folder of github.com/mozillazg/pinyin-data")
	phrases := flag.String("phrases", "", "pinyin.txt of github.com/mozillazg/phrase-pinyin-data")
	unihan := flag.String("unihan", "", "Unihan_Variants.txt of Unihan.zip")
	out := flag.String("o", "internal/romanize/pinyin.bin", "the file to write")
	flag.Parse()
	if *pinyinData == "" || *phrases == "" || *unihan == "" {
		flag.Usage()
		os.Exit(2)
	}
	// The readings of each character, the commonest first.
	readings := map[rune][]string{}
	for _, name := range []string{"kMandarin.txt", "kMandarin_overwrite.txt", "kMandarin_8105.txt"} {
		for r, v := range readCodes(filepath.Join(*pinyinData, name)) {
			if name == "kMandarin.txt" {
				readings[r] = v
				continue
			}
			// The adjustments choose the first reading.
			readings[r] = append([]string{v[0]}, slices.DeleteFunc(slices.Clone(readings[r]), func(s string) bool { return s == v[0] })...)
		}
	}
	tw := map[rune]string{}
	for r, v := range readCodes(filepath.Join(*pinyinData, "kMandarin.txt")) {
		if len(v) > 1 && v[1] != readings[r][0] {
			tw[r] = v[1]
		}
	}
	simplified := readVariants(*unihan)

	syllables := []string{}
	index := map[string]int{}
	syllable := func(s string) int {
		if i, ok := index[s]; ok {
			return i
		}
		index[s] = len(syllables)
		syllables = append(syllables, s)
		return index[s]
	}
	chars := slices.Sorted(func(yield func(rune) bool) {
		for r := range readings {
			if !yield(r) {
				return
			}
		}
	})
	for _, r := range chars {
		syllable(readings[r][0])
	}

	// The words read otherwise than their characters.
	type word struct {
		runes []rune
		read  []string
	}
	var words []word
	seen := map[string]bool{}
	f, err := os.Open(*phrases)
	if err != nil {
		log.Fatal(err)
	}
	sc := bufio.NewScanner(f)
	total := 0
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		text, read, ok := strings.Cut(line, ":")
		text = strings.TrimSpace(text)
		if !ok || text == "" || seen[text] {
			continue
		}
		seen[text] = true
		runes := []rune(text)
		syl := strings.Fields(read)
		if len(syl) != len(runes) || len(runes) < 2 || len(runes) > 255 {
			continue
		}
		total++
		differs := false
		for i, r := range runes {
			v, ok := readings[r]
			if !ok {
				// A character without a reading of its own takes the
				// word's.
				differs = true
				break
			}
			if v[0] != syl[i] {
				differs = true
			}
		}
		if differs {
			words = append(words, word{runes, syl})
		}
	}
	f.Close()
	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}
	slices.SortFunc(words, func(a, b word) int { return strings.Compare(string(a.runes), string(b.runes)) })

	var b bytes.Buffer
	put := func(v uint64) { b.Write(binary.AppendUvarint(nil, v)) }
	put(uint64(len(syllables)))
	for _, s := range syllables {
		put(uint64(len(s)))
		b.WriteString(s)
	}
	put(uint64(len(chars)))
	last := rune(0)
	for _, r := range chars {
		put(uint64(r - last))
		put(uint64(index[readings[r][0]]))
		last = r
	}
	twChars := slices.Sorted(func(yield func(rune) bool) {
		for r := range tw {
			if !yield(r) {
				return
			}
		}
	})
	put(uint64(len(twChars)))
	last = 0
	for _, r := range twChars {
		put(uint64(r - last))
		put(uint64(syllable(tw[r])))
		last = r
	}
	// Only the simplified forms of words kept matter.
	used := map[rune]bool{}
	for _, w := range words {
		for _, r := range w.runes {
			used[r] = true
		}
	}
	trad := slices.Sorted(func(yield func(rune) bool) {
		for r, s := range simplified {
			if used[s] && !yield(r) {
				return
			}
		}
	})
	put(uint64(len(trad)))
	last = 0
	for _, r := range trad {
		put(uint64(r - last))
		put(uint64(simplified[r]))
		last = r
	}
	put(uint64(len(words)))
	for _, w := range words {
		put(uint64(len(w.runes)))
		for _, r := range w.runes {
			put(uint64(r))
		}
		for _, s := range w.read {
			put(uint64(syllable(s)))
		}
	}
	// Syllables first named by the words go after the characters', which
	// read them all the same: the count before is that of the characters'.
	var head bytes.Buffer
	head.Write(binary.AppendUvarint(nil, uint64(len(syllables))))
	for _, s := range syllables {
		head.Write(binary.AppendUvarint(nil, uint64(len(s))))
		head.WriteString(s)
	}
	raw := append(head.Bytes(), b.Bytes()[skipSyllables(b.Bytes()):]...)

	var z bytes.Buffer
	w, err := flate.NewWriter(&z, flate.BestCompression)
	if err != nil {
		log.Fatal(err)
	}
	w.Write(raw)
	w.Close()
	if err := os.WriteFile(*out, z.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d syllables, %d characters (%d read otherwise in traditional), %d traditional forms, %d of %d words: %d bytes, %d compressed\n",
		len(syllables), len(chars), len(twChars), len(trad), len(words), total, len(raw), z.Len())
}

// skipSyllables returns where the syllables written first in b end.
func skipSyllables(b []byte) int {
	n, at := binary.Uvarint(b)
	for range n {
		l, k := binary.Uvarint(b[at:])
		at += k + int(l)
	}
	return at
}

// readCodes reads a file of pinyin-data: "U+4E2D: zhōng,zhòng  # 中".
func readCodes(name string) map[rune][]string {
	data, err := os.ReadFile(name)
	if err != nil {
		log.Fatal(err)
	}
	m := map[rune][]string{}
	for line := range strings.SplitSeq(string(data), "\n") {
		line, _, _ = strings.Cut(line, "#")
		code, read, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		var r rune
		if _, err := fmt.Sscanf(strings.TrimSpace(code), "U+%X", &r); err != nil {
			continue
		}
		v := strings.FieldsFunc(read, func(c rune) bool { return c == ',' || c == ' ' || c == '\t' || c == '\r' })
		if len(v) > 0 && utf8.ValidString(v[0]) {
			m[r] = v
		}
	}
	return m
}

// readVariants reads the simplified form of each traditional character
// of Unihan_Variants.txt: its first kSimplifiedVariant, not itself.
func readVariants(name string) map[rune]rune {
	data, err := os.ReadFile(name)
	if err != nil {
		log.Fatal(err)
	}
	m := map[rune]rune{}
	for line := range strings.SplitSeq(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || f[1] != "kSimplifiedVariant" {
			continue
		}
		var from, to rune
		if _, err := fmt.Sscanf(f[0], "U+%X", &from); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(f[2], "U+%X", &to); err != nil || to == from {
			continue
		}
		m[from] = to
	}
	return m
}
