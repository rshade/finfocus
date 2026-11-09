package webui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAllowedHosts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      []string
		want    []allowedHost
		wantErr bool
	}{
		{name: "plain name", in: []string{"dash.example.ts.net"}, want: []allowedHost{{name: "dash.example.ts.net"}}},
		{name: "mixed case", in: []string{"Dash.Example.TS.net"}, want: []allowedHost{{name: "dash.example.ts.net"}}},
		{
			name: "trailing dot",
			in:   []string{"dash.example.ts.net."},
			want: []allowedHost{{name: "dash.example.ts.net"}},
		},
		{
			name: "surrounding space",
			in:   []string{"  dash.example.ts.net "},
			want: []allowedHost{{name: "dash.example.ts.net"}},
		},
		{
			name: "explicit port",
			in:   []string{"dash.example.ts.net:8443"},
			want: []allowedHost{{name: "dash.example.ts.net", port: "8443"}},
		},
		{
			name: "default https port collapses",
			in:   []string{"dash.example.ts.net:443"},
			want: []allowedHost{{name: "dash.example.ts.net"}},
		},
		{
			name: "duplicates collapse",
			in:   []string{"a.example", "A.example."},
			want: []allowedHost{{name: "a.example"}},
		},
		{
			name: "several hosts",
			in:   []string{"a.example", "b.example"},
			want: []allowedHost{{name: "a.example"}, {name: "b.example"}},
		},
		{name: "empty list", in: nil, want: nil},
		{name: "empty value", in: []string{""}, wantErr: true},
		{name: "blank value", in: []string{"   "}, wantErr: true},
		{name: "wildcard", in: []string{"*.ts.net"}, wantErr: true},
		{name: "bare wildcard", in: []string{"*"}, wantErr: true},
		{name: "ipv4", in: []string{"192.168.1.5"}, wantErr: true},
		{name: "ipv4 with port", in: []string{"127.0.0.1:8080"}, wantErr: true},
		{name: "ipv4 shorthand", in: []string{"127.1"}, wantErr: true},
		{name: "ipv4 hex octets", in: []string{"0x7f.0x1"}, wantErr: true},
		{name: "last label starts with digit", in: []string{"dash.1abc"}, wantErr: true},
		{name: "ipv6 bare", in: []string{"::1"}, wantErr: true},
		{name: "ipv6 bracketed", in: []string{"[::1]"}, wantErr: true},
		{name: "ipv6 bracketed with port", in: []string{"[::1]:8080"}, wantErr: true},
		{name: "scheme", in: []string{"https://dash.example.ts.net"}, wantErr: true},
		{name: "path", in: []string{"dash.example.ts.net/x"}, wantErr: true},
		{name: "userinfo", in: []string{"user@dash.example.ts.net"}, wantErr: true},
		{name: "port zero", in: []string{"dash.example.ts.net:0"}, wantErr: true},
		{name: "port too large", in: []string{"dash.example.ts.net:65536"}, wantErr: true},
		{name: "port not numeric", in: []string{"dash.example.ts.net:https"}, wantErr: true},
		{name: "empty port", in: []string{"dash.example.ts.net:"}, wantErr: true},
		{name: "empty label", in: []string{"dash..example"}, wantErr: true},
		{name: "leading dot", in: []string{".example.ts.net"}, wantErr: true},
		{name: "label starts with hyphen", in: []string{"-dash.example"}, wantErr: true},
		{name: "label ends with hyphen", in: []string{"dash-.example"}, wantErr: true},
		{name: "underscore", in: []string{"da_sh.example"}, wantErr: true},
		{name: "unicode", in: []string{"dásh.example"}, wantErr: true},
		{name: "one bad value among good", in: []string{"a.example", "*.example"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeAllowedHosts(tt.in)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidAllowedHost)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAllowedHostsMatchHost(t *testing.T) {
	t.Parallel()
	bare, err := normalizeAllowedHosts([]string{"dash.example.ts.net"})
	require.NoError(t, err)
	ported, err := normalizeAllowedHosts([]string{"dash.example.ts.net:8443"})
	require.NoError(t, err)
	several, err := normalizeAllowedHosts([]string{"a.example", "b.example"})
	require.NoError(t, err)

	tests := []struct {
		name  string
		hosts []allowedHost
		host  string
		want  bool
	}{
		{name: "exact", hosts: bare, host: "dash.example.ts.net", want: true},
		{name: "mixed case request", hosts: bare, host: "DASH.example.TS.net", want: true},
		{name: "trailing dot request", hosts: bare, host: "dash.example.ts.net.", want: true},
		{name: "default https port", hosts: bare, host: "dash.example.ts.net:443", want: true},
		{name: "other port refused", hosts: bare, host: "dash.example.ts.net:8443", want: false},
		{name: "http port refused", hosts: bare, host: "dash.example.ts.net:80", want: false},
		{name: "prefix near miss", hosts: bare, host: "evil.dash.example.ts.net", want: false},
		{name: "suffix near miss", hosts: bare, host: "dash.example.ts.net.evil.com", want: false},
		{name: "substring near miss", hosts: bare, host: "xdash.example.ts.net", want: false},
		{name: "empty host", hosts: bare, host: "", want: false},
		{name: "userinfo smuggle", hosts: bare, host: "dash.example.ts.net@evil.com", want: false},
		{name: "ipv6 literal", hosts: bare, host: "[::1]", want: false},
		{name: "ported entry matches its port", hosts: ported, host: "dash.example.ts.net:8443", want: true},
		{name: "ported entry refuses no port", hosts: ported, host: "dash.example.ts.net", want: false},
		{name: "ported entry refuses 443", hosts: ported, host: "dash.example.ts.net:443", want: false},
		{name: "second host", hosts: several, host: "b.example", want: true},
		{name: "no hosts configured", hosts: nil, host: "dash.example.ts.net", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, matchAllowedHost(tt.hosts, tt.host))
		})
	}
}

