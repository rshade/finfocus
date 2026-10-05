package notification

import (
	"sort"
	"strings"

	"github.com/rshade/finfocus/internal/config"
)

// RedactedText replaces every secret in reported text.
const RedactedText = config.RedactedValue

// Redactor replaces registered secrets in text. The zero value is ready to use.
type Redactor struct {
	secrets []string
}

// Add registers secret. Empty strings are ignored.
func (r *Redactor) Add(secret string) {
	if secret == "" {
		return
	}
	r.secrets = append(r.secrets, secret)
}

// Redact returns text with every registered secret replaced by RedactedText.
// Longer secrets are replaced first, so a secret that contains a shorter one
// is removed whole.
func (r *Redactor) Redact(text string) string {
	secrets := append([]string(nil), r.secrets...)
	sort.SliceStable(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	for _, secret := range secrets {
		text = strings.ReplaceAll(text, secret, RedactedText)
	}
	return text
}
