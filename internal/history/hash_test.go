package history

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hashOf is a test helper that computes the same 16-char hex hash
// that Hash() should produce for a given canonical string.
func hashOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[0:16]
}

func TestStackContextHash_ShortAndQualifiedMatch(t *testing.T) {
	short := StackContext{
		Organization: "org",
		Project:      "proj",
		Stack:        "dev",
	}
	qualified := StackContext{
		Organization: "org",
		Project:      "proj",
		Stack:        "org/proj/dev",
	}

	shortHash := short.Hash()
	qualifiedHash := qualified.Hash()

	assert.Equal(t, shortHash, qualifiedHash,
		"short stack name and fully-qualified name must produce the same hash")
}

func TestStackContextHash_NoDoublePrefixing(t *testing.T) {
	sc := StackContext{
		Organization: "org",
		Project:      "proj",
		Stack:        "dev",
	}

	// The canonical form for short name "dev" is "org/proj/dev".
	// The hash must match sha256("org/proj/dev"), NOT sha256("org/proj/org/proj/dev").
	expected := hashOf("org/proj/dev")
	actual := sc.Hash()

	require.Len(t, actual, 16, "hash must be 16 hex characters")
	assert.Equal(t, expected, actual,
		"hash of short name must equal hash of canonical form, not double-prefixed")

	// Calling with the already-expanded form should be identical.
	sc2 := StackContext{
		Organization: "org",
		Project:      "proj",
		Stack:        "org/proj/dev",
	}
	assert.Equal(t, expected, sc2.Hash(),
		"hash of qualified name must equal hash of canonical form")
}

func TestStackContextHash_EmptyStack(t *testing.T) {
	sc := StackContext{
		Organization: "org",
		Project:      "proj",
		Stack:        "",
	}
	hash := sc.Hash()
	expected := hashOf("")
	require.Len(t, hash, 16, "empty stack should still produce a valid 16-char hash")
	assert.Equal(t, expected, hash,
		"empty stack hashes the empty string (org/project prefix is not applied)")
}

func TestStackContextHash_EmptyContext(t *testing.T) {
	sc := StackContext{}
	hash := sc.Hash()
	expectedEmpty := hashOf("")
	require.Len(t, hash, 16, "zero-value context should still produce a valid 16-char hash")
	assert.Equal(t, expectedEmpty, hash,
		"zero-value context hashes the empty string")

	// Empty stack with org/project fields produces the same hash as zero-value context,
	// because the canonicalization guard skips the prefix when stack is empty.
	emptyStackHash := StackContext{Organization: "org", Project: "proj", Stack: ""}.Hash()
	assert.Equal(t, hash, emptyStackHash,
		"empty-stack context and zero-value context must produce identical hashes")
}

func TestStackContextHash_CLICallerNoOrg(t *testing.T) {
	// CLI path sets Project and Stack but not Organization.
	sc := StackContext{
		Project: "myproject",
		Stack:   "dev",
	}
	hash := sc.Hash()
	require.Len(t, hash, 16)

	// A second call must be deterministic.
	assert.Equal(t, hash, sc.Hash())
}

func TestStackContextHash_Deterministic(t *testing.T) {
	sc := StackContext{
		Organization: "acme",
		Project:      "infra",
		Stack:        "production",
	}
	h1 := sc.Hash()
	h2 := sc.Hash()
	assert.Equal(t, h1, h2, "hash must be deterministic")
}

func TestURNHash(t *testing.T) {
	urn := "urn:pulumi:dev::myproject::aws:ec2/instance:Instance::web-server"
	hash := URNHash(urn)
	require.Len(t, hash, 16)
	assert.Equal(t, hash, URNHash(urn), "URNHash must be deterministic")
}

func TestBuildHistoryKey(t *testing.T) {
	key := BuildHistoryKey("stackhash", "urnhash", "i-abc123")
	assert.Equal(t, "stackhash/urnhash/i-abc123", key)
}

func TestBuildTagKey(t *testing.T) {
	key := BuildTagKey("stackhash", "env", "prod", "urnhash")
	assert.Equal(t, "stackhash/env:prod/urnhash", key)
}
