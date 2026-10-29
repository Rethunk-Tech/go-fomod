package fomod

import (
	"bytes"
	"encoding/binary"
	"unicode/utf16"
	"unicode/utf8"
)

// decodeXML turns a document into UTF-8 by its bytes. A BOM picks the encoding. Without one, NUL is illegal in XML,
// so a NUL in the first byte pair means UTF-16, with or without a declaration (the four-byte sniff of XML 1.0
// Appendix F alone misses a declaration-less file).
func decodeXML(raw []byte) []byte {
	if len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE {
		return utf16ToUTF8(raw[2:], binary.LittleEndian)
	}
	if len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF {
		return utf16ToUTF8(raw[2:], binary.BigEndian)
	}
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		return raw[3:]
	}
	switch {
	case bytes.HasPrefix(raw, []byte("<\x00?\x00")), len(raw) >= 2 && raw[0] != 0 && raw[1] == 0:
		return utf16ToUTF8(raw, binary.LittleEndian)
	case bytes.HasPrefix(raw, []byte("\x00<\x00?")), len(raw) >= 2 && raw[0] == 0 && raw[1] != 0:
		return utf16ToUTF8(raw, binary.BigEndian)
	}
	return raw
}

// utf16ToUTF8 decodes UTF-16 in order's byte order, dropping a trailing odd byte.
func utf16ToUTF8(b []byte, order binary.ByteOrder) []byte {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = order.Uint16(b[i*2:])
	}
	return []byte(string(utf16.Decode(u)))
}

// cp1252High maps the bytes 0x80 to 0x9F of Windows-1252; every other byte maps to the code point of the same value.
// The five bytes Windows leaves undefined map to U+FFFD.
var cp1252High = [32]rune{
	0x20AC, 0xFFFD, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0xFFFD, 0x017D, 0xFFFD,
	0xFFFD, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0xFFFD, 0x017E, 0x0178,
}

// decodeLegacy reads bytes that are not valid UTF-8 as Windows-1252, the encoding legacy authoring tools wrote.
func decodeLegacy(b []byte) []byte {
	if utf8.Valid(b) {
		return b
	}
	out := make([]byte, 0, len(b)+len(b)/8)
	for _, c := range b {
		r := rune(c)
		if c >= 0x80 && c <= 0x9F {
			r = cp1252High[c-0x80]
		}
		out = utf8.AppendRune(out, r)
	}
	return out
}
