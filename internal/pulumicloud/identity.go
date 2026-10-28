package pulumicloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rshade/finfocus/internal/resourcetype"
)

// MaxExportBytes caps the bytes read from any deployment export.
const MaxExportBytes = 256 * 1024 * 1024

// Identity contains only the resource identity fields permitted in the cache.
type Identity struct {
	URN      string `json:"urn"`
	CloudID  string `json:"cloud_id"`
	Type     string `json:"type"`
	Provider string `json:"provider"`
}

// DecodeIdentities reads a bounded export without retaining resource properties.
func DecodeIdentities(body io.Reader) ([]Identity, error) {
	limited := &io.LimitedReader{R: body, N: MaxExportBytes + 1}
	decoder := json.NewDecoder(limited)
	var export struct {
		Deployment *struct {
			Resources []wireResource `json:"resources"`
		} `json:"deployment"`
	}
	if err := decoder.Decode(&export); err != nil {
		if limited.N == 0 {
			return nil, errExportTooLarge
		}
		return nil, fmt.Errorf("decode deployment identities: %w", err)
	}
	if export.Deployment == nil {
		return nil, errors.New("deployment export is missing a deployment")
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if limited.N == 0 {
			return nil, errExportTooLarge
		}
		return nil, errors.New("deployment export has trailing data")
	}
	if limited.N == 0 {
		return nil, errExportTooLarge
	}
	if export.Deployment.Resources == nil {
		// A snapshot can contain manifest and secret-provider metadata only.
		return []Identity{}, nil
	}
	return toIdentities(export.Deployment.Resources), nil
}

type wireResource struct {
	URN    string `json:"urn"`
	ID     string `json:"id"`
	Type   string `json:"type"`
	Custom bool   `json:"custom"`
}

// identitiesFromDeployment extracts identities from the opaque deployment value
// of an export, and retains none of its properties.
func identitiesFromDeployment(raw json.RawMessage) ([]Identity, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, errors.New("deployment export is missing a deployment")
	}
	var deployment struct {
		Resources []wireResource `json:"resources"`
	}
	if err := json.Unmarshal(raw, &deployment); err != nil {
		return nil, fmt.Errorf("decode deployment identities: %w", err)
	}
	return toIdentities(deployment.Resources), nil
}

func toIdentities(resources []wireResource) []Identity {
	ids := make([]Identity, 0, len(resources))
	for _, r := range resources {
		if !r.Custom || r.ID == "" || strings.HasPrefix(r.Type, "pulumi:") {
			continue
		}
		ids = append(
			ids,
			Identity{
				URN:      r.URN,
				CloudID:  r.ID,
				Type:     r.Type,
				Provider: resourcetype.ExtractProvider(r.Type),
			},
		)
	}
	return ids
}
