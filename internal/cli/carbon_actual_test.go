package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/carbonreport"
	"github.com/rshade/finfocus/internal/esc"
)

func TestCarbonCommandDiscovery(t *testing.T) {
	t.Parallel()
	root := NewRootCmd("test")
	cmd, _, err := root.Find([]string{"carbon", "actual"})
	require.NoError(t, err)
	assert.Equal(t, "actual", cmd.Name())
}
func TestCarbonActualValidation(t *testing.T) {
	t.Parallel()
	cases := [][]string{
		{},
		{"--start", "2026-01-01"},
		{"--start", "2026-01-02", "--end", "2026-03-01"},
		{"--start", "2026-01-01", "--end", "2026-03-01", "--method", "bad"},
		{"--start", "2026-01-01", "--end", "2026-03-01", "--scope", "4"},
		{"--start", "2026-01-01", "--end", "2026-03-01", "--output", "yaml"},
	}
	for _, args := range cases {
		cmd := newCarbonActualCmd(func(context.Context, *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
			t.Fatal("invalid input launched plugins")
			return nil, errors.New("unreachable invalid query")
		})
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err), "args %v", args)
	}
}

func TestCarbonActualRejectsPositionalArgs(t *testing.T) {
	t.Parallel()
	cmd := newCarbonActualCmd(func(context.Context, *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
		t.Fatal("invalid input launched plugins")
		return nil, errors.New("unreachable invalid query")
	})
	cmd.SetArgs([]string{"extra", "--start", "2026-01-01", "--end", "2026-03-01"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err))
}
func TestCarbonActualAtomicOutputAndIO(t *testing.T) {
	t.Parallel()
	for _, output := range []string{"json", "table"} {
		cmd := newCarbonActualCmd(
			func(_ context.Context, req *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
				assert.Equal(t, []string{"111", "222"}, req.GetAccountIds())
				return &carbonreport.Report{
					Request: req,
					Records: []*pbc.CarbonEmissionRecord{
						{
							Id:           "id",
							Provider:     "aws",
							Source:       "aws-sustainability",
							AccountId:    "111",
							Region:       "us-east-1",
							Service:      "AmazonEC2",
							Start:        req.GetStart(),
							End:          req.GetEnd(),
							Method:       1,
							Scope:        1,
							ModelVersion: "model",
							Value:        proto.Float64(0),
							Unit:         "gCO2e",
							RetrievedAt:  req.GetEnd(),
						},
					},
					TotalGrams: proto.Float64(0),
				}, nil
			},
		)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(
			[]string{"--start", "2026-01-01", "--end", "2026-02-01", "--account", "111,222", "--output", output},
		)
		require.NoError(t, cmd.Execute())
		for _, want := range []string{"aws-sustainability", "111", "us-east-1", "AmazonEC2", "model", "gCO2e"} {
			assert.Contains(t, out.String(), want)
		}
	}
	cmd := newCarbonActualCmd(func(context.Context, *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
		return nil, errors.New("later AWS error")
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--start", "2026-01-01", "--end", "2026-02-01"})
	require.Error(t, cmd.Execute())
	assert.Empty(t, out.String())
	cmd = newCarbonActualCmd(func(_ context.Context, req *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
		return &carbonreport.Report{Request: req}, nil
	})
	cmd.SetOut(carbonFailWriter{})
	cmd.SetArgs([]string{"--start", "2026-01-01", "--end", "2026-02-01"})
	require.ErrorContains(t, cmd.Execute(), "write failure")
}

type carbonFailWriter struct{}

func (carbonFailWriter) Write([]byte) (int, error) { return 0, errors.New("write failure") }

func TestCarbonActualRejectsExplicitEmptyFilters(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"account", "region", "service"} {
		cmd := newCarbonActualCmd(func(context.Context, *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
			return nil, errors.New("provider started")
		})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--start", "2026-01-01", "--end", "2026-02-01", "--" + flag + "="})
		err := cmd.Execute()
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "provider started")
	}
}
func TestCarbonReportShortWrite(t *testing.T) {
	t.Parallel()
	err := writeCarbonReport(carbonShortWriter{}, &carbonreport.Report{}, "json")
	require.ErrorIs(t, err, io.ErrShortWrite)
}

type carbonShortWriter struct{}

func (carbonShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestWebCarbonQueryWiring(t *testing.T) {
	t.Parallel()
	options := webSessionOptions(context.Background(), &webData{}, nil, nil, nil)
	require.NotNil(t, options.HistoricalCarbon)
}

func TestSessionCarbonQueryPropagatesESCAndCancellation(t *testing.T) {
	t.Parallel()
	src := &esc.Source{}
	data := &webData{}
	data.noteESC(esc.WithSource(context.Background(), src))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	query := sessionCarbonQuery(
		func(got context.Context, _ *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
			found, ok := esc.SourceFromContext(got)
			assert.True(t, ok)
			assert.Same(t, src, found)
			cancel()
			assert.ErrorIs(t, got.Err(), context.Canceled)
			return &carbonreport.Report{}, nil
		},
		data,
	)
	_, err := query(ctx, &pbc.GetCarbonEmissionsRequest{})
	require.NoError(t, err)
}

func TestCarbonMCPSchemaReadOnlyAndFlags(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	root := NewRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"__schema", "--as=mcp"})
	require.NoError(t, root.Execute())
	var schema struct {
		Tools []struct {
			Name        string `json:"name"`
			Annotations struct {
				ReadOnly bool `json:"readOnlyHint"`
			} `json:"annotations"`
			InputSchema struct {
				Required   []string                   `json:"required"`
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &schema))
	for _, tool := range schema.Tools {
		if tool.Name != "finfocus-carbon-actual" {
			continue
		}
		assert.True(t, tool.Annotations.ReadOnly)
		assert.Contains(t, tool.InputSchema.Required, "start")
		assert.Contains(t, tool.InputSchema.Required, "end")
		for _, flag := range []string{"start", "end", "method", "scope", "account", "region", "service", "output"} {
			assert.Contains(t, tool.InputSchema.Properties, flag)
		}
		return
	}
	t.Fatal("historical carbon tool missing")
}

func TestCarbonActualMachineOutput(t *testing.T) {
	for _, tc := range []struct {
		name, agent, format, output string
		wantJSON                    bool
	}{{name: "format json", format: "json", wantJSON: true}, {name: "agent mode", agent: "1", wantJSON: true}, {name: "explicit table wins", format: "json", agent: "1", output: "table"}, {name: "explicit JSON", format: "human", output: "json", wantJSON: true}, {name: "default table"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(agentModeEnv, tc.agent)
			root := &cobra.Command{Use: "finfocus"}
			root.PersistentFlags().String(axFormatFlag, "", "agent output format")
			cmd := newCarbonActualCmd(
				func(_ context.Context, req *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
					return &carbonreport.Report{Request: req, Records: []*pbc.CarbonEmissionRecord{}}, nil
				},
			)
			root.AddCommand(cmd)
			args := []string{"actual", "--start", "2026-01-01", "--end", "2026-02-01"}
			if tc.format != "" {
				args = append(args, "--format="+tc.format)
			}
			if tc.output != "" {
				args = append(args, "--output="+tc.output)
			}
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&bytes.Buffer{})
			root.SetArgs(args)
			require.NoError(t, root.Execute())
			assert.Equal(t, tc.wantJSON, json.Valid(out.Bytes()), out.String())
			if tc.wantJSON {
				assert.Contains(t, out.String(), `"totalGrams":null`)
			}
		})
	}
}
