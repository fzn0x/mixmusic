package lyrics

import (
	"regexp"
	"strings"
	"unicode"
)

// BilingualResult holds the primary lyrics lines paired line-for-line with
// an embedded translation and/or romanization extracted from bilingual text.
type BilingualResult struct {
	Lines       []Line
	Language    string
	Translation Translation
}

var (
	markedRomanRe = regexp.MustCompile(`^---\s*romanized\s*---$`)
	markedTransRe = regexp.MustCompile(`^---\s*translation(?:\s*\(\s*([A-Za-z0-9-]+)\s*\))?\s*---$`)
)

// PlainBilingual parses plain lyric text and extracts paired translation /
// romanization blocks when the text contains two or three aligned languages
// (stacked sections, divider-separated blocks, or interleaved pairs/triples).
// When the text is monolingual, it behaves identically to Plain.
func PlainBilingual(raw, source string) Lyrics {
	if res, ok := SplitBilingual(raw); ok {
		tr := res.Translation
		if tr.From == "" {
			tr.From = res.Language
		}
		return Lyrics{
			Lines:       res.Lines,
			Source:      source,
			Language:    res.Language,
			Translation: &tr,
		}
	}
	return Plain(raw, source)
}

// DetectLanguage detects lyrics language from lines via GuessLanguage plus
// romanized Japanese detection.
func DetectLanguage(lines []Line) string { return detectLanguage(lines) }

// splitMarkedTranslation extracts explicit "--- Romanized ---" and
// "--- Translation (<code>) ---" sections appended by Lyrics.LRC().
func splitMarkedTranslation(text string) (primary string, roman, trans []string, transTo string, hasMarkers bool) {
	normed := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	rawLines := strings.Split(normed, "\n")
	section := "primary"
	var pLines []string
	for _, line := range rawLines {
		low := strings.ToLower(strings.TrimSpace(line))
		if markedRomanRe.MatchString(low) {
			hasMarkers = true
			section = "roman"
			continue
		}
		if m := markedTransRe.FindStringSubmatch(low); m != nil {
			hasMarkers = true
			section = "trans"
			if m[1] != "" {
				transTo = languageCode(m[1])
			}
			continue
		}
		switch section {
		case "primary":
			pLines = append(pLines, line)
		case "roman":
			roman = append(roman, strings.TrimSpace(line))
		case "trans":
			trans = append(trans, strings.TrimSpace(line))
		}
	}
	if !hasMarkers {
		return text, nil, nil, "", false
	}
	for len(pLines) > 0 && strings.TrimSpace(pLines[len(pLines)-1]) == "" {
		pLines = pLines[:len(pLines)-1]
	}
	return strings.TrimSpace(strings.Join(pLines, "\n")), trimTrailingEmpty(roman), trimTrailingEmpty(trans), transTo, true
}

