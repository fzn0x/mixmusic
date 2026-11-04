package romanize

import _ "embed"

// UnihanLicense is the Unicode License of Unihan, whose readings of
// Chinese characters and simplified forms pinyin.bin holds: copies of
// MixMusic must reproduce it.
//
//go:embed LICENSE.unicode
var UnihanLicense string

// PinyinLicense is the MIT license of mozillazg's pinyin-data and
// phrase-pinyin-data, the readings of characters and of words that
// pinyin.bin holds.
var PinyinLicense = "pinyin-data\n\n" + pinyinDataLicense + "\nphrase-pinyin-data\n\n" + phrasePinyinDataLicense

var (
	//go:embed LICENSE.pinyin-data
	pinyinDataLicense string
	//go:embed LICENSE.phrase-pinyin-data
	phrasePinyinDataLicense string
)
