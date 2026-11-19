package notification

import (
	"strings"

	"github.com/rshade/finfocus/internal/config"
)

func (d *Dispatcher) resolveEmail(
	email config.EmailConfig,
	redactor, credentials *Redactor,
) (config.EmailConfig, error) {
	if email.SMTP != nil {
		for _, secret := range []string{email.SMTP.Username, email.SMTP.Password} {
			credentials.Add(secret)
			redactor.Add(secret)
		}
	}
	return email.MapValues(func(field, raw string) (string, error) {
		isCredential := field == "smtp.username" || field == "smtp.password"
		resolved, err := Expand(raw, func(name string) (string, bool) {
			value, ok := d.lookup(name)
			if ok {
				redactor.Add(value)
				if isCredential {
					credentials.Add(value)
				}
			}
			return value, ok
		})
		if isCredential {
			credentials.Add(resolved)
			redactor.Add(resolved)
		}
		return resolved, err
	})
}

func emailRecipientSummary(email config.EmailConfig, credentials *Redactor) string {
	summary := "To: " + strings.Join(emailMailboxes(email.To, credentials), ", ")
	if len(email.Cc) != 0 {
		summary += "; Cc: " + strings.Join(emailMailboxes(email.Cc, credentials), ", ")
	}
	return summary
}

func emailMailboxes(values []string, credentials *Redactor) []string {
	addresses, err := parseEmailAddresses(values)
	if err != nil {
		// Dry runs only reach this point after address validation.
		return nil
	}
	mailboxes := make([]string, len(addresses))
	for i, address := range addresses {
		if credentials.Redact(address.Address) != address.Address {
			mailboxes[i] = RedactedText
			continue
		}
		mailboxes[i] = credentials.Redact(emailMailbox(address))
	}
	return mailboxes
}
