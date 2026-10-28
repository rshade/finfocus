package esc

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionFormattingOmitsToken(t *testing.T) {
	t.Parallel()
	session := Session{Backend: "https://api.pulumi.com", Token: "pul-secret-token", Org: "acme"}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		out := fmt.Sprintf(verb, session)
		assert.NotContains(t, out, "pul-secret-token", verb)
		assert.Contains(t, out, "acme", verb)
	}
	assert.NotContains(t, fmt.Sprintf("%+v", &session), "pul-secret-token")
}
