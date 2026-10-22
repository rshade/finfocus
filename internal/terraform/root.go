package terraform

import (
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// HasRoot reports whether dir contains a regular file whose name ends in
// .tf or .tf.json. Subdirectories, symlinks, and other names do not count.
func HasRoot(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("reading terraform directory: %w", err)
	}
	return slices.ContainsFunc(entries, isTerraformFile), nil
}

func isTerraformFile(entry fs.DirEntry) bool {
	if !entry.Type().IsRegular() {
		return false
	}
	name := entry.Name()
	return strings.HasSuffix(name, ".tf.json") || strings.HasSuffix(name, ".tf")
}
