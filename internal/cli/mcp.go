package cli

import (
	"context"

	"github.com/rshade/ax-go"
	"github.com/rshade/ax-go/mcp"
	"github.com/spf13/cobra"
)

// mcpFlag is the root-local flag that serves finfocus as a stdio MCP server,
// a thin alias for the reserved mcp-server subcommand.
const mcpFlag = "mcp"

// mcpServerCommandName is the reserved subcommand ax-go's mcp.NewCommand mounts.
const mcpServerCommandName = "mcp-server"

const mcpServerExample = `  # Serve finfocus as an MCP server over stdio (what MCP clients launch)
  finfocus mcp-server

  # Equivalent root-flag entry point
  finfocus --mcp

  # Serve over streamable HTTP on loopback
  finfocus mcp-server --transport=http --addr=127.0.0.1:8080`

// mcpExclusion names a command that must never be exposed as an MCP tool.
type mcpExclusion struct {
	path   []string
	reason string
}

// mcpExcludedCommands are the commands withheld from the MCP tool list via
// ax-go's node-only mcp.Exclude: each stays in --help and __schema --as=ax, and
// its subcommands remain tools. Cobra's help command and pure group commands
// need no entry; ax-go skips them itself.
//
//nolint:gochecknoglobals // Immutable exclusion policy table.
var mcpExcludedCommands = []mcpExclusion{
	{
		path:   nil,
		reason: "opens the interactive overview dashboard, and with --mcp would start a server inside a tool call",
	},
	{
		path:   []string{"analyzer", "serve"},
		reason: "long-running Pulumi gRPC handshake; it would block the serialized MCP dispatcher forever",
	},
	{
		path:   []string{"setup"},
		reason: "interactive first-run wizard",
	},
	{
		path:   []string{"plugin", "init"},
		reason: "scaffolds a plugin project into the working directory; a developer action, not an agent query",
	},
}

// mcpServeFunc starts an MCP server for root. It is a variable so tests can
// observe the --mcp entry point without serving over the real stdio.
//
//nolint:gochecknoglobals // Test seam for the --mcp entry point.
var mcpServeFunc = func(ctx context.Context, root *cobra.Command, ver string) error {
	return mcp.Serve(ctx, root, mcp.WithVersion(ver))
}

// commandLifecycle is per-root state shared by the root hooks and the MCP entry
// points. While serving is true, every command execution is a tools/call
// dispatched by the MCP server, which reuses the server's logging session.
type commandLifecycle struct {
	session *loggingSession
	serving bool
}

// newMCPServerCmd returns ax-go's reserved mcp-server command with finfocus
// examples and the finfocus MCP exclusions applied while it serves.
func newMCPServerCmd(root *cobra.Command, ver string, lc *commandLifecycle) *cobra.Command {
	cmd := mcp.NewCommand(root, mcp.WithVersion(ver))
	cmd.Example = mcpServerExample
	serve := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		return lc.serveMCP(root, func() error { return serve(c, args) })
	}
	return cmd
}

// newSchemaCmd returns ax-go's __schema command, mounted up front so ax.Execute
// does not add its own, with the MCP exclusions applied to --as=mcp so the
// static tool list matches the live one.
func newSchemaCmd(root *cobra.Command, ver string) *cobra.Command {
	cmd := ax.NewSchemaCommand(root, ax.WithSchemaVersion(ver))
	emit := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if as, _ := c.Flags().GetString("as"); as == "mcp" {
			return withPositionalCommandsHidden(root, func() error { return emit(c, args) })
		}
		return emit(c, args)
	}
	return cmd
}

// hostsMCP reports whether cmd runs as part of an MCP server: a dispatched
// tools/call, or an entry point that starts the server.
func (lc *commandLifecycle) hostsMCP(cmd *cobra.Command) bool {
	return lc.serving || isMCPEntryPoint(cmd)
}

