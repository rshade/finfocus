package history

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// StackContext contains the identity of a Pulumi stack.
type StackContext struct {
	Organization string
	Project      string
	Stack        string
}

// Hash returns a 16-character hex hash of the stack context.
// The hash is computed from the string "{Organization}/{Project}/{Stack}".
func (sc StackContext) Hash() string {
	s := fmt.Sprintf("%s/%s/%s", sc.Organization, sc.Project, sc.Stack)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[0:16]
}

// URNHash returns a 16-character hex hash of the provided URN string.
func URNHash(urn string) string {
	h := sha256.Sum256([]byte(urn))
	return hex.EncodeToString(h[:])[0:16]
}

// BuildHistoryKey returns a composite key for a resource history entry.
// The key has the format "{stackHash}/{urnHash}/{cloudID}".
func BuildHistoryKey(stackHash, urnHash, cloudID string) string {
	return fmt.Sprintf("%s/%s/%s", stackHash, urnHash, cloudID)
}

// BuildTagKey returns a composite key for a resource tag entry.
// The key has the format "{stackHash}/{tagKey}:{tagValue}/{urnHash}".
func BuildTagKey(stackHash, tagKey, tagValue, urnHash string) string {
	return fmt.Sprintf("%s/%s:%s/%s", stackHash, tagKey, tagValue, urnHash)
}
