//go:build !windows

package sysinfo

import "context"

func collectSystem(ctx context.Context) Metrics {
	_ = ctx
	result := baseMetrics()
	result.WindowsVersion = "Windows only"
	result.Warnings = append(result.Warnings,
		"PC metrics are only available on Windows.")
	return result
}
