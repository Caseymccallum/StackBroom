package units

import "testing"

func TestBytes(t *testing.T) {
	tests := []struct {
		in   uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1 KB"},
		{5 * 1024 * 1024, "5 MB"},
		{312 * 1024 * 1024, "312 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{15 * 1024 * 1024 * 1024, "15.00 GB"},
		{2 * 1024 * 1024 * 1024 * 1024, "2.00 TB"},
	}
	for _, tc := range tests {
		if got := Bytes(tc.in); got != tc.want {
			t.Errorf("Bytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
