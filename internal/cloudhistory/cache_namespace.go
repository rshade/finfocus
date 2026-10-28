package cloudhistory

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
)

// cloudCacheNamespace binds versioned Cloud data to its backend and fully
// resolved stack. History observations retain their existing StackContext hash
// so previously recorded resources remain visible to local history readers.
func cloudCacheNamespace(req Request) string {
	fields := [...]string{
		strings.TrimRight(req.Session.Backend, "/"),
		req.Stack.Org,
		req.Stack.Project,
		req.Stack.Stack,
	}
	var data []byte
	for _, field := range fields {
		data = binary.BigEndian.AppendUint64(data, uint64(len(field)))
		data = append(data, field...)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