func trimTrailingEmpty(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// attachMarkedTranslation pairs parsed romanized and translated lines back onto l.
func attachMarkedTranslation(l *Lyrics, roman, trans []string, transTo string) {
	if l == nil || len(l.Lines) == 0 || (len(roman) == 0 && len(trans) == 0) {
		return
	}
	from := detectLanguage(l.Lines)
	if l.Language == "" {
		l.Language = from
	}
	tl := make([]TranslatedLine, len(l.Lines))
	mapLines := func(src []string, assign func(i int, s string)) {
		if len(src) == 0 {
			return
		}
		if len(src) == len(l.Lines) {
			for i, s := range src {
				assign(i, s)
			}
			return
		}
		// Match non-empty lines sequentially when blank-line spacing changed during editing.
		srcNE := nonEmptySlice(src)
		if len(srcNE) == nonEmptyLineCount(l.Lines) {
			k := 0
			for i, line := range l.Lines {
				if strings.TrimSpace(line.Text) != "" {
					assign(i, srcNE[k])
					k++
				}
			}
			return
		}
		for i := 0; i < min(len(src), len(l.Lines)); i++ {
			assign(i, src[i])
		}
	}
	mapLines(roman, func(i int, s string) { tl[i].Romanized = s })
	mapLines(trans, func(i int, s string) { tl[i].Text = s })

	hasAny := false
	for i, line := range l.Lines {
		if strings.TrimSpace(line.Text) == "" {
			tl[i] = TranslatedLine{}
			continue
		}
		if tl[i].Text != "" || tl[i].Romanized != "" {
			hasAny = true
		}
	}
	if !hasAny {
		return
	}
	if transTo == "" {
		var trLines []Line
		for _, s := range trans {
			if s != "" {
				trLines = append(trLines, Line{Text: s})
			}
		}
		transTo = detectLanguage(trLines)
	}
	l.Translation = &Translation{
		From:  from,
		To:    transTo,
		Lines: tl,
	}
}

// SplitBilingual detects bilingual lyrics laid out as stacked/sectioned
// blocks (2 or 3 sections) or interleaved line pairs/triples and returns
// the primary lines paired line-for-line with Translation.
func SplitBilingual(raw string) (BilingualResult, bool) {
	normed := strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	rawLines := strings.Split(normed, "\n")
	if len(rawLines) < 8 {
		return BilingualResult{}, false
	}

	if res, ok := splitStackedBilingual(rawLines); ok {
		return res, true
	}
	if res, ok := splitInterleavedBilingual(rawLines); ok {
		return res, true
	}
	return BilingualResult{}, false
}

type sectionHeader struct {
	lang string // ISO 639-1 or ""
	role string // "translation", "romanized", "original", or ""
}

type textBlock struct {
	header *sectionHeader
	lines  []string
}

func splitStackedBilingual(rawLines []string) (BilingualResult, bool) {
	blocks := segmentByHeadersAndDividers(rawLines)

	// Trim empty wrapper lines on each block and drop completely empty divider artifacts.
	var cleanBlocks []textBlock
	for _, b := range blocks {
		trimmed := trimEmptyEnds(b.lines)
		if nonEmptyCount(trimmed) == 0 {
			// Carry header forward onto next block if a divider sat between header and its lines.
			if b.header != nil && len(cleanBlocks) < 4 {
				cleanBlocks = append(cleanBlocks, textBlock{header: b.header})
			}
			continue
		}
		if len(cleanBlocks) > 0 && cleanBlocks[len(cleanBlocks)-1].header != nil && nonEmptyCount(cleanBlocks[len(cleanBlocks)-1].lines) == 0 {
			cleanBlocks[len(cleanBlocks)-1].lines = trimmed
			continue
		}
		cleanBlocks = append(cleanBlocks, textBlock{header: b.header, lines: trimmed})
	}
	// Remove trailing header-only blocks.
	for len(cleanBlocks) > 0 && nonEmptyCount(cleanBlocks[len(cleanBlocks)-1].lines) == 0 {
		cleanBlocks = cleanBlocks[:len(cleanBlocks)-1]
	}

	// If explicit headers/dividers didn't yield 2-3 blocks, check an unmarked 50/50 split.
	if len(cleanBlocks) < 2 || len(cleanBlocks) > 3 {
		if len(cleanBlocks) <= 1 {
			if half, ok := tryHalfSplit(rawLines); ok {
				cleanBlocks = half
			}
		}
		if len(cleanBlocks) < 2 || len(cleanBlocks) > 3 {
			return BilingualResult{}, false
		}
	}

	if len(cleanBlocks) == 3 {
		return combineThreeBlocks(cleanBlocks)
	}
	return combineTwoBlocks(cleanBlocks[0], cleanBlocks[1])
}

func segmentByHeadersAndDividers(rawLines []string) []textBlock {
	var blocks []textBlock
	cur := textBlock{}
	flush := func() {
		if cur.header != nil || len(cur.lines) > 0 {
			blocks = append(blocks, cur)
			cur = textBlock{}
		}
	}
	for i, raw := range rawLines {
		trimmed := strings.TrimSpace(raw)
		if isSectionDivider(trimmed) {
			if nonEmptyCount(cur.lines) > 0 {
				flush()
			}
			continue
		}
		if hdr, ok := parseLangHeader(trimmed); ok && looksLikeBoundaryHeader(rawLines, i, len(nonEmptyLines(cur.lines))) {
			if nonEmptyCount(cur.lines) > 0 || cur.header != nil {
				flush()
			}
			cur.header = &hdr
			continue
		}
		cur.lines = append(cur.lines, trimmed)
	}
	flush()
	return blocks
}

func looksLikeBoundaryHeader(rawLines []string, idx, prevNonEmpty int) bool {
	if prevNonEmpty >= 4 || idx <= 2 {
		return true
	}
	// Check if preceded by a blank line or divider and followed by multiple lines.
	if idx > 0 {
		prev := strings.TrimSpace(rawLines[idx-1])
		if prev == "" || isSectionDivider(prev) {
			return true
		}
	}
	return false
}

func isSectionDivider(s string) bool {
	if isDividerLine(s) {
		return true
	}
	if len([]rune(s)) < 3 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("-=_*~—•·─━│═", r) {
			return false
		}
	}
	return true
}

