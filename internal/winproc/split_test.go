package winproc

import (
	"reflect"
	"testing"
)

func TestSplitCommandLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "simple",
			in:   "node server.js",
			want: []string{"node", "server.js"},
		},
		{
			name: "quoted path with spaces",
			in:   `"C:\Program Files\nodejs\node.exe" "C:\My App\server.js"`,
			want: []string{`C:\Program Files\nodejs\node.exe`, `C:\My App\server.js`},
		},
		{
			name: "multiple spaces",
			in:   "node   --port    3000",
			want: []string{"node", "--port", "3000"},
		},
		{
			name: "escaped quote inside quotes",
			in:   `node "C:\a\"b.js"`,
			want: []string{"node", `C:\a"b.js`},
		},
		{
			name: "literal backslash before quote",
			in:   `node "C:\dir\\"`,
			want: []string{"node", `C:\dir\`},
		},
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "unterminated quote keeps content",
			in:   `node "C:\proj\server.js`,
			want: []string{"node", `C:\proj\server.js`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitCommandLine(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SplitCommandLine(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormaliseName(t *testing.T) {
	tests := map[string]string{
		"Node.EXE":       "node",
		"node.exe":       "node",
		"  python.exe  ": "python",
		"go":             "go",
	}
	for in, want := range tests {
		if got := NormaliseName(in); got != want {
			t.Errorf("NormaliseName(%q) = %q, want %q", in, got, want)
		}
	}
}
