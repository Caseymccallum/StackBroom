// Package wintext decodes Windows console output.
//
// Windows tools are inconsistent about encoding: wsl.exe writes UTF-16LE with
// no byte-order mark, PowerShell writes the console code page unless told
// otherwise, and redirected output may carry a BOM. These helpers normalise
// that without guessing at content.
package wintext

import (
	"bytes"
	"encoding/binary"
	"unicode/utf16"
	"unicode/utf8"
)

// DecodeUTF16LE converts little-endian UTF-16 bytes to a string, stopping at
// the first NUL code unit (which is how fixed-size Windows buffers terminate).
func DecodeUTF16LE(b []byte) string {
	units := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u := binary.LittleEndian.Uint16(b[i:])
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}

// DecodeConsole normalises redirected console output.
//
// Order of checks:
//  1. UTF-16 byte-order mark.
//  2. UTF-8 byte-order mark.
//  3. UTF-16LE detected by NUL bytes in odd positions (wsl.exe's default).
//  4. UTF-8, falling back to Latin-1-ish byte preservation for invalid input.
//
// Invalid UTF-8 is never silently dropped; each invalid byte is kept as its
// Latin-1 equivalent so that parsing sees the original shape of the data.
func DecodeConsole(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if len(b) >= 2 {
		switch {
		case b[0] == 0xFF && b[1] == 0xFE:
			return DecodeUTF16LE(b[2:])
		case b[0] == 0xFE && b[1] == 0xFF:
			return decodeUTF16BE(b[2:])
		}
	}
	if looksUTF16LE(b) {
		return DecodeUTF16LE(b)
	}
	s := string(b)
	if utf8.ValidString(s) {
		return s
	}
	var out []rune
	for _, r := range s {
		if r == utf8.RuneError {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// looksUTF16LE reports whether the byte slice has the signature of UTF-16LE
// text: plenty of NUL high bytes and very few NUL low bytes.
func looksUTF16LE(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	sample := b
	if len(sample) > 512 {
		sample = sample[:512]
	}
	var evenNUL, oddNUL int
	pairs := len(sample) / 2
	if pairs == 0 {
		return false
	}
	for i := 0; i+1 < len(sample); i += 2 {
		if sample[i] == 0 {
			evenNUL++
		}
		if sample[i+1] == 0 {
			oddNUL++
		}
	}
	return oddNUL*4 >= pairs && evenNUL*10 <= pairs
}

func decodeUTF16BE(b []byte) string {
	units := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u := binary.BigEndian.Uint16(b[i:])
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}
