package notification

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rshade/finfocus/internal/config"
)

// ErrUnsetVariable is returned when an allowed ${FINFOCUS_NOTIFY_*} variable
// is unset or empty.
var ErrUnsetVariable = errors.New("variable is unset or empty")

// Expand replaces every ${NAME} reference in value with lookup(NAME).
// Only names that start with FINFOCUS_NOTIFY_ are expanded. Every reference is
// checked before lookup runs, so a value that names any other variable returns
// config.ErrNotificationVariableNotAllowed without lookup being called at all.
// An unset or empty variable returns ErrUnsetVariable naming the variable.
// Errors never contain other parts of value. "$NAME", a bare "$", and "$$"
// stay literal.
func Expand(value string, lookup func(string) (string, bool)) (string, error) {
	refs, err := config.FindNotificationReferences(value)
	if err != nil {
		return "", err
	}
	if len(refs) == 0 {
		return value, nil
	}
	for _, ref := range refs {
		if !config.IsNotificationVariable(ref.Name) {
			return "", fmt.Errorf("variable %s is %w", ref.Name, config.ErrNotificationVariableNotAllowed)
		}
	}

	var out strings.Builder
	last := 0
	for _, ref := range refs {
		resolved, ok := lookup(ref.Name)
		if !ok || resolved == "" {
			return "", fmt.Errorf("%w: %s", ErrUnsetVariable, ref.Name)
		}
		out.WriteString(value[last:ref.Start])
		out.WriteString(resolved)
		last = ref.End
	}
	out.WriteString(value[last:])
	return out.String(), nil
}

// IsSingleReference reports whether value is exactly one ${NAME} reference and
// nothing else. Such a value holds no secret itself, so it can be displayed.
func IsSingleReference(value string) bool {
	refs, err := config.FindNotificationReferences(value)
	return err == nil && len(refs) == 1 && refs[0].Start == 0 && refs[0].End == len(value)
}

// HasReference reports whether value contains any "${".
func HasReference(value string) bool {
	return strings.Contains(value, "${")
}
