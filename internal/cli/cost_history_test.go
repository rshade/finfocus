package cli

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func preparedHistoryCmd(
	t *testing.T,
	cmd *cobra.Command,
	args ...string,
) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	parent := &cobra.Command{Use: "cost"}
	parent.PersistentFlags().String("stack", "", "")
	parent.PersistentFlags().Bool("yes", false, "")
	parent.PersistentFlags().Bool("dry-run", false, "")
	parent.PersistentFlags().String("format", "", "")
	parent.AddCommand(cmd)
	found, rest, err := parent.Find(append([]string{cmd.Name()}, args...))
	require.NoError(t, err)
	found.SetOut(stdout)
	found.SetErr(stderr)
	found.SetContext(context.Background())
	require.NoError(t, found.ParseFlags(rest))
	return found, stdout
}

func pulumiLook(string) (string, error) {
	return "pulumi", nil
}

func scriptedHistoryPulumi(
	history []byte,
	exports map[int][]byte,
	stacks []byte,
) func(context.Context, string, ...string) ([]byte, error) {
	return func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) == 0 {
			return nil, strconv.ErrSyntax
		}
		switch args[0] {
		case "version":
			return []byte("v3.120.0\n"), nil
		case "stack":
			return scriptedStack(args, history, exports, stacks)
		default:
			return nil, strconv.ErrSyntax
		}
	}
}

func scriptedStack(args []string, history []byte, exports map[int][]byte, stacks []byte) ([]byte, error) {
	if len(args) < 2 {
		return nil, strconv.ErrSyntax
	}
	switch args[1] {
	case "ls":
		return stacks, nil
	case "history":
		return history, nil
	case "export":
		for i, arg := range args {
			if arg == "--version" && i+1 < len(args) {
				version, err := strconv.Atoi(args[i+1])
				if err != nil {
					return nil, err
				}
				body, ok := exports[version]
				if !ok {
					return nil, strconv.ErrSyntax
				}
				return body, nil
			}
		}
	}
	return nil, strconv.ErrSyntax
}
