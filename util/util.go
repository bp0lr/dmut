// Package util contains file and string helpers used by dmut.
package util

import (
	"os"
	"path/filepath"
	"strings"
)

// Insert inserts value at index using the existing slice storage when possible.
func Insert(a []string, index int, value string) []string {
	if len(a) == index {
		return append(a, value)
	}
	a = append(a[:index+1], a[index:]...)
	a[index] = value
	return a
}

// TrimLastPoint removes a trailing suffix.
func TrimLastPoint(s, suffix string) string { return strings.TrimSuffix(s, suffix) }

// RemoveDuplicatesSlice removes duplicate strings. Output order is unspecified.
func RemoveDuplicatesSlice(s []string) []string {
	m := make(map[string]struct{}, len(s))
	for _, item := range s {
		m[item] = struct{}{}
	}
	result := make([]string, 0, len(m))
	for item := range m {
		result = append(result, item)
	}
	return result
}

// TrimChars removes trailing dots.
func TrimChars(s string) string { return strings.TrimRight(s, ".") }

// GetDir returns the configuration directory without creating it.
func GetDir() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".dmut"), nil
}
