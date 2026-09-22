//go:build linux

package pluginhost

import (
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigureProcessGroup_SetsPdeathsig(t *testing.T) {
	cmd := exec.Command("sleep", "1")
	configureProcessGroup(cmd)
	assert.Equal(t, syscall.SIGKILL, cmd.SysProcAttr.Pdeathsig,
		"plugin must receive SIGKILL when Core dies (issue #1231)")
}
