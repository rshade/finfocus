package esc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    Ref
		wantErr bool
	}{
		{name: "one segment", in: "name", want: Ref{Project: "default", Name: "name"}},
		{name: "two segments", in: "proj/name", want: Ref{Project: "proj", Name: "name"}},
		{name: "three segments", in: "acme/proj/name", want: Ref{Org: "acme", Project: "proj", Name: "name"}},
		{name: "revision", in: "proj/name@3", want: Ref{Project: "proj", Name: "name", Version: "3"}},
		{name: "tag", in: "proj/name@stable", want: Ref{Project: "proj", Name: "name", Version: "stable"}},
		{name: "latest", in: "proj/name@latest", want: Ref{Project: "proj", Name: "name", Version: "latest"}},
		{name: "empty segment", in: "a//b", wantErr: true},
		{name: "four segments", in: "a/b/c/d", wantErr: true},
		{name: "bad character", in: "proj/has space", wantErr: true},
		{name: "short org", in: "ab/proj/name", wantErr: true},
		{name: "long org", in: strings.Repeat("a", 41) + "/proj/name", wantErr: true},
		{name: "missing version", in: "proj/name@", wantErr: true},
		{name: "org bounds", in: "abc/" + strings.Repeat("a", 40) + "/name", want: Ref{
			Org: "abc", Project: strings.Repeat("a", 40), Name: "name",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRef(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRefStringOmitsEmptyOrgAndVersion(t *testing.T) {
	t.Parallel()
	ref := Ref{Project: "proj", Name: "name"}
	assert.Equal(t, "proj/name", ref.String())
	ref.Version = "3"
	assert.Equal(t, "proj/name@3", ref.String())
	ref.Org = "acme"
	assert.Equal(t, "acme/proj/name@3", ref.String())
	assert.Empty(t, Ref{}.String())
}
