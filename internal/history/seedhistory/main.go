// Command seedhistory writes two cost-timeline snapshots for the web e2e.
// The e2e module cannot import internal/history, so the test runs this
// package from the root module.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rshade/finfocus/internal/history"
)

const (
	seedArgCount = 4
	exitUsage    = 2
)

var errUsage = errors.New("usage: seedhistory db project stack urn")

func main() {
	if err := seed(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, errUsage) {
			os.Exit(exitUsage)
		}
		os.Exit(1)
	}
}

func seed(args []string) error {
	if len(args) != seedArgCount {
		return errUsage
	}
	db, err := history.OpenCostDBFor(args[0], args[1], args[2])
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	urn := args[3]
	snapshots := []struct {
		version int
		when    string
		total   float64
		kind    string
		message string
	}{
		{35, "2026-09-01T00:00:00Z", 8, "update", "baseline"},
		{36, "2026-10-01T00:00:00Z", 12.5, "update", "resized"},
	}
	for _, snapshot := range snapshots {
		when, parseErr := time.Parse(time.RFC3339, snapshot.when)
		if parseErr != nil {
			return parseErr
		}
		putErr := db.Put(history.CostSnapshot{
			Timestamp:     when,
			Version:       snapshot.version,
			TotalMonthly:  snapshot.total,
			Currency:      "USD",
			ResourceCount: 1,
			Resources: []history.CostResource{{
				URN:         urn,
				Type:        "aws:ec2/instance:Instance",
				Provider:    "aws",
				MonthlyCost: snapshot.total,
			}},
		}, history.CostAnnotation{
			Version: snapshot.version,
			Kind:    snapshot.kind,
			Message: snapshot.message,
		})
		if putErr != nil {
			return putErr
		}
	}
	return nil
}
