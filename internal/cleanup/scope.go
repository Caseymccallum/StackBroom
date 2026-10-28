package cleanup

import "strings"

// trustedRootForToken resolves a rule token against a Windows known folder,
// not merely against the process environment. Tests override this resolver
// with a disposable fixture; production never accepts an environment override
// as sufficient evidence.
var trustedRootForToken = platformTrustedRoot

func isDeniedScope(path string) bool {
	clean := strings.ToLower(strings.TrimRight(path, `\/`))
	denied := []string{
		`c:\windows`,
		`c:\program files`,
		`c:\program files (x86)`,
		`c:\programdata`,
		`c:\users\public`,
	}
	for _, prefix := range denied {
		if clean == prefix || strings.HasPrefix(clean, prefix+`\`) {
			return true
		}
	}
	return clean == `c:`
}
