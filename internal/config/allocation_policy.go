package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/rshade/ax-go"
	"github.com/rshade/ax-go/contract"
)

// AllocationPolicyFile is the allocation policy file name in a project or global config dir.
const AllocationPolicyFile = "allocation.hujson"

// AllocationPolicy is a resolved policy document, standardized to JSON.
type AllocationPolicy struct {
	JSON   []byte
	Source string
}

// ResolveAllocationPolicy finds the allocation policy: flagPath, then the project
// config dir, then the global config dir. The first file found wins; files are
// never merged. No file yields an empty policy (plugin defaults).
//
// A file that cannot be read is a plain error naming the file. A file that is
// not valid HuJSON, or is larger than the read limit, is an *ax.Error that keeps
// ax-go's config error code and exit 2 and names the file instead of "config".
func ResolveAllocationPolicy(ctx context.Context, flagPath string) (AllocationPolicy, error) {
	if flagPath != "" {
		return readAllocationPolicy(ctx, flagPath)
	}
	var candidates []string
	if dir := GetResolvedProjectDir(); dir != "" {
		candidates = append(candidates, filepath.Join(dir, AllocationPolicyFile))
	}
	candidates = append(candidates, filepath.Join(ResolveConfigDir(), AllocationPolicyFile))
	for _, path := range candidates {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		}
		return readAllocationPolicy(ctx, path)
	}
	return AllocationPolicy{}, nil
}

func readAllocationPolicy(ctx context.Context, path string) (AllocationPolicy, error) {
	f, err := os.Open(path)
	if err != nil {
		return AllocationPolicy{}, policyReadError(path, err)
	}
	defer func() { _ = f.Close() }()
	var data json.RawMessage
	if parseErr := ax.ParseConfig(ctx, f, &data); parseErr != nil {
		return AllocationPolicy{}, policyParseError(ctx, path, parseErr)
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		data = nil
	}
	return AllocationPolicy{JSON: data, Source: path}, nil
}

// policyReadError names the file once: an [fs.PathError] would repeat it.
func policyReadError(path string, err error) error {
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		err = pathErr.Err
	}
	return fmt.Errorf("cannot read allocation policy %s: %w", path, err)
}

var (
	// hujsonPosition matches the position tailscale/hujson puts in a parse error.
	hujsonPosition = regexp.MustCompile(`line (\d+), column (\d+): (.+)$`)
	// hujsonLiteral matches the one hujson reason that quotes the file's text.
	hujsonLiteral = regexp.MustCompile(`invalid literal: .*$`)
)

// policyParseError rewrites ax-go's generic config parse errors so they name
// the allocation policy file and, for a syntax error, the line, column, and
// parser reason. File text never reaches the message: the one parser reason
// that quotes it (an invalid literal) is cut to its kind.
func policyParseError(ctx context.Context, path string, err error) error {
	axErr, ok := errors.AsType[*ax.Error](err)
	if !ok {
		return policyReadError(path, err)
	}
	switch axErr.ErrorCode {
	case contract.ErrorCodeConfigTooLarge:
		return ax.NewError(ctx, axErr.ErrorCode,
			fmt.Sprintf("allocation policy %s is larger than the %v-byte limit", path, axErr.Context["max_bytes"]),
			ax.WithActionableFix(fmt.Sprintf("shorten the allocation policy %s and retry", path)),
			ax.WithErrorContext(map[string]any{"path": path, "max_bytes": axErr.Context["max_bytes"]}),
			ax.WithErrorCause(err),
			ax.WithErrorExitCode(ax.ExitValidation))
	case contract.ErrorCodeConfigInvalid:
		return policySyntaxError(ctx, path, axErr)
	default:
		return err
	}
}

func policySyntaxError(ctx context.Context, path string, axErr *ax.Error) error {
	fields := map[string]any{"path": path}
	message := fmt.Sprintf("allocation policy %s is not valid HuJSON", path)
	fix := fmt.Sprintf("fix the HuJSON syntax of the allocation policy %s and retry", path)
	if cause := errors.Unwrap(axErr); cause != nil {
		if m := hujsonPosition.FindStringSubmatch(cause.Error()); m != nil {
			line, lineErr := strconv.Atoi(m[1])
			column, columnErr := strconv.Atoi(m[2])
			if lineErr == nil && columnErr == nil {
				reason := hujsonLiteral.ReplaceAllString(m[3], "invalid literal")
				message = fmt.Sprintf("%s: line %d, column %d: %s", message, line, column, reason)
				fix = fmt.Sprintf("fix the HuJSON syntax at line %d, column %d of the allocation policy %s and retry",
					line, column, path)
				fields["line"], fields["column"] = line, column
			}
		}
	}
	return ax.NewError(ctx, axErr.ErrorCode, message,
		ax.WithActionableFix(fix),
		ax.WithErrorContext(fields),
		ax.WithErrorCause(axErr),
		ax.WithErrorExitCode(ax.ExitValidation))
}
