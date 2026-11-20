//go:build windows

package procjob

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	adoptOnce sync.Once      //nolint:gochecknoglobals // One job per process.
	adoptErr  error          //nolint:gochecknoglobals // Result of the single adoption.
	jobHandle windows.Handle //nolint:gochecknoglobals // Held for the life of the process; closing it kills the job.
)

// Adopt places the current process in a job object that kills every member
// when its last handle closes. Children started afterward inherit membership,
// so plugins, their descendants, and the Pulumi child all die with finfocus,
// including when finfocus is terminated without running any cleanup. The
// handle is held for the life of the process and never closed by this
// package, so the kernel closing it at exit is what triggers the kill.
//
// Adopt is idempotent and returns the first call's result. It must run before
// the first child starts, since membership applies to processes created after
// it. A process that cannot be assigned (for example one in a job that
// forbids nesting) leaves the previous behavior in place and returns an error.
func Adopt() error {
	adoptOnce.Do(func() {
		adoptErr = createAndAssign()
	})
	return adoptErr
}

func createAndAssign() error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("creating job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return fmt.Errorf("configuring job object: %w", err)
	}
	if err = windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
		_ = windows.CloseHandle(job)
		return fmt.Errorf("assigning process to job object: %w", err)
	}
	jobHandle = job
	return nil
}
