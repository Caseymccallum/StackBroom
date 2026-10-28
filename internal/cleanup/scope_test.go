package cleanup

import "testing"

func TestDeniedSystemRootsAreRejected(t *testing.T) {
	for _, path := range []string{`C:\Windows`, `C:\Windows\System32`, `C:\Program Files\App`, `C:\ProgramData\Vendor`} {
		if !isDeniedScope(path) {
			t.Errorf("protected root %q was not denied", path)
		}
	}
	if isDeniedScope(`C:\Users\Casey\AppData\Local\Temp`) {
		t.Error("a user temporary root was denied")
	}
}
