package playlistio

import (
	"bytes"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

// Decode reads the text of a file: UTF-8, with a byte order mark or
// without; UTF-16 of either order, with a mark or without one; else the
// code page of Windows the computer uses, or Windows-1252 off Windows,
// as spreadsheets save CSV files there.
func Decode(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return string(data[3:])
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return decodeWith(unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), data[2:])
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return decodeWith(unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM), data[2:])
	}
	if order, ok := utf16Order(data); ok {
		return decodeWith(unicode.UTF16(order, unicode.IgnoreBOM), data)
	}
	if utf8.Valid(data) {
		return string(data)
	}
	return decodeWith(codePage(systemCodePage()), data)
}

// utf16Order tells UTF-16 without a mark by its zero bytes: text of the
// Latin script has a zero in every other byte, the odd ones of little
// endian, the even ones of big endian.
func utf16Order(data []byte) (unicode.Endianness, bool) {
	n := min(len(data), 512) &^ 1
	if n < 4 {
		return unicode.BigEndian, false
	}
	var even, odd int
	for i := 0; i < n; i += 2 {
		if data[i] == 0 {
			even++
		}
		if data[i+1] == 0 {
			odd++
		}
	}
	half := n / 2
	switch {
	case odd*10 >= half*6 && even*10 < half:
		return unicode.LittleEndian, true
	case even*10 >= half*6 && odd*10 < half:
		return unicode.BigEndian, true
	}
	return unicode.BigEndian, false
}

func decodeWith(e encoding.Encoding, data []byte) string {
	out, err := e.NewDecoder().Bytes(data)
	if err != nil {
		return string(bytes.ToValidUTF8(data, []byte("\uFFFD")))
	}
	return string(out)
}

// codePage is the encoding of a Windows code page, Windows-1252 for one
// it doesn't know.
func codePage(cp int) encoding.Encoding {
	switch cp {
	case 874:
		return charmap.Windows874
	case 932:
		return japanese.ShiftJIS
	case 936:
		return simplifiedchinese.GBK
	case 949:
		return korean.EUCKR
	case 950:
		return traditionalchinese.Big5
	case 1250:
		return charmap.Windows1250
	case 1251:
		return charmap.Windows1251
	case 1253:
		return charmap.Windows1253
	case 1254:
		return charmap.Windows1254
	case 1255:
		return charmap.Windows1255
	case 1256:
		return charmap.Windows1256
	case 1257:
		return charmap.Windows1257
	case 1258:
		return charmap.Windows1258
	}
	return charmap.Windows1252
}
