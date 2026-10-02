package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rshade/ax-go"
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
		return AllocationPolicy{}, fmt.Errorf("open allocation policy %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	var data json.RawMessage
	if parseErr := ax.ParseConfig(ctx, f, &data); parseErr != nil {
		return AllocationPolicy{}, fmt.Errorf("parse allocation policy %s: %w", path, parseErr)
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		data = nil
	}
	return AllocationPolicy{JSON: data, Source: path}, nil
}
