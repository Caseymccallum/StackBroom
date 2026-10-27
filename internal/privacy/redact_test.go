package privacy

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		secrets    []string
		stillThere []string
	}{
		{
			name:       "password flag with space",
			in:         `node server.js --password hunter2 --port 3000`,
			secrets:    []string{"hunter2"},
			stillThere: []string{"--port 3000", "--password"},
		},
		{
			name:       "token flag with equals",
			in:         `tool --token=abc123 --verbose`,
			secrets:    []string{"abc123"},
			stillThere: []string{"--verbose"},
		},
		{
			name:       "api key flag",
			in:         `tool --api-key sk-live-999`,
			secrets:    []string{"sk-live-999"},
			stillThere: []string{"--api-key"},
		},
		{
			name:       "environment assignment",
			in:         `set API_TOKEN=xyz789 && node server.js`,
			secrets:    []string{"xyz789"},
			stillThere: []string{"server.js"},
		},
		{
			name:       "database url assignment",
			in:         `DATABASE_URL=postgres://app:dbpass@db.internal:5432/app`,
			secrets:    []string{"dbpass"},
			stillThere: []string{"DATABASE_URL"},
		},
		{
			name:       "url userinfo",
			in:         `psql postgres://app:s3cret@db.internal/app`,
			secrets:    []string{"s3cret"},
			stillThere: []string{"db.internal", "postgres://app"},
		},
		{
			name:       "bearer token",
			in:         `curl -H "Authorization: Bearer abcdef123456"`,
			secrets:    []string{"abcdef123456"},
			stillThere: []string{"curl"},
		},
		{
			name:       "ordinary flags untouched",
			in:         `node server.js --port 3000 --host 127.0.0.1`,
			stillThere: []string{"--port 3000", "--host 127.0.0.1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := redactSecrets(tc.in)
			for _, secret := range tc.secrets {
				if strings.Contains(got, secret) {
					t.Errorf("secret %q survived: %q", secret, got)
				}
			}
			for _, keep := range tc.stillThere {
				if !strings.Contains(got, keep) {
					t.Errorf("expected %q in %q", keep, got)
				}
			}
		})
	}
}

func TestRedactPaths(t *testing.T) {
	home := `C:\Users\Casey`
	tests := []struct {
		in   string
		want string
	}{
		{`C:\Users\Casey\Web Apps\demo`, `%USERPROFILE%\Web Apps\demo`},
		{`C:/Users/Casey/Web Apps/demo`, `%USERPROFILE%/Web Apps/demo`},
		{`C:\Users\Alice\private`, `C:\Users\<user>\private`},
		{`/home/casey/project`, `/home/<user>/project`},
		{`C:\Windows\System32`, `C:\Windows\System32`},
	}
	for _, tc := range tests {
		got := redactPaths(tc.in, home)
		if got != tc.want {
			t.Errorf("redactPaths(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
