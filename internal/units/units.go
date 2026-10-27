// Package units formats byte counts consistently across the analysis and
// presentation layers.
package units

import "fmt"

// Bytes renders a byte count using 1024-based units with the familiar
// MB/GB labels. StackBroom documents this convention so a figure is never
// mistaken for Docker's SI-based size text.
func Bytes(n uint64) string {
	const (
		kib = 1024
		mib = kib * 1024
		gib = mib * 1024
		tib = gib * 1024
	)
	switch {
	case n >= tib:
		return fmt.Sprintf("%.2f TB", float64(n)/float64(tib))
	case n >= gib:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(gib))
	case n >= mib:
		return fmt.Sprintf("%.0f MB", float64(n)/float64(mib))
	case n >= kib:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(kib))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
