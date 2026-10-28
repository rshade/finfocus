package cloudhistory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/pulumicloud"
)

func TestImportCacheSeparatesBackendAndOrganization(t *testing.T) {
	t.Parallel()
	for _, dimension := range []string{"backend", "organization"} {
		t.Run(dimension, func(t *testing.T) {
			t.Parallel()
			deps, req, first, store := importFixture(t)
			// A bare selected stack retains its legacy history scope across logins.
			req.StackContext.Organization = ""
			first.updates = []pulumicloud.UpdateRecord{{Version: 1, Kind: "update", End: req.From.Unix() + 1}}
			first.ids[1] = []pulumicloud.Identity{
				{URN: "first", CloudID: "first-id", Type: "aws:s3/bucket:Bucket", Provider: "aws"},
			}
			initial, err := Import(context.Background(), deps, req)
			require.NoError(t, err)
			require.Equal(t, 1, initial.Fetched)
			other := req
			if dimension == "backend" {
				other.Session.Backend = "https://other.example"
			} else {
				other.Session.Org = "other-org"
				other.Stack.Org = "other-org"
			}
			require.Equal(t, req.StackContext.Hash(), other.StackContext.Hash())
			second := &fakeClient{
				updates: []pulumicloud.UpdateRecord{{Version: 1, Kind: "update", End: req.From.Unix() + 2}},
				ids: map[int][]pulumicloud.Identity{
					1: {{URN: "second", CloudID: "second-id", Type: "aws:s3/bucket:Bucket", Provider: "aws"}},
				},
			}
			deps.NewClient = func(esc.Session) pulumicloud.Client { return second }
			changed, err := Import(context.Background(), deps, other)
			require.NoError(t, err)
			assert.Equal(t, 1, changed.Fetched, "a different Cloud stack must fetch its own version")
			assert.Zero(t, changed.FromCache)
			assert.Equal(t, 1, second.lists, "a different Cloud stack must list its own updates")
			entries, err := store.GetAllForStack(req.StackContext.Hash(), req.From.Unix(), req.To.Unix())
			require.NoError(t, err)
			require.Len(t, entries, 2, "both imports retain legacy history visibility")
			ids := make([]string, 0, len(entries))
			for _, entry := range entries {
				ids = append(ids, entry.CloudID)
			}
			assert.ElementsMatch(t, []string{"first-id", "second-id"}, ids)
			repeated, err := Import(context.Background(), deps, other)
			require.NoError(t, err)
			assert.Zero(t, repeated.Fetched)
			assert.Equal(t, 1, repeated.FromCache)
			assert.Equal(t, 1, second.lists)
			assert.Len(t, second.fetched, 1)
		})
	}
}

func TestCloudCacheNamespace(t *testing.T) {
	t.Parallel()
	base := Request{
		Session: esc.Session{Backend: "https://cloud.example/path/", Token: "first-token"},
		Stack:   pulumicloud.StackRef{Org: "org", Project: "project", Stack: "stack"},
	}
	hash := cloudCacheNamespace(base)
	assert.Len(t, hash, 64)
	for _, tc := range []struct {
		name   string
		mutate func(*Request)
		same   bool
	}{
		{"trailing slash", func(r *Request) { r.Session.Backend = "https://cloud.example/path" }, true},
		{"token rotation", func(r *Request) { r.Session.Token = "other-token" }, true},
		{"backend", func(r *Request) { r.Session.Backend = "https://other.example/path" }, false},
		{"organization", func(r *Request) { r.Stack.Org = "other-org" }, false},
		{"project", func(r *Request) { r.Stack.Project = "other-project" }, false},
		{"stack", func(r *Request) { r.Stack.Stack = "other-stack" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := base
			tc.mutate(&changed)
			if tc.same {
				assert.Equal(t, hash, cloudCacheNamespace(changed))
			} else {
				assert.NotEqual(t, hash, cloudCacheNamespace(changed))
			}
		})
	}
	first, second := base, base
	first.Stack.Org = "a/b"
	first.Stack.Project = "c"
	second.Stack.Org = "a"
	second.Stack.Project = "b/c"
	assert.NotEqual(t, cloudCacheNamespace(first), cloudCacheNamespace(second), "field boundaries must be unambiguous")
}
