// Fixture plugin that behaves like a router: it spawns a long-lived child in its
// own process group membership and records both PIDs for the orphan test.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

type plugin struct {
	*pluginsdk.BasePlugin
}

func (p *plugin) Supports(context.Context, *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: true}, nil
}

func (p *plugin) GetProjectedCost(
	_ context.Context, _ *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
	_ = os.WriteFile(os.Getenv("ORPHAN_SERVED_FILE"), []byte("served"), 0o600)
	return &pbc.GetProjectedCostResponse{
		Currency:      "USD",
		CostPerMonth:  1,
		UnitPrice:     1,
		CostBreakdown: map[string]float64{"compute": 1},
		BillingDetail: "orphan fixture",
	}, nil
}

func main() {
	name := strings.TrimPrefix(os.Args[0][strings.LastIndex(os.Args[0], "/")+1:], "finfocus-plugin-")

	child := exec.Command("sleep", "600")
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "start child:", err)
		os.Exit(1)
	}
	pids := fmt.Sprintf("%d %d\n", os.Getpid(), child.Process.Pid)
	tmp := os.Getenv("ORPHAN_PID_FILE") + ".tmp"
	if err := os.WriteFile(tmp, []byte(pids), 0o600); err != nil {
		os.Exit(1)
	}
	if err := os.Rename(tmp, os.Getenv("ORPHAN_PID_FILE")); err != nil {
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger := zerolog.New(os.Stderr)
	p := &plugin{BasePlugin: pluginsdk.NewBasePlugin(name)}
	info := pluginsdk.NewPluginInfo(name, "0.1.0")
	if err := pluginsdk.Serve(ctx, pluginsdk.ServeConfig{Plugin: p, PluginInfo: info, Logger: &logger}); err != nil {
		os.Exit(1)
	}
}