func parseLangHeader(s string) (sectionHeader, bool) {
	if s == "" || len([]rune(s)) > 48 {
		return sectionHeader{}, false
	}
	clean := strings.TrimSpace(s)
	clean = strings.Trim(clean, "-=_*~—•·─━═")
	clean = strings.TrimSpace(clean)
	if strings.HasPrefix(clean, "[") && strings.HasSuffix(clean, "]") {
		clean = clean[1 : len(clean)-1]
	} else if strings.HasPrefix(clean, "(") && strings.HasSuffix(clean, ")") {
		clean = clean[1 : len(clean)-1]
	} else if strings.HasPrefix(clean, "**") && strings.HasSuffix(clean, "**") && len(clean) > 4 {
		clean = clean[2 : len(clean)-2]
	}
	clean = strings.TrimRight(strings.TrimSpace(clean), ":：-–—|")
	low := strings.ToLower(strings.TrimSpace(clean))
	if low == "" {
		return sectionHeader{}, false
	}

	switch low {
	case "english", "english translation", "english translations", "english lyrics", "english ver", "english ver.", "english version", "eng", "eng trans", "eng translation", "en", "en translation", "en trans", "engsub":
		return sectionHeader{lang: "en", role: "translation"}, true
	case "translation", "translations", "trans", "translated", "translated lyrics", "tl", "sub", "subs":
		return sectionHeader{lang: "", role: "translation"}, true
	case "romaji", "rōmaji", "romanized", "romanised", "romanization", "romanisation", "romaji lyrics", "romaja", "pinyin", "hepburn":
		return sectionHeader{lang: "", role: "romanized"}, true
	case "original", "original lyrics", "lyrics", "native":
		return sectionHeader{lang: "", role: "original"}, true
	case "japanese", "japanese lyrics", "jp", "jpn", "ja", "nihongo", "kanji", "kana", "kanji/kana", "日本語", "歌詞", "日本語歌詞":
		return sectionHeader{lang: "ja", role: "original"}, true
	case "japanese translation", "jp translation", "和訳", "日本語訳":
		return sectionHeader{lang: "ja", role: "translation"}, true
	case "korean", "korean lyrics", "kr", "kor", "ko", "hangul", "hangeul", "한국어", "가사":
		return sectionHeader{lang: "ko", role: "original"}, true
	case "korean translation", "kr translation", "한국어 번역", "번역":
		return sectionHeader{lang: "ko", role: "translation"}, true
	case "chinese", "chinese lyrics", "cn", "zh", "hanzi", "中文", "華語", "华语", "中文歌詞", "中文歌词":
		return sectionHeader{lang: "zh", role: "original"}, true
	case "chinese translation", "cn translation", "中文翻譯", "中文翻译":
		return sectionHeader{lang: "zh", role: "translation"}, true
	case "spanish", "español", "espanol", "es", "spanish translation", "traducción", "traduccion", "traducción al español":
		role := "original"
		if strings.Contains(low, "trans") || strings.Contains(low, "traduc") {
			role = "translation"
		}
		return sectionHeader{lang: "es", role: role}, true
	case "french", "français", "francais", "fr", "french translation", "traduction", "traduction française":
		role := "original"
		if strings.Contains(low, "trans") || strings.Contains(low, "traduc") {
			role = "translation"
		}
		return sectionHeader{lang: "fr", role: role}, true
	case "german", "deutsch", "de", "german translation", "übersetzung":
		role := "original"
		if strings.Contains(low, "trans") || strings.Contains(low, "übers") {
			role = "translation"
		}
		return sectionHeader{lang: "de", role: role}, true
	case "portuguese", "português", "portugues", "pt", "portuguese translation", "tradução", "traducao":
		role := "original"
		if strings.Contains(low, "trans") || strings.Contains(low, "tradu") {
			role = "translation"
		}
		return sectionHeader{lang: "pt", role: role}, true
	case "russian", "русский", "ru", "russian translation", "перевод":
		role := "original"
		if strings.Contains(low, "trans") || strings.Contains(low, "перевод") {
			role = "translation"
		}
		return sectionHeader{lang: "ru", role: role}, true
	case "italian", "italiano", "it", "italian translation", "traduzione":
		role := "original"
		if strings.Contains(low, "trans") || strings.Contains(low, "traduz") {
			role = "translation"
		}
		return sectionHeader{lang: "it", role: role}, true
	case "indonesian", "indonesian translation", "terjemahan", "id":
		role := "original"
		if strings.Contains(low, "trans") || strings.Contains(low, "terjemah") {
			role = "translation"
		}
		return sectionHeader{lang: "id", role: role}, true
	}
	return sectionHeader{}, false
}

