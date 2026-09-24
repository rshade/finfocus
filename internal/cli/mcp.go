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

// mcpExcludedCommands are the leaf commands withheld from the MCP tool list.
//
// Interim mechanism for ax-go v0.6.0: exclusion is done by setting Hidden on
// these leaves only while an MCP tool list is being built, because ax-go prunes
// a hidden command's whole subtree. The root command, Cobra's help command, and
// pure group commands therefore cannot be excluded this way and remain listed
// until finfocus adopts ax-go's node-only mcp.Exclude annotation, at which point
// this list becomes mcp.Exclude calls and withMCPExclusions goes away.
//
//nolint:gochecknoglobals // Immutable exclusion policy table.
var mcpExcludedCommands = []mcpExclusion{
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
			return withMCPExclusions(root, func() error { return emit(c, args) })
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

// serveMCP runs serve with the MCP exclusions applied and the lifecycle marked
// as serving, so dispatched calls share the already-open logging session.
func (lc *commandLifecycle) serveMCP(root *cobra.Command, serve func() error) error {
	lc.serving = true
	defer func() { lc.serving = false }()
	return withMCPExclusions(root, serve)
}

// withMCPExclusions hides every command that must not be an MCP tool while fn
// runs and restores the original Hidden values afterwards, so normal --help
// output is never affected.
//
// Besides mcpExcludedCommands it hides leaf commands whose Args validator
// rejects an empty argument list: the live server already drops those (an MCP
// call cannot supply positional arguments), and hiding them here makes the
// static __schema --as=mcp list identical to the live tools/list.
func withMCPExclusions(root *cobra.Command, fn func() error) error {
	var hidden []*cobra.Command
	hide := func(cmd *cobra.Command) {
		if cmd != nil && !cmd.Hidden {
			cmd.Hidden = true
			hidden = append(hidden, cmd)
		}
	}
	for _, exclusion := range mcpExcludedCommands {
		hide(findSubcommand(root, exclusion.path))
	}
	walkCommands(root, func(cmd *cobra.Command) {
		if !cmd.HasSubCommands() && cmd.Args != nil && cmd.Args(cmd, []string{}) != nil {
			hide(cmd)
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

// errRootNotATool is returned when an MCP client calls the root command, which
// would otherwise open the overview dashboard or, with --mcp, start a server
// inside a tool call.
func errRootNotATool(ctx context.Context) error {
	return ax.NewError(ctx, "validation_error",
		"the finfocus root command is not callable as an MCP tool; call a subcommand tool such as finfocus-overview",
		ax.WithErrorExitCode(ax.ExitValidation))
}
