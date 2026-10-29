package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/carbonreport"
	"github.com/rshade/finfocus/internal/registry"
)

const (
	carbonActualCommand = "actual"
	carbonTableTabWidth = 4
	carbonTablePadding  = 2
)

type carbonQuery func(context.Context, *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error)

func newCarbonCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "carbon", Short: "Query provider-reported historical carbon emissions"}
	cmd.AddCommand(newCarbonActualCmd(queryHistoricalCarbon))
	return cmd
}

func newCarbonActualCmd(query carbonQuery) *cobra.Command {
	var selectors carbonreport.Selectors
	var output string
	cmd := &cobra.Command{
		Use:          carbonActualCommand,
		SilenceUsage: true,
		Short:        "Report available monthly AWS emissions with source provenance",
		Long:         "Query aws-ce historical carbon emissions for complete UTC months. End is exclusive. Totals sum returned records; missing data is not zero. Enable FINFOCUS_AWS_CARBON_ENABLED=true or configure plugins.aws-ce.carbon_enabled=true.",
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, filter := range []struct {
				name   string
				values []string
			}{{"account", selectors.Accounts}, {"region", selectors.Regions}, {"service", selectors.Services}} {
				if cmd.Flags().Changed(filter.name) && len(filter.values) == 0 {
					return toValidationError(cmd.Context(), fmt.Errorf("--%s must not be empty", filter.name))
				}
			}
			req, err := selectors.Request()
			if err != nil {
				return toValidationError(cmd.Context(), err)
			}
			format := resolveOutputFormat(cmd, "output", output)
			if format != "table" && format != "json" {
				return toValidationError(cmd.Context(), errors.New("output must be table or json"))
			}
			report, err := query(cmd.Context(), req)
			if err != nil {
				return err
			}
			return writeCarbonReport(cmd.OutOrStdout(), report, format)
		},
	}
	cmd.Flags().StringVar(&selectors.Start, "start", "", "inclusive UTC month boundary (YYYY-MM-DD; required)")
	cmd.Flags().StringVar(&selectors.End, "end", "", "exclusive UTC month boundary (YYYY-MM-DD; required)")
	cmd.Flags().
		StringVar(&selectors.Method, "method", "market-based", "accounting method: market-based or location-based")
	cmd.Flags().StringVar(&selectors.Scope, "scope", "total", "emissions scope: total, 1, 2 or 3")
	cmd.Flags().StringSliceVar(&selectors.Accounts, "account", nil, "account IDs (comma-separated or repeated)")
	cmd.Flags().StringSliceVar(&selectors.Regions, "region", nil, "AWS regions (comma-separated or repeated)")
	cmd.Flags().StringSliceVar(&selectors.Services, "service", nil, "AWS service names (comma-separated or repeated)")
	cmd.Flags().StringVar(&output, "output", "table", "output format: table or json")
	// These declarations use existing command flags and fixed valid schema values.
	_ = cmd.MarkFlagRequired("start")
	_ = cmd.MarkFlagRequired("end")
	_ = ax.DeclareCapability(
		cmd,
		ax.CapabilityReadOnly,
		"Queries provider-reported historical emissions without changing AWS resources",
	)
	_ = ax.DeclareFlagEnum(cmd, "method", "market-based", "location-based")
	_ = ax.DeclareFlagEnum(cmd, "scope", "total", "1", "2", "3")
	_ = ax.DeclareFlagEnum(cmd, "output", "table", "json")
	return cmd
}

// queryHistoricalCarbon is the sole provider-opening operation used by both CLI
// and authenticated web queries. Aggregate reports never use resource Supports.
func queryHistoricalCarbon(ctx context.Context, req *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
	clients, cleanup, err := registry.NewDefault().Open(ctx, "aws-ce")
	if err != nil {
		return nil, fmt.Errorf("opening aws-ce: %w", err)
	}
	defer cleanup()
	if len(clients) != 1 {
		return nil, status.Error(
			codes.FailedPrecondition,
			"aws-ce historical carbon provider unavailable: install aws-ce and enable FINFOCUS_AWS_CARBON_ENABLED=true or plugins.aws-ce.carbon_enabled=true",
		)
	}
	return carbonreport.Query(ctx, clients[0], req)
}

func writeCarbonReport(out io.Writer, report *carbonreport.Report, format string) error {
	var buffer bytes.Buffer
	if format == "json" {
		if err := json.NewEncoder(&buffer).Encode(report); err != nil {
			return fmt.Errorf("encoding carbon report: %w", err)
		}
	} else {
		tw := tabwriter.NewWriter(&buffer, 0, carbonTableTabWidth, carbonTablePadding, ' ', 0)
		if err := renderCarbonTable(tw, report); err != nil {
			return err
		}
		if err := tw.Flush(); err != nil {
			return fmt.Errorf("formatting carbon report: %w", err)
		}
	}
	if _, err := buffer.WriteTo(out); err != nil {
		return fmt.Errorf("writing carbon report: %w", err)
	}
	return nil
}

func renderCarbonTable(out io.Writer, report *carbonreport.Report) error {
	req := report.Request
	if _, err := fmt.Fprintf(
		out,
		"Available records: %s to %s (exclusive), %s, %s\n",
		req.GetStart().AsTime().Format(time.DateOnly),
		req.GetEnd().AsTime().Format(time.DateOnly),
		req.GetMethod(),
		req.GetScope(),
	); err != nil {
		return err
	}
	if report.TotalGrams == nil {
		_, err := fmt.Fprintln(out, "No available emissions data; total unavailable.")
		return err
	}
	if _, err := fmt.Fprintln(
		out,
		"ID\tPROVIDER\tSOURCE\tACCOUNT\tREGION\tSERVICE\tSTART\tEND (EXCLUSIVE)\tMETHOD\tSCOPE\tVALUE\tUNIT\tMODEL\tRETRIEVED AT",
	); err != nil {
		return err
	}
	for _, r := range report.Records {
		if _, err := fmt.Fprintf(
			out,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%g\t%s\t%s\t%s\n",
			r.GetId(),
			r.GetProvider(),
			r.GetSource(),
			r.GetAccountId(),
			r.GetRegion(),
			r.GetService(),
			r.GetStart().AsTime().Format(time.RFC3339Nano),
			r.GetEnd().AsTime().Format(time.RFC3339Nano),
			r.GetMethod(),
			r.GetScope(),
			r.GetValue(),
			r.GetUnit(),
			r.GetModelVersion(),
			r.GetRetrievedAt().AsTime().Format(time.RFC3339Nano),
		); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(out, "Total of returned records: %g gCO2e\n", *report.TotalGrams)
	return err
}
