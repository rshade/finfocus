---
layout: default
title: Plugin Examples and Patterns
description: Common code patterns and real-world plugin implementations
---

This document provides common code patterns, real-world examples, and best
practices for FinFocus plugin development based on actual plugin
implementations.

## Table of Contents

1. [Common Patterns](#common-patterns)
2. [Error Handling](#error-handling)
3. [Authentication](#authentication)
4. [API Integration](#api-integration)
5. [Caching Strategies](#caching-strategies)
6. [Real-World Examples](#real-world-examples)

---

## Common Patterns

### Price Table Management

Maintain pricing tables for different resource types:

```go
type PriceTable struct {
    mu     sync.RWMutex
    prices map[string]map[string]float64
}

func NewPriceTable() *PriceTable {
    return &PriceTable{
        prices: make(map[string]map[string]float64),
    }
}

func (pt *PriceTable) AddProvider(provider string, prices map[string]float64) {
    pt.mu.Lock()
    defer pt.mu.Unlock()
    pt.prices[provider] = prices
}

func (pt *PriceTable) GetPrice(provider, sku string) (float64, bool) {
    pt.mu.RLock()
    defer pt.mu.RUnlock()

    providerPrices, exists := pt.prices[provider]
    if !exists {
        return 0, false
    }

    price, exists := providerPrices[sku]
    return price, exists
}
```

### Region-Based Pricing

Handle regional pricing variations:

```go
type RegionalPricing struct {
    basePrice        float64
    regionMultipliers map[string]float64
}

func NewRegionalPricing(basePrice float64) *RegionalPricing {
    return &RegionalPricing{
        basePrice: basePrice,
        regionMultipliers: map[string]float64{
            "us-east-1":      1.0,
            "us-west-2":      1.0,
            "eu-west-1":      1.1,
            "ap-southeast-1": 1.2,
        },
    }
}

func (rp *RegionalPricing) GetPrice(region string) float64 {
    multiplier, exists := rp.regionMultipliers[region]
    if !exists {
        multiplier = 1.0 // Default to US pricing
    }
    return rp.basePrice * multiplier
}

func (rp *RegionalPricing) AddRegion(region string, multiplier float64) {
    rp.regionMultipliers[region] = multiplier
}
```

### Resource Tag Extraction

Extract and validate resource tags:

```go
type TagExtractor struct{}

func NewTagExtractor() *TagExtractor {
    return &TagExtractor{}
}

func (te *TagExtractor) GetString(
    tags map[string]string,
    key, defaultValue string,
) string {
    value, exists := tags[key]
    if !exists || value == "" {
        return defaultValue
    }
    return value
}

func (te *TagExtractor) GetInt(
    tags map[string]string,
    key string,
    defaultValue int,
) int {
    value, exists := tags[key]
    if !exists {
        return defaultValue
    }

    intValue, err := strconv.Atoi(value)
    if err != nil {
        return defaultValue
    }

    return intValue
}

func (te *TagExtractor) GetBool(
    tags map[string]string,
    key string,
    defaultValue bool,
) bool {
    value, exists := tags[key]
    if !exists {
        return defaultValue
    }

    boolValue, err := strconv.ParseBool(value)
    if err != nil {
        return defaultValue
    }

    return boolValue
}

// Usage
extractor := NewTagExtractor()
instanceType := extractor.GetString(
    resource.GetTags(),
    "instanceType",
    "t3.micro",
)
```

### Billing Mode Detection

Determine billing mode from resource tags:

```go
type BillingMode int

const (
    BillingModeOnDemand BillingMode = iota
    BillingModeReserved
    BillingModeSpot
    BillingModeSavingsPlan
)

func (bm BillingMode) String() string {
    switch bm {
    case BillingModeOnDemand:
        return "on-demand"
    case BillingModeReserved:
        return "reserved-instance"
    case BillingModeSpot:
        return "spot-instance"
    case BillingModeSavingsPlan:
        return "savings-plan"
    default:
        return "unknown"
    }
}

func DetectBillingMode(tags map[string]string) BillingMode {
    mode := tags["billingMode"]

    switch mode {
    case "reserved", "ri":
        return BillingModeReserved
    case "spot":
        return BillingModeSpot
    case "savings-plan", "sp":
        return BillingModeSavingsPlan
    default:
        return BillingModeOnDemand
    }
}

// Apply discounts based on billing mode
func ApplyBillingDiscount(basePrice float64, mode BillingMode) float64 {
    switch mode {
    case BillingModeReserved:
        return basePrice * 0.6  // 40% discount
    case BillingModeSpot:
        return basePrice * 0.3  // 70% discount
    case BillingModeSavingsPlan:
        return basePrice * 0.7  // 30% discount
    default:
        return basePrice
    }
}
```

---

## Error Handling

### Structured Error Types

Define structured error types for better error handling:

```go
type PluginError struct {
    Code    string
    Message string
    Err     error
}

func (pe *PluginError) Error() string {
    if pe.Err != nil {
        return fmt.Sprintf("%s: %s (%v)", pe.Code, pe.Message, pe.Err)
    }
    return fmt.Sprintf("%s: %s", pe.Code, pe.Message)
}

func (pe *PluginError) Unwrap() error {
    return pe.Err
}

// Error constructors
func NewNotFoundError(resourceID string, err error) *PluginError {
    return &PluginError{
        Code:    "RESOURCE_NOT_FOUND",
        Message: fmt.Sprintf("resource %s not found", resourceID),
        Err:     err,
    }
}

func NewAPIError(operation string, err error) *PluginError {
    return &PluginError{
        Code:    "API_ERROR",
        Message: fmt.Sprintf("API call failed: %s", operation),
        Err:     err,
    }
}

func NewConfigError(field string, err error) *PluginError {
    return &PluginError{
        Code:    "CONFIG_ERROR",
        Message: fmt.Sprintf("invalid configuration: %s", field),
        Err:     err,
    }
}
```

### Retry Logic

Implement retry logic for transient failures:

```go
type RetryConfig struct {
    MaxAttempts int
    Delay       time.Duration
    Backoff     float64
}

func DefaultRetryConfig() *RetryConfig {
    return &RetryConfig{
        MaxAttempts: 3,
        Delay:       1 * time.Second,
        Backoff:     2.0,
    }
}

func RetryWithBackoff(
    ctx context.Context,
    config *RetryConfig,
    operation func() error,
) error {
    var lastErr error
    delay := config.Delay

    for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        lastErr = operation()
        if lastErr == nil {
            return nil
        }

        // Check if error is retryable
        if !isRetryable(lastErr) {
            return lastErr
        }

        if attempt < config.MaxAttempts {
            time.Sleep(delay)
            delay = time.Duration(float64(delay) * config.Backoff)
        }
    }

    return fmt.Errorf("operation failed after %d attempts: %w",
        config.MaxAttempts, lastErr)
}

func isRetryable(err error) bool {
    // Network timeouts
    if errors.Is(err, context.DeadlineExceeded) {
        return true
    }

    // HTTP errors
    if httpErr, ok := err.(*HTTPError); ok {
        return httpErr.StatusCode >= 500 || httpErr.StatusCode == 429
    }

    return false
}

// Usage
err := RetryWithBackoff(ctx, DefaultRetryConfig(), func() error {
    return client.FetchPricing(resourceID)
})
```

### Error Context

Add context to errors for better debugging:

```go
type ErrorContext struct {
    Operation  string
    ResourceID string
    Provider   string
    Timestamp  time.Time
    Details    map[string]interface{}
}

func (ec *ErrorContext) Wrap(err error) error {
    if err == nil {
        return nil
    }

    return fmt.Errorf(
        "operation=%s resource=%s provider=%s time=%s: %w",
        ec.Operation,
        ec.ResourceID,
        ec.Provider,
        ec.Timestamp.Format(time.RFC3339),
        err,
    )
}

// Usage
func (p *MyPlugin) GetProjectedCost(
    ctx context.Context,
    req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
    resource := req.GetResource()

    errCtx := &ErrorContext{
        Operation:  "GetProjectedCost",
        ResourceID: resource.GetSku(),
        Provider:   resource.GetProvider(),
        Timestamp:  time.Now(),
    }

    price, err := p.fetchPrice(ctx, resource)
    if err != nil {
        return nil, errCtx.Wrap(err)
    }

    return p.Calculator().CreateProjectedCostResponse(
        "USD",
        price,
        "api-pricing",
    ), nil
}
```

---

## Authentication

### API Key Authentication

Handle API key-based authentication:

```go
type APIKeyAuth struct {
    apiKey string
    header string
}

func NewAPIKeyAuth(apiKey, header string) *APIKeyAuth {
    return &APIKeyAuth{
        apiKey: apiKey,
        header: header,
    }
}

func (a *APIKeyAuth) ApplyToRequest(req *http.Request) {
    req.Header.Set(a.header, a.apiKey)
}

// Usage
auth := NewAPIKeyAuth(
    os.Getenv("MY_PLUGIN_API_KEY"),
    "X-API-Key",
)

req, _ := http.NewRequest("GET", apiURL, nil)
auth.ApplyToRequest(req)
```

### OAuth2 Token Authentication

Implement OAuth2 token authentication:

```go
import "golang.org/x/oauth2"

type OAuth2Auth struct {
    config *oauth2.Config
    token  *oauth2.Token
    mu     sync.RWMutex
}

func NewOAuth2Auth(
    clientID, clientSecret, tokenURL string,
) *OAuth2Auth {
    return &OAuth2Auth{
        config: &oauth2.Config{
            ClientID:     clientID,
            ClientSecret: clientSecret,
            Endpoint: oauth2.Endpoint{
                TokenURL: tokenURL,
            },
        },
    }
}

func (a *OAuth2Auth) GetToken(ctx context.Context) (*oauth2.Token, error) {
    a.mu.RLock()
    if a.token != nil && a.token.Valid() {
        token := a.token
        a.mu.RUnlock()
        return token, nil
    }
    a.mu.RUnlock()

    // Acquire write lock to refresh token
    a.mu.Lock()
    defer a.mu.Unlock()

    // Double-check after acquiring write lock
    if a.token != nil && a.token.Valid() {
        return a.token, nil
    }

    token, err := a.config.Token(ctx)
    if err != nil {
        return nil, fmt.Errorf("fetching OAuth2 token: %w", err)
    }

    a.token = token
    return token, nil
}

func (a *OAuth2Auth) CreateClient(ctx context.Context) (*http.Client, error) {
    token, err := a.GetToken(ctx)
    if err != nil {
        return nil, err
    }

    return a.config.Client(ctx, token), nil
}
```

### Credential Loading

Load credentials from configuration:

```go
type Credentials struct {
    APIKey       string
    ClientID     string
    ClientSecret string
    Endpoint     string
}

func LoadCredentials(pluginName string) (*Credentials, error) {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("getting home dir: %w", err)
    }

    configPath := filepath.Join(homeDir, ".finfocus", "config.yaml")
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("reading config: %w", err)
    }

    var config map[string]interface{}
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("parsing config: %w", err)
    }

    integrations, ok := config["integrations"].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("missing integrations section")
    }

    pluginConfig, ok := integrations[pluginName].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("missing config for %s", pluginName)
    }

    return &Credentials{
        APIKey:       getString(pluginConfig, "api_key"),
        ClientID:     getString(pluginConfig, "client_id"),
        ClientSecret: getString(pluginConfig, "client_secret"),
        Endpoint:     getString(pluginConfig, "endpoint"),
    }, nil
}

func getString(m map[string]interface{}, key string) string {
    if value, ok := m[key].(string); ok {
        return value
    }
    return ""
}
```

---

## API Integration

### REST API Client

Create a robust REST API client:

```go
type RestAPIClient struct {
    baseURL    string
    httpClient *http.Client
    auth       AuthProvider
    logger     *slog.Logger
}

type AuthProvider interface {
    ApplyToRequest(*http.Request)
}

func NewRestAPIClient(
    baseURL string,
    auth AuthProvider,
) *RestAPIClient {
    return &RestAPIClient{
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
        auth:   auth,
        logger: slog.Default(),
    }
}

func (c *RestAPIClient) Get(
    ctx context.Context,
    path string,
    result interface{},
) error {
    url := c.baseURL + path

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return fmt.Errorf("creating request: %w", err)
    }

    c.auth.ApplyToRequest(req)

    c.logger.Info("API request", "method", "GET", "url", url)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("executing request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf(
            "API error %d: %s",
            resp.StatusCode,
            string(body),
        )
    }

    if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
        return fmt.Errorf("decoding response: %w", err)
    }

    return nil
}
```

### GraphQL Client

Implement a GraphQL client:

```go
type GraphQLClient struct {
    endpoint string
    client   *http.Client
    auth     AuthProvider
}

type GraphQLRequest struct {
    Query     string                 `json:"query"`
    Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLResponse struct {
    Data   json.RawMessage        `json:"data"`
    Errors []GraphQLError         `json:"errors,omitempty"`
}

type GraphQLError struct {
    Message string `json:"message"`
    Path    []interface{} `json:"path,omitempty"`
}

func NewGraphQLClient(endpoint string, auth AuthProvider) *GraphQLClient {
    return &GraphQLClient{
        endpoint: endpoint,
        client:   &http.Client{Timeout: 30 * time.Second},
        auth:     auth,
    }
}

func (c *GraphQLClient) Query(
    ctx context.Context,
    query string,
    variables map[string]interface{},
    result interface{},
) error {
    reqBody := GraphQLRequest{
        Query:     query,
        Variables: variables,
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return fmt.Errorf("marshaling request: %w", err)
    }

    req, err := http.NewRequestWithContext(
        ctx,
        "POST",
        c.endpoint,
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return fmt.Errorf("creating request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    c.auth.ApplyToRequest(req)

    resp, err := c.client.Do(req)
    if err != nil {
        return fmt.Errorf("executing request: %w", err)
    }
    defer resp.Body.Close()

    var gqlResp GraphQLResponse
    if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
        return fmt.Errorf("decoding response: %w", err)
    }

    if len(gqlResp.Errors) > 0 {
        return fmt.Errorf("GraphQL errors: %v", gqlResp.Errors)
    }

    if err := json.Unmarshal(gqlResp.Data, result); err != nil {
        return fmt.Errorf("unmarshaling data: %w", err)
    }

    return nil
}
```

---

## Caching Strategies

### In-Memory Cache with TTL

Implement time-based caching:

```go
type CacheEntry struct {
    Value     interface{}
    ExpiresAt time.Time
}

type TTLCache struct {
    mu      sync.RWMutex
    entries map[string]*CacheEntry
    ttl     time.Duration
}

func NewTTLCache(ttl time.Duration) *TTLCache {
    cache := &TTLCache{
        entries: make(map[string]*CacheEntry),
        ttl:     ttl,
    }

    // Start cleanup goroutine
    go cache.cleanup()

    return cache
}

func (c *TTLCache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    entry, exists := c.entries[key]
    if !exists {
        return nil, false
    }

    if time.Now().After(entry.ExpiresAt) {
        return nil, false
    }

    return entry.Value, true
}

func (c *TTLCache) Set(key string, value interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.entries[key] = &CacheEntry{
        Value:     value,
        ExpiresAt: time.Now().Add(c.ttl),
    }
}

func (c *TTLCache) cleanup() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        c.mu.Lock()
        now := time.Now()
        for key, entry := range c.entries {
            if now.After(entry.ExpiresAt) {
                delete(c.entries, key)
            }
        }
        c.mu.Unlock()
    }
}

// Usage in plugin
type CachedPlugin struct {
    *pluginsdk.BasePlugin
    priceCache *TTLCache
    apiClient  *RestAPIClient
}

func (p *CachedPlugin) GetProjectedCost(
    ctx context.Context,
    req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
    resource := req.GetResource()
    cacheKey := fmt.Sprintf(
        "%s:%s:%s",
        resource.GetProvider(),
        resource.GetResourceType(),
        resource.GetSku(),
    )

    // Check cache
    if cached, exists := p.priceCache.Get(cacheKey); exists {
        price := cached.(float64)
        return p.Calculator().CreateProjectedCostResponse(
            "USD",
            price,
            "cached",
        ), nil
    }

    // Fetch from API
    price, err := p.apiClient.FetchPrice(ctx, resource)
    if err != nil {
        return nil, err
    }

    // Cache result
    p.priceCache.Set(cacheKey, price)

    return p.Calculator().CreateProjectedCostResponse(
        "USD",
        price,
        "api",
    ), nil
}
```

---

## Real-World Examples

### Example: AWS Cost Plugin

Complete example based on the AWS example plugin:

```go
package main

import (
    "context"
    "errors"

    "github.com/rshade/finfocus/pkg/pluginsdk"
    pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

type AWSPlugin struct {
    *pluginsdk.BasePlugin
    ec2Prices            map[string]float64
    s3Prices             map[string]float64
    regionMultipliers    map[string]float64
}

func NewAWSPlugin() *AWSPlugin {
    base := pluginsdk.NewBasePlugin("aws-cost-plugin")
    base.Matcher().AddProvider("aws")
    base.Matcher().AddResourceType("aws:ec2:Instance")
    base.Matcher().AddResourceType("aws:s3:Bucket")

    return &AWSPlugin{
        BasePlugin: base,
        ec2Prices: map[string]float64{
            "t3.micro":  0.0104,
            "t3.small":  0.0208,
            "t3.medium": 0.0416,
        },
        s3Prices: map[string]float64{
            "STANDARD":     0.023,
            "STANDARD_IA":  0.0125,
            "GLACIER":      0.004,
        },
        regionMultipliers: map[string]float64{
            "us-east-1": 1.0,
            "eu-west-1": 1.1,
        },
    }
}

func (p *AWSPlugin) GetProjectedCost(
    ctx context.Context,
    req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
    if req == nil {
        return nil, errors.New("request cannot be nil")
    }

    resource := req.GetResource()
    if resource == nil {
        return nil, errors.New("resource cannot be nil")
    }

    if !p.Matcher().Supports(resource) {
        return nil, pluginsdk.NotSupportedError(resource)
    }

    var price float64
    var detail string

    switch resource.GetResourceType() {
    case "aws:ec2:Instance":
        price = p.calculateEC2Cost(resource)
        detail = "EC2 on-demand hourly"
    case "aws:s3:Bucket":
        price = p.calculateS3Cost(resource)
        detail = "S3 storage per GB/month"
    }

    return p.Calculator().CreateProjectedCostResponse(
        "USD",
        price,
        detail,
    ), nil
}

func (p *AWSPlugin) calculateEC2Cost(
    resource *pbc.ResourceDescriptor,
) float64 {
    tags := resource.GetTags()
    instanceType := tags["instanceType"]
    if instanceType == "" {
        instanceType = "t3.micro"
    }

    price := p.ec2Prices[instanceType]

    region := tags["region"]
    if multiplier, ok := p.regionMultipliers[region]; ok {
        price *= multiplier
    }

    return price
}

func (p *AWSPlugin) calculateS3Cost(
    resource *pbc.ResourceDescriptor,
) float64 {
    tags := resource.GetTags()
    storageClass := tags["storageClass"]
    if storageClass == "" {
        storageClass = "STANDARD"
    }

    return p.s3Prices[storageClass]
}
```

---

## Related Documentation

- [Plugin Development Guide](plugin-development.md) - Complete guide
- [Plugin SDK Reference](plugin-sdk.md) - API documentation
- [Plugin Protocol](../architecture/plugin-protocol.md) - gRPC spec
- [Plugin Checklist](plugin-checklist.md) - Implementation checklist
