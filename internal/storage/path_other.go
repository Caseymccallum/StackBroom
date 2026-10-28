//go:build !windows

package storage

func platformPath(path string) string { return path }