// tryHalfSplit checks an unmarked comment split cleanly into two equal/parallel
// halves (exact line count or stanza structure) written in two different languages.
func tryHalfSplit(rawLines []string) ([]textBlock, bool) {
	trimmed := trimEmptyEnds(rawLines)
	// Try splitting by paragraph groups if blank lines exist.
	paras := splitParagraphs(trimmed)
	if len(paras) >= 2 && len(paras)%2 == 0 {
		half := len(paras) / 2
		countsMatch := true
		for i := range half {
			if len(paras[i]) != len(paras[i+half]) {
				countsMatch = false
				break
			}
		}
		if countsMatch {
			b1 := joinParagraphs(paras[:half])
			b2 := joinParagraphs(paras[half:])
			if nonEmptyCount(b1) >= 4 && blocksDistinctLanguages(b1, b2) {
				return []textBlock{{lines: b1}, {lines: b2}}, true
			}
		}
	}
	ne := nonEmptySlice(trimmed)
	if len(ne) >= 8 && len(ne)%2 == 0 {
		half := len(ne) / 2
		if blocksDistinctLanguages(ne[:half], ne[half:]) {
			return []textBlock{{lines: ne[:half]}, {lines: ne[half:]}}, true
		}
	}
	return nil, false
}

func combineTwoBlocks(a, b textBlock) (BilingualResult, bool) {
	aNE := nonEmptySlice(a.lines)
	bNE := nonEmptySlice(b.lines)
	if len(aNE) < 4 || len(bNE) < 4 {
		return BilingualResult{}, false
	}

	orig, trans := a, b
	// If header explicitly marks block a as translation and block b as original/romaji, swap.
	if a.header != nil && a.header.role == "translation" && (b.header == nil || b.header.role != "translation") {
		orig, trans = b, a
		aNE, bNE = bNE, aNE
	}

	origLang := inferBlockLanguage(orig.lines, orig.header)
	transLang := inferBlockLanguage(trans.lines, trans.header)

	// Require two genuinely different languages (or explicit translation/romanized header on one side).
	explicitRole := (orig.header != nil && orig.header.role != "") || (trans.header != nil && trans.header.role != "")
	if origLang == transLang && origLang != "" && !explicitRole {
		return BilingualResult{}, false
	}
	if !blocksDistinctLanguages(orig.lines, trans.lines) && (!explicitRole || origLang == transLang && origLang != "") {
		return BilingualResult{}, false
	}

	// Only accept if line counts match or align closely across stanzas.
	paired, ok := pairAlignedLines(orig.lines, trans.lines)
	if !ok {
		return BilingualResult{}, false
	}

	onlyRoman := trans.header != nil && trans.header.role == "romanized"
	lines := make([]Line, len(orig.lines))
	trLines := make([]TranslatedLine, len(orig.lines))
	for i, l := range orig.lines {
		lines[i] = Line{Text: l}
		if strings.TrimSpace(l) == "" || paired[i] == "" {
			continue
		}
		if onlyRoman {
			trLines[i] = TranslatedLine{Romanized: paired[i]}
		} else {
			trLines[i] = TranslatedLine{Text: paired[i]}
		}
	}
	return BilingualResult{
		Lines:    lines,
		Language: origLang,
		Translation: Translation{
			From:  origLang,
			To:    transLang,
			Lines: trLines,
		},
	}, true
}

