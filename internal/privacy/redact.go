package privacy

import (
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

var (
	// Flag-style secrets: --password value, --token=value, --api-key value.
	// A following "-" is not consumed, so "--token --verbose" only masks the
	// token slot rather than the next flag.
	secretFlagPattern = regexp.MustCompile(`(?i)(--?[a-z0-9_.-]*(?:password|passwd|pwd|token|secret|api[-_]?key|apikey|access[-_]?key|private[-_]?key|client[-_]?secret|credential|connection[-_]?string|conn[-_]?str|dsn|authorization|auth)\b)(\s*=\s*|\s+)("[^"]*"|'[^']*'|[^\s-]\S*)`)

	// Environment-style assignments with sensitive names.
	secretEnvPattern = regexp.MustCompile(`(?i)\b([a-z0-9_]*(?:password|passwd|pwd|token|secret|api_?key|access_?key|private_?key|client_?secret|auth|credentials?|connection_?string|database_url|dsn)[a-z0-9_]*)\s*=\s*("[^"]*"|'[^']*'|[^\s-]\S*)`)

	// Credentials embedded in a URL: scheme://user:password@host.
	urlCredentialPattern = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.-]*://)([^/\s:@]+):([^/\s@]+)@`)

	// Bearer tokens in headers.
	bearerPattern = regexp.MustCompile(`(?i)\b(bearer\s+)([a-z0-9._~+/=-]{8,})`)
)

// RedactText masks secrets and tokenises the user's home directory.
func RedactText(value, home string) string { return redactText(value, home) }

func redactText(value, home string) string {
	if value == "" {
		return value
	}
	return redactPaths(redactSecrets(value), home)
}

// redactSecrets masks values that look like credentials. It is deliberately
// conservative: it masks the value after known sensitive flag names and
// environment-style assignments, URL userinfo passwords and bearer tokens. It
// does not attempt to detect arbitrary high-entropy strings, which would create
// false positives in ordinary paths and versions.
func redactSecrets(value string) string {
	out := secretFlagPattern.ReplaceAllString(value, "${1}${2}"+redacted)
	out = secretEnvPattern.ReplaceAllString(out, "${1}="+redacted)
	out = urlCredentialPattern.ReplaceAllString(out, "${1}${2}:"+redacted+"@")
	out = bearerPattern.ReplaceAllString(out, "${1}"+redacted)
	return out
}

// redactPaths replaces the user's home directory and other users' profile
// folders with stable placeholders.
func redactPaths(value, home string) string {
	if value == "" {
		return value
	}
	out := value
	if home = strings.TrimSpace(home); home != "" {
		for _, variant := range []string{home, strings.ReplaceAll(home, `\`, `/`)} {
			pattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(variant))
			out = pattern.ReplaceAllString(out, "%USERPROFILE%")
		}
	}
	// Any remaining drive:\Users\<name> path is redacted to <user> so other
	// accounts on the machine are not disclosed either.
	out = userPathPattern.ReplaceAllString(out, "${1}<user>")
	out = posixUserPathPattern.ReplaceAllString(out, "${1}<user>")
	return out
}

var (
	userPathPattern      = regexp.MustCompile(`(?i)([a-z]:[\\/]users[\\/])([^\\/\s"']+)`)
	posixUserPathPattern = regexp.MustCompile(`(?i)(/(?:home|users)/)([^/\s"']+)`)
)
