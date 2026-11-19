//go:build !windows

package procjob

// Adopt does nothing off Windows; see the package documentation.
func Adopt() error { return nil }