func combineThreeBlocks(blocks []textBlock) (BilingualResult, bool) {
	// Identify native original, romanized, and translation blocks.
	origIdx, romanIdx, transIdx := -1, -1, -1
	for i, b := range blocks {
		if b.header == nil {
			continue
		}
		switch b.header.role {
		case "romanized":
			if romanIdx < 0 {
				romanIdx = i
			}
		case "translation":
			if transIdx < 0 {
				transIdx = i
			}
		case "original":
			if origIdx < 0 {
				origIdx = i
			}
		}
	}
	// Heuristic order fallback for 3 blocks: [Native (non-Latin), Romaji, Translation].
	if origIdx < 0 && romanIdx < 0 && transIdx < 0 {
		l0 := detectLanguage(linesFromStrings(blocks[0].lines))
		l1 := detectLanguage(linesFromStrings(blocks[1].lines))
		l2 := detectLanguage(linesFromStrings(blocks[2].lines))
		if Romanizable(l0) && !Romanizable(l1) && !Romanizable(l2) {
			origIdx, romanIdx, transIdx = 0, 1, 2
		}
	}
	// Assign any single missing role from remaining indices.
	used := func(i int) bool { return i == origIdx || i == romanIdx || i == transIdx }
	remaining := func() int {
		for i := range 3 {
			if !used(i) {
				return i
			}
		}
		return -1
	}
	if origIdx < 0 {
		// Prefer a non-Latin block as native original if present.
		for i := range 3 {
			if !used(i) && Romanizable(detectLanguage(linesFromStrings(blocks[i].lines))) {
				origIdx = i
				break
			}
		}
		if origIdx < 0 {
			origIdx = remaining()
		}
	}
	if romanIdx < 0 {
		romanIdx = remaining()
	}
	if transIdx < 0 {
		transIdx = remaining()
	}
	if origIdx < 0 || romanIdx < 0 || transIdx < 0 {
		return BilingualResult{}, false
	}

	orig, roman, trans := blocks[origIdx], blocks[romanIdx], blocks[transIdx]
	pairedTrans, okT := pairAlignedLines(orig.lines, trans.lines)
	pairedRoman, okR := pairAlignedLines(orig.lines, roman.lines)
	if !okT && !okR {
		return BilingualResult{}, false
	}
	if !okT {
		return combineTwoBlocks(orig, roman)
	}

	origLang := inferBlockLanguage(orig.lines, orig.header)
	transLang := inferBlockLanguage(trans.lines, trans.header)
	if origLang == transLang && origLang != "" {
		return BilingualResult{}, false
	}

	lines := make([]Line, len(orig.lines))
	trLines := make([]TranslatedLine, len(orig.lines))
	for i, l := range orig.lines {
		lines[i] = Line{Text: l}
		if strings.TrimSpace(l) == "" {
			continue
		}
		var tl TranslatedLine
		if okT {
			tl.Text = pairedTrans[i]
		}
		if okR {
			tl.Romanized = pairedRoman[i]
		}
		trLines[i] = tl
	}
	return BilingualResult{
		Lines:    lines,
		Language: origLang,
		Translation: Translation{
			From:  origLang,
			To:    transLang,
			Lines: trLines,
		},
	}, true
}

