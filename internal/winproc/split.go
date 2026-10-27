package winproc

import "strings"

// SplitCommandLine splits a Windows command line using the CommandLineToArgvW
// rules that Windows runtimes follow:
//
//   - spaces and tabs separate arguments unless quoted;
//   - a double quote toggles quoting and is not part of the argument;
//   - backslashes are literal except immediately before a double quote, where
//     2n backslashes produce n backslashes and 2n+1 produce n backslashes plus
//     a literal quote.
//
// The function never executes anything; it only shapes text for inspection.
func SplitCommandLine(s string) []string {
	var (
		args     []string
		current  strings.Builder
		inQuotes bool
		haveArg  bool
	)

	flush := func() {
		if haveArg {
			args = append(args, current.String())
			current.Reset()
			haveArg = false
		}
	}

	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			if inQuotes {
				// Spaces inside quotes belong to the argument.
				current.WriteByte(c)
				haveArg = true
				i++
				continue
			}
			flush()
			i++
		case c == '"':
			inQuotes = !inQuotes
			haveArg = true
			i++
		case c == '\\':
			j := i
			for j < len(s) && s[j] == '\\' {
				j++
			}
			n := j - i
			if j < len(s) && s[j] == '"' {
				current.WriteString(strings.Repeat(`\`, n/2))
				haveArg = true
				if n%2 == 1 {
					current.WriteByte('"')
					j++
				}
				i = j
				continue
			}
			current.WriteString(strings.Repeat(`\`, n))
			haveArg = true
			i = j
		default:
			current.WriteByte(c)
			haveArg = true
			i++
		}
	}
	if inQuotes {
		// An unterminated quote is preserved as-is: better to show something
		// odd than to silently invent a different command line.
		flush()
		return args
	}
	flush()
	return args
}

// NormaliseName lowercases a Windows executable name and removes the `.exe`
// suffix, e.g. "Node.EXE" becomes "node".
func NormaliseName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, ".exe")
	return n
}

// tokenBase returns the lowercased file name of a token, so that
// `C:\Program Files\nodejs\npm.cmd` is recognised as `npm.cmd`.
func tokenBase(tok string) string {
	lower := strings.ToLower(strings.Trim(tok, `"'`))
	lower = strings.ReplaceAll(lower, "/", `\`)
	if idx := strings.LastIndex(lower, `\`); idx >= 0 {
		lower = lower[idx+1:]
	}
	return lower
}
