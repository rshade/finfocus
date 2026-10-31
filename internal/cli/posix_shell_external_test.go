package cli_test

import (
	"runtime"
	"testing"
)

// requirePOSIXShell skips a test that stubs an external command with a shell
// script, which a Windows runner cannot execute.
func requirePOSIXShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the test stubs an external command with a POSIX shell script")
	}
}
