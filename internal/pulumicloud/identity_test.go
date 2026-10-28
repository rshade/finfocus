package pulumicloud

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type endlessBody struct{ read int64 }

func (r *endlessBody) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	r.read += int64(len(p))
	return len(p), nil
}

func TestDecodeIdentities(t *testing.T) {
	t.Parallel()
	f, err := os.Open("testdata/export.json")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	ids, err := DecodeIdentities(f)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	assert.Equal(t, "i-test", ids[0].CloudID)
	assert.Equal(t, "aws", ids[0].Provider)
	typ := reflect.TypeFor[Identity]()
	require.Equal(t, 4, typ.NumField())
	for i, name := range []string{"URN", "CloudID", "Type", "Provider"} {
		assert.Equal(t, name, typ.Field(i).Name)
	}
	assert.NotContains(t, fmt.Sprintf("%+v", ids), "SENTINEL-CIPHERTEXT")
	for _, raw := range []string{`{`, `{"deployment":{"resources":null}} trailing`, `{"deployment":{"resources":[{"id":2}]}}`} {
		_, err = DecodeIdentities(strings.NewReader(raw))
		require.Error(t, err)
	}
	body := &endlessBody{}
	_, err = DecodeIdentities(body)
	require.Error(t, err)
	assert.LessOrEqual(t, body.read, int64(MaxExportBytes+1))
	_, err = DecodeIdentities(io.LimitReader(strings.NewReader(`{}`), 1))
	require.Error(t, err)
}
