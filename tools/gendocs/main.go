// Command gendocs writes the generated CLI reference pages. Run it from the
// repository root through `make docs-cli`. It is not part of a release.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rshade/finfocus/internal/clidocs"
)

func main() {
	out := flag.String("out", clidocs.OutputDir, "directory to write the pages to")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	root, err := clidocs.NewRoot()
	if err != nil {
		return err
	}
	pages, err := clidocs.Render(root)
	if err != nil {
		return err
	}
	return clidocs.Write(dir, pages)
}
