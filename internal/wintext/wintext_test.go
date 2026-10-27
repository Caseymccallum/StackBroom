package wintext

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func utf16leBytes(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(units)*2)
	for _, u := range units {
		var buf [2]byte
		binary.LittleEndian.PutUint16(buf[:], u)
		out = append(out, buf[:]...)
	}
	return out
}

func utf16beBytes(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(units)*2)
	for _, u := range units {
		var buf [2]byte
		binary.BigEndian.PutUint16(buf[:], u)
		out = append(out, buf[:]...)
	}
	return out
}

func TestDecodeUTF16LEStopsAtNUL(t *testing.T) {
	raw := append(utf16leBytes("node.exe"), 0x00, 0x00, 'x', 0x00)
	if got := DecodeUTF16LE(raw); got != "node.exe" {
		t.Fatalf("DecodeUTF16LE = %q, want %q", got, "node.exe")
	}
}

func TestDecodeConsole(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{
			name: "utf16le without bom (wsl.exe)",
			in:   utf16leBytes("  NAME    STATE\r\n  Ubuntu  Running  2\r\n"),
			want: "  NAME    STATE\r\n  Ubuntu  Running  2\r\n",
		},
		{
			name: "utf16le with bom",
			in:   append([]byte{0xFF, 0xFE}, utf16leBytes("Images 15.64GB")...),
			want: "Images 15.64GB",
		},
		{
			name: "utf16be with bom",
			in:   append([]byte{0xFE, 0xFF}, utf16beBytes("Images 15.64GB")...),
			want: "Images 15.64GB",
		},
		{
			name: "utf8 with bom",
			in:   append([]byte{0xEF, 0xBB, 0xBF}, []byte("12.41GB (79%)")...),
			want: "12.41GB (79%)",
		},
		{
			name: "plain ascii",
			in:   []byte("Build Cache 630.1MB 42.68MB"),
			want: "Build Cache 630.1MB 42.68MB",
		},
		{
			name: "empty",
			in:   nil,
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DecodeConsole(tc.in); got != tc.want {
				t.Fatalf("DecodeConsole() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDecodeConsoleDoesNotMistakeASCIIForUTF16(t *testing.T) {
	in := []byte("TYPE  TOTAL  ACTIVE  SIZE  RECLAIMABLE")
	if got := DecodeConsole(in); got != string(in) {
		t.Fatalf("ASCII input was decoded as UTF-16: %q", got)
	}
}