// pairAlignedLines aligns non-empty lines from partner onto orig's line layout
// (keeping orig's blank lines), allowing exact counts or close stanza-matched
// counts without drifting later stanzas out of sync.
func pairAlignedLines(origLines, partnerLines []string) ([]string, bool) {
	origNE := nonEmptySlice(origLines)
	partNE := nonEmptySlice(partnerLines)
	n1, n2 := len(origNE), len(partNE)
	if n1 < 4 || n2 < 4 {
		return nil, false
	}

	alignedNE := make([]string, n1)
	if n1 == n2 {
		copy(alignedNE, partNE)
		return mapNonEmptyBack(origLines, alignedNE), true
	}

	// Allow slight mismatch (e.g. a split/merged line in one stanza) only when
	// paragraph/stanza counts match and total mismatch is small.
	diff := n1 - n2
	if diff < 0 {
		diff = -diff
	}
	if diff > max(2, n1/10) {
		return nil, false
	}

	origParas := splitParagraphs(trimEmptyEnds(origLines))
	partParas := splitParagraphs(trimEmptyEnds(partnerLines))
	if len(origParas) != len(partParas) || len(origParas) < 2 {
		return nil, false
	}

	k := 0
	for pIdx := range origParas {
		op := origParas[pIdx]
		pp := partParas[pIdx]
		d := len(op) - len(pp)
		if d < 0 {
			d = -d
		}
		if d > 1 {
			return nil, false
		}
		for i := range op {
			if i < len(pp) {
				alignedNE[k] = pp[i]
			}
			k++
		}
	}
	return mapNonEmptyBack(origLines, alignedNE), true
}

func mapNonEmptyBack(origLines, alignedNE []string) []string {
	out := make([]string, len(origLines))
	k := 0
	for i, l := range origLines {
		if strings.TrimSpace(l) != "" && k < len(alignedNE) {
			out[i] = alignedNE[k]
			k++
		}
	}
	return out
}

func splitInterleavedBilingual(rawLines []string) (BilingualResult, bool) {
	trimmed := trimEmptyEnds(rawLines)
	paras := splitParagraphs(trimmed)

	// Case 1: Blank-line-separated 2-line pairs or 3-line triples (Native / Romaji / English).
	if len(paras) >= 4 {
		twoCount, threeCount := 0, 0
		for _, p := range paras {
			switch len(p) {
			case 2:
				twoCount++
			case 3:
				threeCount++
			}
		}
		if threeCount == len(paras) {
			return extractInterleavedTriples(paras)
		}
		if twoCount*10 >= len(paras)*8 {
			if res, ok := extractInterleavedPairs(paras); ok {
				return res, true
			}
		}
	}

	// Case 2: Contiguous even-line alternating pairs within paragraphs.
	nonEmpty := nonEmptySlice(trimmed)
	if len(nonEmpty) >= 8 && len(nonEmpty)%2 == 0 {
		// Build single-pair paragraphs preserving original stanza breaks between double pairs.
		if res, ok := extractContiguousAlternating(paras); ok {
			return res, true
		}
	}
	return BilingualResult{}, false
}

func extractInterleavedTriples(paras [][]string) (BilingualResult, bool) {
	var l0, l1, l2 []string
	nonLatinFirst := 0
	for _, p := range paras {
		l0 = append(l0, p[0])
		l1 = append(l1, p[1])
		l2 = append(l2, p[2])
		if hasNonLatinLetter(p[0]) && !hasNonLatinLetter(p[1]) && !hasNonLatinLetter(p[2]) {
			nonLatinFirst++
		}
	}
	if nonLatinFirst*5 < len(paras)*4 {
		return BilingualResult{}, false
	}
	origLang := detectLanguage(linesFromStrings(l0))
	transLang := detectLanguage(linesFromStrings(l2))
	if origLang == "" || origLang == transLang {
		return BilingualResult{}, false
	}
	lines := make([]Line, len(l0))
	trLines := make([]TranslatedLine, len(l0))
	for i := range l0 {
		lines[i] = Line{Text: l0[i]}
		trLines[i] = TranslatedLine{Text: unwrapParenthetical(l2[i]), Romanized: l1[i]}
	}
	return BilingualResult{
		Lines:    lines,
		Language: origLang,
		Translation: Translation{
			From:  origLang,
			To:    transLang,
			Lines: trLines,
		},
	}, true
}

func extractInterleavedPairs(paras [][]string) (BilingualResult, bool) {
	var aList, bList []string
	for _, p := range paras {
		if len(p) != 2 {
			return BilingualResult{}, false
		}
		aList = append(aList, p[0])
		bList = append(bList, p[1])
	}
	return buildInterleavedPairResult(aList, bList, nil)
}

