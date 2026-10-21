package esc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	sdk "github.com/pulumi/esc-sdk/sdk/go"
)

var errRevisionMoved = errors.New("esc environment changed while it was opened")

type sdkClient struct {
	api   *sdk.EscClient
	token string
}

func newSDKClient(backend, token string, httpClient *http.Client) (*sdkClient, error) {
	serverURL, err := escServerURL(backend)
	if err != nil {
		return nil, err
	}
	cfg := &sdk.Configuration{
		DefaultHeader: map[string]string{},
		UserAgent:     "esc-sdk",
		Servers: sdk.ServerConfigurations{{
			URL:         serverURL,
			Description: "Pulumi ESC",
		}},
		OperationServers: map[string]sdk.ServerConfigurations{},
		HTTPClient:       withoutRedirects(httpClient),
	}
	return &sdkClient{api: sdk.NewClient(cfg), token: token}, nil
}

func escServerURL(backend string) (string, error) {
	parsed, err := url.Parse(backend)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/esc"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil
	return parsed.String(), nil
}

func withoutRedirects(base *http.Client) *http.Client {
	client := &http.Client{}
	if base != nil {
		cloned := *base
		client = &cloned
	}
	if client.CheckRedirect == nil {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client
}

func (c *sdkClient) authContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, sdk.ContextAPIKeys, map[string]sdk.APIKey{
		"Authorization": {Key: c.token, Prefix: "token"},
	})
}

//nolint:nonamedreturns // recover writes the error this function returns.
func (c *sdkClient) open(ctx context.Context, ref Ref) (vars map[string]entry, revision string, err error) {
	defer func() {
		if recover() != nil {
			vars = nil
			revision = ""
			err = panicError{}
		}
	}()
	revision, latest, pinErr := c.pin(ctx, ref)
	if pinErr != nil && !latest {
		return nil, "", pinErr
	}
	pinned := ref
	if !latest {
		pinned.Version = revision
	}
	// A failed HEAD has no body to classify, so latest reads first and lets a
	// 401, 403 or 404 from the open report its own category.
	vars, err = c.read(ctx, pinned)
	if err != nil {
		return nil, "", err
	}
	if pinErr != nil {
		return nil, "", pinErr
	}
	if latest {
		after, afterErr := c.etag(ctx, ref)
		if afterErr != nil {
			return nil, "", afterErr
		}
		if after != revision {
			return nil, "", errRevisionMoved
		}
	}
	return vars, revision, nil
}

func (c *sdkClient) read(ctx context.Context, ref Ref) (map[string]entry, error) {
	authCtx := c.authContext(ctx)
	env, err := c.openEnvironment(authCtx, ref)
	if err != nil {
		return nil, err
	}
	return environmentVariables(ctx, env), nil
}

func (c *sdkClient) openEnvironment(ctx context.Context, ref Ref) (*sdk.Environment, error) {
	if ref.Version == "" || ref.Version == "latest" {
		env, _, err := c.api.OpenAndReadEnvironment(ctx, ref.Org, ref.Project, ref.Name)
		return env, err
	}
	env, _, err := c.api.OpenAndReadEnvironmentAtVersion(ctx, ref.Org, ref.Project, ref.Name, ref.Version)
	return env, err
}

// pin returns the revision the values must be read at, and whether that
// revision is the ETag of latest. A tag resolves to its numeric revision, so
// the read cannot see a later move. Latest has no numeric handle: its ETag is
// read here and checked again after the read.
func (c *sdkClient) pin(ctx context.Context, ref Ref) (string, bool, error) {
	switch {
	case ref.Version == "" || ref.Version == "latest":
		etag, err := c.etag(ctx, ref)
		return etag, true, err
	case isRevision(ref.Version):
		return ref.Version, false, nil
	default:
		revision, err := c.tagRevision(ctx, ref)
		return revision, false, err
	}
}

func (c *sdkClient) etag(ctx context.Context, ref Ref) (string, error) {
	resp, err := c.api.EscAPI.GetEnvironmentETag(c.authContext(ctx), ref.Org, ref.Project, ref.Name).Execute()
	defer closeResponse(resp)
	if err != nil || resp == nil || resp.StatusCode >= 300 {
		return "", errOrStatus(err)
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		return "", errOrStatus(nil)
	}
	return etag, nil
}

func (c *sdkClient) tagRevision(ctx context.Context, ref Ref) (string, error) {
	tag, resp, err := c.api.EscAPI.GetEnvironmentRevisionTag(
		c.authContext(ctx), ref.Org, ref.Project, ref.Name, ref.Version,
	).Execute()
	defer closeResponse(resp)
	if err != nil || tag == nil || tag.Revision <= 0 {
		return "", errOrStatus(err)
	}
	return strconv.FormatInt(int64(tag.Revision), 10), nil
}

func errOrStatus(err error) error {
	if err != nil {
		return err
	}
	return errors.New("esc revision key unavailable")
}

func closeResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func isRevision(version string) bool {
	if version == "" {
		return false
	}
	for _, r := range version {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
