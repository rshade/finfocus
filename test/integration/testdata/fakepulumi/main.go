// Fixture stand-in for the pulumi CLI. `stack export` prints FAKEPULUMI_STATE.
// `preview` starts a long-lived child, records both pids in FAKEPULUMI_PID_FILE
// and then waits, as a real preview does while it runs the language host.
// With FAKEPULUMI_IGNORE_TERM=1 it also ignores interrupt and terminate.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if os.Getenv("FAKEPULUMI_CHILD") == "1" {
		signal.Ignore(syscall.SIGINT, syscall.SIGTERM)
		time.Sleep(10 * time.Minute)
		return
	}
	args := strings.Join(os.Args[1:], " ")
	switch {
	case strings.HasPrefix(args, "stack ls"):
		fmt.Print(`[{"name":"dev","current":true}]`)
	case strings.HasPrefix(args, "stack export"):
		data, err := os.ReadFile(os.Getenv("FAKEPULUMI_STATE"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_, _ = os.Stdout.Write(data)
	case strings.HasPrefix(args, "preview"):
		preview()
	default:
		fmt.Fprintln(os.Stderr, "unexpected fixture command:", args)
		os.Exit(1)
	}
}

func preview() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	child := exec.Command(os.Args[0])
	child.Env = append(os.Environ(), "FAKEPULUMI_CHILD=1")
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "start child:", err)
		os.Exit(1)
	}
	pids := fmt.Sprintf("%d %d\n", os.Getpid(), child.Process.Pid)
	if err := os.WriteFile(os.Getenv("FAKEPULUMI_PID_FILE"), []byte(pids), 0o600); err != nil {
		os.Exit(1)
	}

	for range signals {
		if os.Getenv("FAKEPULUMI_IGNORE_TERM") == "1" {
			continue
		}
		_ = child.Process.Kill()
		os.Exit(130)
	}
}