func extractContiguousAlternating(paras [][]string) (BilingualResult, bool) {
	var aList, bList []string
	var breaks []bool
	for _, p := range paras {
		if len(p)%2 != 0 {
			return BilingualResult{}, false
		}
		if len(aList) > 0 {
			breaks = append(breaks, true)
		}
		for i := 0; i < len(p); i += 2 {
			aList = append(aList, p[i])
			bList = append(bList, p[i+1])
			if i+2 < len(p) {
				breaks = append(breaks, false)
			}
		}
	}
	return buildInterleavedPairResult(aList, bList, breaks)
}

func buildInterleavedPairResult(aList, bList []string, breaks []bool) (BilingualResult, bool) {
	n := len(aList)
	if n < 4 || n != len(bList) {
		return BilingualResult{}, false
	}
	// Check either script alternation or wrapped parenthetical translations.
	nonLatinA, nonLatinB := 0, 0
	parenB := 0
	sameLine := 0
	for i := range n {
		nlA := hasNonLatinLetter(aList[i])
		nlB := hasNonLatinLetter(bList[i])
		if nlA && !nlB {
			nonLatinA++
		}
		if !nlA && nlB {
			nonLatinB++
		}
		if u := unwrapParenthetical(bList[i]); u != bList[i] && len([]rune(u)) >= 8 && !isAdlib(u) {
			parenB++
		}
		if strings.EqualFold(strings.TrimSpace(aList[i]), strings.TrimSpace(bList[i])) {
			sameLine++
		}
	}
	if sameLine*3 >= n {
		return BilingualResult{}, false
	}

	scriptAlt := max(nonLatinA, nonLatinB)*100 >= n*80
	parenAlt := parenB*100 >= n*80
	aLang := detectLanguage(linesFromStrings(aList))
	bLang := detectLanguage(linesFromStrings(bList))
	langAlt := aLang != "" && bLang != "" && aLang != bLang && blocksDistinctLanguages(aList, bList)
	if !scriptAlt && !parenAlt && !langAlt {
		return BilingualResult{}, false
	}
	if nonLatinB > nonLatinA && aLang != "" && bLang != aLang {
		// Unusual Translation-then-Original layout? Keep A first unless B is clearly non-Latin and A is target English.
		_ = bLang
	}

	var lines []Line
	var trLines []TranslatedLine
	for i := range n {
		if i > 0 && len(breaks) == n-1 && breaks[i-1] {
			lines = append(lines, Line{Text: ""})
			trLines = append(trLines, TranslatedLine{})
		}
		lines = append(lines, Line{Text: aList[i]})
		bText := bList[i]
		if parenAlt {
			bText = unwrapParenthetical(bText)
		}
		trLines = append(trLines, TranslatedLine{Text: bText})
	}
	if bLang == aLang {
		bLang = detectLanguage(linesFromStrings(bList))
	}
	return BilingualResult{
		Lines:    lines,
		Language: aLang,
		Translation: Translation{
			From:  aLang,
			To:    bLang,
			Lines: trLines,
		},
	}, true
}

func unwrapParenthetical(s string) string {
	t := strings.TrimSpace(s)
	if strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")") && len(t) > 2 {
		return strings.TrimSpace(t[1 : len(t)-1])
	}
	if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") && len(t) > 2 {
		return strings.TrimSpace(t[1 : len(t)-1])
	}
	return t
}

func isAdlib(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	switch low {
	case "yeah", "oh", "ooh", "ah", "uh", "huh", "hey", "la la", "x2", "x3", "x4":
		return true
	}
	return len(strings.Fields(low)) <= 1
}

func inferBlockLanguage(lines []string, hdr *sectionHeader) string {
	if hdr != nil && hdr.lang != "" {
		return hdr.lang
	}
	return detectLanguage(linesFromStrings(lines))
}

func detectLanguage(lines []Line) string {
	if lang := GuessLanguage(lines); lang != "" {
		return lang
	}
	strs := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l.Text) != "" {
			strs = append(strs, l.Text)
		}
	}
	if looksLikeRomaji(strs) {
		return "ja"
	}
	return ""
}

func blocksDistinctLanguages(a, b []string) bool {
	aNonLat, bNonLat := nonLatinLineShare(a), nonLatinLineShare(b)
	if (aNonLat >= 0.6 && bNonLat <= 0.2) || (bNonLat >= 0.6 && aNonLat <= 0.2) {
		return true
	}
	la := detectLanguage(linesFromStrings(a))
	lb := detectLanguage(linesFromStrings(b))
	if la != "" && lb != "" && la != lb {
		return true
	}
	return looksLikeRomaji(a) != looksLikeRomaji(b)
}

