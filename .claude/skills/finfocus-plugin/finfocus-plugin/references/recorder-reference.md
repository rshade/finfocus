# Recorder Plugin Reference

## Table of Contents

- [Overview](#overview)
- [File Structure](#file-structure)
- [Implementation Walkthrough](#implementation-walkthrough)
- [Configuration](#configuration)
- [Recording Format](#recording-format)
- [Build and Install](#build-and-install)

## Overview

The recorder plugin (`plugins/recorder/`) is a reference implementation that:

- Records all incoming gRPC requests to JSON files
- Optionally returns mock cost responses
- Demonstrates all pluginsdk v0.4.6 patterns

## File Structure

```text
plugins/recorder/
├── plugin.go      # Main plugin with CostSourceService methods
├── config.go      # Environment-based configuration
├── recorder.go    # JSON request recording logic
├── mocker.go      # Mock response generation
└── cmd/main.go    # Entry point
```

## Implementation Walkthrough

### Plugin Struct

```go
type RecorderPlugin struct {
    *pluginsdk.BasePlugin  // Embed for wildcard provider matcher
    config   *Config
    recorder *Recorder
    mocker   *Mocker
    logger   zerolog.Logger
    mu       sync.Mutex     // Thread safety for all gRPC handlers
}
```

### Constructor

```go
func NewRecorderPlugin(cfg *Config, logger zerolog.Logger) *RecorderPlugin {
    return &RecorderPlugin{
        BasePlugin: pluginsdk.NewBasePlugin("recorder", []string{"*"}),
        config:     cfg,
        recorder:   NewRecorder(cfg.OutputDir),
        mocker:     NewMocker(),
        logger:     logger,
    }
}
```

### GetProjectedCost

```go
func (p *RecorderPlugin) GetProjectedCost(ctx context.Context,
    req *pbc.GetProjectedCostRequest) (*pbc.GetProjectedCostResponse, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    // 1. Validate request
    if err := pluginsdk.ValidateProjectedCostRequest(req); err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "validation: %v", err)
    }

    // 2. Record request
    if err := p.recorder.Record("GetProjectedCost", req); err != nil {
        p.logger.Warn().Err(err).Msg("failed to record request")
    }

    // 3. Return mock or empty response
    if p.config.MockResponse {
        return p.mocker.ProjectedCost(req), nil
    }
    return &pbc.GetProjectedCostResponse{}, nil
}
```

### GetRecommendations (with pagination)

```go
func (p *RecorderPlugin) GetRecommendations(ctx context.Context,
    req *pbc.GetRecommendationsRequest) (*pbc.GetRecommendationsResponse, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    // Validate PageSize bounds (0-1000)
    if req.PageSize < 0 || req.PageSize > 1000 {
        return nil, status.Errorf(codes.InvalidArgument,
            "page_size must be between 0 and 1000")
    }

    if err := p.recorder.Record("GetRecommendations", req); err != nil {
        p.logger.Warn().Err(err).Msg("failed to record request")
    }

    return &pbc.GetRecommendationsResponse{}, nil
}
```

### JSON Recording

Uses `protojson.Marshal` for human-readable output and ULID for filenames:

```go
type RecordedRequest struct {
    Timestamp string          `json:"timestamp"`
    Method    string          `json:"method"`
    RequestID string          `json:"requestId"`
    Request   json.RawMessage `json:"request"`
}
```

## Configuration

| Env Var | Default | Purpose |
|---------|---------|---------|
| `FINFOCUS_RECORDER_OUTPUT_DIR` | `./recorded_data` | JSON output directory |
| `FINFOCUS_RECORDER_MOCK_RESPONSE` | `false` | Enable mock responses |

## Recording Format

```json
{
  "timestamp": "2025-12-11T14:30:52Z",
  "method": "GetProjectedCost",
  "requestId": "01JEK7X2J3K4M5N6P7Q8R9S1T2",
  "request": {
    "resource": {
      "provider": "aws",
      "resourceType": "aws:ec2/instance:Instance",
      "sku": "t3.micro",
      "region": "us-east-1"
    }
  }
}
```

## Build and Install

```bash
make build-recorder     # Build to bin/finfocus-plugin-recorder
make install-recorder   # Install to ~/.finfocus/plugins/recorder/0.1.0/

# Test with core
export FINFOCUS_RECORDER_OUTPUT_DIR=./debug
./bin/finfocus cost projected --pulumi-json plan.json
cat ./debug/*.json | jq .
```
