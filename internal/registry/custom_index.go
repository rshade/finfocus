package registry

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/rshade/finfocus/internal/config"
)

const (
	customIndexMaxSize   = 1 << 20
	customIndexTimeout   = 10 * time.Second
	customIndexCacheSize = 16
	customIndexRedirects = 10
	customIndexSchemaURL = "https://spec.finfocus.dev/schemas/plugin_registry.schema.json"
)

var errCustomIndexRedirect = errors.New("registry index redirect must stay on the same HTTPS origin")

// customIndexSchema is the unmodified finfocus-spec v0.7.12 schema, licensed
// under Apache-2.0. Embedding it keeps validation offline and pinned to go.mod.
//
//go:embed custom_index.schema.json
var customIndexSchema []byte

type cachedCustomIndex struct {
	etag    string
	url     string
	entries []RegistryEntry
}

// CustomIndexClient fetches third-party plugin indexes and retains a bounded
// in-memory cache of index contents and ETags.
type CustomIndexClient struct {
	client *http.Client
	schema func() (*jsonschema.Schema, error)
	mu     sync.Mutex
	cache  map[config.PluginRegistrySource]cachedCustomIndex
}

// NewCustomIndexClient uses the provided HTTP transport or the default client.
// Requests never use a cookie jar or redirect to a different origin.
func NewCustomIndexClient(client *http.Client) *CustomIndexClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &CustomIndexClient{
		client: &http.Client{Transport: client.Transport, Timeout: client.Timeout},
		schema: sync.OnceValues(compileCustomIndexSchema),
		cache:  make(map[config.PluginRegistrySource]cachedCustomIndex),
	}
}

// Fetch retrieves and validates an index, revalidating a cached representation
// with its ETag. The returned entries are owned by the caller.
func (c *CustomIndexClient) Fetch(
	ctx context.Context, source config.PluginRegistrySource,
) ([]RegistryEntry, error) {
	if err := config.ValidatePluginRegistrySource(source); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, customIndexTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, errors.New("cannot construct registry index request")
	}

	c.mu.Lock()
	cached := c.cache[source]
	c.mu.Unlock()
	req.Header.Set("Accept", "application/json")
	applyCustomIndexETag(req, cached)

	client := *c.client
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if next.URL.Scheme != "https" || next.URL.User != nil ||
			!strings.EqualFold(next.URL.Host, via[0].URL.Host) || len(via) >= customIndexRedirects {
			return errCustomIndexRedirect
		}
		applyCustomIndexETag(next, cached)
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, customIndexRequestError(err)
	}
	defer resp.Body.Close()

	return c.handleCustomIndexResponse(ctx, resp, source, cached)
}

func applyCustomIndexETag(req *http.Request, cached cachedCustomIndex) {
	req.Header.Del("If-None-Match")
	if cached.etag != "" && req.URL.String() == cached.url {
		req.Header.Set("If-None-Match", cached.etag)
	}
}

func customIndexRequestError(err error) error {
	switch {
	case errors.Is(err, errCustomIndexRedirect):
		return errCustomIndexRedirect
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("fetch registry index: %w", context.DeadlineExceeded)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("fetch registry index: %w", context.Canceled)
	default:
		return errors.New("could not fetch registry index over HTTPS")
	}
}

func (c *CustomIndexClient) handleCustomIndexResponse(
	ctx context.Context, resp *http.Response, source config.PluginRegistrySource, cached cachedCustomIndex,
) ([]RegistryEntry, error) {
	switch resp.StatusCode {
	case http.StatusNotModified:
		if cached.etag == "" || resp.Request.Header.Get("If-None-Match") != cached.etag {
			return nil, errors.New("registry server returned 304 without a matching cached index")
		}
		return cloneCustomIndexEntries(cached.entries), nil
	case http.StatusOK:
	default:
		return nil, fmt.Errorf("registry index request returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, customIndexMaxSize+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, customIndexRequestError(ctx.Err())
		}
		return nil, errors.New("could not read registry index")
	}
	if len(body) > customIndexMaxSize {
		return nil, errors.New("registry index exceeds the 1 MiB size limit")
	}

	entries, err := c.parseCustomIndex(body, source)
	if err != nil {
		return nil, err
	}
	c.cacheCustomIndex(source, resp, entries)
	return entries, nil
}

func (c *CustomIndexClient) cacheCustomIndex(
	source config.PluginRegistrySource, resp *http.Response, entries []RegistryEntry,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	etag := resp.Header.Get("ETag")
	if etag == "" {
		delete(c.cache, source)
		return
	}
	if _, ok := c.cache[source]; !ok && len(c.cache) >= customIndexCacheSize {
		for key := range c.cache {
			delete(c.cache, key)
			break
		}
	}
	c.cache[source] = cachedCustomIndex{
		etag: etag, url: resp.Request.URL.String(), entries: cloneCustomIndexEntries(entries),
	}
}

func compileCustomIndexSchema() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(customIndexSchema))
	if err != nil {
		return nil, fmt.Errorf("decode embedded plugin registry schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if addErr := compiler.AddResource(customIndexSchemaURL, doc); addErr != nil {
		return nil, fmt.Errorf("load embedded plugin registry schema: %w", addErr)
	}
	schema, err := compiler.Compile(customIndexSchemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile embedded plugin registry schema: %w", err)
	}
	return schema, nil
}

func (c *CustomIndexClient) parseCustomIndex(body []byte, source config.PluginRegistrySource) ([]RegistryEntry, error) {
	if source.SHA256 != "" && !strings.EqualFold(source.SHA256, fmt.Sprintf("%x", sha256.Sum256(body))) {
		return nil, errors.New("registry index checksum does not match the configured SHA-256 pin")
	}
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, errors.New("registry index contains invalid JSON")
	}
	schema, err := c.schema()
	if err != nil {
		return nil, err
	}
	if validateErr := schema.Validate(doc); validateErr != nil {
		return nil, errors.New("registry index does not match the finfocus-spec plugin registry schema")
	}

	var index struct {
		Plugins map[string]RegistryEntry `json:"plugins"`
	}
	if decodeErr := json.Unmarshal(body, &index); decodeErr != nil {
		return nil, errors.New("could not decode registry entries")
	}

	entries := make([]RegistryEntry, 0, len(index.Plugins))
	for key, entry := range index.Plugins {
		if key != entry.Name || !pluginNamePattern.MatchString(entry.Name) {
			return nil, errors.New("registry plugin name must match its index key and use a valid FinFocus name")
		}
		_, repository, _ := strings.Cut(entry.Repository, "/")
		if repository == "." || repository == ".." {
			return nil, errors.New("registry plugin has an invalid GitHub repository name")
		}
		entry.SecurityLevel = customIndexSecurityLevel(source.Trust, entry.SecurityLevel)
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(a, b RegistryEntry) int { return strings.Compare(a.Name, b.Name) })
	return entries, nil
}

func customIndexSecurityLevel(trust, declared string) string {
	if trust == config.PluginRegistryTrustExperimental || declared == "untrusted" {
		return config.PluginRegistryTrustExperimental
	}
	return config.PluginRegistryTrustCommunity
}

func cloneCustomIndexEntries(entries []RegistryEntry) []RegistryEntry {
	cloned := slices.Clone(entries)
	for i := range cloned {
		cloned[i].SupportedProviders = slices.Clone(cloned[i].SupportedProviders)
		cloned[i].Capabilities = slices.Clone(cloned[i].Capabilities)
		cloned[i].Keywords = slices.Clone(cloned[i].Keywords)
	}
	return cloned
}