func nonLatinLineShare(lines []string) float64 {
	total, nonLat := 0, 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		total++
		if hasNonLatinLetter(l) {
			nonLat++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(nonLat) / float64(total)
}

func hasNonLatinLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r) {
			return true
		}
	}
	return false
}

var romajiParticles = map[string]bool{
	"wa": true, "wo": true, "ga": true, "ni": true, "kara": true, "dakara": true,
	"kedo": true, "zutto": true, "itsumo": true, "kono": true,
	"sono": true, "ano": true, "koko": true, "soko": true, "kimi": true, "boku": true,
	"watashi": true, "kokoro": true, "namida": true, "sora": true, "yoru": true, "hikari": true,
	"yami": true, "koe": true, "ame": true, "kaze": true, "yume": true, "uta": true,
	"tsubasa": true, "tenshi": true, "eien": true, "hitori": true, "sekai": true, "inochi": true,
	"kizu": true, "mune": true, "kagami": true, "subete": true, "ashita": true, "anata": true,
	"ima": true, "nani": true, "naze": true, "tada": true, "tooku": true, "shizuka": true,
	"hibiku": true, "motome": true, "inoru": true, "warau": true, "matsu": true,
	"nureru": true, "sasayaku": true, "kumo": true, "hashigo": true, "aoi": true, "jimen": true,
}

// looksLikeRomaji detects romanized Japanese lyric blocks while rejecting English
// and common European languages.
func looksLikeRomaji(lines []string) bool {
	if nonLatinLineShare(lines) >= 0.25 {
		return false
	}
	if lang := GuessLanguage(linesFromStrings(lines)); lang == "en" || lang == "es" || lang == "pt" || lang == "fr" || lang == "de" || lang == "it" {
		return false
	}
	words, hits, macrons, hepburn := 0, 0, 0, 0
	for _, line := range lines {
		for _, r := range line {
			if strings.ContainsRune("āīūēōĀĪŪĒŌ", r) {
				macrons++
			}
		}
		normed := strings.NewReplacer(
			"ā", "a", "ī", "i", "ū", "u", "ē", "e", "ō", "o",
			"’", "'",
		).Replace(strings.ToLower(line))
		for _, w := range strings.FieldsFunc(normed, func(r rune) bool { return !unicode.IsLetter(r) }) {
			if w == "" {
				continue
			}
			words++
			if romajiParticles[w] {
				hits++
			}
			if strings.Contains(w, "shi") || strings.Contains(w, "chi") || strings.Contains(w, "tsu") ||
				strings.Contains(w, "sha") || strings.Contains(w, "shu") || strings.Contains(w, "sho") ||
				strings.Contains(w, "kya") || strings.Contains(w, "kyu") || strings.Contains(w, "kyo") {
				hepburn++
			}
		}
	}
	if words < 10 {
		return false
	}
	return hits >= 3 || (macrons >= 2 && (hits >= 1 || hepburn >= 2)) || (hepburn*10 >= words*2 && hits >= 2)
}

func linesFromStrings(strs []string) []Line {
	out := make([]Line, len(strs))
	for i, s := range strs {
		out[i] = Line{Text: s}
	}
	return out
}

func splitParagraphs(lines []string) [][]string {
	var paras [][]string
	var cur []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			if len(cur) > 0 {
				paras = append(paras, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 {
		paras = append(paras, cur)
	}
	return paras
}

func joinParagraphs(paras [][]string) []string {
	var out []string
	for i, p := range paras {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, p...)
	}
	return out
}

func trimEmptyEnds(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	res := make([]string, end-start)
	for i := start; i < end; i++ {
		res[i-start] = strings.TrimSpace(lines[i])
	}
	return res
}

func nonEmptySlice(lines []string) []string {
	var out []string
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func nonEmptyLineCount(lines []Line) int {
	n := 0
	for _, l := range lines {
		if strings.TrimSpace(l.Text) != "" {
			n++
		}
	}
	return n
}