// runRootMCP handles the root command's MCP cases: during dispatch the root is
// not a tool (so --mcp cannot recurse), otherwise --mcp serves over stdio.
func (lc *commandLifecycle) runRootMCP(cmd *cobra.Command, ver string) error {
	if lc.serving {
		return errRootNotATool(cmd.Context())
	}
	root := cmd.Root()
	return lc.serveMCP(root, func() error {
		return mcpServeFunc(cmd.Context(), root, ver)
	})
}

// startLogging opens the logging session for a top-level invocation. A
// dispatched MCP tools/call reuses the server's session instead.
func (lc *commandLifecycle) startLogging(cmd *cobra.Command) {
	if lc.serving && lc.session != nil {
		lc.session.attach(cmd)
		return
	}
	lc.session = setupLogging(cmd)
}

// stopLogging closes the session opened by startLogging; dispatched MCP calls
// leave the server's session open.
func (lc *commandLifecycle) stopLogging() error {
	if lc.serving {
		return nil
	}
	return lc.session.Close()
}

// serveMCP runs serve with the lifecycle marked as serving, so dispatched calls
// share the already-open logging session.
func (lc *commandLifecycle) serveMCP(root *cobra.Command, serve func() error) error {
	lc.serving = true
	defer func() { lc.serving = false }()
	return withPositionalCommandsHidden(root, serve)
}

// applyMCPExclusions marks every mcpExcludedCommands entry with mcp.Exclude.
func applyMCPExclusions(root *cobra.Command) {
	for _, exclusion := range mcpExcludedCommands {
		mcp.Exclude(findSubcommand(root, exclusion.path))
	}
}

// withPositionalCommandsHidden hides leaf commands whose Args validator rejects
// an empty argument list while fn runs, then restores their Hidden values so
// --help is never affected. The live server already drops those commands (an
// MCP call cannot supply positional arguments); hiding them here keeps the
// static __schema --as=mcp list identical to the live tools/list.
func withPositionalCommandsHidden(root *cobra.Command, fn func() error) error {
	var hidden []*cobra.Command
	walkCommands(root, func(cmd *cobra.Command) {
		if !cmd.Hidden && !cmd.HasSubCommands() && cmd.Args != nil && cmd.Args(cmd, []string{}) != nil {
			cmd.Hidden = true
			hidden = append(hidden, cmd)
		}
	})
	defer func() {
		for _, cmd := range hidden {
			cmd.Hidden = false
		}
	}()
	return fn()
}

// findSubcommand returns the command at path below root, or nil if absent.
func findSubcommand(root *cobra.Command, path []string) *cobra.Command {
	cmd := root
	for _, name := range path {
		var next *cobra.Command
		for _, child := range cmd.Commands() {
			if child.Name() == name {
				next = child
				break
			}
		}
		if next == nil {
			return nil
		}
		cmd = next
	}
	return cmd
}

// walkCommands visits root and every descendant command.
func walkCommands(root *cobra.Command, visit func(*cobra.Command)) {
	visit(root)
	for _, child := range root.Commands() {
		walkCommands(child, visit)
	}
}

// isMCPEntryPoint reports whether cmd is an invocation that starts an MCP
// server: the mcp-server subcommand or the root command with --mcp.
func isMCPEntryPoint(cmd *cobra.Command) bool {
	if cmd.Name() == mcpServerCommandName {
		return true
	}
	if cmd != cmd.Root() {
		return false
	}
	serve, _ := cmd.Flags().GetBool(mcpFlag)
	return serve
}

// errRootNotATool is returned if the root command runs while serving. The root
// is excluded via mcp.Exclude, so MCP clients cannot call it; this guard keeps
// any nested execution from opening the overview dashboard or, with --mcp,
// starting a server inside a tool call.
func errRootNotATool(ctx context.Context) error {
	return ax.NewError(ctx, "validation_error",
		"the finfocus root command is not callable as an MCP tool; call a subcommand tool such as finfocus-overview",
		ax.WithErrorExitCode(ax.ExitValidation))
}
