//go:build unix

package history

import "syscall"

// noFollowFlag makes an open fail on a symlink at the last path component.
const noFollowFlag = syscall.O_NOFOLLOW