func TestAllowedHostsMatchOrigin(t *testing.T) {
	t.Parallel()
	bare, err := normalizeAllowedHosts([]string{"dash.example.ts.net"})
	require.NoError(t, err)
	ported, err := normalizeAllowedHosts([]string{"dash.example.ts.net:8443"})
	require.NoError(t, err)

	tests := []struct {
		name   string
		hosts  []allowedHost
		origin string
		want   bool
	}{
		{name: "https exact", hosts: bare, origin: "https://dash.example.ts.net", want: true},
		{name: "http refused", hosts: bare, origin: "http://dash.example.ts.net", want: false},
		{
			name:   "explicit default port not canonical",
			hosts:  bare,
			origin: "https://dash.example.ts.net:443",
			want:   false,
		},
		{name: "foreign", hosts: bare, origin: "https://evil.example", want: false},
		{name: "suffix near miss", hosts: bare, origin: "https://dash.example.ts.net.evil.com", want: false},
		{name: "trailing slash", hosts: bare, origin: "https://dash.example.ts.net/", want: false},
		{name: "trailing dot host", hosts: bare, origin: "https://dash.example.ts.net.", want: true},
		{name: "mixed case host", hosts: bare, origin: "https://Dash.Example.TS.net", want: true},
		{name: "empty port", hosts: bare, origin: "https://dash.example.ts.net:", want: false},
		{name: "userinfo", hosts: bare, origin: "https://user@dash.example.ts.net", want: false},
		{name: "path", hosts: bare, origin: "https://dash.example.ts.net/x", want: false},
		{name: "query", hosts: bare, origin: "https://dash.example.ts.net?x", want: false},
		{name: "ported trailing dot", hosts: ported, origin: "https://dash.example.ts.net.:8443", want: true},
		{name: "ported wrong port", hosts: ported, origin: "https://dash.example.ts.net:9443", want: false},
		{name: "null origin", hosts: bare, origin: "null", want: false},
		{name: "ported", hosts: ported, origin: "https://dash.example.ts.net:8443", want: true},
		{name: "ported without port", hosts: ported, origin: "https://dash.example.ts.net", want: false},
		{name: "none configured", hosts: nil, origin: "https://dash.example.ts.net", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, matchAllowedOrigin(tt.hosts, tt.origin))
		})
	}
}

func TestNormalizeAllowedUsers(t *testing.T) {
	t.Parallel()
	got, err := normalizeAllowedUsers([]string{" Alice@Example.com ", "alice@example.com", "bob@example.com"})
	require.NoError(t, err)
	assert.Equal(t, []string{"alice@example.com", "bob@example.com"}, got)

	for _, bad := range [][]string{{""}, {"   "}, {"alice@example.com", ""}} {
		_, err = normalizeAllowedUsers(bad)
		require.ErrorIs(t, err, ErrInvalidTrustedProxy, "input %q", bad)
	}
}
